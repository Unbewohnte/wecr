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
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
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
	Search             *config.Search
	Requests           *config.Requests
	Save               *config.Save
	BlacklistedDomains []string
	AllowedDomains     []string
	VisitQueue         VisitQueue
	TextOutput         io.Writer
	EmailsOutput       io.Writer
}

// Web worker
type Worker struct {
	Jobs    chan web.Job
	Conf    *WorkerConf
	visited *visited
	stats   *Statistics
	Stopped bool
}

// Create a new worker
func NewWorker(jobs chan web.Job, conf *WorkerConf, visited *visited, stats *Statistics) Worker {
	return Worker{
		Jobs:    jobs,
		Conf:    conf,
		visited: visited,
		stats:   stats,
		Stopped: false,
	}
}

func (w *Worker) saveContent(links []url.URL, pageURL *url.URL) {
	var alreadyProcessedUrls []url.URL
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

		var fileName string = fmt.Sprintf("%s_%d_%s", pageURL.Host, count, path.Base(link.Path))

		var filePath string
		if web.HasImageExtention(link.Path) {
			filePath = filepath.Join(w.Conf.Save.OutputDir, config.SaveImagesDir, fileName)
		} else if web.HasVideoExtention(link.Path) {
			filePath = filepath.Join(w.Conf.Save.OutputDir, config.SaveVideosDir, fileName)
		} else if web.HasAudioExtention(link.Path) {
			filePath = filepath.Join(w.Conf.Save.OutputDir, config.SaveAudioDir, fileName)
		} else if web.HasDocumentExtention(link.Path) {
			filePath = filepath.Join(w.Conf.Save.OutputDir, config.SaveDocumentsDir, fileName)
		} else {
			filePath = filepath.Join(w.Conf.Save.OutputDir, fileName)
		}

		err := web.FetchFile(
			link.String(),
			w.Conf.Requests.UserAgent,
			w.Conf.Requests.ContentFetchTimeoutMs,
			filePath,
		)
		if err != nil {
			logger.Error("Failed to fetch file located at %s: %s", link.String(), err)
			return
		}

		logger.Info("Outputted \"%s\"", fileName)
		w.stats.MatchesFound++
	}
}

// Save page to the disk with a corresponding name; Download any src files, stylesheets and JS along the way
func (w *Worker) savePage(baseURL url.URL, pageData []byte) {
	var findPageFileContentURLs func([]byte) []url.URL = func(pageBody []byte) []url.URL {
		var urls []url.URL

		for _, link := range web.FindPageLinksDontResolve(pageData) {
			if strings.Contains(link.Path, ".css") ||
				strings.Contains(link.Path, ".scss") ||
				strings.Contains(link.Path, ".js") ||
				strings.Contains(link.Path, ".mjs") {
				urls = append(urls, link)
			}
		}
		urls = append(urls, web.FindPageSrcLinksDontResolve(pageBody)...)

		return urls
	}

	var cleanLink func(url.URL, url.URL) url.URL = func(link url.URL, from url.URL) url.URL {
		resolvedLink := web.ResolveLink(link, from.Host)
		cleanLink, err := url.Parse(resolvedLink.Scheme + "://" + resolvedLink.Host + resolvedLink.Path)
		if err != nil {
			return resolvedLink
		}
		return *cleanLink
	}

	// Create directory with all file content on the page
	var pageFilesDirectoryName string = fmt.Sprintf(
		"%s_%s_files",
		baseURL.Host,
		strings.ReplaceAll(baseURL.Path, "/", "_"),
	)
	err := os.MkdirAll(filepath.Join(w.Conf.Save.OutputDir, config.SavePagesDir, pageFilesDirectoryName), os.ModePerm)
	if err != nil {
		logger.Error("Failed to create directory to store file contents of %s: %s", baseURL.String(), err)
		return
	}

	// Save files on page
	srcLinks := findPageFileContentURLs(pageData)
	for _, srcLink := range srcLinks {
		web.FetchFile(srcLink.String(),
			w.Conf.Requests.UserAgent,
			w.Conf.Requests.ContentFetchTimeoutMs,
			filepath.Join(
				w.Conf.Save.OutputDir,
				config.SavePagesDir,
				pageFilesDirectoryName,
				path.Base(srcLink.String()),
			),
		)
	}

	// Redirect old content URLs to local files
	for _, srcLink := range srcLinks {
		cleanLink := cleanLink(srcLink, baseURL)
		pageData = bytes.ReplaceAll(
			pageData,
			[]byte(srcLink.String()),
			[]byte("./"+filepath.Join(pageFilesDirectoryName, path.Base(cleanLink.String()))),
		)
	}

	// Create page output file
	pageName := fmt.Sprintf(
		"%s_%s.html",
		baseURL.Host,
		strings.ReplaceAll(baseURL.Path, "/", "_"),
	)
	outfile, err := os.Create(filepath.Join(
		filepath.Join(w.Conf.Save.OutputDir, config.SavePagesDir),
		pageName,
	))
	if err != nil {
		fmt.Printf("Failed to create output file: %s\n", err)
	}
	defer outfile.Close()

	outfile.Write(pageData)

	logger.Info("Saved \"%s\"", pageName)
	w.stats.PagesSaved++
}

const (
	textTypeMatch = iota
	textTypeEmail = iota
)

// Save text result to an appropriate file
func (w *Worker) saveResult(result web.Result, textType int) {
	// write result to the output file
	var output io.Writer
	switch textType {
	case textTypeEmail:
		output = w.Conf.EmailsOutput

	default:
		output = w.Conf.TextOutput
	}

	// each entry in output file is a self-standing JSON object
	entryBytes, err := json.MarshalIndent(result, " ", "\t")
	if err != nil {
		return
	}
	output.Write(entryBytes)
	output.Write([]byte("\n"))
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
		pageLinks := web.FindPageLinks(pageData, *pageURL)
		go func() {
			if job.Depth > 1 {
				// decrement depth and add new jobs
				job.Depth--

				if w.Conf.VisitQueue.VisitQueue != nil {
					// add to the visit queue
					w.Conf.VisitQueue.Lock.Lock()
					for _, link := range pageLinks {
						if link.String() != job.URL {
							err = queue.InsertNewJob(w.Conf.VisitQueue.VisitQueue, web.Job{
								URL:    link.String(),
								Search: *w.Conf.Search,
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
						if link.String() != job.URL {
							w.Jobs <- web.Job{
								URL:    link.String(),
								Search: *w.Conf.Search,
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
		case config.QueryArchive:
			savePage = true

		case config.QueryImages:
			// find image URLs, output images to the file while not saving already outputted ones
			imageLinks := web.FindPageImages(pageData, *pageURL)
			if len(imageLinks) > 0 {
				w.saveContent(imageLinks, pageURL)
				savePage = true
			}

		case config.QueryVideos:
			// search for videos
			// find video URLs, output videos to the files while not saving already outputted ones
			videoLinks := web.FindPageVideos(pageData, *pageURL)
			if len(videoLinks) > 0 {
				w.saveContent(videoLinks, pageURL)
				savePage = true
			}

		case config.QueryAudio:
			// search for audio
			// find audio URLs, output audio to the file while not saving already outputted ones
			audioLinks := web.FindPageAudio(pageData, *pageURL)
			if len(audioLinks) > 0 {
				w.saveContent(audioLinks, pageURL)
				savePage = true
			}

		case config.QueryDocuments:
			// search for various documents
			// find documents URLs, output docs to the file while not saving already outputted ones
			docsLinks := web.FindPageDocuments(pageData, *pageURL)
			if len(docsLinks) > 0 {
				w.saveContent(docsLinks, pageURL)
				savePage = true
			}

		case config.QueryEmail:
			// search for email
			emailAddresses := web.FindPageEmailsWithCheck(pageData)
			if len(emailAddresses) > 0 {
				w.saveResult(web.Result{
					PageURL: job.URL,
					Search:  job.Search,
					Data:    emailAddresses,
				}, textTypeEmail)
				w.stats.MatchesFound += uint64(len(emailAddresses))
				savePage = true
			}

		case config.QueryEverything:
			// search for everything

			// files
			var contentLinks []url.URL
			contentLinks = append(contentLinks, web.FindPageImages(pageData, *pageURL)...)
			contentLinks = append(contentLinks, web.FindPageAudio(pageData, *pageURL)...)
			contentLinks = append(contentLinks, web.FindPageVideos(pageData, *pageURL)...)
			contentLinks = append(contentLinks, web.FindPageDocuments(pageData, *pageURL)...)
			w.saveContent(contentLinks, pageURL)

			if len(contentLinks) > 0 {
				savePage = true
			}

			// email
			emailAddresses := web.FindPageEmailsWithCheck(pageData)
			if len(emailAddresses) > 0 {
				w.saveResult(web.Result{
					PageURL: job.URL,
					Search:  job.Search,
					Data:    emailAddresses,
				}, textTypeEmail)
				w.stats.MatchesFound += uint64(len(emailAddresses))
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
					w.saveResult(web.Result{
						PageURL: job.URL,
						Search:  job.Search,
						Data:    matches,
					}, textTypeMatch)
					logger.Info("Found matches: %+v", matches)
					w.stats.MatchesFound += uint64(len(matches))
					savePage = true
				}
			case false:
				// just text
				if web.IsTextOnPage(job.Search.Query, true, pageData) {
					w.saveResult(web.Result{
						PageURL: job.URL,
						Search:  job.Search,
						Data:    []string{job.Search.Query},
					}, textTypeMatch)
					logger.Info("Found \"%s\" on page", job.Search.Query)
					w.stats.MatchesFound++
					savePage = true
				}
			}
		}

		// save page
		if savePage && w.Conf.Save.SavePages {
			w.savePage(*pageURL, pageData)
		}
		pageData = nil
		pageURL = nil

		// sleep before the next request
		time.Sleep(time.Duration(w.Conf.Requests.RequestPauseMs * uint64(time.Millisecond)))
	}
}
