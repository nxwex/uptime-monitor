package models

import (
	"time"
)

type CheckResult struct {
	Status       string        `json:"status"`
	StatusCode   int           `json:"status_code"`
	ResponseTime time.Duration `json:"response_time"`
	CheckedAt    time.Time     `json:"checked_at"`
}

type Monitor struct {
	ID             int           `json:"id"`
	Url            string        `json:"url"`
	Interval       time.Duration `json:"interval"`
	Status         string        `json:"status"`
	LastCheck      time.Time     `json:"last_check"`
	ResponseTimeMs time.Duration `json:"response_time_ms"`
	History        []CheckResult `json:"-"`
}

func NewMonitor(nextID int, url string, interval time.Duration) *Monitor {
	return &Monitor{
		ID:       nextID,
		Url:      url,
		Interval: interval,
		Status:   "unknown",
		History:  make([]CheckResult, 0, 3),
	}
}
