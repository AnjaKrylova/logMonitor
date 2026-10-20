package ingest

import (
	"testing"

	"logmonitor/internal/logs"
)

func TestProducer(t *testing.T) {
	ch := Producer(5)
	var in []logs.LogEntry
	for e := range ch {
		in = append(in, e)
	}
	if len(in) != 5 {
		t.Errorf("TestProducer\n got: %v\nwant: %v", len(in), 5)
	}
}
