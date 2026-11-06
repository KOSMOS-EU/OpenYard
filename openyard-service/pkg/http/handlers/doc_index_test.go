package handlers

import (
	"testing"
)

func TestIsGUID(t *testing.T) {
	cases := map[string]bool{
		"6f1c2b9e-3a4d-4e5f-8a9b-0c1d2e3f4a5b": true,
		"6F1C2B9E-3A4D-4E5F-8A9B-0C1D2E3F4A5B": true,
		"6f1c2b9e-3a4d-4e5f-8a9b-0c1d2e3f4a5":  false,
		"6f1c2b9e_3a4d-4e5f-8a9b-0c1d2e3f4a5b": false,
		"node-xyz-123":                         false,
	}
	for value, want := range cases {
		if got := isGUID(value); got != want {
			t.Errorf("isGUID(%q) = %v, want %v", value, got, want)
		}
	}
}
