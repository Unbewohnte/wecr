/*
	Wecr - crawl the web for data
	Copyright (C) 2023 Kasyanov Nikolay Alexeyevich (Unbewohnte)

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

package dashboard

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"net/http"
	"unbewohnte/wecr/config"
	"unbewohnte/wecr/logger"
	"unbewohnte/wecr/worker"
)

type Dashboard struct {
	Server *http.Server
}

//go:embed res
var resFS embed.FS

type PageData struct {
	Conf  config.Conf
	Stats worker.Statistics
}

func NewDashboard(port uint16, webConf *config.Conf, statistics *worker.Statistics) *Dashboard {
	mux := http.NewServeMux()
	res, err := fs.Sub(resFS, "res")
	if err != nil {
		logger.Error("Failed to Sub embedded dashboard FS: %s", err)
		return nil
	}

	mux.Handle("/static/", http.FileServer(http.FS(res)))
	mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		template, err := template.ParseFS(res, "*.html")
		if err != nil {
			logger.Error("Failed to parse embedded dashboard FS: %s", err)
			return
		}

		template.ExecuteTemplate(w, "index.html", nil)
	})

	mux.HandleFunc("/stats", func(w http.ResponseWriter, req *http.Request) {
		jsonStats, err := json.MarshalIndent(statistics, "", " ")
		if err != nil {
			http.Error(w, "Failed to marshal statistics", http.StatusInternalServerError)
			logger.Error("Failed to marshal stats to send to the dashboard: %s", err)
			return
		}
		w.Header().Add("Content-type", "application/json")
		w.Write(jsonStats)
	})

	mux.HandleFunc("/conf", func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodPost:
			var newConfig config.Conf

			defer req.Body.Close()
			newConfigData, err := io.ReadAll(req.Body)
			if err != nil {
				http.Error(w, "Failed to read request body", http.StatusInternalServerError)
				logger.Error("Failed to read new configuration from dashboard request: %s", err)
				return
			}
			err = json.Unmarshal(newConfigData, &newConfig)
			if err != nil {
				http.Error(w, "Failed to unmarshal new configuration", http.StatusInternalServerError)
				logger.Error("Failed to unmarshal new configuration from dashboard UI: %s", err)
				return
			}

			// DO NOT blindly replace global configuration. Manually check and replace values
			webConf.Search.IsRegexp = newConfig.Search.IsRegexp
			if len(newConfig.Search.Query) != 0 {
				webConf.Search.Query = newConfig.Search.Query
			}

			webConf.Logging.OutputLogs = newConfig.Logging.OutputLogs

		default:
			jsonConf, err := json.MarshalIndent(webConf, "", " ")
			if err != nil {
				http.Error(w, "Failed to marshal configuration", http.StatusInternalServerError)
				logger.Error("Failed to marshal current configuration to send to the dashboard UI: %s", err)
				return
			}
			w.Header().Add("Content-type", "application/json")
			w.Write(jsonConf)
		}
	})

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	return &Dashboard{
		Server: server,
	}
}

func (board *Dashboard) Launch() error {
	return board.Server.ListenAndServe()
}
