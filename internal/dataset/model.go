package dataset

import "time"

type Dataset struct {
 ID string `json:"id"`
 Filename string `json:"filename"`
 Format string `json:"format"`
 SizeBytes int64 `json:"size_bytes"`
 RecordCount int64 `json:"record_count"`
 Checksum string `json:"checksum"`
 Status string `json:"status"`
 CreatedAt time.Time `json:"created_at"`
}

type Partition struct {
 ID string `json:"id"`
 DatasetID string `json:"dataset_id"`
 Index int `json:"index"`
 StartOffset int64 `json:"start_offset"`
 EndOffset int64 `json:"end_offset"`
 RecordCount int64 `json:"record_count"`
}
