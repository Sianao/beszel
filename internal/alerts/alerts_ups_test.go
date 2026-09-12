package alerts

import (
	"testing"

	"github.com/henrygd/beszel/internal/entities/system"
)

func TestUPSAlertValues(t *testing.T) {
	device := system.UPSStats{Online: true, Status: "OB LB", Metrics: map[string]float64{"battery.charge": 0, "battery.runtime": 120, "ups.load": 90}}
	for _, tc := range []struct {
		name            string
		threshold, want float64
	}{
		{"UPSCharge", 20, 0}, {"UPSRuntime", 5, 2}, {"UPSLoad", 80, 90},
		{"UPSOnBattery", 0, 1}, {"UPSFault", 0, 1}, {"UPSDisconnected", 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			value, ok := upsAlertValue(tc.name, map[string]system.UPSStats{"ups": device}, tc.threshold)
			if !ok || value != tc.want {
				t.Fatalf("got %v, %v", value, ok)
			}
			if _, ok := upsAlertValue(tc.name, nil, tc.threshold); ok {
				t.Fatal("missing UPS should be skipped")
			}
		})
	}
	devices := map[string]system.UPSStats{"first": device, "missing": {}}
	if value, ok := upsAlertValue("UPSCharge", devices, 20); !ok || value != 0 {
		t.Fatal("known low charge must trigger despite another missing UPS")
	}
	if value, ok := upsAlertValue("UPSDisconnected", devices, 0); !ok || value != 1 {
		t.Fatal("missing UPS must trigger disconnect alarm")
	}
	device.Status = "OL"
	device.Metrics["battery.charge"] = 100
	devices["first"] = device
	for _, name := range []string{"UPSCharge", "UPSOnBattery", "UPSFault"} {
		if _, ok := upsAlertValue(name, devices, 20); ok {
			t.Fatalf("%s falsely recovered with missing UPS", name)
		}
	}
	delete(devices, "missing")
	if value, ok := upsAlertValue("UPSCharge", devices, 20); !ok || value != 100 {
		t.Fatal("charge recovery not recognized")
	}
	if value, ok := upsAlertValue("UPSOnBattery", devices, 0); !ok || value != 0 {
		t.Fatal("utility power recovery not recognized")
	}
	device.Status = "NOTOB NOTLB"
	devices["first"] = device
	if value, _ := upsAlertValue("UPSOnBattery", devices, 0); value != 0 {
		t.Fatal("status matched a substring")
	}
}
