package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func hiddenCase(t *testing.T) studyCase {
	t.Helper()
	var c studyCase
	if err := json.Unmarshal([]byte(`{"id":"hidden","kind":"workflow","task":"fix","required_facts":["fixed"],"required_skill_order":["test-first-implementation"],"files":{"go.mod":"module example.com/hidden\n\ngo 1.24\n","value.go":"package hidden\nfunc Value() int { return 0 }\n"},"hidden_files":{"hidden_test.go":"package hidden\nimport \"testing\"\nfunc TestHidden(t *testing.T) { if Value() != 7 { t.Fatal(Value()) } }\n"},"oracle_command":["go","test","./..."]}`), &c); err != nil {
		t.Fatal(err)
	}
	return c
}

func TestHiddenOracleIsAbsentFromActorAndBindsFixture(t *testing.T) {
	c := hiddenCase(t)
	dir := t.TempDir()
	if err := materializeCaseFixture(dir, c); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "hidden_test.go")); !os.IsNotExist(err) {
		t.Fatalf("hidden test exposed: %v", err)
	}
	// Decode without hidden_files to exercise the public serialization contract.
	data, _ := json.Marshal(c)
	var fields map[string]json.RawMessage
	_ = json.Unmarshal(data, &fields)
	delete(fields, "hidden_files")
	data, _ = json.Marshal(fields)
	var plain studyCase
	_ = json.Unmarshal(data, &plain)
	if caseFixtureHash(c) == caseFixtureHash(plain) {
		t.Fatal("hidden oracle not bound into fixture identity")
	}
	rc, _, err := runCaseOracleDetailed(context.Background(), dir, c)
	if err != nil || rc == 0 {
		t.Fatalf("missing behavior accepted: rc=%d err=%v", rc, err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".actor-cache", "go-build")); err != nil {
		t.Fatalf("hidden oracle did not reuse the project-private build cache: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "value.go"), []byte("package hidden\nfunc Value() int { return 7 }\n"), 0600); err != nil {
		t.Fatal(err)
	}
	rc, out, err := runCaseOracleDetailed(context.Background(), dir, c)
	if err != nil || rc != 0 {
		t.Fatalf("reference rejected: rc=%d err=%v\n%s", rc, err, out)
	}
	if _, err := os.Stat(filepath.Join(dir, "hidden_test.go")); !os.IsNotExist(err) {
		t.Fatalf("hidden test leaked after grading: %v", err)
	}
}

func TestHiddenOracleRejectsActorCollisionAndSymlink(t *testing.T) {
	for _, symlink := range []bool{false, true} {
		c := hiddenCase(t)
		dir := t.TempDir()
		if err := materializeCaseFixture(dir, c); err != nil {
			t.Fatal(err)
		}
		if symlink {
			if err := os.Symlink(t.TempDir(), filepath.Join(dir, "escape")); err != nil {
				t.Fatal(err)
			}
		} else if err := os.WriteFile(filepath.Join(dir, "hidden_test.go"), []byte("package hidden\n"), 0600); err != nil {
			t.Fatal(err)
		}
		rc, _, err := runCaseOracleDetailed(context.Background(), dir, c)
		if err != nil || rc == 0 {
			t.Fatalf("unsafe actor snapshot accepted: rc=%d err=%v", rc, err)
		}
	}
}

func TestHiddenOracleConfiguration(t *testing.T) {
	_, gates, err := loadConfiguration("cases.json", "gates.json")
	if err != nil {
		t.Fatal(err)
	}
	base := hiddenCase(t)
	for _, field := range []string{"../escape_test.go", "value.go", "not-a-test.txt"} {
		data, _ := json.Marshal(base)
		var fields map[string]json.RawMessage
		_ = json.Unmarshal(data, &fields)
		fields["hidden_files"], _ = json.Marshal(map[string]string{field: "package hidden\n"})
		data, _ = json.Marshal(fields)
		var c studyCase
		_ = json.Unmarshal(data, &c)
		if err := validateConfiguration(casesFile{SchemaVersion: "1", Cases: []studyCase{c}}, gates); err == nil || !strings.Contains(err.Error(), "hidden") {
			t.Errorf("accepted hidden file %q: %v", field, err)
		}
	}
}
