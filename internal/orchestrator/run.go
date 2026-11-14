package orchestrator

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"
)

var jobPattern = regexp.MustCompile(`^[a-f0-9]{32}$`)

func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return errors.New("cannot encode workflow input")
	}
	if err := os.WriteFile(path, append(data, '\n'), 0600); err != nil {
		return errors.New("cannot write workflow input")
	}
	return nil
}
func (o *Options) launch(ctx context.Context, c Candidate) error {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return errors.New("cannot allocate workspace")
	}
	job := hex.EncodeToString(id[:])
	dir := filepath.Join(o.raw.Storage, job)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return errors.New("cannot prepare workspace")
	}
	owner, repo, _ := strings.Cut(c.Project.Repository, "/")
	metadata, _, err := o.GitHub.Repositories.Get(ctx, owner, repo)
	if err != nil {
		return errors.New("GitHub repository lookup failed")
	}
	base := metadata.GetDefaultBranch()
	if len(base) == 0 {
		return errors.New("repository has no default branch")
	}
	source := filepath.Join(dir, "source")
	if _, err := o.Execute(ctx, "git", []string{"clone", "--single-branch", "--branch", base, "https://github.com/" + c.Project.Repository + ".git", source}, ""); err != nil {
		return errors.New("checkout failed")
	}
	head, err := o.Execute(ctx, "git", []string{"rev-parse", "HEAD"}, source)
	if err != nil {
		return errors.New("cannot resolve checkout HEAD")
	}
	config := struct {
		Project
		Issue   int    `json:"issue"`
		Base    string `json:"base"`
		BaseSHA string `json:"base_sha"`
	}{c.Project, c.Issue.GetNumber(), base, strings.TrimSpace(string(head))}
	if err := writeJSON(filepath.Join(dir, "config.json"), config); err != nil {
		return err
	}
	// Fabro CLI owns packaging and target resolution. Never persist raw CLI output.
	output, err := o.Execute(ctx, "fabro", []string{"create", o.raw.Workflow, "--json", "--server", o.raw.FabroURL, "--environment", "local", "--target-from", source, "-I", "repository=" + c.Project.Repository, "-I", fmt.Sprintf("issue=%d", c.Issue.GetNumber()), "-I", "job=" + job, "--label", "github_issue=" + c.key()}, "")
	if err != nil {
		return errors.New("fabro create not confirmed; reconcile on next poll")
	}
	var created struct {
		RunID string `json:"run_id"`
	}
	if json.Unmarshal(output, &created) != nil || !runPattern.MatchString(created.RunID) {
		return errors.New("fabro create not confirmed; reconcile on next poll")
	}
	if err := writeJSON(filepath.Join(dir, "run.json"), map[string]string{"run_id": created.RunID}); err != nil {
		return err
	}
	if err := o.Fabro.Start(ctx, created.RunID); err != nil {
		return err
	}
	o.Log.Info("Run started", "issue", c.key(), "run", created.RunID)
	return nil
}
func (o *Options) Cycle(ctx context.Context) ([]Decision, error) {
	candidates, runs, err := o.snapshot(ctx)
	if err != nil {
		return nil, err
	}
	feedbackDecision, err := o.feedbackCycle(ctx, runs)
	if err != nil {
		return nil, err
	}
	if feedbackDecision != nil {
		return []Decision{*feedbackDecision}, nil
	}
	decisions := Plan(candidates, runs)
	if o.raw.Preview {
		return decisions, nil
	}
	for _, c := range candidates {
		if c.latest() != nil && c.latest().failed() && len(c.latest().Labels["feedback_cursor"]) == 0 && !c.CommentExists {
			if err := o.postFailure(ctx, c); err != nil {
				return decisions, err
			}
		}
	}
	for _, d := range decisions {
		if d.Action != "start" && d.Action != "resume" {
			continue
		}
		// Refresh all conditions immediately before a mutation; no stale PR/label decision.
		fresh, all, err := o.snapshot(ctx)
		if err != nil {
			return decisions, err
		}
		for _, now := range Plan(fresh, all) {
			if now.Repository != d.Repository || now.Issue != d.Issue || now.Action != d.Action {
				continue
			}
			for _, c := range fresh {
				if c.Project.Repository != now.Repository || c.Issue.GetNumber() != now.Issue {
					continue
				}
				if now.Action == "resume" {
					if len(c.Job) == 0 {
						return decisions, errors.New("submitted run has no workspace identity")
					}
					if err := writeJSON(filepath.Join(o.raw.Storage, c.Job, "run.json"), map[string]string{"run_id": now.RunID}); err != nil {
						return decisions, err
					}
					return decisions, o.Fabro.Start(ctx, now.RunID)
				}
				return decisions, o.launch(ctx, c)
			}
		}
	}
	return decisions, nil
}
func (o *Options) Run(ctx context.Context) error {
	// Ephemeral process lock only: no persistent orchestration state.
	lock, err := os.OpenFile(filepath.Join(os.TempDir(), "fabro-orchestrator.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return errors.New("cannot open orchestrator lock")
	}
	defer func() { _ = lock.Close() }()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return errors.New("another orchestrator is running")
	}
	defer func() { _ = syscall.Flock(int(lock.Fd()), syscall.LOCK_UN) }()
	ticker := time.NewTicker(o.raw.Interval)
	defer ticker.Stop()
	for {
		decisions, err := o.Cycle(ctx)
		if err != nil {
			o.Log.Error(err, "Reconciliation failed")
		} else {
			for _, d := range decisions {
				o.Log.Info("Issue decision", "repository", d.Repository, "issue", d.Issue, "action", d.Action, "reason", d.Reason)
			}
		}
		if o.raw.Once {
			return err
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}
