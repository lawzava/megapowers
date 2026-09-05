package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDatesAloneDoNotEstablishSupportReview(t *testing.T) {
	root := t.TempDir()
	t.Setenv("MEGAPOWERS_ROOT", root)
	date := time.Now().UTC().Format("2006-01-02")
	for file, content := range map[string]string{
		"docs/harness-support.md":    "Last reviewed: " + date + "\n",
		"scripts/check-freshness.sh": "# Harness tooling reviewed: " + date + "\n",
	} {
		path := filepath.Join(root, file)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if runCheckFreshness(nil) == 0 {
		t.Fatal("dated prose passed without source, tested CLI, or compatibility oracle")
	}
}

func TestReviewEvidence(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "check.go"), []byte("package main\n"), 0600); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 4, 23, 0, 0, 0, time.UTC)
	base := []sourceReview{
		{Source: "https://code.claude.com/docs/en/skills", Reviewed: "2026-09-04", CLI: "claude", Version: "2.1.258", Oracle: []string{"check.go"}},
		{Source: "https://learn.chatgpt.com/docs/build-skills", Reviewed: "2026-09-04", CLI: "codex", Version: "0.153.3", Oracle: []string{"check.go"}},
	}
	if err := validateReviews(base, now, 30, root); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*sourceReview){
		func(r *sourceReview) { r.Reviewed = "2026-09-05" },
		func(r *sourceReview) { r.Reviewed = "2026-07-01" },
		func(r *sourceReview) { r.Version = "latest" },
		func(r *sourceReview) { r.Oracle = nil },
		func(r *sourceReview) { r.Oracle = []string{"../check.go"} },
		func(r *sourceReview) { r.Oracle = []string{"absent.go"} },
		func(r *sourceReview) { r.Source = "https://secret@example.com/private" },
	} {
		changed := append([]sourceReview(nil), base...)
		change(&changed[0])
		if err := validateReviews(changed, now, 30, root); err == nil {
			t.Errorf("invalid review accepted: %#v", changed[0])
		}
	}
}
