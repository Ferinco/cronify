package scheduler

import (
	"math"
	"math/rand"
	"time"
)

// BackoffConfig is CLAUDE.md's retry policy, numerically: exponential, base 30s,
// x2 per attempt, +-20% jitter, capped at 5 min.
type BackoffConfig struct {
	Base       time.Duration
	Multiplier float64
	JitterPct  float64
	Cap        time.Duration
}

func DefaultBackoff() BackoffConfig {
	return BackoffConfig{
		Base:       30 * time.Second,
		Multiplier: 2,
		JitterPct:  0.20,
		Cap:        5 * time.Minute,
	}
}

// Delay returns the wait before retryNumber (1 = first retry, ~Base later;
// 2 = second retry, ~Base*Multiplier later; ...), jittered and capped.
func (c BackoffConfig) Delay(retryNumber int) time.Duration {
	d := float64(c.Base) * math.Pow(c.Multiplier, float64(retryNumber-1))
	if cap := float64(c.Cap); d > cap {
		d = cap
	}
	jitter := 1 + (rand.Float64()*2-1)*c.JitterPct
	return time.Duration(d * jitter)
}
