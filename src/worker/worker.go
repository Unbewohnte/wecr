/*
	websurf - surf the web for data recursively
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
	"unbewohnte/websurf/config"
	"unbewohnte/websurf/logger"
	"unbewohnte/websurf/web"
)

type WorkerConf struct {
	Requests config.Requests
	Save     config.Save
}

type Worker struct {
	Jobs    chan web.Job
	Results chan web.Result
	Conf    WorkerConf
	visited *visited
	stats   *Statistics
	Stopped bool
}

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

		// check if it is the first occurence
		w.visited.Lock.Lock()
		var skip bool = false
		for _, visitedURL := range w.visited.URLs {
			if job.URL == visitedURL {
				// okay, don't even bother. Move onto the next job
				skip = true
				logger.Info("Skipping %s", job.URL)
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
		pageLinks := web.FindPageLinks(pageData)

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
				savePage = true
			}

		case config.QueryImages:
			// find image URLs, output data to the file
			parsedURL, err := url.Parse(job.URL)
			if err != nil {
				logger.Error("Failed to parse URL \"%s\" to get hostname: %s", job.URL, err)
				continue
			}

			imageLinks := web.FindPageImages(pageData, parsedURL.Host)

			for count, imageLink := range imageLinks {
				var imageName string = fmt.Sprintf("%s_%d_%s", parsedURL.Host, count, path.Base(imageLink))

				response, err := http.Get(imageLink)
				if err != nil {
					logger.Error("Failed to get %s", imageLink)
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

			// save page
			if savePage && w.Conf.Save.SavePages && w.Conf.Save.OutputDir != "" {
				url, err := url.Parse(job.URL)
				if err != nil {
					logger.Error("Failed to parse \"%s\" to save page: %s", job.URL, err)
					break
				}

				var pageName string = fmt.Sprintf("%s_%s.html", url.Host, path.Base(job.URL))
				pageFile, err := os.Create(filepath.Join(w.Conf.Save.OutputDir, pageName))
				if err != nil {
					logger.Error("Failed to create page of \"%s\": %s", job.URL, err)
				} else {
					pageFile.Write(pageData)
				}

				pageFile.Close()
			}

			// sleep before the next request
			time.Sleep(time.Duration(w.Conf.Requests.RequestPauseMs * uint64(time.Millisecond)))
		}
	}
}
