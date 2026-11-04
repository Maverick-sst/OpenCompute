package logs

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

const maxRecordBytes = 1 << 20

func Process(input io.Reader) (Aggregate, error) {
	out := Aggregate{
		StatusCounts:   map[string]int64{},
		ServiceCounts:  map[string]int64{},
		EndpointCounts: map[string]int64{},
	}
	r := bufio.NewReaderSize(input, 64*1024)
	for {
		line, err := r.ReadBytes('\n')
		if len(line) > 0 {
			raw := bytes.TrimSuffix(line, []byte("\n"))
			raw = bytes.TrimSuffix(raw, []byte("\r"))
			if len(raw) > 0 {
				out.RecordsRead++
				if int64(len(raw)) > maxRecordBytes {
					out.RecordsRejected++
				} else if rec, parseErr := parse(raw); parseErr != nil {
					out.RecordsRejected++
				} else {
					out.RecordsProcessed++
					if strings.EqualFold(rec.Level, "error") {
						out.ErrorCount++
					}
					if rec.Status != nil {
						out.StatusCounts[fmt.Sprint(*rec.Status)]++
					}
					if rec.Service != "" {
						out.ServiceCounts[rec.Service]++
					}
					if rec.Path != "" {
						out.EndpointCounts[rec.Path]++
					}
					if rec.LatencyMS != nil {
						out.LatencySumMS += *rec.LatencyMS
						out.LatencyCount++
						if *rec.LatencyMS > out.MaxLatencyMS {
							out.MaxLatencyMS = *rec.LatencyMS
						}
					}
				}
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return Aggregate{}, err
		}
	}
	return out, nil
}

func parse(raw []byte) (Record, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		return Record{}, fmt.Errorf("record must be a JSON object")
	}
	var rec Record
	if v, ok := fields["timestamp"]; ok {
		if err := json.Unmarshal(v, &rec.Timestamp); err != nil {
			return rec, err
		}
		if rec.Timestamp != "" {
			if _, err := time.Parse(time.RFC3339Nano, rec.Timestamp); err != nil {
				return rec, err
			}
		}
	}
	for key, dst := range map[string]*string{
		"level":   &rec.Level,
		"service": &rec.Service,
		"path":    &rec.Path,
	} {
		if v, ok := fields[key]; ok {
			if err := json.Unmarshal(v, dst); err != nil {
				return rec, err
			}
		}
	}
	if v, ok := fields["status"]; ok {
		var n int
		if err := json.Unmarshal(v, &n); err != nil {
			return rec, err
		}
		rec.Status = &n
	}
	if v, ok := fields["latency_ms"]; ok {
		var n int64
		if err := json.Unmarshal(v, &n); err != nil || n < 0 {
			return rec, fmt.Errorf("latency_ms must be nonnegative")
		}
		rec.LatencyMS = &n
	}
	return rec, nil
}
