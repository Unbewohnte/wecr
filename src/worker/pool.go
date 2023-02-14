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
	"sync"
	"time"
	"unbewohnte/wecr/web"
)

// Already visited URLs
type visited struct {
	URLs []string
	Lock sync.Mutex
}

// Whole worker pool's statistics
type Statistics struct {
	PagesVisited  uint64 `json:"pages_visited"`
	MatchesFound  uint64 `json:"matches_found"`
	PagesSaved    uint64 `json:"pages_saved"`
	StartTimeUnix uint64 `json:"start_time_unix"`
	Stopped       bool   `json:"stopped"`
}

// Web-Worker pool
type Pool struct {
	workersCount uint
	workers      []*Worker
	visited      visited
	Stats        *Statistics
}

// Create a new worker pool
func NewWorkerPool(initialJobs chan web.Job, workerCount uint, workerConf *WorkerConf, stats *Statistics) *Pool {
	var newPool Pool = Pool{
		workersCount: workerCount,
		workers:      nil,
		visited: visited{
			URLs: nil,
			Lock: sync.Mutex{},
		},
		Stats: stats,
	}

	var i uint
	for i = 0; i < workerCount; i++ {
		newWorker := NewWorker(initialJobs, workerConf, &newPool.visited, newPool.Stats)
		newPool.workers = append(newPool.workers, &newWorker)
	}

	return &newPool
}

// Notify all workers in pool to start scraping
func (p *Pool) Work() {
	p.Stats.StartTimeUnix = uint64(time.Now().Unix())
	p.Stats.Stopped = false

	for _, worker := range p.workers {
		worker.Stopped = false
		go worker.Work()
	}
}

// Notify all workers in pool to stop scraping
func (p *Pool) Stop() {
	p.Stats.Stopped = true
	for _, worker := range p.workers {
		worker.Stopped = true
	}
}
