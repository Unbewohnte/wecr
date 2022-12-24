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

type visited struct {
	URLs []string
	Lock sync.Mutex
}

type Statistics struct {
	PagesVisited uint64
	MatchesFound uint64
	StartTime    time.Time
}

type Pool struct {
	workersCount uint
	workers      []*Worker
	visited      visited
	Stats        Statistics
}

func NewWorkerPool(jobs chan web.Job, results chan web.Result, workerCount uint, workerConf WorkerConf) *Pool {
	var newPool Pool = Pool{
		workersCount: workerCount,
		workers:      nil,
		visited: visited{
			URLs: nil,
			Lock: sync.Mutex{},
		},
		Stats: Statistics{
			StartTime:    time.Time{},
			PagesVisited: 0,
			MatchesFound: 0,
		},
	}

	var i uint
	for i = 0; i < workerCount; i++ {
		newWorker := NewWorker(jobs, results, workerConf, &newPool.visited, &newPool.Stats)
		newPool.workers = append(newPool.workers, &newWorker)
	}

	return &newPool
}

func (p *Pool) Work() {
	p.Stats.StartTime = time.Now()

	for _, worker := range p.workers {
		worker.Stopped = false
		go worker.Work()
	}
}

func (p *Pool) Stop() {
	for _, worker := range p.workers {
		worker.Stopped = true
	}
}
