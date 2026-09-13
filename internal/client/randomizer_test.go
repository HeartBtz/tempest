package client

import (
	"math"
	"testing"
)

func TestTransferDeltaSaturatesOnOverflow(t *testing.T) {
	randomizer := NewRandomizer(false)
	if got := randomizer.SimulateUploadDelta(math.MaxInt64, 0, 2); got != math.MaxInt64 {
		t.Fatalf("upload delta = %d, want %d", got, int64(math.MaxInt64))
	}
	if got := randomizer.SimulateDownloadDelta(math.MaxInt64, 0, 2, math.MaxInt64); got != math.MaxInt64 {
		t.Fatalf("download delta = %d, want %d", got, int64(math.MaxInt64))
	}
}

func TestTransferDeltaRejectsNonPositiveInputs(t *testing.T) {
	randomizer := NewRandomizer(false)
	for _, test := range []struct {
		name     string
		speed    int64
		interval int
	}{
		{name: "negative speed", speed: -1, interval: 10},
		{name: "zero interval", speed: 10, interval: 0},
		{name: "negative interval", speed: 10, interval: -1},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := randomizer.SimulateUploadDelta(test.speed, 0, test.interval); got != 0 {
				t.Fatalf("delta = %d, want 0", got)
			}
		})
	}
}

func TestRandomizedSpeedSaturatesAtMaxInt64(t *testing.T) {
	randomizer := NewRandomizer(true)
	for range 100 {
		got := randomizer.RandomizeSpeedWithVariance(math.MaxInt64, math.MaxInt64)
		if got < 0 {
			t.Fatalf("randomized speed overflowed: %d", got)
		}
	}
}
