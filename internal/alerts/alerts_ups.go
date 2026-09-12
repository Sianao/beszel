package alerts

import (
	"strings"

	"github.com/henrygd/beszel/internal/entities/system"
)

func upsBinaryAlert(name string) bool {
	return name == "UPSOnBattery" || name == "UPSDisconnected" || name == "UPSFault"
}

// UPS rules apply to all UPS devices attached to a system, matching the existing
// per-system alert model. Unknown readings must not resolve an active alarm.
func upsAlertValue(name string, devices map[string]system.UPSStats, threshold float64) (float64, bool) {
	if len(devices) == 0 {
		return 0, false
	}
	var value float64
	valid, unknown := false, false
	for _, device := range devices {
		if name == "UPSDisconnected" {
			valid = true
			if !device.Online {
				value = 1
			}
			continue
		}
		if !device.Online {
			unknown = true
			continue
		}
		flags := " " + strings.Join(strings.Fields(device.Status), " ") + " "
		if upsBinaryAlert(name) {
			if device.Status == "" {
				unknown = true
				continue
			}
			valid = true
			if name == "UPSOnBattery" && strings.Contains(flags, " OB ") {
				value = 1
			}
			if name == "UPSFault" {
				for _, flag := range []string{"LB", "OVER", "RB", "FSD", "ALARM"} {
					if strings.Contains(flags, " "+flag+" ") {
						value = 1
					}
				}
			}
			continue
		}
		key := map[string]string{"UPSCharge": "battery.charge", "UPSRuntime": "battery.runtime", "UPSLoad": "ups.load"}[name]
		if key == "" {
			return 0, false
		}
		n, ok := device.Metrics[key]
		if !ok {
			unknown = true
			continue
		}
		if name == "UPSRuntime" {
			n /= 60
		}
		if !valid || (isLowAlert(name) && n < value) || (!isLowAlert(name) && n > value) {
			value = n
		}
		valid = true
	}
	// Known anomalies may trigger even if another UPS is missing. Recovery requires
	// complete data, since the missing device could still be in an alarm state.
	anomaly := valid && ((upsBinaryAlert(name) && value > 0) ||
		(!upsBinaryAlert(name) && ((isLowAlert(name) && value < threshold) || (!isLowAlert(name) && value > threshold))))
	if unknown && !anomaly {
		return 0, false
	}
	return value, valid
}
