package orchestrator

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/google/go-github/v75/github"
)

type Project struct {
	Repository   string   `json:"repository"`
	Instructions string   `json:"instructions"`
	Verification []string `json:"verification"`
	AutoMerge    bool     `json:"auto_merge,omitempty"`
	MergeChecks  []string `json:"merge_checks,omitempty"`
}

type Run struct {
	ID        string            `json:"id"`
	Labels    map[string]string `json:"labels"`
	Lifecycle struct {
		Status struct {
			Kind string `json:"kind"`
		} `json:"status"`
	} `json:"lifecycle"`
	Timestamps struct {
		CreatedAt time.Time `json:"created_at"`
	} `json:"timestamps"`
	PullRequest *struct {
		Number int `json:"number"`
	} `json:"pull_request"`
}

func (r Run) terminal() bool {
	switch r.Lifecycle.Status.Kind {
	case "succeeded", "failed", "dead":
		return true
	}
	return false
}
func (r Run) failed() bool {
	return r.Lifecycle.Status.Kind == "failed" || r.Lifecycle.Status.Kind == "dead"
}

type Candidate struct {
	Project       Project
	Issue         *github.Issue
	Blockers      []*github.Issue
	PRs           []*github.PullRequest
	Runs          []Run
	Job           string
	Retry         bool
	FailedStage   string
	CommentExists bool
}

func (c Candidate) key() string {
	return fmt.Sprintf("%s#%d", c.Project.Repository, c.Issue.GetNumber())
}
func (c Candidate) latest() *Run {
	if len(c.Runs) == 0 {
		return nil
	}
	return &c.Runs[0]
}

type Decision struct {
	Repository string `json:"repository"`
	Issue      int    `json:"issue"`
	Action     string `json:"action"`
	Reason     string `json:"reason"`
	RunID      string `json:"run_id,omitempty"`
}

func references(pr *github.PullRequest, repo string, issue int) bool {
	text := pr.GetTitle() + "\n" + pr.GetBody()
	patterns := []string{
		fmt.Sprintf(`https://github\.com/%s/issues/%d(?:\D|$)`, regexp.QuoteMeta(repo), issue),
		fmt.Sprintf(`(?:^|[^\w/])%s#%d\b`, regexp.QuoteMeta(repo), issue),
		fmt.Sprintf(`(?:^|[^\w/])#%d\b`, issue),
	}
	for _, pattern := range patterns {
		if regexp.MustCompile(pattern).MatchString(text) {
			return true
		}
	}
	return false
}

// Decide has no I/O. Input snapshots must be complete; API failures never become empty lists.
func Decide(c Candidate) Decision {
	d := Decision{Repository: c.Project.Repository, Issue: c.Issue.GetNumber(), Action: "wait"}
	if c.Issue.GetState() != "open" || c.Issue.IsPullRequest() {
		d.Reason = "issue_closed"
		return d
	}
	ready := false
	for _, label := range c.Issue.Labels {
		if strings.HasPrefix(label.GetName(), "wayfinder:") {
			d.Reason = "planning"
			return d
		}
		if label.GetName() == "ready-for-agent" || label.GetName() == "ready-for-test" {
			ready = true
		}
	}
	if !ready {
		d.Reason = "not_ready"
		return d
	}
	for _, b := range c.Blockers {
		if b.GetState() != "closed" {
			d.Reason = "blocked"
			return d
		}
	}
	latest := c.latest()
	for _, pr := range c.PRs {
		associated := references(pr, c.Project.Repository, c.Issue.GetNumber())
		own := latest != nil && ((latest.PullRequest != nil && latest.PullRequest.Number == pr.GetNumber()) ||
			(len(c.Job) > 0 && pr.GetHead().GetRef() == fmt.Sprintf("fabro/issue-%d-%s", c.Issue.GetNumber(), c.Job) && strings.EqualFold(pr.GetHead().GetRepo().GetFullName(), c.Project.Repository)))
		if pr.GetState() == "open" && (associated || own) {
			d.Reason = "open_pr"
			return d
		}
		if own && pr.GetMergedAt().IsZero() {
			d.Reason = "closed_unmerged_pr"
			return d
		}
	}
	for _, r := range c.Runs {
		if !r.terminal() {
			d.RunID = r.ID
			if r.Lifecycle.Status.Kind == "submitted" && latest != nil && r.ID == latest.ID {
				d.Action = "resume"
				d.Reason = "submitted"
			} else {
				d.Reason = "active_run"
			}
			return d
		}
	}
	if latest != nil {
		d.RunID = latest.ID
		if latest.failed() && !c.Retry {
			d.Reason = "retry_required"
			return d
		}
		if latest.Lifecycle.Status.Kind == "succeeded" {
			merged := false
			for _, pr := range c.PRs {
				own := (latest.PullRequest != nil && latest.PullRequest.Number == pr.GetNumber()) ||
					(len(c.Job) > 0 && pr.GetHead().GetRef() == fmt.Sprintf("fabro/issue-%d-%s", c.Issue.GetNumber(), c.Job) && strings.EqualFold(pr.GetHead().GetRepo().GetFullName(), c.Project.Repository))
				if own && !pr.GetMergedAt().IsZero() {
					merged = true
				}
			}
			if !merged {
				d.Reason = "pr_not_reconciled"
				return d
			}
		}
	}
	d.Action = "start"
	d.Reason = "eligible"
	return d
}

func Plan(candidates []Candidate, runs []Run) []Decision {
	ordered := append([]Candidate(nil), candidates...)
	sort.Slice(ordered, func(i, j int) bool {
		a, b := ordered[i].Issue.GetCreatedAt(), ordered[j].Issue.GetCreatedAt()
		if a.Equal(b) {
			return ordered[i].key() < ordered[j].key()
		}
		return a.Before(b.Time)
	})
	active := 0
	for _, r := range runs {
		if !r.terminal() {
			active++
		}
	}
	result := make([]Decision, 0, len(ordered))
	for _, c := range ordered {
		d := Decide(c)
		if d.Action == "start" {
			if active > 0 {
				d.Action = "wait"
				d.Reason = "capacity"
			} else {
				active++
			}
		}
		if d.Action == "resume" && active > 1 {
			d.Action = "wait"
			d.Reason = "capacity"
		}
		result = append(result, d)
	}
	return result
}
