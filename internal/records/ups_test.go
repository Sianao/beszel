package records

import (
	"reflect"
	"testing"

	"github.com/henrygd/beszel/internal/entities/system"
)

func TestUPSAggregation(t *testing.T) {
	record := func(value float64, status string) system.Stats {
		return system.Stats{UPS: map[string]system.UPSStats{"ups": {Name: "ups", Online: true, Status: status, Metrics: map[string]float64{"battery.charge": value}}}}
	}
	a := record(0, "OB LB")
	a.UPS["ups"].Metrics["ups.load"] = 0
	b := record(30, "OL")
	c := record(90, "OL")
	disconnected := system.Stats{UPS: map[string]system.UPSStats{"ups": {Name: "ups", Online: false}}}
	first := AverageSystemStatsSlice([]system.Stats{a, b, disconnected})
	result := AverageSystemStatsSlice([]system.Stats{first, c, disconnected}).UPS["ups"]
	if result.Metrics["battery.charge"] != 40 || result.Counts["battery.charge"] != 3 {
		t.Fatalf("weighted average: %#v", result)
	}
	if result.Min["battery.charge"] != 0 || result.Max["battery.charge"] != 90 || result.Online {
		t.Fatalf("extrema or status: %#v", result)
	}
	if result.Counts["ups.load"] != 1 || result.Metrics["ups.load"] != 0 {
		t.Fatalf("missing values treated as zero: %#v", result)
	}
	if !reflect.DeepEqual(result.States, []string{"LB", "OB", "OL"}) {
		t.Fatalf("lost outage flags: %#v", result.States)
	}
	if a.UPS["ups"].Metrics["battery.charge"] != 0 {
		t.Fatal("input mutated")
	}
	if AverageSystemStatsSlice([]system.Stats{{}}).UPS != nil {
		t.Fatal("UPS appeared without data")
	}
}
