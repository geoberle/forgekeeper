package orchestrator

import (
	"testing"
	"time"
)

func TestValidateOptions(t *testing.T) {
	for _, tt := range []struct {
		name, url string
		issue     int
		interval  time.Duration
		valid     bool
	}{
		{"loopback", "http://127.0.0.1:32276", 0, time.Minute, true},
		{"tls", "https://fabro.example", 0, time.Minute, true},
		{"credentials", "https://user:secret@fabro.example", 0, time.Minute, false},
		{"plaintext remote", "http://fabro.example", 0, time.Minute, false},
		{"query secret", "https://fabro.example?token=secret", 0, time.Minute, false},
		{"missing repo", "http://localhost:32276", 8, time.Minute, false},
		{"busy loop", "http://localhost:32276", 0, 0, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			raw := DefaultOptions()
			raw.RetryUser = "retry-reviewer"
			raw.FabroURL = tt.url
			raw.Issue = tt.issue
			raw.Interval = tt.interval
			valid, err := raw.Validate()
			if (err == nil) != tt.valid {
				t.Fatalf("got %v", err)
			}
			if err == nil {
				raw.Interval = 0
				if valid.raw.Interval != tt.interval {
					t.Fatal("validated values alias raw options")
				}
			}
		})
	}
}
func TestPublicStageNameDoesNotContainExceptionData(t *testing.T) {
	state := RunState{}
	state.Stages = map[string]RunStage{"synthetic-secret@1": {State: "failed"}}
	if got := safeStage(state); got != "workflow" {
		t.Fatalf("unsafe stage: %s", got)
	}
}

func TestClusterHTTPOptIn(t *testing.T) {
	for _, tt := range []struct {
		name, api, web string
		allow, valid   bool
	}{
		{"cluster opted in", "http://factory-fabro:32276", "http://localhost:32277", true, true},
		{"cluster default denied", "http://factory-fabro:32276", "http://localhost:32277", false, false},
		{"web still requires TLS", "http://factory-fabro:32276", "http://public.example", true, false},
		{"credentials still denied", "http://user:secret@factory-fabro:32276", "http://localhost:32277", true, false},
		{"other protocols denied", "ftp://factory-fabro", "http://localhost:32277", true, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			raw := DefaultOptions()
			raw.RetryUser = "retry-reviewer"
			raw.FabroURL, raw.WebURL, raw.AllowHTTP = tt.api, tt.web, tt.allow
			_, err := raw.Validate()
			if (err == nil) != tt.valid {
				t.Fatalf("unexpected validation result: %v", err)
			}
		})
	}
}

func TestRetryUserValidation(t *testing.T) {
	for _, tt := range []struct {
		name  string
		valid bool
	}{
		{"", false}, {"reviewer", true}, {"other-reviewer", true},
		{"@reviewer", false}, {"-reviewer", false}, {"reviewer--two", false},
		{"reviewer\nmalicious", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			raw := DefaultOptions()
			raw.RetryUser = tt.name
			_, err := raw.Validate()
			if (err == nil) != tt.valid {
				t.Fatalf("unexpected validation: %v", err)
			}
		})
	}
}

func TestConcurrentRunLimit(t *testing.T) {
	for _, limit := range []int{-1, 0, 1, 2} {
		raw := DefaultOptions()
		raw.RetryUser = "reviewer"
		raw.MaxConcurrentRuns = limit
		_, err := raw.Validate()
		if (err == nil) != (limit > 0) {
			t.Errorf("limit=%d: %v", limit, err)
		}
	}
}
