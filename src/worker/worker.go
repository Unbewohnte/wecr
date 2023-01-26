/*
	Wecr - crawl the web for data
	Copyright (C) 2022, 2023 Kasyanov Nikolay Alexeyevich (Unbewohnte)

	This program is free software: you can redistribute it and/or modify
	it under the terms of the GNU Affero General Public License as published by
	the Free Software Foundation, either version 3 of the License, or
	(at your option) any later version.

	This program is distributed in the hope that it will be useful,
	but WITHOUT ANY WARRANTY; without even the implied warranty of
	MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
	GNU Affero General Public License for more details.

	You should have received a copy of the GNU Affero General Public License
	along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

package worker

import (
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sync"
	"time"
	"unbewohnte/wecr/config"
	"unbewohnte/wecr/logger"
	"unbewohnte/wecr/queue"
	"unbewohnte/wecr/web"
)

type VisitQueue struct {
	VisitQueue *os.File
	Lock       *sync.Mutex
}

// Worker configuration
type WorkerConf struct {
	Requests           config.Requests
	Save               config.Save
	BlacklistedDomains []string
	AllowedDomains     []string
	VisitQueue         VisitQueue
}

// Web worker
type Worker struct {
	Jobs    chan web.Job
	Results chan web.Result
	Conf    WorkerConf
	visited *visited
	stats   *Statistics
	Stopped bool
}

// Create a new worker
func NewWorker(jobs chan web.Job, results chan web.Result, conf WorkerConf, visited *visited, stats *Statistics) Worker {
	return Worker{
		Jobs:    jobs,
		Results: results,
		Conf:    conf,
		visited: visited,
		stats:   stats,
		Stopped: false,
	}
}

func (w *Worker) saveContent(links []string, pageURL *url.URL) {
	var alreadyProcessedUrls []string
	for count, link := range links {
		// check if this URL has been processed already
		var skip bool = false

		for _, processedURL := range alreadyProcessedUrls {
			if link == processedURL {
				skip = true
				break
			}
		}

		if skip {
			skip = false
			continue
		}
		alreadyProcessedUrls = append(alreadyProcessedUrls, link)

		var fileName string = fmt.Sprintf("%s_%d_%s", pageURL.Host, count, path.Base(link))

		var filePath string
		if web.HasImageExtention(link) {
			filePath = filepath.Join(w.Conf.Save.OutputDir, config.SaveImagesDir, fileName)
		} else if web.HasVideoExtention(link) {
			filePath = filepath.Join(w.Conf.Save.OutputDir, config.SaveVideosDir, fileName)
		} else if web.HasAudioExtention(link) {
			filePath = filepath.Join(w.Conf.Save.OutputDir, config.SaveAudioDir, fileName)
		} else if web.HasDocumentExtention(link) {
			filePath = filepath.Join(w.Conf.Save.OutputDir, config.SaveDocumentsDir, fileName)
		} else {
			filePath = filepath.Join(w.Conf.Save.OutputDir, fileName)
		}

		err := web.FetchFile(
			link,
			w.Conf.Requests.UserAgent,
			w.Conf.Requests.ContentFetchTimeoutMs,
			filePath,
		)
		if err != nil {
			logger.Error("Failed to fetch file at %s: %s", link, err)
			return
		}

		logger.Info("Outputted \"%s\"", fileName)
		w.stats.MatchesFound++
	}
}

// Save page to the disk with a corresponding name
func (w *Worker) savePage(baseURL *url.URL, pageData []byte) {
	if w.Conf.Save.SavePages && w.Conf.Save.OutputDir != "" {
		var pageName string = fmt.Sprintf("%s_%s.html", baseURL.Host, path.Base(baseURL.String()))
		pageFile, err := os.Create(filepath.Join(w.Conf.Save.OutputDir, config.SavePagesDir, pageName))
		if err != nil {
			logger.Error("Failed to create page of \"%s\": %s", baseURL.String(), err)
			return
		}
		defer pageFile.Close()

		pageFile.Write(pageData)

		logger.Info("Saved \"%s\"", pageName)
		w.stats.PagesSaved++
	}
}

// Launch scraping process on this worker
func (w *Worker) Work() {
	if w.Stopped {
		return
	}

	for {
		var job web.Job
		if w.Conf.VisitQueue.VisitQueue != nil {
			w.Conf.VisitQueue.Lock.Lock()
			newJob, err := queue.PopLastJob(w.Conf.VisitQueue.VisitQueue)
			if err != nil {
				logger.Error("Failed to get a new job from visit queue: %s", err)
				w.Conf.VisitQueue.Lock.Unlock()
				continue
			}
			if newJob == nil {
				w.Conf.VisitQueue.Lock.Unlock()
				continue
			}

			job = *newJob
			w.Conf.VisitQueue.Lock.Unlock()
		} else {
			job = <-w.Jobs
		}

		// check if the worker has been stopped
		if w.Stopped {
			// stop working
			return
		}

		pageURL, err := url.Parse(job.URL)
		if err != nil {
			logger.Error("Failed to parse URL \"%s\" to get hostname: %s", job.URL, err)
			continue
		}

		var skip bool = false
		// see if the domain is allowed and is not blacklisted
		if len(w.Conf.AllowedDomains) > 0 {
			skip = true
			for _, allowedDomain := range w.Conf.AllowedDomains {
				if pageURL.Host == allowedDomain {
					skip = false
					break
				}
			}
			if skip {
				logger.Info("Skipped non-allowed %s", job.URL)
				continue
			}
		}

		if len(w.Conf.BlacklistedDomains) > 0 {
			for _, blacklistedDomain := range w.Conf.BlacklistedDomains {
				if pageURL.Host == blacklistedDomain {
					skip = true
					logger.Info("Skipped blacklisted %s", job.URL)
					break
				}
			}
			if skip {
				continue
			}
		}

		// check if it is the first occurence
		w.visited.Lock.Lock()
		for _, visitedURL := range w.visited.URLs {
			if job.URL == visitedURL {
				// okay, don't even bother. Move onto the next job
				skip = true
				logger.Info("Skipping visited %s", job.URL)
				w.visited.Lock.Unlock()
				break
			}
		}

		if skip {
			continue
		}

		// add this url to the visited list
		w.visited.URLs = append(w.visited.URLs, job.URL)
		w.visited.Lock.Unlock()
		w.stats.PagesVisited++

		// get page
		logger.Info("Visiting %s", job.URL)
		pageData, err := web.GetPage(job.URL, w.Conf.Requests.UserAgent, w.Conf.Requests.RequestWaitTimeoutMs)
		if err != nil {
			logger.Error("Failed to get \"%s\": %s", job.URL, err)
			continue
		}

		// find links
		pageLinks := web.FindPageLinks(pageData, pageURL)

		go func() {
			if job.Depth > 1 {
				// decrement depth and add new jobs
				job.Depth--

				if w.Conf.VisitQueue.VisitQueue != nil {
					// add to the visit queue
					w.Conf.VisitQueue.Lock.Lock()
					for _, link := range pageLinks {
						if link != job.URL {
							err = queue.InsertNewJob(w.Conf.VisitQueue.VisitQueue, web.Job{
								URL:    link,
								Search: job.Search,
								Depth:  job.Depth,
							})
							if err != nil {
								logger.Error("Failed to encode a new job to a visit queue: %s", err)
								continue
							}
						}
					}
					w.Conf.VisitQueue.Lock.Unlock()
				} else {
					//  add to the in-memory channel
					for _, link := range pageLinks {
						if link != job.URL {
							w.Jobs <- web.Job{
								URL:    link,
								Search: job.Search,
								Depth:  job.Depth,
							}
						}
					}
				}

			}
			pageLinks = nil
		}()

		// process and output result
		var savePage bool = false

		switch job.Search.Query {
		case config.QueryImages:
			// find image URLs, output images to the file while not saving already outputted ones
			imageLinks := web.FindPageImages(pageData, pageURL)
			if len(imageLinks) > 0 {
				w.saveContent(imageLinks, pageURL)
				savePage = true
			}

		case config.QueryVideos:
			// search for videos
			// find video URLs, output videos to the files while not saving already outputted ones
			videoLinks := web.FindPageVideos(pageData, pageURL)
			if len(videoLinks) > 0 {
				w.saveContent(videoLinks, pageURL)
				savePage = true
			}

		case config.QueryAudio:
			// search for audio
			// find audio URLs, output audio to the file while not saving already outputted ones
			audioLinks := web.FindPageAudio(pageData, pageURL)
			if len(audioLinks) > 0 {
				w.saveContent(audioLinks, pageURL)
				savePage = true
			}

		case config.QueryDocuments:
			// search for various documents
			// find documents URLs, output docs to the file while not saving already outputted ones
			docsLinks := web.FindPageDocuments(pageData, pageURL)
			if len(docsLinks) > 0 {
				w.saveContent(docsLinks, pageURL)
				savePage = true
			}

		case config.QueryEmail:
			// search for email
			emailAddresses := web.FindPageEmailsWithCheck(pageData)
			if len(emailAddresses) > 0 {
				w.Results <- web.Result{
					PageURL: job.URL,
					Search:  job.Search,
					Data:    emailAddresses,
				}
				w.stats.MatchesFound += uint64(len(emailAddresses))
				savePage = true
			}

		case config.QueryEverything:
			// search for everything

			// files
			var contentLinks []string
			contentLinks = append(contentLinks, web.FindPageImages(pageData, pageURL)...)
			contentLinks = append(contentLinks, web.FindPageAudio(pageData, pageURL)...)
			contentLinks = append(contentLinks, web.FindPageVideos(pageData, pageURL)...)
			contentLinks = append(contentLinks, web.FindPageDocuments(pageData, pageURL)...)
			w.saveContent(contentLinks, pageURL)

			// email
			emailAddresses := web.FindPageEmailsWithCheck(pageData)
			if len(emailAddresses) > 0 {
				w.Results <- web.Result{
					PageURL: job.URL,
					Search:  job.Search,
					Data:    emailAddresses,
				}
				w.stats.MatchesFound += uint64(len(emailAddresses))
				savePage = true
			}

			if len(contentLinks) > 0 || len(emailAddresses) > 0 {
				savePage = true
			}

		default:
			// text search
			switch job.Search.IsRegexp {
			case true:
				// find by regexp
				re, err := regexp.Compile(job.Search.Query)
				if err != nil {
					logger.Error("Failed to compile regexp %s: %s", job.Search.Query, err)
					continue
				}

				matches := web.FindPageRegexp(re, pageData)
				if len(matches) > 0 {
					w.Results <- web.Result{
						PageURL: job.URL,
						Search:  job.Search,
						Data:    matches,
					}
					logger.Info("Found matches: %+v", matches)
					w.stats.MatchesFound += uint64(len(matches))

					savePage = true
				}
			case false:
				// just text
				if web.IsTextOnPage(job.Search.Query, true, pageData) {
					w.Results <- web.Result{
						PageURL: job.URL,
						Search:  job.Search,
						Data:    nil,
					}
					logger.Info("Found \"%s\" on page", job.Search.Query)
					w.stats.MatchesFound++

					savePage = true
				}
			}
		}

		// save page
		if savePage {
			w.savePage(pageURL, pageData)
		}
		pageData = nil
		pageURL = nil

		// sleep before the next request
		time.Sleep(time.Duration(w.Conf.Requests.RequestPauseMs * uint64(time.Millisecond)))
	}
}
