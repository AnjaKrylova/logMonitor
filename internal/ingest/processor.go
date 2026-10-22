package ingest

import (
	"sync"

	"logmonitor/internal/logs"
	"logmonitor/internal/storage"
)

type Processor struct {
	NumberOfWorkers int
	Repository      storage.LogRepository
}

func (p *Processor) Run(ch <-chan logs.LogEntry) {
	var wg sync.WaitGroup
	for range p.NumberOfWorkers {
		wg.Go(func() {
			Consumer(ch, p.Repository)
		})
	}
	wg.Wait()
}
