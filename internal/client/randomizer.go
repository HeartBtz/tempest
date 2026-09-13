package client

import (
	"math"
	"math/rand"
)

type Randomizer struct {
	enabled bool
}

func NewRandomizer(enabled bool) *Randomizer {
	return &Randomizer{enabled: enabled}
}

// RandomizeSpeed adds realistic variance to a base speed (bytes/sec).
// Variance is ±15% of the base speed.
func (r *Randomizer) RandomizeSpeed(baseSpeed int64) int64 {
	if baseSpeed <= 0 {
		return 0
	}
	if !r.enabled {
		return baseSpeed
	}
	variance := float64(baseSpeed) * 0.15
	delta := (rand.Float64()*2 - 1) * variance
	result := float64(baseSpeed) + delta
	return nonNegativeInt64(result)
}

// RandomizeSpeedWithVariance adds a configurable absolute variance (bytes/sec) to a base speed.
// e.g. base=1MB/s, variance=500KB/s → result in [500KB/s, 1.5MB/s]
func (r *Randomizer) RandomizeSpeedWithVariance(baseSpeed int64, varianceBytes int64) int64 {
	if baseSpeed <= 0 {
		return 0
	}
	if !r.enabled {
		return baseSpeed
	}
	if varianceBytes <= 0 {
		return r.RandomizeSpeed(baseSpeed)
	}
	delta := (rand.Float64()*2 - 1) * float64(varianceBytes)
	result := float64(baseSpeed) + delta
	return nonNegativeInt64(result)
}

// RandomizeInterval adds variance to announce interval (seconds).
// Variance is ±10% of the interval.
func (r *Randomizer) RandomizeInterval(interval int) int {
	if interval <= 0 {
		return 0
	}
	if !r.enabled {
		return interval
	}
	variance := float64(interval) * 0.10
	delta := (rand.Float64()*2 - 1) * variance
	result := float64(interval) + delta
	return int(math.Max(60, result))
}

// SimulateUploadDelta computes how much data was "uploaded" between two announces.
func (r *Randomizer) SimulateUploadDelta(speedBytesPerSec int64, varianceBytes int64, intervalSec int) int64 {
	speed := r.RandomizeSpeedWithVariance(speedBytesPerSec, varianceBytes)
	return transferDelta(speed, intervalSec)
}

// SimulateDownloadDelta computes how much data was "downloaded" between two announces.
func (r *Randomizer) SimulateDownloadDelta(speedBytesPerSec int64, varianceBytes int64, intervalSec int, left int64) int64 {
	if left <= 0 {
		return 0
	}
	speed := r.RandomizeSpeedWithVariance(speedBytesPerSec, varianceBytes)
	delta := transferDelta(speed, intervalSec)
	if delta > left {
		delta = left
	}
	return delta
}

func nonNegativeInt64(value float64) int64 {
	if value <= 0 || math.IsNaN(value) {
		return 0
	}
	if value >= float64(math.MaxInt64) {
		return math.MaxInt64
	}
	return int64(value)
}

func transferDelta(speed int64, intervalSec int) int64 {
	if speed <= 0 || intervalSec <= 0 {
		return 0
	}
	interval := int64(intervalSec)
	if speed > math.MaxInt64/interval {
		return math.MaxInt64
	}
	return speed * interval
}
