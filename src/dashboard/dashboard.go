package dashboard

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"unbewohnte/wecr/config"
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
		return nil
	}

	mux.Handle("/static/", http.FileServer(http.FS(res)))
	mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		template, err := template.ParseFS(res, "*.html")
		if err != nil {
			return
		}

		template.ExecuteTemplate(w, "index.html", nil)
	})

	mux.HandleFunc("/stats", func(w http.ResponseWriter, req *http.Request) {
		jsonStats, err := json.MarshalIndent(statistics, "", " ")
		if err != nil {
			http.Error(w, "Failed to marshal statistics", http.StatusInternalServerError)
			return
		}
		w.Header().Add("Content-type", "application/json")
		w.Write(jsonStats)
	})

	mux.HandleFunc("/conf", func(w http.ResponseWriter, req *http.Request) {
		jsonConf, err := json.MarshalIndent(webConf, "", " ")
		if err != nil {
			http.Error(w, "Failed to marshal configuration", http.StatusInternalServerError)
			return
		}
		w.Header().Add("Content-type", "application/json")
		w.Write(jsonConf)
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
