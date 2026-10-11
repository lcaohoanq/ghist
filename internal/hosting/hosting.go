// Package hosting integrates commits with reviews on Git hosting services.
// It is deliberately independent of the Git history adapter: failures here
// must never prevent local history, diffs, or snapshots from loading.
package hosting

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os/exec"
	"path"
	"strings"
	"sync"
)

type Provider string

const (
	GitHub Provider = "github"
	GitLab Provider = "gitlab"
)

type ReviewLink struct {
	Provider Provider
	Number   int
	URL      string
}

type Remote struct {
	URL         string
	Host        string
	ProjectPath string
	Provider    Provider
}

// Reviewer is the small interface consumed by the TUI.
type Reviewer interface {
	Provider() Provider
	Fetch(context.Context, string) ([]ReviewLink, error)
	Open(context.Context, ReviewLink) error
}

type commandRunner interface {
	Run(context.Context, string, string, ...string) ([]byte, error)
}

type execRunner struct{}

func (execRunner) Run(ctx context.Context, dir, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		detail := strings.TrimSpace(stderr.String())
		if detail != "" {
			return nil, fmt.Errorf("%s: %s: %w", name, detail, err)
		}
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return out, nil
}

type Service struct {
	root    string
	runner  commandRunner
	remote  Remote
	initErr error
	mu      sync.Mutex
	cache   map[string][]ReviewLink
}

// New discovers only origin. Discovery errors are retained and reported by
// Fetch so callers can keep the integration optional.
func New(ctx context.Context, root string) *Service {
	return newService(ctx, root, execRunner{})
}

func newService(ctx context.Context, root string, runner commandRunner) *Service {
	s := &Service{root: root, runner: runner, cache: make(map[string][]ReviewLink)}
	out, err := runner.Run(ctx, root, "git", "remote", "get-url", "origin")
	if err != nil {
		s.initErr = errors.New("no origin remote; review lookup unavailable")
		return s
	}
	s.remote, err = ParseRemote(strings.TrimSpace(string(out)))
	if err != nil {
		s.initErr = err
		return s
	}
	if s.remote.Provider == "" {
		s.remote.Provider, s.initErr = detectProvider(ctx, root, s.remote.Host, runner)
	}
	return s
}

func (s *Service) Provider() Provider { return s.remote.Provider }

func ParseRemote(raw string) (Remote, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Remote{}, errors.New("origin has an empty URL; review lookup unavailable")
	}
	normalized := raw
	// Git's SCP-like syntax is not understood by net/url. A colon before the
	// first slash denotes the path; bracketed IPv6 is intentionally left to
	// explicit ssh:// syntax.
	if !strings.Contains(raw, "://") {
		at := strings.LastIndex(raw, "@")
		colon := strings.Index(raw[max(0, at+1):], ":")
		if colon >= 0 {
			colon += max(0, at+1)
			host := raw[:colon]
			project := raw[colon+1:]
			if at >= 0 {
				host = raw[at+1 : colon]
			}
			normalized = "ssh://" + host + "/" + project
		}
	}
	u, err := url.Parse(normalized)
	if err != nil || u.Hostname() == "" {
		return Remote{}, fmt.Errorf("unsupported origin URL %q; review lookup unavailable", raw)
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https", "ssh", "git":
	default:
		return Remote{}, fmt.Errorf("unsupported origin URL scheme %q; review lookup unavailable", u.Scheme)
	}
	project := strings.Trim(strings.TrimSuffix(u.Path, ".git"), "/")
	if project == "" || !strings.Contains(project, "/") {
		return Remote{}, fmt.Errorf("origin URL has no owner/project path; review lookup unavailable")
	}
	host := strings.ToLower(u.Hostname())
	provider := Provider("")
	if host == "github.com" || strings.Contains(host, "github") {
		provider = GitHub
	} else if host == "gitlab.com" || strings.Contains(host, "gitlab") {
		provider = GitLab
	}
	return Remote{URL: raw, Host: host, ProjectPath: project, Provider: provider}, nil
}

func detectProvider(ctx context.Context, root, host string, runner commandRunner) (Provider, error) {
	_, ghErr := runner.Run(ctx, root, "gh", "auth", "status", "--active", "--hostname", host)
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	_, glabErr := runner.Run(ctx, root, "glab", "auth", "status", "--hostname", host)
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	gh, glab := ghErr == nil, glabErr == nil
	switch {
	case gh && !glab:
		return GitHub, nil
	case glab && !gh:
		return GitLab, nil
	case gh && glab:
		return "", fmt.Errorf("both gh and glab are authenticated for %s; review provider is ambiguous", host)
	default:
		return "", fmt.Errorf("authenticate gh or glab for %s to enable review lookup", host)
	}
}

func (s *Service) Fetch(ctx context.Context, sha string) ([]ReviewLink, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s.initErr != nil {
		return nil, s.initErr
	}
	s.mu.Lock()
	if links, ok := s.cache[sha]; ok {
		result := append([]ReviewLink(nil), links...)
		s.mu.Unlock()
		return result, nil
	}
	s.mu.Unlock()

	var name string
	var args []string
	switch s.remote.Provider {
	case GitHub:
		name = "gh"
		args = []string{"api", "--hostname", s.remote.Host, path.Join("repos", s.remote.ProjectPath, "commits", sha, "pulls")}
	case GitLab:
		name = "glab"
		args = []string{"api", "--hostname", s.remote.Host, "projects/" + url.PathEscape(s.remote.ProjectPath) + "/repository/commits/" + url.PathEscape(sha) + "/merge_requests"}
	default:
		return nil, errors.New("unsupported origin host; review lookup unavailable")
	}
	out, err := s.runner.Run(ctx, s.root, name, args...)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return nil, err
		}
		var executableError *exec.Error
		if errors.As(err, &executableError) {
			return nil, fmt.Errorf("%s CLI is required; install and authenticate with %s auth login", name, name)
		}
		return nil, fmt.Errorf("%s review lookup failed; check network and %s auth status", providerName(s.remote.Provider), name)
	}
	links, err := parseLinks(s.remote.Provider, s.remote.Host, out)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.cache[sha] = append([]ReviewLink(nil), links...)
	s.mu.Unlock()
	return links, nil
}

func parseLinks(provider Provider, remoteHost string, data []byte) ([]ReviewLink, error) {
	type item struct {
		Number  int    `json:"number"`
		HTMLURL string `json:"html_url"`
		IID     int    `json:"iid"`
		WebURL  string `json:"web_url"`
	}
	var items []item
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, fmt.Errorf("invalid %s review response", providerName(provider))
	}
	links := make([]ReviewLink, 0, len(items))
	for _, item := range items {
		number, rawURL := item.Number, item.HTMLURL
		if provider == GitLab {
			number, rawURL = item.IID, item.WebURL
		}
		u, err := url.Parse(rawURL)
		if err != nil || number <= 0 || (u.Scheme != "http" && u.Scheme != "https") || !strings.EqualFold(u.Hostname(), remoteHost) {
			return nil, fmt.Errorf("%s returned an unsafe review URL", providerName(provider))
		}
		links = append(links, ReviewLink{Provider: provider, Number: number, URL: u.String()})
	}
	return links, nil
}

func (s *Service) Open(ctx context.Context, link ReviewLink) error {
	if link.Provider != s.remote.Provider {
		return errors.New("review provider does not match origin")
	}
	if _, err := parseLinks(link.Provider, s.remote.Host, mustJSON(link)); err != nil {
		return err
	}
	var name string
	var args []string
	if link.Provider == GitHub {
		name = "gh"
		args = []string{"pr", "view", fmt.Sprint(link.Number), "--repo", s.remote.Host + "/" + s.remote.ProjectPath, "--web"}
	} else {
		name = "glab"
		args = []string{"mr", "view", fmt.Sprint(link.Number), "--repo", s.remote.URL, "--web"}
	}
	_, err := s.runner.Run(ctx, s.root, name, args...)
	if err != nil && !errors.Is(err, context.Canceled) {
		return fmt.Errorf("could not open review with %s", name)
	}
	return err
}

func mustJSON(link ReviewLink) []byte {
	if link.Provider == GitHub {
		b, _ := json.Marshal([]any{map[string]any{"number": link.Number, "html_url": link.URL}})
		return b
	}
	b, _ := json.Marshal([]any{map[string]any{"iid": link.Number, "web_url": link.URL}})
	return b
}

func providerName(provider Provider) string {
	if provider == GitLab {
		return "GitLab"
	}
	return "GitHub"
}
