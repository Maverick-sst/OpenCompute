package logs

func Merge(values ...Aggregate) Aggregate {
	out := Aggregate{
		StatusCounts:   map[string]int64{},
		ServiceCounts:  map[string]int64{},
		EndpointCounts: map[string]int64{},
	}
	for _, v := range values {
		out.RecordsRead += v.RecordsRead
		out.RecordsProcessed += v.RecordsProcessed
		out.RecordsRejected += v.RecordsRejected
		out.ErrorCount += v.ErrorCount
		out.LatencySumMS += v.LatencySumMS
		out.LatencyCount += v.LatencyCount
		if v.MaxLatencyMS > out.MaxLatencyMS {
			out.MaxLatencyMS = v.MaxLatencyMS
		}
		for k, n := range v.StatusCounts {
			out.StatusCounts[k] += n
		}
		for k, n := range v.ServiceCounts {
			out.ServiceCounts[k] += n
		}
		for k, n := range v.EndpointCounts {
			out.EndpointCounts[k] += n
		}
	}
	return out
}

func (a Aggregate) AverageLatencyMS() float64 {
	if a.LatencyCount == 0 {
		return 0
	}
	return float64(a.LatencySumMS) / float64(a.LatencyCount)
}
