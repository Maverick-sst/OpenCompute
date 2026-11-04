package logs

import (
	"strings"
	"testing"
)

func TestProcessAndMerge(t *testing.T) {
	input := "{\"level\":\"ERROR\",\"service\":\"api\",\"status\":500,\"path\":\"/users\",\"latency_ms\":20}\n{\"level\":\"INFO\",\"service\":\"api\",\"status\":200,\"latency_ms\":40}\nnot-json\n"
	a, err := Process(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if a.RecordsRead != 3 || a.RecordsProcessed != 2 || a.RecordsRejected != 1 || a.ErrorCount != 1 || a.MaxLatencyMS != 40 {
		t.Fatalf("aggregate=%+v", a)
	}
	b := Merge(a, a)
	if b.RecordsProcessed != 4 || b.LatencyCount != 4 || b.AverageLatencyMS() != 30 {
		t.Fatalf("merged=%+v", b)
	}
}

func TestProcessRejectsInvalidFieldTypes(t *testing.T) {
	a, err := Process(strings.NewReader("{\"status\":\"200\"}\n{\"latency_ms\":-1}\n"))
	if err != nil {
		t.Fatal(err)
	}
	if a.RecordsRejected != 2 || a.RecordsProcessed != 0 {
		t.Fatalf("aggregate=%+v", a)
	}
}
