package queue

import (
	"encoding/json"
	"io"
	"os"
	"unbewohnte/wecr/web"
)

func PopLastJob(queue *os.File) (*web.Job, error) {
	stats, err := queue.Stat()
	if err != nil {
		return nil, err
	}

	if stats.Size() == 0 {
		return nil, nil
	}

	// find the last job in the queue
	var job web.Job
	var offset int64 = -1
	for {
		currentOffset, err := queue.Seek(offset, io.SeekEnd)
		if err != nil {
			return nil, err
		}

		decoder := json.NewDecoder(queue)
		err = decoder.Decode(&job)
		if err != nil || job.URL == "" || job.Search.Query == "" {
			offset -= 1
			continue
		}

		queue.Truncate(currentOffset)
		return &job, nil
	}
}

func InsertNewJob(queue *os.File, newJob web.Job) error {
	_, err := queue.Seek(0, io.SeekEnd)
	if err != nil {
		return err
	}

	encoder := json.NewEncoder(queue)
	err = encoder.Encode(&newJob)
	if err != nil {
		return err
	}

	return nil
}
