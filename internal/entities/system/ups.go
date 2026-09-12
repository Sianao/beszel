package system

// UPSStats is a NUT snapshot. Missing measurements are absent, never zero-filled.
// Updated is the last successful sample's Unix timestamp, even on communication failure.
type UPSStats struct {
	Name    string             `json:"name" cbor:"0,keyasint"`
	Model   string             `json:"model,omitempty" cbor:"1,keyasint,omitempty"`
	Status  string             `json:"status,omitempty" cbor:"2,keyasint,omitempty"`
	Online  bool               `json:"online" cbor:"3,keyasint"`
	Updated int64              `json:"updated" cbor:"4,keyasint"`
	Metrics map[string]float64 `json:"metrics,omitempty" cbor:"5,keyasint,omitempty"`
	Details map[string]string  `json:"details,omitempty" cbor:"6,keyasint,omitempty"`
	// Aggregate-only fields retain extrema, weights, and observed status flags.
	Min    map[string]float64 `json:"min,omitempty" cbor:"-"`
	Max    map[string]float64 `json:"max,omitempty" cbor:"-"`
	Counts map[string]uint64  `json:"counts,omitempty" cbor:"-"`
	States []string           `json:"states,omitempty" cbor:"-"`
}
