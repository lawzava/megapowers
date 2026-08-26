package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	maxManifestBytes = 1 << 20
	maxRecords       = 10000
	maxClaimBytes    = 8192
	maxDetailBytes   = 4096
	maxAgeDays       = 36500
)

type manifest struct {
	SchemaVersion string   `json:"schema_version"`
	Records       []record `json:"records"`
}

type record struct {
	ID         string `json:"id"`
	Claim      string `json:"claim"`
	Origin     string `json:"origin"`
	Evidence   string `json:"evidence"`
	Decision   string `json:"decision"`
	Source     string `json:"source,omitempty"`
	ObservedAt string `json:"observed_at,omitempty"`
	VerifiedAt string `json:"verified_at,omitempty"`
	Scope      string `json:"scope,omitempty"`
	Volatile   bool   `json:"volatile,omitempty"`
	MaxAgeDays *int   `json:"max_age_days,omitempty"`
}

type options struct {
	input string
	asOf  time.Time
}

var (
	idPattern             = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,127}$`)
	secretContentPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)-----BEGIN (?:RSA |EC |OPENSSH |DSA )?PRIVATE KEY-----`),
		regexp.MustCompile(`AKIA[0-9A-Z]{16}`),
		regexp.MustCompile(`gh[pousr]_[A-Za-z0-9]{30,}`),
		regexp.MustCompile(`sk-[A-Za-z0-9_-]{20,}`),
		regexp.MustCompile(`(?im)^\s*(?:export\s+)?(?:api[_-]?key|access[_-]?token|auth[_-]?token|password|passwd|client[_-]?secret|secret[_-]?key|aws_secret_access_key)\s*[:=]\s*["']?[A-Za-z0-9/+_.=-]{12,}`),
	}
	allowedEvidence = map[string]bool{
		"direct-statement":   true,
		"direct-observation": true,
		"source-backed":      true,
		"history-entry-only": true,
		"inferred":           true,
		"speculative":        true,
		"unknown":            true,
		"contested":          true,
	}
	hardFactEvidence = map[string]bool{
		"direct-statement":   true,
		"direct-observation": true,
		"source-backed":      true,
		"history-entry-only": true,
	}
	allowedDecisions = map[string]bool{
		"retain":     true,
		"quarantine": true,
		"revalidate": true,
		"remove":     true,
	}
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	opt, err := parseOptions(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "memory-hygiene: %v\n", err)
		return 2
	}
	m, err := loadManifest(opt.input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "memory-hygiene: %v\n", err)
		return 2
	}
	findings, counts := validateManifest(m, opt.asOf)
	if len(findings) != 0 {
		for _, finding := range findings {
			fmt.Fprintf(os.Stderr, "memory-hygiene: %s\n", finding)
		}
		fmt.Fprintf(os.Stderr, "memory-hygiene: %d finding(s)\n", len(findings))
		return 1
	}
	fmt.Printf("memory-hygiene: valid as of %s: %d records: %d retain, %d quarantine, %d revalidate, %d remove\n",
		opt.asOf.Format("2006-01-02"), len(m.Records), counts["retain"], counts["quarantine"], counts["revalidate"], counts["remove"])
	return 0
}

func parseOptions(args []string) (options, error) {
	var opt options
	var asOfRaw string
	seen := map[string]bool{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg != "--input" && arg != "--as-of" {
			return opt, fmt.Errorf("unknown arg: %s", arg)
		}
		if seen[arg] {
			return opt, fmt.Errorf("%s may be provided only once", arg)
		}
		seen[arg] = true
		if i+1 >= len(args) {
			return opt, fmt.Errorf("%s requires a value", arg)
		}
		i++
		switch arg {
		case "--input":
			opt.input = args[i]
		case "--as-of":
			asOfRaw = args[i]
		}
	}
	if strings.TrimSpace(opt.input) == "" {
		return opt, fmt.Errorf("--input is required")
	}
	if asOfRaw == "" {
		return opt, fmt.Errorf("--as-of is required")
	}
	asOf, err := time.Parse("2006-01-02", asOfRaw)
	if err != nil {
		return opt, fmt.Errorf("--as-of must use YYYY-MM-DD")
	}
	opt.asOf = asOf
	return opt, nil
}

func loadManifest(path string) (manifest, error) {
	var m manifest
	info, err := os.Lstat(path)
	if err != nil {
		return m, fmt.Errorf("cannot inspect input")
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return m, fmt.Errorf("symlink input rejected")
	}
	if !info.Mode().IsRegular() {
		return m, fmt.Errorf("input must be a regular file")
	}
	if info.Size() > maxManifestBytes {
		return m, fmt.Errorf("input exceeds %d-byte size limit", maxManifestBytes)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return m, fmt.Errorf("cannot read input")
	}
	if bytes.IndexByte(data, 0) >= 0 || !utf8.Valid(data) {
		return m, fmt.Errorf("input must be UTF-8 JSON")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&m); err != nil {
		return m, fmt.Errorf("invalid manifest: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return m, fmt.Errorf("invalid manifest: multiple JSON values")
		}
		return m, fmt.Errorf("invalid manifest: %w", err)
	}
	if len(m.Records) > maxRecords {
		return m, fmt.Errorf("manifest exceeds %d-record limit", maxRecords)
	}
	return m, nil
}

func validateManifest(m manifest, asOf time.Time) ([]string, map[string]int) {
	var findings []string
	counts := map[string]int{"retain": 0, "quarantine": 0, "revalidate": 0, "remove": 0}
	if m.SchemaVersion != "1" {
		findings = append(findings, "schema_version must be 1")
	}
	if m.Records == nil {
		findings = append(findings, "records array is required")
	}
	seen := map[string]bool{}
	for i, rec := range m.Records {
		label := fmt.Sprintf("record %d", i+1)
		validID := idPattern.MatchString(rec.ID)
		if !validID {
			findings = append(findings, label+": id must use lowercase letters, digits, '.', '_', or '-'")
		} else {
			label = fmt.Sprintf("record %q", rec.ID)
			if seen[rec.ID] {
				findings = append(findings, label+": duplicate id")
			}
			seen[rec.ID] = true
		}
		if allowedDecisions[rec.Decision] {
			counts[rec.Decision]++
		} else {
			findings = append(findings, label+": unsupported decision")
		}
		findings = append(findings, validateRecord(label, rec, asOf)...)
	}
	return findings, counts
}

func validateRecord(label string, rec record, asOf time.Time) []string {
	var findings []string
	if blank(rec.Claim) {
		findings = append(findings, label+": claim is required")
	} else if len(rec.Claim) > maxClaimBytes {
		findings = append(findings, label+": claim exceeds size limit")
	}
	if blank(rec.Origin) {
		findings = append(findings, label+": origin is required")
	}
	for _, detail := range []struct {
		field string
		value string
	}{
		{field: "origin", value: rec.Origin},
		{field: "source", value: rec.Source},
		{field: "scope", value: rec.Scope},
	} {
		if len(detail.value) > maxDetailBytes {
			findings = append(findings, fmt.Sprintf("%s: %s exceeds size limit", label, detail.field))
		}
	}
	if hasSecret(rec) {
		findings = append(findings, label+": secret-like content rejected")
	}
	if !allowedEvidence[rec.Evidence] {
		findings = append(findings, label+": unsupported evidence class")
	}
	activeDecision := rec.Decision == "retain" || rec.Decision == "revalidate"
	if activeDecision {
		if blank(rec.Source) {
			findings = append(findings, label+": retain or revalidate record requires source")
		}
		if blank(rec.ObservedAt) {
			findings = append(findings, label+": retain or revalidate record requires observed_at")
		}
		if blank(rec.Scope) {
			findings = append(findings, label+": retain or revalidate record requires scope")
		}
	}
	if rec.Decision == "retain" && allowedEvidence[rec.Evidence] && !hardFactEvidence[rec.Evidence] {
		findings = append(findings, fmt.Sprintf("%s: cannot retain %s evidence", label, rec.Evidence))
	}

	observedAt, observedOK, observedFinding := parseRecordDate(label, "observed_at", rec.ObservedAt, asOf)
	if observedFinding != "" {
		findings = append(findings, observedFinding)
	}
	verifiedAt, verifiedOK, verifiedFinding := parseRecordDate(label, "verified_at", rec.VerifiedAt, asOf)
	if verifiedFinding != "" {
		findings = append(findings, verifiedFinding)
	}
	if observedOK && verifiedOK && verifiedAt.Before(observedAt) {
		findings = append(findings, label+": verified_at cannot precede observed_at")
	}
	if rec.Decision == "retain" && rec.Evidence == "source-backed" && !verifiedOK {
		findings = append(findings, label+": retained source-backed record requires verified_at")
	}
	if rec.Volatile {
		if rec.MaxAgeDays == nil || *rec.MaxAgeDays <= 0 {
			findings = append(findings, label+": volatile record requires positive max_age_days")
		} else if *rec.MaxAgeDays > maxAgeDays {
			findings = append(findings, fmt.Sprintf("%s: max_age_days cannot exceed %d", label, maxAgeDays))
		}
		if rec.Decision == "retain" && !verifiedOK {
			findings = append(findings, label+": retained volatile record requires verified_at")
		}
		if rec.Decision == "retain" && verifiedOK && rec.MaxAgeDays != nil && *rec.MaxAgeDays > 0 {
			age := int(asOf.Sub(verifiedAt).Hours() / 24)
			if age > *rec.MaxAgeDays {
				findings = append(findings, fmt.Sprintf("%s: volatile fact expired %d days after verification (max %d)", label, age, *rec.MaxAgeDays))
			}
		}
	} else if rec.MaxAgeDays != nil {
		findings = append(findings, label+": max_age_days requires volatile true")
	}
	return findings
}

func parseRecordDate(label, field, raw string, asOf time.Time) (time.Time, bool, string) {
	if blank(raw) {
		return time.Time{}, false, ""
	}
	parsed, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return time.Time{}, false, fmt.Sprintf("%s: %s must use YYYY-MM-DD", label, field)
	}
	if parsed.After(asOf) {
		return parsed, true, fmt.Sprintf("%s: %s is after as-of date", label, field)
	}
	return parsed, true, ""
}

func hasSecret(rec record) bool {
	data := []byte(strings.Join([]string{rec.Claim, rec.Origin, rec.Source, rec.Scope}, "\n"))
	for _, pattern := range secretContentPatterns {
		if pattern.Match(data) {
			return true
		}
	}
	return false
}

func blank(value string) bool {
	return strings.TrimSpace(value) == ""
}
