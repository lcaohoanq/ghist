package hosting

import (
	"context"
	"errors"
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

type call struct {
	dir, name string
	args      []string
}

type fakeRunner struct {
	responses map[string][]byte
	errors    map[string]error
	calls     []call
}

type cancelRunner struct{}

func (cancelRunner) Run(ctx context.Context, _, _ string, _ ...string) ([]byte, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func commandKey(name string, args []string) string { return name + " " + strings.Join(args, " ") }

func (f *fakeRunner) Run(_ context.Context, dir, name string, args ...string) ([]byte, error) {
	f.calls = append(f.calls, call{dir: dir, name: name, args: append([]string(nil), args...)})
	key := commandKey(name, args)
	return f.responses[key], f.errors[key]
}

func TestParseRemote(t *testing.T) {
	tests := []struct {
		raw, host, project string
		provider           Provider
	}{
		{"https://github.com/acme/widget.git", "github.com", "acme/widget", GitHub},
		{"ssh://git@gitlab.com:2222/group/sub/repo.git", "gitlab.com", "group/sub/repo", GitLab},
		{"git@github.example.test:team/repo.git", "github.example.test", "team/repo", GitHub},
		{"https://code.example.test:8443/group/sub/repo", "code.example.test", "group/sub/repo", ""},
	}
	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			got, err := ParseRemote(tt.raw)
			if err != nil || got.Host != tt.host || got.ProjectPath != tt.project || got.Provider != tt.provider {
				t.Fatalf("ParseRemote() = %+v, %v", got, err)
			}
		})
	}
	for _, raw := range []string{"", "file:///tmp/repo", "https://github.com/repo"} {
		if _, err := ParseRemote(raw); err == nil {
			t.Fatalf("ParseRemote(%q) succeeded", raw)
		}
	}
}

func TestSelfHostedDetection(t *testing.T) {
	for _, tt := range []struct {
		name     string
		gh, glab bool
		want     Provider
		wantErr  string
	}{
		{"github", true, false, GitHub, ""},
		{"gitlab", false, true, GitLab, ""},
		{"both", true, true, "", "ambiguous"},
		{"neither", false, false, "", "authenticate"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := &fakeRunner{responses: map[string][]byte{}, errors: map[string]error{}}
			if !tt.gh {
				f.errors["gh auth status --active --hostname code.example.test"] = errors.New("no gh auth")
			}
			if !tt.glab {
				f.errors["glab auth status --hostname code.example.test"] = errors.New("no glab auth")
			}
			got, err := detectProvider(context.Background(), "/repo", "code.example.test", f)
			if got != tt.want || (tt.wantErr == "") != (err == nil) || (err != nil && !strings.Contains(err.Error(), tt.wantErr)) {
				t.Fatalf("detectProvider() = %q, %v", got, err)
			}
		})
	}
}

func configuredService(provider Provider, runner commandRunner) *Service {
	s := &Service{
		root:   "/repo",
		runner: runner,
		remote: Remote{URL: "git@example.test:group/sub/repo.git", Host: "example.test", ProjectPath: "group/sub/repo", Provider: provider},
		cache:  make(map[string][]ReviewLink),
	}
	s.initOnce.Do(func() {})
	return s
}

func TestFetchGitHubAndCache(t *testing.T) {
	sha := strings.Repeat("a", 40)
	args := []string{"api", "--hostname", "example.test", "repos/group/sub/repo/commits/" + sha + "/pulls"}
	f := &fakeRunner{responses: map[string][]byte{
		commandKey("gh", args): []byte(`[{"number":12,"html_url":"https://example.test/group/sub/repo/pull/12"},{"number":3,"html_url":"https://example.test/group/sub/repo/pull/3"}]`),
	}, errors: map[string]error{}}
	s := configuredService(GitHub, f)
	links, err := s.Fetch(context.Background(), sha)
	if err != nil || len(links) != 2 || links[0].Number != 12 || links[1].Number != 3 {
		t.Fatalf("Fetch() = %+v, %v", links, err)
	}
	if _, err := s.Fetch(context.Background(), sha); err != nil || len(f.calls) != 1 {
		t.Fatalf("successful result not cached: calls=%d err=%v", len(f.calls), err)
	}
}

func TestFetchGitLabAndOpenArguments(t *testing.T) {
	sha := "abc123"
	apiArgs := []string{"api", "--hostname", "example.test", "projects/group%2Fsub%2Frepo/repository/commits/abc123/merge_requests"}
	f := &fakeRunner{responses: map[string][]byte{
		commandKey("glab", apiArgs): []byte(`[{"iid":7,"web_url":"http://example.test/group/sub/repo/-/merge_requests/7"}]`),
	}, errors: map[string]error{}}
	s := configuredService(GitLab, f)
	links, err := s.Fetch(context.Background(), sha)
	if err != nil || len(links) != 1 || links[0].Number != 7 {
		t.Fatalf("Fetch() = %+v, %v; calls=%+v", links, err, f.calls)
	}
	if err := s.Open(context.Background(), links[0]); err != nil {
		t.Fatal(err)
	}
	want := call{dir: "/repo", name: "glab", args: []string{"mr", "view", "7", "--repo", "git@example.test:group/sub/repo.git", "--web"}}
	if !reflect.DeepEqual(f.calls[len(f.calls)-1], want) {
		t.Fatalf("open call = %+v, want %+v", f.calls[len(f.calls)-1], want)
	}
}

func TestFetchGitLabFallsBackForOlderCLI(t *testing.T) {
	sha := "abc123"
	modernArgs := []string{"api", "--hostname", "example.test", "projects/group%2Fsub%2Frepo/repository/commits/abc123/merge_requests"}
	legacyArgs := []string{"api", "projects/group%2Fsub%2Frepo/repository/commits/abc123/merge_requests"}
	f := &fakeRunner{responses: map[string][]byte{
		commandKey("glab", legacyArgs): []byte(`[{"iid":7,"web_url":"https://example.test/group/sub/repo/-/merge_requests/7"}]`),
	}, errors: map[string]error{
		commandKey("glab", modernArgs): errors.New("accepts 1 arg(s), received 3"),
	}}
	links, err := configuredService(GitLab, f).Fetch(context.Background(), sha)
	if err != nil || len(links) != 1 || links[0].Number != 7 || len(f.calls) != 2 {
		t.Fatalf("legacy fallback = %+v, %v; calls=%+v", links, err, f.calls)
	}
}

func TestFetchRejectsMalformedAndUnsafeResponses(t *testing.T) {
	for _, data := range []string{
		`not json`,
		`[{"number":1,"html_url":"javascript:alert(1)"}]`,
		`[{"number":1,"html_url":"https://evil.test/pull/1"}]`,
	} {
		f := &fakeRunner{responses: map[string][]byte{}, errors: map[string]error{}}
		s := configuredService(GitHub, f)
		sha := "abc"
		args := []string{"api", "--hostname", "example.test", "repos/group/sub/repo/commits/abc/pulls"}
		f.responses[commandKey("gh", args)] = []byte(data)
		if _, err := s.Fetch(context.Background(), sha); err == nil {
			t.Fatalf("unsafe response accepted: %s", data)
		}
	}
}

func TestNewMissingOriginIsOptionalError(t *testing.T) {
	f := &fakeRunner{responses: map[string][]byte{}, errors: map[string]error{"git remote get-url origin": errors.New("missing")}}
	s := newService(context.Background(), "/repo", f)
	if len(f.calls) != 0 {
		t.Fatalf("newService performed discovery: calls=%+v", f.calls)
	}
	if _, err := s.Fetch(context.Background(), "abc"); err == nil || !strings.Contains(err.Error(), "no origin") {
		t.Fatalf("Fetch error = %v", err)
	}
	if _, err := s.Fetch(context.Background(), "def"); err == nil || len(f.calls) != 1 {
		t.Fatalf("discovery result not cached: calls=%+v err=%v", f.calls, err)
	}
}

func TestFetchDefersAndCachesProviderDetection(t *testing.T) {
	f := &fakeRunner{
		responses: map[string][]byte{
			"git remote get-url origin": []byte("https://code.example.test/team/repo.git\n"),
			"gh api --hostname code.example.test repos/team/repo/commits/abc/pulls": []byte("[]"),
			"gh api --hostname code.example.test repos/team/repo/commits/def/pulls": []byte("[]"),
		},
		errors: map[string]error{
			"glab auth status --hostname code.example.test": errors.New("not authenticated"),
		},
	}
	s := newService(context.Background(), "/repo", f)
	if len(f.calls) != 0 {
		t.Fatalf("newService performed discovery: calls=%+v", f.calls)
	}
	if _, err := s.Fetch(context.Background(), "abc"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Fetch(context.Background(), "def"); err != nil {
		t.Fatal(err)
	}
	if got := s.Provider(); got != GitHub {
		t.Fatalf("Provider() = %q, want %q", got, GitHub)
	}
	if len(f.calls) != 5 {
		t.Fatalf("provider detection repeated: calls=%+v", f.calls)
	}
}

func TestFetchAPIErrorAndCancellation(t *testing.T) {
	f := &fakeRunner{responses: map[string][]byte{}, errors: map[string]error{}}
	sha := "abc"
	args := []string{"api", "--hostname", "example.test", "repos/group/sub/repo/commits/abc/pulls"}
	f.errors[commandKey("gh", args)] = errors.New("network down")
	if _, err := configuredService(GitHub, f).Fetch(context.Background(), sha); err == nil || !strings.Contains(err.Error(), "network") {
		t.Fatalf("API error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := configuredService(GitHub, cancelRunner{}).Fetch(ctx, sha); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation = %v", err)
	}
}

func TestFetchMissingCLIHasInstallGuidance(t *testing.T) {
	f := &fakeRunner{responses: map[string][]byte{}, errors: map[string]error{}}
	args := []string{"api", "--hostname", "example.test", "repos/group/sub/repo/commits/abc/pulls"}
	f.errors[commandKey("gh", args)] = &exec.Error{Name: "gh", Err: exec.ErrNotFound}
	_, err := configuredService(GitHub, f).Fetch(context.Background(), "abc")
	if err == nil || !strings.Contains(err.Error(), "CLI is required") || !strings.Contains(err.Error(), "gh auth login") {
		t.Fatalf("missing CLI error = %v", err)
	}
}

func TestOpenGitHubArguments(t *testing.T) {
	f := &fakeRunner{responses: map[string][]byte{}, errors: map[string]error{}}
	s := configuredService(GitHub, f)
	link := ReviewLink{Provider: GitHub, Number: 42, URL: "https://example.test/group/sub/repo/pull/42"}
	if err := s.Open(context.Background(), link); err != nil {
		t.Fatal(err)
	}
	want := call{dir: "/repo", name: "gh", args: []string{"pr", "view", "42", "--repo", "example.test/group/sub/repo", "--web"}}
	if !reflect.DeepEqual(f.calls[0], want) {
		t.Fatalf("open call = %+v, want %+v", f.calls[0], want)
	}
}
