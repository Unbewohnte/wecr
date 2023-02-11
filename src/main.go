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

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unbewohnte/wecr/config"
	"unbewohnte/wecr/dashboard"
	"unbewohnte/wecr/logger"
	"unbewohnte/wecr/queue"
	"unbewohnte/wecr/utilities"
	"unbewohnte/wecr/web"
	"unbewohnte/wecr/worker"
)

const version = "v0.3.0"

const (
	defaultConfigFile           string = "conf.json"
	defaultOutputFile           string = "output.json"
	defaultPrettifiedOutputFile string = "extracted_data.txt"
	defaultVisitQueueFile       string = "visit_queue.tmp"
)

var (
	printVersion = flag.Bool(
		"version", false,
		"Print version and exit",
	)

	wDir = flag.String(
		"wdir", "",
		"Force set working directory",
	)

	configFile = flag.String(
		"conf", defaultConfigFile,
		"Configuration file name to create|look for",
	)

	outputFile = flag.String(
		"out", defaultOutputFile,
		"Output file name to output information into",
	)

	extractDataFilename = flag.String(
		"extractData", "",
		"Set filename for output JSON file and extract data from it, put each entry nicely on a new line in a new file, then exit",
	)

	workingDirectory string
	configFilePath   string
	outputFilePath   string
)

func init() {
	// set log output
	logger.SetOutput(os.Stdout)

	// make default http logger silent
	log.SetOutput(io.Discard)

	// parse and process flags
	flag.Parse()

	if *printVersion {
		fmt.Printf(
			"Wecr %s - crawl the web for data\n(c) 2023 Kasyanov Nikolay Alexeyevich (Unbewohnte)\n",
			version,
		)
		os.Exit(0)
	}

	// print logo
	logger.GetOutput().Write([]byte(
		`██╗    ██╗███████╗ ██████╗██████╗ 
██║    ██║██╔════╝██╔════╝██╔══██╗
██║ █╗ ██║█████╗  ██║     ██████╔╝
██║███╗██║██╔══╝  ██║     ██╔══██╗
╚███╔███╔╝███████╗╚██████╗██║  ██║
 ╚══╝╚══╝ ╚══════╝ ╚═════╝╚═╝  ╚═╝`),
	)
	logger.GetOutput().Write([]byte(version + " by Unbewohnte\n\n"))

	// work out working directory path
	if *wDir != "" {
		workingDirectory = *wDir
	} else {
		exePath, err := os.Executable()
		if err != nil {
			logger.Error("Failed to determine executable's path: %s", err)
			return
		}
		workingDirectory = filepath.Dir(exePath)
	}

	logger.Info("Working in \"%s\"", workingDirectory)

	// extract data if needed
	if strings.TrimSpace(*extractDataFilename) != "" {
		logger.Info("Extracting data from %s...", *extractDataFilename)
		err := utilities.ExtractDataFromOutput(*extractDataFilename, defaultPrettifiedOutputFile, "\n", false)
		if err != nil {
			logger.Error("Failed to extract data from %s: %s", *extractDataFilename, err)
			os.Exit(1)
		}
		logger.Info("Outputted \"%s\"", defaultPrettifiedOutputFile)
		os.Exit(0)
	}

	// global path to configuration file
	configFilePath = filepath.Join(workingDirectory, *configFile)

	// global path to output file
	outputFilePath = filepath.Join(workingDirectory, *outputFile)
}

func main() {
	// open config
	logger.Info("Trying to open config \"%s\"", configFilePath)

	var conf *config.Conf
	conf, err := config.OpenConfigFile(configFilePath)
	if err != nil {
		logger.Error(
			"Failed to open configuration file: %s. Creating a new one with the same name instead...",
			err,
		)

		err = config.CreateConfigFile(*config.Default(), configFilePath)
		if err != nil {
			logger.Error("Could not create new configuration file: %s", err)
			return
		}
		logger.Info("Created new configuration file. Exiting...")

		return
	}
	logger.Info("Successfully opened configuration file")

	// Prepare global statistics variable
	statistics := worker.Statistics{}

	// open dashboard if needed
	var board *dashboard.Dashboard = nil
	if conf.Dashboard.UseDashboard {
		board = dashboard.NewDashboard(conf.Dashboard.Port, conf, &statistics)
		go board.Launch()
		logger.Info("Launched dashboard at http://localhost:%d", conf.Dashboard.Port)
	}

	// sanitize and correct inputs
	if len(conf.InitialPages) == 0 {
		logger.Error("No initial page URLs have been set")
		return
	} else if len(conf.InitialPages) != 0 && conf.InitialPages[0] == "" {
		logger.Error("No initial page URLs have been set")
		return
	}

	var sanitizedBlacklistedDomains []string
	for _, blacklistedDomain := range conf.BlacklistedDomains {
		if strings.TrimSpace(blacklistedDomain) == "" {
			continue
		}

		parsedURL, err := url.Parse(blacklistedDomain)
		if err != nil {
			logger.Warning("Failed to parse blacklisted \"%s\": %s", blacklistedDomain, err)
			continue
		}

		if parsedURL.Scheme == "" {
			// parsing is invalid, as stdlib says
			logger.Warning("Failed to parse blacklisted \"%s\": no scheme specified", blacklistedDomain)
			continue
		}

		sanitizedBlacklistedDomains = append(sanitizedBlacklistedDomains, parsedURL.Host)
	}
	conf.BlacklistedDomains = sanitizedBlacklistedDomains

	var sanitizedAllowedDomains []string
	for _, allowedDomain := range conf.AllowedDomains {
		if strings.TrimSpace(allowedDomain) == "" {
			continue
		}

		parsedURL, err := url.Parse(allowedDomain)
		if err != nil {
			logger.Warning("Failed to parse allowed \"%s\": %s", allowedDomain, err)
			continue
		}

		if parsedURL.Scheme == "" {
			// parsing is invalid, as stdlib says
			logger.Warning("Failed to parse allowed \"%s\": no scheme specified", allowedDomain)
			continue
		}

		sanitizedAllowedDomains = append(sanitizedAllowedDomains, parsedURL.Host)
	}
	conf.AllowedDomains = sanitizedAllowedDomains

	if conf.Depth <= 0 {
		conf.Depth = 1
		logger.Warning("Depth is <= 0. Set to %d", conf.Depth)
	}

	if conf.Workers <= 0 {
		conf.Workers = 5
		logger.Warning("Workers number is <= 0. Set to %d", conf.Workers)
	}

	if conf.Search.Query == "" {
		logger.Warning("Search query has not been set")
		return
	}

	if conf.Requests.UserAgent == "" {
		conf.Requests.UserAgent = "Mozilla/5.0 (Windows NT 6.1; Win64; x64; rv:47.0) Gecko/20100101 Firefox/47.0"
		logger.Warning("User agent is not set. Forced to \"%s\"", conf.Requests.UserAgent)
	}

	// create output directories and corresponding specialized ones
	if !filepath.IsAbs(conf.Save.OutputDir) {
		conf.Save.OutputDir = filepath.Join(workingDirectory, conf.Save.OutputDir)
	}
	err = os.MkdirAll(conf.Save.OutputDir, os.ModePerm)
	if err != nil {
		logger.Error("Failed to create output directory: %s", err)
		return
	}

	err = os.MkdirAll(filepath.Join(conf.Save.OutputDir, config.SavePagesDir), os.ModePerm)
	if err != nil {
		logger.Error("Failed to create output directory for pages: %s", err)
		return
	}

	err = os.MkdirAll(filepath.Join(conf.Save.OutputDir, config.SaveImagesDir), os.ModePerm)
	if err != nil {
		logger.Error("Failed to create output directory for images: %s", err)
		return
	}

	err = os.MkdirAll(filepath.Join(conf.Save.OutputDir, config.SaveVideosDir), os.ModePerm)
	if err != nil {
		logger.Error("Failed to create output directory for video: %s", err)
		return
	}

	err = os.MkdirAll(filepath.Join(conf.Save.OutputDir, config.SaveAudioDir), os.ModePerm)
	if err != nil {
		logger.Error("Failed to create output directory for audio: %s", err)
		return
	}

	err = os.MkdirAll(filepath.Join(conf.Save.OutputDir, config.SaveDocumentsDir), os.ModePerm)
	if err != nil {
		logger.Error("Failed to create output directory for documents: %s", err)
		return
	}

	switch conf.Search.Query {
	case config.QueryEmail:
		logger.Info("Looking for email addresses")
	case config.QueryImages:
		logger.Info("Looking for images (%+s)", web.ImageExtentions)
	case config.QueryVideos:
		logger.Info("Looking for videos (%+s)", web.VideoExtentions)
	case config.QueryAudio:
		logger.Info("Looking for audio (%+s)", web.AudioExtentions)
	case config.QueryDocuments:
		logger.Info("Looking for documents (%+s)", web.DocumentExtentions)
	case config.QueryEverything:
		logger.Info("Looking for email addresses, images, videos, audio and various documents (%+s - %+s - %+s - %+s)",
			web.ImageExtentions,
			web.VideoExtentions,
			web.AudioExtentions,
			web.DocumentExtentions,
		)
	default:
		if conf.Search.IsRegexp {
			logger.Info("Looking for RegExp matches (%s)", conf.Search.Query)
		} else {
			logger.Info("Looking for text matches (%s)", conf.Search.Query)
		}
	}

	// create output file
	outputFile, err := os.Create(outputFilePath)
	if err != nil {
		logger.Error("Failed to create output file: %s", err)
		return
	}
	defer outputFile.Close()

	// create logs if needed
	if conf.Logging.OutputLogs {
		if conf.Logging.LogsFile != "" {
			// output logs to a file
			logFile, err := os.Create(filepath.Join(workingDirectory, conf.Logging.LogsFile))
			if err != nil {
				logger.Error("Failed to create logs file: %s", err)
				return
			}
			defer logFile.Close()

			logger.Info("Outputting logs to %s", conf.Logging.LogsFile)
			logger.SetOutput(logFile)
		} else {
			// output logs to stdout
			logger.Info("Outputting logs to stdout")
			logger.SetOutput(os.Stdout)
		}
	} else {
		// no logging needed
		logger.Info("No further logs will be outputted")
		logger.SetOutput(nil)
	}

	jobs := make(chan web.Job, conf.Workers*5)
	results := make(chan web.Result, conf.Workers*5)

	// create visit queue file if not turned off
	var visitQueueFile *os.File = nil
	if !conf.InMemoryVisitQueue {
		var err error
		visitQueueFile, err = os.Create(filepath.Join(workingDirectory, defaultVisitQueueFile))
		if err != nil {
			logger.Error("Could not create visit queue temporary file: %s", err)
			return
		}
		defer func() {
			visitQueueFile.Close()
			os.Remove(filepath.Join(workingDirectory, defaultVisitQueueFile))
		}()
	}

	// create initial jobs
	if !conf.InMemoryVisitQueue {
		for _, initialPage := range conf.InitialPages {
			var newJob web.Job = web.Job{
				URL:    initialPage,
				Search: conf.Search,
				Depth:  conf.Depth,
			}
			err = queue.InsertNewJob(visitQueueFile, newJob)
			if err != nil {
				logger.Error("Failed to encode an initial job to the visit queue: %s", err)
				continue
			}
		}
		visitQueueFile.Seek(0, io.SeekStart)
	} else {
		for _, initialPage := range conf.InitialPages {
			jobs <- web.Job{
				URL:    initialPage,
				Search: conf.Search,
				Depth:  conf.Depth,
			}
		}
	}

	// form a worker pool
	workerPool := worker.NewWorkerPool(jobs, results, conf.Workers, &worker.WorkerConf{
		Requests:           conf.Requests,
		Save:               conf.Save,
		BlacklistedDomains: conf.BlacklistedDomains,
		AllowedDomains:     conf.AllowedDomains,
		VisitQueue: worker.VisitQueue{
			VisitQueue: visitQueueFile,
			Lock:       &sync.Mutex{},
		},
	}, &statistics)
	logger.Info("Created a worker pool with %d workers", conf.Workers)

	// set up graceful shutdown
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	go func() {
		<-sig
		logger.Info("Received interrupt signal. Exiting...")

		// stop workers
		workerPool.Stop()

		// close results channel
		close(results)
	}()

	// launch concurrent scraping !
	workerPool.Work()
	logger.Info("Started scraping...")

	// if logs are not used or are printed to the file - output a nice statistics message on the screen
	if !conf.Logging.OutputLogs || (conf.Logging.OutputLogs && conf.Logging.LogsFile != "") {
		go func() {
			var lastPagesVisited uint64 = 0
			fmt.Printf("\n")
			for {
				time.Sleep(time.Second)

				timeSince := time.Since(time.Unix(int64(statistics.StartTimeUnix), 0)).Round(time.Second)
				fmt.Fprintf(os.Stdout, "\r[%s] %d pages visited; %d pages saved; %d matches (%d pages/sec)",
					timeSince.String(),
					statistics.PagesVisited,
					statistics.PagesSaved,
					statistics.MatchesFound,
					statistics.PagesVisited-lastPagesVisited,
				)
				lastPagesVisited = statistics.PagesVisited
			}
		}()
	}

	// get text results and write them to the output file (files are handled by each worker separately)
	for {
		result, ok := <-results
		if !ok {
			break
		}

		// each entry in output file is a self-standing JSON object
		entryBytes, err := json.MarshalIndent(result, " ", "\t")
		if err != nil {
			continue
		}
		outputFile.Write(entryBytes)
		outputFile.Write([]byte("\n"))
	}
}
