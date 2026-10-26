package ingest

import (
	"context"
)

type LogSource interface {
	Stream(context.Context) <-chan string
}

type FakeLogSource struct {
	NumLines int
}

func (fls *FakeLogSource) Stream(ctx context.Context) <-chan string {
	ch := make(chan string)
	go func() {
		defer close(ch)
		for i := 0; i < fls.NumLines; i++ {
			line := `2026-06-24 15:02:19.446989072 +0200 CEST[router] method=GET path="/" status=200 duration=0.030s`
			if i%2 == 0 {
				line = `2026-06-24 15:02:19.446989072 +0200 CEST[web-1] {"message": "Hello, world!", "level": "info"}`
			}
			select {
			case <-ctx.Done():
				return
			case ch <- line:
			}
		}
	}()
	return ch
}
