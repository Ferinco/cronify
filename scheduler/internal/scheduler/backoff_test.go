package scheduler

import (
	"testing"
	"time"
)

func TestBackoffDelayBoundsAndCap(t *testing.T) {
	cfg := DefaultBackoff()

	for retry := 1; retry <= 3; retry++ {
		base := float64(cfg.Base) * pow(cfg.Multiplier, retry-1)
		if base > float64(cfg.Cap) {
			base = float64(cfg.Cap)
		}
		lo := time.Duration(base * (1 - cfg.JitterPct))
		hi := time.Duration(base * (1 + cfg.JitterPct))

		for i := 0; i < 200; i++ {
			d := cfg.Delay(retry)
			if d < lo || d > hi {
				t.Fatalf("retry %d: delay %v outside expected [%v,%v]", retry, d, lo, hi)
			}
		}
	}
}

func TestBackoffDelayCapEnforced(t *testing.T) {
	cfg := DefaultBackoff()
	for i := 0; i < 50; i++ {
		d := cfg.Delay(20) // would be enormous without the cap
		maxAllowed := time.Duration(float64(cfg.Cap) * (1 + cfg.JitterPct))
		if d > maxAllowed {
			t.Fatalf("delay %v exceeds capped max %v", d, maxAllowed)
		}
	}
}

func pow(base float64, exp int) float64 {
	result := 1.0
	for i := 0; i < exp; i++ {
		result *= base
	}
	return result
}
