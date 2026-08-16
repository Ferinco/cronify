// Package scheduler drives the tick loop, retry/backoff, and per-job HTTP firing.
package scheduler

import (
	"time"

	"github.com/robfig/cron/v3"
)

// Schedule computes the next occurrence after a given time. Wraps robfig/cron's
// parser so tests can inject a fake Schedule (e.g. "next call returns now") without
// waiting on real wall-clock minute boundaries.
type Schedule interface {
	Next(from time.Time) time.Time
}

// ParseSchedule parses a standard 5-field cron expression (e.g. "0 9 * * *").
func ParseSchedule(spec string) (Schedule, error) {
	return cron.ParseStandard(spec)
}

// Next is a store.ScheduleFunc-shaped adapter: parse spec, compute the next run after from.
func Next(spec string, from time.Time) (time.Time, error) {
	sched, err := ParseSchedule(spec)
	if err != nil {
		return time.Time{}, err
	}
	return sched.Next(from), nil
}
