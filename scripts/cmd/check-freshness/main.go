package main

import (
	"flag"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/lawzava/megapowers/internal/strictjson"
)

const defaultMaxAge = 30
const sourceMarker = "<!-- freshness-sources -->"

type sourceReview struct {
	Source   string   `json:"source"`
	Reviewed string   `json:"reviewed"`
	CLI      string   `json:"cli"`
	Version  string   `json:"version"`
	Oracle   []string `json:"oracle"`
}

func main() { os.Exit(runCheckFreshness(os.Args[1:])) }

func runCheckFreshness(args []string) int {
	flags := flag.NewFlagSet("check-freshness", flag.ContinueOnError)
	maxAge := flags.Int("max-age-days", defaultMaxAge, "maximum age of a source review")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *maxAge < 0 {
		fmt.Fprintln(os.Stderr, "--max-age-days must be non-negative")
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "unexpected arguments")
		return 2
	}
	root := os.Getenv("MEGAPOWERS_ROOT")
	if root == "" {
		root = "."
	}
	data, err := os.ReadFile(filepath.Join(root, "docs", "harness-support.md"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	reviews, err := parseReviews(string(data))
	if err == nil {
		err = validateReviews(reviews, time.Now().UTC(), *maxAge, root)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "freshness:", err)
		return 1
	}
	fmt.Printf("freshness: %d source reviews current; source content and runtime behavior require the named oracles\n", len(reviews))
	return 0
}

func parseReviews(content string) ([]sourceReview, error) {
	_, rest, found := strings.Cut(content, sourceMarker)
	if !found {
		return nil, fmt.Errorf("docs/harness-support.md: missing source review records")
	}
	_, rest, found = strings.Cut(rest, "```json\n")
	if !found {
		return nil, fmt.Errorf("source reviews need a JSON block")
	}
	block, _, found := strings.Cut(rest, "\n```")
	if !found {
		return nil, fmt.Errorf("unterminated source reviews")
	}
	var reviews []sourceReview
	if err := strictjson.Decode([]byte(block), &reviews); err != nil {
		return nil, err
	}
	return reviews, nil
}

func validateReviews(reviews []sourceReview, now time.Time, maxAge int, root string) error {
	if len(reviews) == 0 {
		return fmt.Errorf("source reviews are empty")
	}
	seen, harnesses := map[string]bool{}, map[string]bool{}
	version := regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:[-+][A-Za-z0-9.-]+)?$`)
	for _, review := range reviews {
		u, err := url.Parse(review.Source)
		if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			return fmt.Errorf("invalid public source URL %q", review.Source)
		}
		if seen[review.Source] {
			return fmt.Errorf("duplicate source %s", review.Source)
		}
		seen[review.Source] = true
		if review.CLI != "claude" && review.CLI != "codex" {
			return fmt.Errorf("unsupported CLI %q", review.CLI)
		}
		harnesses[review.CLI] = true
		if !version.MatchString(review.Version) {
			return fmt.Errorf("%s: missing exact tested CLI version", review.Source)
		}
		then, err := time.Parse("2006-01-02", review.Reviewed)
		if err != nil {
			return fmt.Errorf("%s: invalid review date", review.Source)
		}
		if then.After(now) {
			return fmt.Errorf("%s: review date is in the future", review.Source)
		}
		if int(now.Sub(then)/(24*time.Hour)) > maxAge {
			return fmt.Errorf("%s: review is older than %d days; inspect current documentation and run its oracle", review.Source, maxAge)
		}
		if len(review.Oracle) == 0 {
			return fmt.Errorf("%s: missing compatibility oracle", review.Source)
		}
		for _, path := range review.Oracle {
			if !filepath.IsLocal(path) {
				return fmt.Errorf("%s: oracle must be repository-relative", review.Source)
			}
			info, err := os.Stat(filepath.Join(root, path))
			if err != nil || !info.Mode().IsRegular() {
				return fmt.Errorf("%s: oracle %q is missing", review.Source, path)
			}
		}
	}
	if !harnesses["claude"] || !harnesses["codex"] {
		return fmt.Errorf("source reviews must cover Claude Code and Codex")
	}
	return nil
}
