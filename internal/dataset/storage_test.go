package dataset

import (
	"strings"
	"testing"
)

func TestSaveAndPartitionJSONL(t *testing.T) {
	m, err := NewManager(t.TempDir(), 1<<20, 1024, 30)
	if err != nil {
		t.Fatal(err)
	}
	input := "{\"level\":\"INFO\"}\n{\"level\":\"ERROR\"}\n{\"level\":\"WARN\"}"
	d, p, err := m.Save("logs.jsonl", strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if d.RecordCount != 3 || d.Format != "jsonl" || d.Status != "ready" {
		t.Fatalf("dataset=%+v", d)
	}
	var total int64
	for i, x := range p {
		if x.Index != i {
			t.Fatalf("index %d", x.Index)
		}
		total += x.RecordCount
	}
	if total != 3 || p[0].StartOffset != 0 || p[len(p)-1].EndOffset != int64(len(input)) {
		t.Fatalf("manifest=%+v", p)
	}
	f, err := m.OpenPartition(d.ID, p[0].StartOffset, p[0].EndOffset)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
}

func TestSaveRejectsMalformedJSONL(t *testing.T) {
	m, _ := NewManager(t.TempDir(), 1<<20, 1024, 1024)
	if _, _, err := m.Save("bad.jsonl", strings.NewReader("{bad}\n")); err == nil {
		t.Fatal("expected invalid JSONL")
	}
}

func TestSaveAcceptsBlankLinesAndNoTrailingNewline(t *testing.T) {
	m, _ := NewManager(t.TempDir(), 1<<20, 1024, 1024)
	d, p, err := m.Save("logs.jsonl", strings.NewReader("\n{\"ok\":true}"))
	if err != nil {
		t.Fatal(err)
	}
	if d.RecordCount != 1 || len(p) != 1 || p[0].EndOffset != int64(len("\n{\"ok\":true}")) {
		t.Fatalf("dataset=%+v parts=%+v", d, p)
	}
}
