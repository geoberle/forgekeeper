package orchestrator

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/google/go-github/v75/github"
	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"
)

type RawOptions struct {
	ProjectsDir, Workflow, Storage, GitHubTokenFile, FabroURL, WebURL string
	Interval                                                          time.Duration
	Once, Preview, AllowHTTP                                          bool
	Repository, RetryUser                                             string
	Issue                                                             int
}

func DefaultOptions() *RawOptions {
	return &RawOptions{
		ProjectsDir: "/experiment/projects", Workflow: "/experiment/workflows/implement-issue/workflow.toml",
		Storage: "/storage/issue-jobs", GitHubTokenFile: "/storage/home/.github-pat", FabroURL: "http://127.0.0.1:32276", WebURL: "http://localhost:32277", Interval: time.Minute,
	}
}
func BindOptions(o *RawOptions, c *cobra.Command) {
	f := c.Flags()
	f.StringVar(&o.ProjectsDir, "projects", o.ProjectsDir, "Project YAML directory")
	f.StringVar(&o.Workflow, "workflow", o.Workflow, "Fabro workflow TOML")
	f.StringVar(&o.Storage, "workspaces", o.Storage, "Workflow job files (not orchestration state)")
	f.StringVar(&o.GitHubTokenFile, "github-token-file", o.GitHubTokenFile, "Bot PAT file")
	f.StringVar(&o.FabroURL, "fabro-url", o.FabroURL, "Fabro API origin")
	f.StringVar(&o.WebURL, "web-url", o.WebURL, "Fabro links in issue comments")
	f.DurationVar(&o.Interval, "interval", o.Interval, "Polling interval")
	f.BoolVar(&o.AllowHTTP, "allow-http-fabro", false, "Allow unencrypted Fabro API traffic on a trusted cluster network")
	f.BoolVar(&o.Once, "once", false, "Reconcile once")
	f.BoolVar(&o.Preview, "preview", false, "Read-only: show decisions without starting runs or posting comments")
	f.StringVar(&o.RetryUser, "retry-user", o.RetryUser, "GitHub login allowed to authorize retries (required)")
	f.StringVar(&o.Repository, "repo", "", "Restrict selection to one configured repository")
	f.IntVar(&o.Issue, "issue", 0, "Restrict selection to one issue (requires --repo)")
}

type validatedOptions struct{ raw RawOptions }
type ValidatedOptions struct{ *validatedOptions }
type completedOptions struct {
	raw         RawOptions
	Projects    []Project
	GitHub      *github.Client
	Fabro       *FabroClient
	BotID       int64
	RetryUserID int64
	Log         logr.Logger
	Execute     func(context.Context, string, []string, string) ([]byte, error)
}
type Options struct{ *completedOptions }

var loginPattern = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9-]{0,37}[A-Za-z0-9])?$`)

var repoPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)
var runPattern = regexp.MustCompile(`^[0-9A-HJKMNP-TV-Z]{26}$`)

func (o *RawOptions) Validate() (*ValidatedOptions, error) {
	if !loginPattern.MatchString(o.RetryUser) || strings.Contains(o.RetryUser, "--") {
		return nil, errors.New("retry-user must be a GitHub login")
	}
	if o.Interval < time.Second || o.Issue < 0 || (o.Issue > 0 && len(o.Repository) == 0) {
		return nil, errors.New("invalid polling interval or issue selection")
	}
	if len(o.Repository) > 0 && !repoPattern.MatchString(o.Repository) {
		return nil, errors.New("invalid repository")
	}
	for i, v := range []string{o.FabroURL, o.WebURL} {
		u, e := url.Parse(v)
		if e != nil || u.User != nil || len(u.RawQuery) > 0 || len(u.Fragment) > 0 || len(u.Host) == 0 || (u.Path != "" && u.Path != "/") {
			return nil, errors.New("expected an HTTP(S) origin without credentials")
		}
		allowHTTP := u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || (i == 0 && o.AllowHTTP)
		if u.Scheme != "https" && (u.Scheme != "http" || !allowHTTP) {
			return nil, errors.New("HTTP requires loopback or explicit Fabro API opt-in")
		}
	}
	if len(o.ProjectsDir) == 0 || len(o.Workflow) == 0 || len(o.Storage) == 0 || len(o.GitHubTokenFile) == 0 {
		return nil, errors.New("required path missing")
	}
	copy := *o
	copy.FabroURL = strings.TrimRight(copy.FabroURL, "/")
	copy.WebURL = strings.TrimRight(copy.WebURL, "/")
	return &ValidatedOptions{&validatedOptions{copy}}, nil
}
func (o *ValidatedOptions) Complete(ctx context.Context, logger logr.Logger) (*Options, error) {
	files, err := filepath.Glob(filepath.Join(o.raw.ProjectsDir, "*.yaml"))
	if err != nil {
		return nil, errors.New("cannot list projects")
	}
	var projects []Project
	seen := map[string]bool{}
	for _, path := range files {
		data, e := os.ReadFile(path)
		if e != nil {
			return nil, errors.New("cannot read project")
		}
		var p Project
		if yaml.UnmarshalStrict(data, &p) != nil || !repoPattern.MatchString(p.Repository) || len(p.Verification) == 0 {
			return nil, errors.New("invalid project configuration")
		}
		for _, v := range p.Verification {
			if len(strings.TrimSpace(v)) == 0 {
				return nil, errors.New("empty verification command")
			}
		}
		p.Repository = strings.ToLower(p.Repository)
		if seen[p.Repository] {
			return nil, errors.New("duplicate repository configuration")
		}
		seen[p.Repository] = true
		projects = append(projects, p)
	}
	if len(projects) == 0 {
		return nil, errors.New("no projects configured")
	}
	if len(o.raw.Repository) > 0 && !seen[strings.ToLower(o.raw.Repository)] {
		return nil, errors.New("selected repository not configured")
	}
	token, err := os.ReadFile(o.raw.GitHubTokenFile)
	if err != nil || len(strings.TrimSpace(string(token))) == 0 {
		return nil, errors.New("bot PAT missing")
	}
	if len(os.Getenv("FABRO_DEV_TOKEN")) == 0 {
		return nil, errors.New("fabro dev token missing")
	}
	httpClient := &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	gh := github.NewClient(httpClient).WithAuthToken(strings.TrimSpace(string(token)))
	bot, _, err := gh.Users.Get(ctx, "")
	if err != nil {
		return nil, errors.New("cannot identify GitHub bot")
	}
	retryUser, _, err := gh.Users.Get(ctx, o.raw.RetryUser)
	if err != nil {
		return nil, errors.New("cannot identify retry author")
	}
	if bot.GetID() == 0 || retryUser.GetID() == 0 {
		return nil, errors.New("GitHub user identity missing")
	}
	if bot.GetID() == retryUser.GetID() {
		return nil, errors.New("use a separate bot identity")
	}
	return &Options{&completedOptions{raw: o.raw, Projects: projects, GitHub: gh,
		Fabro: &FabroClient{HTTP: httpClient, BaseURL: o.raw.FabroURL, Token: os.Getenv("FABRO_DEV_TOKEN")}, BotID: bot.GetID(), RetryUserID: retryUser.GetID(), Log: logger, Execute: func(ctx context.Context, name string, args []string, dir string) ([]byte, error) {
			cmd := exec.CommandContext(ctx, name, args...)
			cmd.Dir = dir
			for _, entry := range os.Environ() {
				key, _, _ := strings.Cut(entry, "=")
				if strings.HasPrefix(key, "GIT_") || key == "GH_TOKEN" || key == "GITHUB_TOKEN" {
					continue
				}
				cmd.Env = append(cmd.Env, entry)
			}
			cmd.Env = append(cmd.Env, "GH_TOKEN="+strings.TrimSpace(string(token)), "GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=credential.helper", "GIT_CONFIG_VALUE_0=!gh auth git-credential", "GIT_TERMINAL_PROMPT=0")
			output, err := cmd.Output()
			if err != nil {
				return nil, errors.New("external command failed")
			}
			return output, nil
		}}}, nil
}
