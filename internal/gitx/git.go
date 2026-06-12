package gitx

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Refs struct {
	Repo     string   `json:"repo"`
	Head     string   `json:"head,omitempty"`
	Branches []string `json:"branches"`
	Tags     []string `json:"tags"`
}

type Commit struct {
	Hash    string `json:"hash"`
	Ref     string `json:"ref,omitempty"`
	Author  string `json:"author,omitempty"`
	Date    string `json:"date,omitempty"`
	Subject string `json:"subject,omitempty"`
}

type Status struct {
	Repo    string   `json:"repo"`
	Branch  string   `json:"branch,omitempty"`
	Clean   bool     `json:"clean"`
	Changes []string `json:"changes,omitempty"`
}

func ListRefs(repo string) (Refs, error) {
	repo, err := cleanRepo(repo)
	if err != nil {
		return Refs{}, err
	}
	head, _ := git(repo, "rev-parse", "--abbrev-ref", "HEAD")
	branches, err := gitLines(repo, "for-each-ref", "--format=%(refname:short)", "refs/heads")
	if err != nil {
		return Refs{}, err
	}
	tags, err := gitLines(repo, "tag", "--list")
	if err != nil {
		return Refs{}, err
	}
	return Refs{Repo: repo, Head: strings.TrimSpace(head), Branches: branches, Tags: tags}, nil
}

func ListCommits(repo, ref string, limit int) ([]Commit, error) {
	repo, err := cleanRepo(repo)
	if err != nil {
		return nil, err
	}
	if ref == "" {
		ref = "HEAD"
	}
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	format := "%H%x1f%an%x1f%aI%x1f%s"
	lines, err := gitLines(repo, "log", "--date=iso-strict", "--format="+format, "-n", strconv.Itoa(limit), ref)
	if err != nil {
		return nil, err
	}
	out := make([]Commit, 0, len(lines))
	for _, line := range lines {
		parts := strings.SplitN(line, "\x1f", 4)
		if len(parts) != 4 {
			continue
		}
		out = append(out, Commit{Hash: parts[0], Ref: ref, Author: parts[1], Date: parts[2], Subject: parts[3]})
	}
	return out, nil
}

func WorktreeStatus(repo string) (Status, error) {
	repo, err := cleanRepo(repo)
	if err != nil {
		return Status{}, err
	}
	branch, _ := git(repo, "rev-parse", "--abbrev-ref", "HEAD")
	changes, err := gitLines(repo, "status", "--short")
	if err != nil {
		return Status{}, err
	}
	return Status{Repo: repo, Branch: strings.TrimSpace(branch), Clean: len(changes) == 0, Changes: changes}, nil
}

func cleanRepo(repo string) (string, error) {
	repo = strings.TrimSpace(repo)
	if repo == "" {
		return "", errors.New("repo is required")
	}
	abs, err := filepath.Abs(repo)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", errors.New("repo must be a directory")
	}
	if _, err := git(abs, "rev-parse", "--git-dir"); err != nil {
		return "", errors.New("repo is not a git repository")
	}
	return abs, nil
}

func gitLines(repo string, args ...string) ([]string, error) {
	out, err := git(repo, args...)
	if err != nil {
		return nil, err
	}
	var lines []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines, nil
}

func git(repo string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = repo
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", errors.New(msg)
	}
	return stdout.String(), nil
}
