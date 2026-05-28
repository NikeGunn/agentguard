package pipeline

import "encoding/json"

// detailJSON renders a stage detail map as compact JSON for storage.
// Returns an empty string on marshal failure rather than burdening every
// caller with error handling — detail is diagnostic, never load-bearing.
func detailJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}
