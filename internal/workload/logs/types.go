package logs

type Record struct {
 Timestamp string `json:"timestamp"`
 Level string `json:"level"`
 Service string `json:"service"`
 Status *int `json:"status"`
 Path string `json:"path"`
 LatencyMS *int64 `json:"latency_ms"`
}

type Aggregate struct {
 RecordsRead int64 `json:"records_read"`
 RecordsProcessed int64 `json:"records_processed"`
 RecordsRejected int64 `json:"records_rejected"`
 ErrorCount int64 `json:"error_count"`
 StatusCounts map[string]int64 `json:"status_counts"`
 ServiceCounts map[string]int64 `json:"service_counts"`
 EndpointCounts map[string]int64 `json:"endpoint_counts"`
 LatencySumMS int64 `json:"latency_sum_ms"`
 LatencyCount int64 `json:"latency_count"`
 MaxLatencyMS int64 `json:"max_latency_ms"`
}
