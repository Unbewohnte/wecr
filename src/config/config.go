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

package config

import (
	"encoding/json"
	"io"
	"os"
)

const (
	QueryImages     string = "images"
	QueryVideos     string = "videos"
	QueryAudio      string = "audio"
	QueryEmail      string = "email"
	QueryEverything string = "everything"
)

const (
	SavePagesDir  string = "pages"
	SaveImagesDir string = "images"
	SaveVideosDir string = "videos"
	SaveAudioDir  string = "audio"
)

type Search struct {
	IsRegexp bool   `json:"is_regexp"`
	Query    string `json:"query"`
}

type Save struct {
	OutputDir  string `json:"output_dir"`
	OutputFile string `json:"output_file"`
	SavePages  bool   `json:"save_pages"`
}

type Requests struct {
	RequestWaitTimeoutMs  uint64 `json:"request_wait_timeout_ms"`
	RequestPauseMs        uint64 `json:"request_pause_ms"`
	ContentFetchTimeoutMs uint64 `json:"content_fetch_timeout_ms"`
	UserAgent             string `json:"user_agent"`
}

type Logging struct {
	OutputLogs bool   `json:"output_logs"`
	LogsFile   string `json:"logs_file"`
}

// Configuration file structure
type Conf struct {
	Search             Search   `json:"search"`
	Requests           Requests `json:"requests"`
	Depth              uint     `json:"depth"`
	Workers            uint     `json:"workers"`
	InitialPages       []string `json:"initial_pages"`
	AllowedDomains     []string `json:"allowed_domains"`
	BlacklistedDomains []string `json:"blacklisted_domains"`
	Save               Save     `json:"save"`
	Logging            Logging  `json:"logging"`
}

// Default configuration file structure
func Default() *Conf {
	return &Conf{
		Search: Search{
			IsRegexp: false,
			Query:    "",
		},
		Save: Save{
			OutputDir:  "scraped",
			SavePages:  false,
			OutputFile: "scraped.json",
		},
		Requests: Requests{
			UserAgent:             "",
			RequestWaitTimeoutMs:  2500,
			RequestPauseMs:        100,
			ContentFetchTimeoutMs: 0,
		},
		InitialPages:       []string{""},
		Depth:              5,
		Workers:            20,
		AllowedDomains:     []string{""},
		BlacklistedDomains: []string{""},
		Logging: Logging{
			OutputLogs: true,
			LogsFile:   "logs.log",
		},
	}
}

// Write current configuration to w
func (c *Conf) WriteTo(w io.Writer) error {
	jsonData, err := json.MarshalIndent(c, " ", "\t")
	if err != nil {
		return err
	}

	_, err = w.Write(jsonData)
	if err != nil {
		return err
	}

	return nil
}

// Read configuration from r
func (c *Conf) ReadFrom(r io.Reader) error {
	jsonData, err := io.ReadAll(r)
	if err != nil {
		return nil
	}

	err = json.Unmarshal(jsonData, c)
	if err != nil {
		return err
	}

	return nil
}

// Creates configuration file at path
func CreateConfigFile(conf Conf, path string) error {
	confFile, err := os.Create(path)
	if err != nil {
		return err
	}
	defer confFile.Close()

	err = conf.WriteTo(confFile)
	if err != nil {
		return err
	}

	return nil
}

// Tries to open configuration file at path. If it fails - returns default configuration
func OpenConfigFile(path string) (*Conf, error) {
	confFile, err := os.Open(path)
	if err != nil {
		return Default(), err
	}
	defer confFile.Close()

	var conf Conf
	err = conf.ReadFrom(confFile)
	if err != nil {
		return Default(), err
	}

	return &conf, nil
}
