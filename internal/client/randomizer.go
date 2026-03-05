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
	if !r.enabled || baseSpeed == 0 {
		return baseSpeed
	}
	variance := float64(baseSpeed) * 0.15
	delta := (rand.Float64()*2 - 1) * variance
	result := float64(baseSpeed) + delta
	return int64(math.Max(0, result))
}

// RandomizeSpeedWithVariance adds a configurable absolute variance (bytes/sec) to a base speed.
// e.g. base=1MB/s, variance=500KB/s → result in [500KB/s, 1.5MB/s]
func (r *Randomizer) RandomizeSpeedWithVariance(baseSpeed int64, varianceBytes int64) int64 {
	if !r.enabled || baseSpeed == 0 {
		return baseSpeed
	}
	if varianceBytes <= 0 {
		return r.RandomizeSpeed(baseSpeed)
	}
	delta := (rand.Float64()*2 - 1) * float64(varianceBytes)
	result := float64(baseSpeed) + delta
	return int64(math.Max(0, result))
}

// RandomizeInterval adds variance to announce interval (seconds).
// Variance is ±10% of the interval.
func (r *Randomizer) RandomizeInterval(interval int) int {
	if !r.enabled || interval == 0 {
		return interval
	}
	variance := float64(interval) * 0.10
	delta := (rand.Float64()*2 - 1) * variance
	result := float64(interval) + delta
	return int(math.Max(60, result))
}

// RandomizePort returns a port in the typical BitTorrent range (6881-6999).
func (r *Randomizer) RandomizePort() int {
	return 6881 + rand.Intn(119)
}

// SimulateUploadDelta computes how much data was "uploaded" between two announces.
func (r *Randomizer) SimulateUploadDelta(speedBytesPerSec int64, varianceBytes int64, intervalSec int) int64 {
	speed := r.RandomizeSpeedWithVariance(speedBytesPerSec, varianceBytes)
	return speed * int64(intervalSec)
}

// SimulateDownloadDelta computes how much data was "downloaded" between two announces.
func (r *Randomizer) SimulateDownloadDelta(speedBytesPerSec int64, varianceBytes int64, intervalSec int, left int64) int64 {
	if left <= 0 {
		return 0
	}
	speed := r.RandomizeSpeedWithVariance(speedBytesPerSec, varianceBytes)
	delta := speed * int64(intervalSec)
	if delta > left {
		delta = left
	}
	return delta
}
