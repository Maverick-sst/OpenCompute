package batch

import "fmt"

// Range is a half-open interval [Start, End) in the input item sequence.
type Range struct {
	Index int   `json:"index"`
	Start int64 `json:"start"`
	End   int64 `json:"end"`
}

// Partition splits total items into deterministic, contiguous, non-overlapping
// ranges. Every range except possibly the last has at most batchSize items.
func Partition(total, batchSize int64) ([]Range, error) {
	if total <= 0 {
		return nil, fmt.Errorf("total must be greater than zero")
	}
	if batchSize <= 0 {
		return nil, fmt.Errorf("batch size must be greater than zero")
	}

	count := (total + batchSize - 1) / batchSize
	ranges := make([]Range, 0, count)
	for start, index := int64(0), 0; start < total; index++ {
		end := start + batchSize
		if end > total {
			end = total
		}
		ranges = append(ranges, Range{Index: index, Start: start, End: end})
		start = end
	}
	return ranges, nil
}
