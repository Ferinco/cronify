package dashboard

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestLoginThrottlePenalizeGrowsAndCaps(t *testing.T) {
	th := &loginThrottle{}
	want := []time.Duration{
		250 * time.Millisecond,
		500 * time.Millisecond,
		1 * time.Second,
		2 * time.Second,
		4 * time.Second,
		4 * time.Second, // stays capped past the 5th failure
	}
	for i, w := range want {
		if got := th.penalize("1.2.3.4"); got != w {
			t.Fatalf("attempt %d: penalize() = %v, want %v", i+1, got, w)
		}
	}
}

func TestLoginThrottleClearResetsDelay(t *testing.T) {
	th := &loginThrottle{}
	th.penalize("1.2.3.4")
	th.penalize("1.2.3.4")
	if got := th.penalize("1.2.3.4"); got != time.Second {
		t.Fatalf("expected 1s after 3 failures, got %v", got)
	}

	th.clear("1.2.3.4")

	if got := th.penalize("1.2.3.4"); got != 250*time.Millisecond {
		t.Fatalf("expected delay to reset to base after clear, got %v", got)
	}
}

func TestLoginThrottleKeysAreIndependent(t *testing.T) {
	th := &loginThrottle{}
	th.penalize("1.2.3.4")
	th.penalize("1.2.3.4")
	th.penalize("1.2.3.4") // key "a" now at 1s

	if got := th.penalize("5.6.7.8"); got != 250*time.Millisecond {
		t.Fatalf("expected an unrelated key to start at base delay, got %v", got)
	}
}

func TestLoginThrottleStaleAttemptResets(t *testing.T) {
	th := &loginThrottle{}
	th.penalize("1.2.3.4")
	th.penalize("1.2.3.4")

	// Simulate the bucket having gone quiet past throttleResetAfter.
	th.attempts["1.2.3.4"].last = time.Now().Add(-throttleResetAfter - time.Second)

	if got := th.penalize("1.2.3.4"); got != 250*time.Millisecond {
		t.Fatalf("expected a stale bucket to reset to base delay, got %v", got)
	}
}

func TestClientKeyStripsPort(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "203.0.113.7:54321"
	if got := clientKey(req); got != "203.0.113.7" {
		t.Fatalf("clientKey() = %q, want %q", got, "203.0.113.7")
	}
}

func TestClientKeyFallsBackWhenUnparseable(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "not-a-host-port"
	if got := clientKey(req); got != "not-a-host-port" {
		t.Fatalf("clientKey() = %q, want the raw RemoteAddr as a fallback", got)
	}
}

// TestWithBasicAuthThrottlesRepeatedFailures is an end-to-end check (through
// the real HTTP handler, not loginThrottle directly) that a wrong password
// actually incurs the delay — httptest.NewRequest gives every request in
// this test the same default RemoteAddr, so these share one throttle bucket.
func TestWithBasicAuthThrottlesRepeatedFailures(t *testing.T) {
	h, _ := newTestHandlers(t)
	mux := NewMux(h)

	start := time.Now()
	doRequestAs(t, mux, "GET", "/", "admin", "wrong", nil, "")
	first := time.Since(start)

	start = time.Now()
	doRequestAs(t, mux, "GET", "/", "admin", "wrong", nil, "")
	second := time.Since(start)

	if first < 250*time.Millisecond {
		t.Fatalf("expected first failure to be delayed at least 250ms, took %v", first)
	}
	if second < 500*time.Millisecond {
		t.Fatalf("expected second failure to be delayed at least 500ms, took %v", second)
	}

	// A correct login clears the penalty for this key.
	doRequest(t, mux, "GET", "/", true, nil, "")

	start = time.Now()
	doRequestAs(t, mux, "GET", "/", "admin", "wrong", nil, "")
	afterSuccess := time.Since(start)
	if afterSuccess >= 500*time.Millisecond {
		t.Fatalf("expected delay to reset to base after a successful login, took %v", afterSuccess)
	}
}
