/*
	Wecr - crawl the web for data
	Copyright (C) 2022 Kasyanov Nikolay Alexeyevich (Unbewohnte)

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
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"time"
	"unbewohnte/wecr/config"
	"unbewohnte/wecr/logger"
	"unbewohnte/wecr/web"
)

// Worker configuration
type WorkerConf struct {
	Requests           config.Requests
	Save               config.Save
	BlacklistedDomains []string
	AllowedDomains     []string
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

// Save page to the disk with a corresponding name
func (w *Worker) savePage(baseURL *url.URL, pageData []byte) {
	if w.Conf.Save.SavePages && w.Conf.Save.OutputDir != "" {
		var pageName string = fmt.Sprintf("%s_%s.html", baseURL.Host, path.Base(baseURL.String()))
		pageFile, err := os.Create(filepath.Join(w.Conf.Save.OutputDir, pageName))
		if err != nil {
			logger.Error("Failed to create page of \"%s\": %s", baseURL.String(), err)
		} else {
			pageFile.Write(pageData)
		}

		pageFile.Close()

		logger.Info("Saved \"%s\"", pageName)
	}
}

// Launch scraping process on this worker
func (w *Worker) Work() {
	if w.Stopped {
		return
	}

	for job := range w.Jobs {
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
		pageData, err := web.GetPage(job.URL, w.Conf.Requests.UserAgent, w.Conf.Requests.WaitTimeoutMs)
		if err != nil {
			logger.Error("Failed to get \"%s\": %s", job.URL, err)
			continue
		}

		// find links
		pageLinks := web.FindPageLinks(pageData, pageURL)

		go func() {
			if job.Depth > 1 {
				// decrement depth and add new jobs to the channel
				job.Depth--

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
		}()

		// process and output result
		var savePage bool = false

		switch job.Search.Query {
		case config.QueryLinks:
			// simply output links
			if len(pageLinks) > 0 {
				w.Results <- web.Result{
					PageURL: job.URL,
					Search:  job.Search,
					Data:    pageLinks,
				}
				w.stats.MatchesFound += uint64(len(pageLinks))
				savePage = true
			}

		case config.QueryImages:
			// find image URLs, output images to the file while not saving already outputted ones
			imageLinks := web.FindPageImages(pageData, pageURL)

			var alreadyProcessedImgUrls []string
			for count, imageLink := range imageLinks {
				// check if this URL has been processed already
				var skipImage bool = false

				for _, processedURL := range alreadyProcessedImgUrls {
					if imageLink == processedURL {
						skipImage = true
						break
					}
				}

				if skipImage {
					skipImage = false
					continue
				}
				alreadyProcessedImgUrls = append(alreadyProcessedImgUrls, imageLink)

				var imageName string = fmt.Sprintf("%s_%d_%s", pageURL.Host, count, path.Base(imageLink))

				response, err := http.Get(imageLink)
				if err != nil {
					logger.Error("Failed to get image %s", imageLink)
					continue
				}

				imageFile, err := os.Create(filepath.Join(w.Conf.Save.OutputDir, imageName))
				if err != nil {
					logger.Error("Failed to create image file \"%s\": %s", imageName, err)
					continue
				}

				_, _ = io.Copy(imageFile, response.Body)

				response.Body.Close()
				imageFile.Close()

				logger.Info("Outputted \"%s\"", imageName)
				w.stats.MatchesFound++
			}

			if len(imageLinks) > 0 {
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
			w.stats.PagesSaved++
		}

		// sleep before the next request
		time.Sleep(time.Duration(w.Conf.Requests.RequestPauseMs * uint64(time.Millisecond)))
	}
}
