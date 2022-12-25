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
	"time"
	"unbewohnte/wecr/config"
	"unbewohnte/wecr/logger"
	"unbewohnte/wecr/web"
	"unbewohnte/wecr/worker"
)

const version = "v0.1.1"

const (
	defaultConfigFile string = "conf.json"
	defaultOutputFile string = "output.json"
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

	workingDirectory string
	configFilePath   string
	outputFilePath   string
)

func init() {
	// set log output
	logger.SetOutput(os.Stdout)

	// and work around random log prints by /x/net library
	log.SetOutput(io.Discard)
	log.SetFlags(0)

	// parse and process flags
	flag.Parse()

	if *printVersion {
		fmt.Printf(
			"Wecr %s - crawl the web for data\n(c) 2022 Kasyanov Nikolay Alexeyevich (Unbewohnte)\n",
			version,
		)
		os.Exit(0)
	}

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
		logger.Info("No logs will be outputted")
		logger.SetOutput(nil)
	}

	// sanitize and correct inputs
	if len(conf.InitialPages) == 0 {
		logger.Warning("No initial page URLs have been set")
		return
	} else if len(conf.InitialPages) != 0 && conf.InitialPages[0] == "" {
		logger.Warning("No initial page URLs have been set")
		return
	}

	for index, blacklistedDomain := range conf.BlacklistedDomains {
		parsedURL, err := url.Parse(blacklistedDomain)
		if err != nil {
			logger.Warning("Failed to parse blacklisted %s: %s", blacklistedDomain, err)
			continue
		}

		conf.BlacklistedDomains[index] = parsedURL.Host
	}

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

	if !filepath.IsAbs(conf.Save.OutputDir) {
		conf.Save.OutputDir = filepath.Join(workingDirectory, conf.Save.OutputDir)
	}
	err = os.MkdirAll(conf.Save.OutputDir, os.ModePerm)
	if err != nil {
		logger.Error("Failed to create output directory: %s", err)
		return
	}

	switch conf.Search.Query {
	case config.QueryLinks:
		logger.Info("Looking for links")
	case config.QueryImages:
		logger.Info("Looking for images")
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

	// prepare channels
	jobs := make(chan web.Job, conf.Workers*5)
	results := make(chan web.Result, conf.Workers*5)

	// create initial jobs
	for _, initialPage := range conf.InitialPages {
		jobs <- web.Job{
			URL:    initialPage,
			Search: conf.Search,
			Depth:  conf.Depth,
		}
	}

	// form a worker pool
	workerPool := worker.NewWorkerPool(jobs, results, conf.Workers, worker.WorkerConf{
		Requests:           conf.Requests,
		Save:               conf.Save,
		BlacklistedDomains: conf.BlacklistedDomains,
	})
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
			fmt.Printf("\n")
			for {
				time.Sleep(time.Second)

				timeSince := time.Since(workerPool.Stats.StartTime).Round(time.Second)

				fmt.Fprintf(os.Stdout, "\r[%s] %d pages; %d matches (%d pages/sec)",
					timeSince.String(),
					workerPool.Stats.PagesVisited,
					workerPool.Stats.MatchesFound,
					workerPool.Stats.PagesVisited/uint64(timeSince.Seconds()),
				)
			}
		}()
	}

	// get results and write them to the output file
	for {
		result, ok := <-results
		if !ok {
			break
		}

		entryBytes, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			continue
		}
		outputFile.Write(entryBytes)
		outputFile.Write([]byte("\n"))
	}
}
