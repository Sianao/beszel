package system

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/fxamacker/cbor/v2"
)

func TestUPSWireRoundTrip(t *testing.T) {
	device := UPSStats{Name: "ups", Online: true, Status: "OB LB", Updated: 100, Metrics: map[string]float64{"battery.charge": 0}}
	want := CombinedData{Stats: Stats{UPS: map[string]UPSStats{"ups": device}}, Info: Info{UPS: map[string]UPSStats{"ups": device}}}
	for _, codec := range []struct {
		name      string
		marshal   func(any) ([]byte, error)
		unmarshal func([]byte, any) error
	}{
		{"json", json.Marshal, json.Unmarshal}, {"cbor", cbor.Marshal, cbor.Unmarshal},
	} {
		t.Run(codec.name, func(t *testing.T) {
			data, err := codec.marshal(want)
			if err != nil {
				t.Fatal(err)
			}
			var got CombinedData
			if err := codec.unmarshal(data, &got); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got.Stats.UPS, want.Stats.UPS) || !reflect.DeepEqual(got.Info.UPS, want.Info.UPS) {
				t.Fatalf("round trip: %#v", got)
			}
		})
	}
}
