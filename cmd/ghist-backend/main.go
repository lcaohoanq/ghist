// ghist-backend handles one JSON request per process for editor integrations.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	gitrepo "github.com/lcaohoanq/ghist/internal/git"
	"github.com/lcaohoanq/ghist/internal/history"
	"github.com/lcaohoanq/ghist/internal/hosting"
)

const protocolVersion = 1

type request struct {
	ProtocolVersion int    `json:"protocolVersion"`
	Operation       string `json:"operation"`
	FilePath        string `json:"filePath,omitempty"`
	Root            string `json:"root,omitempty"`
	Revision        string `json:"revision,omitempty"`
	Path            string `json:"path,omitempty"`
}

type version struct {
	Hash       string    `json:"hash"`
	ShortHash  string    `json:"shortHash"`
	Author     string    `json:"author"`
	Email      string    `json:"email"`
	Subject    string    `json:"subject"`
	Date       time.Time `json:"date"`
	Parent     string    `json:"parent"`
	Merge      bool      `json:"merge"`
	BeforePath string    `json:"beforePath"`
	AfterPath  string    `json:"afterPath"`
	Change     string    `json:"change"`
}

type fileHistory struct {
	Root     string    `json:"root"`
	Path     string    `json:"path"`
	Head     string    `json:"head"`
	Versions []version `json:"versions"`
}

type snapshot struct {
	Content   string `json:"content"`
	Exists    bool   `json:"exists"`
	Binary    bool   `json:"binary"`
	Truncated bool   `json:"truncated"`
}

type reviewLink struct {
	Provider string `json:"provider"`
	Number   int    `json:"number"`
	URL      string `json:"url"`
}

type reviews struct {
	Links []reviewLink `json:"links"`
}

var fullCommitHash = regexp.MustCompile(`^[0-9a-fA-F]{40}$`)

type protocolError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type response struct {
	ProtocolVersion int            `json:"protocolVersion"`
	Result          any            `json:"result,omitempty"`
	Error           *protocolError `json:"error,omitempty"`
}

func dispatch(ctx context.Context, req request) (any, error) {
	if req.ProtocolVersion != protocolVersion {
		return nil, fmt.Errorf("unsupported protocol version: %d", req.ProtocolVersion)
	}
	switch req.Operation {
	case "history":
		if !filepath.IsAbs(req.FilePath) {
			return nil, errors.New("filePath must be an absolute file path")
		}
		repo, err := gitrepo.Open(ctx, req.FilePath)
		if err != nil {
			return nil, err
		}
		h, err := history.NewService(repo).ExploreFile(ctx)
		if err != nil {
			return nil, err
		}
		result := fileHistory{Root: repo.Root, Path: h.Path, Head: h.Head, Versions: make([]version, 0, len(h.Versions))}
		for _, v := range h.Versions {
			result.Versions = append(result.Versions, version{
				Hash: v.Commit.Hash, ShortHash: v.Commit.ShortHash, Author: v.Commit.Author,
				Email: v.Commit.Email, Subject: v.Commit.Subject, Date: v.Commit.Date,
				Parent: v.Parent, Merge: v.Merge, BeforePath: v.BeforePath, AfterPath: v.AfterPath, Change: v.Change,
			})
		}
		return result, nil
	case "snapshot":
		if !filepath.IsAbs(req.Root) {
			return nil, errors.New("root must be an absolute repository path")
		}
		// Git paths are slash-separated, including on Windows. Reject navigation
		// components rather than normalizing a path to a different tracked file.
		if req.Path == "" || path.IsAbs(req.Path) || path.Clean(req.Path) != req.Path || req.Path == "." || req.Path == ".." || strings.HasPrefix(req.Path, "../") {
			return nil, errors.New("path must be a repository-relative Git path")
		}
		repo, err := gitrepo.OpenDirectory(ctx, req.Root)
		if err != nil {
			return nil, err
		}
		v := history.FileVersion{Commit: history.Commit{Hash: req.Revision, ShortHash: req.Revision}, AfterPath: req.Path}
		s, err := repo.Snapshot(ctx, v)
		if err != nil {
			return nil, err
		}
		return snapshot{Content: s.Content, Exists: s.Exists, Binary: s.Binary, Truncated: s.Truncated}, nil
	case "reviews":
		if !filepath.IsAbs(req.Root) {
			return nil, errors.New("root must be an absolute repository path")
		}
		if !fullCommitHash.MatchString(req.Revision) {
			return nil, errors.New("revision must be a full commit hash")
		}
		repo, err := gitrepo.OpenDirectory(ctx, req.Root)
		if err != nil {
			return nil, err
		}
		links, err := hosting.New(ctx, repo.Root).Fetch(ctx, req.Revision)
		if err != nil {
			return nil, err
		}
		result := reviews{Links: make([]reviewLink, 0, len(links))}
		for _, link := range links {
			result.Links = append(result.Links, reviewLink{
				Provider: string(link.Provider), Number: link.Number, URL: link.URL,
			})
		}
		return result, nil
	default:
		return nil, fmt.Errorf("unknown operation: %q", req.Operation)
	}
}

func serve(ctx context.Context, in io.Reader, out io.Writer) error {
	data, err := io.ReadAll(io.LimitReader(in, (1<<20)+1))
	if len(data) > 1<<20 {
		err = errors.New("JSON request exceeds the 1 MiB limit")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var req request
	if err == nil {
		err = decoder.Decode(&req)
	}
	if err == nil {
		var trailing any
		if trailingErr := decoder.Decode(&trailing); trailingErr != io.EOF {
			err = errors.New("expected exactly one JSON request")
		}
	}
	res := response{ProtocolVersion: protocolVersion}
	if err == nil {
		res.Result, err = dispatch(ctx, req)
	}
	if err != nil {
		code := "request_failed"
		switch {
		case errors.Is(err, history.ErrNoHistory):
			code = "no_history"
		case errors.Is(err, history.ErrNoCommits):
			code = "no_commits"
		case errors.Is(err, history.ErrNotRepository):
			code = "not_repository"
		case errors.Is(err, context.Canceled):
			code = "cancelled"
		}
		res.Error = &protocolError{Code: code, Message: err.Error()}
	}
	return json.NewEncoder(out).Encode(res)
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := serve(ctx, os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "ghist-backend:", err)
		os.Exit(1)
	}
}
