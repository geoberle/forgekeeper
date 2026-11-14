package orchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/go-github/v75/github"
)

const riskContext = "forgekeeper/low-risk"

type mergeEvidence struct {
	Head        string `json:"head"`
	Tree        string `json:"tree"`
	PublishHead string `json:"publish_head"`
	Mode        string `json:"mode"`
	Reviewers   map[string]struct {
		Status   string `json:"status"`
		Risk     string `json:"risk"`
		Reason   string `json:"risk_reason"`
		Findings []struct {
			Severity string `json:"severity"`
		} `json:"findings"`
	} `json:"reviewers"`
}

// Missing old-run artifacts and unknown fields never imply merge approval.
func readMergeEvidence(dir, name string, out any) bool {
	data, err := os.ReadFile(filepath.Join(dir, name+".json"))
	return err == nil && json.Unmarshal(data, out) == nil
}

func lowRisk(review mergeEvidence) bool {
	if len(review.Reviewers) != 2 {
		return false
	}
	for _, name := range []string{"steve", "david"} {
		r, ok := review.Reviewers[name]
		if !ok || r.Status != "complete" || r.Risk != "low" || len(strings.TrimSpace(r.Reason)) == 0 || r.Findings == nil {
			return false
		}
		for _, f := range r.Findings {
			if f.Severity != "suggestion" {
				return false
			}
		}
	}
	return true
}

func ownMergeablePR(pr *github.PullRequest, bot int64, repository string) bool {
	return bot != 0 && pr.GetUser().GetID() == bot && pr.GetState() == "open" && !pr.GetMerged() && !pr.GetDraft() && pr.GetMergeable() && (pr.GetMergeableState() == "clean" || pr.GetMergeableState() == "blocked") && len(pr.GetHead().GetSHA()) > 0 && strings.EqualFold(pr.GetHead().GetRepo().GetFullName(), repository) && strings.EqualFold(pr.GetBase().GetRepo().GetFullName(), repository)
}

func changesRequested(f feedback) bool {
	latest := map[int64]*github.PullRequestReview{}
	for _, r := range f.Reviews {
		if r.GetState() == "PENDING" || r.GetState() == "COMMENTED" {
			continue
		}
		if old := latest[r.GetUser().GetID()]; old == nil || r.GetID() > old.GetID() {
			latest[r.GetUser().GetID()] = r
		}
	}
	for _, r := range latest {
		if r.GetState() == "CHANGES_REQUESTED" {
			return true
		}
	}
	return false
}

// All observed checks must pass, and each configured CI check must exist as a
// successful GitHub Actions check. Pagination prevents hidden failures.
func (o *Options) mergeChecks(ctx context.Context, owner, repo, sha string, required []string) (bool, error) {
	if len(required) == 0 {
		return false, nil
	}
	seen := map[string]bool{}
	for page := 1; page != 0; {
		checks, response, err := o.GitHub.Checks.ListCheckRunsForRef(ctx, owner, repo, sha, &github.ListCheckRunsOptions{Filter: github.Ptr("latest"), ListOptions: github.ListOptions{Page: page, PerPage: 100}})
		if err != nil {
			return false, errors.New("merge check lookup failed")
		}
		for _, c := range checks.CheckRuns {
			if c.GetStatus() != "completed" || c.GetConclusion() != "success" {
				return false, nil
			}
			if c.GetApp().GetSlug() == "github-actions" {
				seen[c.GetName()] = true
			}
		}
		page = response.NextPage
	}
	for _, name := range required {
		if !seen[name] {
			return false, nil
		}
	}
	for page := 1; page != 0; {
		status, response, err := o.GitHub.Repositories.GetCombinedStatus(ctx, owner, repo, sha, &github.ListOptions{Page: page, PerPage: 100})
		if err != nil {
			return false, errors.New("merge status lookup failed")
		}
		for _, s := range status.Statuses {
			if s.GetContext() != riskContext && s.GetState() != "success" {
				return false, nil
			}
		}
		page = response.NextPage
	}
	return true, nil
}

func (o *Options) mergePR(ctx context.Context, project Project, pr *github.PullRequest, run Run, f feedback) error {
	if !project.AutoMerge || !ownMergeablePR(pr, o.BotID, project.Repository) || changesRequested(f) || run.Lifecycle.Status.Kind != "succeeded" {
		return nil
	}
	state, err := o.Fabro.State(ctx, run.ID)
	if err != nil {
		return err
	}
	job, _ := state.Spec.Settings.Run.Inputs["job"].(string)
	if !jobPattern.MatchString(job) {
		return nil
	}
	dir := filepath.Join(o.raw.Storage, job)
	var config struct {
		Repository string `json:"repository"`
		Base       string `json:"base"`
		BaseSHA    string `json:"base_sha"`
	}
	var identity struct {
		RunID string `json:"run_id"`
	}
	var publication, review, request, verified, approved mergeEvidence
	if !readMergeEvidence(dir, "config", &config) || !readMergeEvidence(dir, "run", &identity) || identity.RunID != run.ID || config.Repository != project.Repository || config.Base != pr.GetBase().GetRef() || config.BaseSHA != pr.GetBase().GetSHA() {
		return nil
	}
	if !readMergeEvidence(dir, "publication", &publication) || publication.Mode != "ready" || publication.PublishHead != pr.GetHead().GetSHA() || len(publication.Tree) == 0 {
		return nil
	}
	if !readMergeEvidence(dir, "review", &review) || !lowRisk(review) || !readMergeEvidence(dir, "review-request", &request) || review.Head != request.Head || request.Tree != publication.Tree {
		return nil
	}
	if !readMergeEvidence(dir, "verified", &verified) || verified.Tree != publication.Tree || !readMergeEvidence(dir, "review-approved", &approved) || approved.Tree != publication.Tree {
		return nil
	}
	owner, repo, _ := strings.Cut(project.Repository, "/")
	commit, _, err := o.GitHub.Git.GetCommit(ctx, owner, repo, publication.PublishHead)
	if err != nil {
		return errors.New("published commit lookup failed")
	}
	if commit.GetTree().GetSHA() != publication.Tree {
		return nil
	}
	ok, err := o.mergeChecks(ctx, owner, repo, publication.PublishHead, project.MergeChecks)
	if err != nil || !ok {
		return err
	}
	metadata, _, err := o.GitHub.Repositories.Get(ctx, owner, repo)
	if err != nil {
		return errors.New("merge repository lookup failed")
	}
	// Refresh PR state and human feedback after the potentially slow checks.
	fresh, _, err := o.GitHub.PullRequests.Get(ctx, owner, repo, pr.GetNumber())
	if err != nil {
		return errors.New("merge PR refresh failed")
	}
	if !ownMergeablePR(fresh, o.BotID, project.Repository) || fresh.GetHead().GetSHA() != publication.PublishHead || fresh.GetBase().GetSHA() != config.BaseSHA {
		return nil
	}
	current, err := o.readFeedback(ctx, owner, repo, pr.GetNumber())
	if err != nil {
		return err
	}
	before, _ := f.pending(feedbackCursor{}, o.RetryUserID, o.BotID)
	if _, pending := current.pending(before, o.RetryUserID, o.BotID); pending || changesRequested(current) {
		return nil
	}
	if o.raw.Preview {
		o.Log.Info("Would merge low-risk PR", "repository", project.Repository, "pr", pr.GetNumber())
		return nil
	}
	if metadata.GetPrivate() {
		if !metadata.GetAllowMergeCommit() || fresh.GetMergeableState() != "clean" {
			return nil
		}
		result, _, err := o.GitHub.PullRequests.Merge(ctx, owner, repo, pr.GetNumber(), "", &github.PullRequestOptions{SHA: publication.PublishHead, MergeMethod: "merge"})
		if err != nil {
			return errors.New("private PR merge not confirmed; reconcile next poll")
		}
		if !result.GetMerged() {
			return errors.New("private PR was not merged")
		}
	} else {
		if !metadata.GetAllowAutoMerge() || !metadata.GetAllowMergeCommit() {
			return nil
		}
		protection, _, err := o.GitHub.Repositories.GetBranchProtection(ctx, owner, repo, config.Base)
		if err != nil {
			return errors.New("public auto-merge requires branch protection")
		}
		checks := protection.GetRequiredStatusChecks()
		if checks == nil || !checks.Strict {
			return nil
		}
		required := map[string]bool{}
		for _, c := range checks.GetContexts() {
			required[c] = true
		}
		for _, c := range checks.GetChecks() {
			required[c.Context] = true
		}
		if !required[riskContext] {
			return nil
		}
		for _, c := range project.MergeChecks {
			if !required[c] {
				return nil
			}
		}
		if _, _, err := o.GitHub.Repositories.CreateStatus(ctx, owner, repo, publication.PublishHead, &github.RepoStatus{State: github.Ptr("success"), Context: github.Ptr(riskContext), Description: github.Ptr("Both reviewers: low risk; verification passed")}); err != nil {
			return errors.New("risk status publication failed")
		}
		if fresh.AutoMerge != nil {
			return nil
		}
		body := map[string]any{"query": `mutation($id:ID!,$head:GitObjectID!) { enablePullRequestAutoMerge(input:{pullRequestId:$id,expectedHeadOid:$head,mergeMethod:MERGE}) { pullRequest { id } } }`, "variables": map[string]string{"id": fresh.GetNodeID(), "head": publication.PublishHead}}
		req, err := o.GitHub.NewRequest("POST", "graphql", body)
		if err != nil {
			return errors.New("invalid auto-merge request")
		}
		var result struct {
			Errors []json.RawMessage `json:"errors"`
			Data   struct {
				Enabled *json.RawMessage `json:"enablePullRequestAutoMerge"`
			} `json:"data"`
		}
		if _, err := o.GitHub.Do(ctx, req, &result); err != nil || len(result.Errors) > 0 || result.Data.Enabled == nil {
			return errors.New("GitHub auto-merge not confirmed")
		}
	}
	o.Log.Info("Low-risk merge authorized", "repository", project.Repository, "pr", pr.GetNumber(), "head", publication.PublishHead, "private", metadata.GetPrivate())
	return nil
}
