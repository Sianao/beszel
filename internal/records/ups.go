package records

import (
	"sort"
	"strings"

	"github.com/henrygd/beszel/internal/entities/system"
)

// averageUPS counts each metric independently. Recursive rollups preserve weights
// and extrema; the newest snapshot supplies communication status and metadata.
func averageUPS(records []system.Stats) map[string]system.UPSStats {
	var result map[string]system.UPSStats
	for _, record := range records {
		for id, sample := range record.UPS {
			if result == nil {
				result = make(map[string]system.UPSStats)
			}
			value, exists := result[id]
			if !exists {
				value = system.UPSStats{Metrics: make(map[string]float64), Min: make(map[string]float64), Max: make(map[string]float64), Counts: make(map[string]uint64)}
			}
			value.Name, value.Model, value.Status = sample.Name, sample.Model, sample.Status
			value.Online, value.Updated = sample.Online, sample.Updated
			flags := sample.States
			if flags == nil {
				flags = strings.Fields(sample.Status)
			}
			for _, flag := range flags {
				found := false
				for _, existing := range value.States {
					if existing == flag {
						found = true
						break
					}
				}
				if !found {
					value.States = append(value.States, flag)
				}
			}
			for key, metric := range sample.Metrics {
				// An aggregate may end with a disconnected sample but still contain earlier valid readings.
				if !sample.Online && sample.Counts[key] == 0 {
					continue
				}
				count := sample.Counts[key]
				if count == 0 {
					count = 1
				}
				low, ok := sample.Min[key]
				if !ok {
					low = metric
				}
				high, ok := sample.Max[key]
				if !ok {
					high = metric
				}
				if value.Counts[key] == 0 || low < value.Min[key] {
					value.Min[key] = low
				}
				if value.Counts[key] == 0 || high > value.Max[key] {
					value.Max[key] = high
				}
				value.Metrics[key] += metric * float64(count)
				value.Counts[key] += count
			}
			result[id] = value
		}
	}
	for id, value := range result {
		for key, total := range value.Metrics {
			value.Metrics[key] = total / float64(value.Counts[key])
		}
		sort.Strings(value.States)
		result[id] = value
	}
	return result
}
