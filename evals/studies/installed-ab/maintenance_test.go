//go:build unix

package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const maintainedStore = `package store

import (
 "bufio"
 "encoding/json"
 "fmt"
 "os"
 "strings"
)

type Record struct { ID string ` + "`json:\"id\"`" + `; Category string ` + "`json:\"category\"`" + ` }

func Load(path string) ([]Record, error) {
 file, err := os.Open(path)
 if err != nil { return nil, err }
 defer file.Close()
 var records []Record
 scanner := bufio.NewScanner(file)
 for scanner.Scan() {
  if strings.TrimSpace(scanner.Text()) == "" { continue }
  var record Record
  if err := json.Unmarshal(scanner.Bytes(), &record); err != nil { return nil, fmt.Errorf("decode record: %w", err) }
  records = append(records, record)
 }
 if err := scanner.Err(); err != nil { return nil, err }
 return records, nil
}
`

const maintainedCatalog = `package catalog

import (
 "fmt"
 "sort"
 "example.com/records/store"
)

type Catalog struct { Path string }

func (c Catalog) List() ([]store.Record, error) { return store.Load(c.Path) }

func (c Catalog) Find(category string) ([]store.Record, error) {
 records, err := c.List()
 if err != nil { return nil, err }
 var selected []store.Record
 for _, record := range records {
  if category == "" || record.Category == category { selected = append(selected, record) }
 }
 return selected, nil
}

func (c Catalog) Paginate(category string, offset, limit int) ([]store.Record, error) {
 if offset < 0 || limit <= 0 { return nil, fmt.Errorf("invalid page bounds") }
 selected, err := c.Find(category)
 if err != nil { return nil, err }
 sort.Slice(selected, func(i, j int) bool { return selected[i].ID < selected[j].ID })
 if offset >= len(selected) { return nil, nil }
 selected = selected[offset:]
 if len(selected) > limit { selected = selected[:limit] }
 return selected, nil
}
`

func TestSuccessiveMaintenanceOraclePreservesEarlierBehavior(t *testing.T) {
	catalog, _, err := loadConfiguration("cases.json", "gates.json")
	if err != nil {
		t.Fatal(err)
	}
	var c studyCase
	for _, candidate := range catalog.Cases {
		if candidate.ID == "maintenance-record-catalog" {
			c = candidate
			break
		}
	}
	if c.ID == "" {
		t.Fatal("maintenance-record-catalog case missing")
	}
	if len(c.FollowupTasks) != 2 || len(c.ProtectedFiles) == 0 {
		t.Fatal("maintenance must use three real turns and protected acceptance tests")
	}
	dir := t.TempDir()
	if err := materializeFixture(dir, c.Files); err != nil {
		t.Fatal(err)
	}
	if rc, err := runOracle(context.Background(), dir, []string{"go", "test", "./..."}); err != nil || rc != 0 {
		t.Fatalf("baseline tests must start green: rc=%d err=%v", rc, err)
	}
	if rc, err := runOracle(context.Background(), dir, c.OracleCommand); err != nil || rc == 0 {
		t.Fatalf("unchanged fixture must fail acceptance: rc=%d err=%v", rc, err)
	}
	write := func(path, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, path), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("store/store.go", maintainedStore)
	write("catalog/catalog.go", maintainedCatalog)
	if rc, err := runOracle(context.Background(), dir, c.OracleCommand); err != nil || rc != 0 {
		t.Fatalf("complete implementation must pass: rc=%d err=%v", rc, err)
	}
	pageBeforeFilter := strings.Replace(maintainedCatalog, "selected, err := c.Find(category)", "selected, err := c.Find(\"\")", 1)
	pageBeforeFilter = strings.Replace(pageBeforeFilter,
		"if len(selected) > limit { selected = selected[:limit] }\n return selected, nil",
		"if len(selected) > limit { selected = selected[:limit] }\n var filtered []store.Record\n for _, record := range selected { if category == \"\" || record.Category == category { filtered = append(filtered, record) } }\n return filtered, nil", 1)
	for _, mutation := range []struct{ name, path, original, broken string }{
		{"malformed records", "store/store.go", "return nil, fmt.Errorf(\"decode record: %w\", err)", "_ = fmt.Sprintf(\"decode record: %v\", err); return records, nil"},
		{"filter preserved", "catalog/catalog.go", "category == \"\" || record.Category == category", "true"},
		{"stable ordering", "catalog/catalog.go", "selected[i].ID < selected[j].ID", "selected[i].ID > selected[j].ID"},
		{"offset preserved", "catalog/catalog.go", "selected = selected[offset:]", "selected = selected[:]"},
		{"pagination before filtering", "catalog/catalog.go", maintainedCatalog, pageBeforeFilter},
	} {
		t.Run(mutation.name, func(t *testing.T) {
			body := maintainedCatalog
			if mutation.path == "store/store.go" {
				body = maintainedStore
			}
			if !strings.Contains(body, mutation.original) {
				t.Fatal("mutation target missing")
			}
			write(mutation.path, strings.Replace(body, mutation.original, mutation.broken, 1))
			t.Cleanup(func() { write(mutation.path, body) })
			if rc, err := runOracle(context.Background(), dir, []string{"go", "test", "./..."}); err != nil || rc != 0 {
				t.Fatalf("mutation must compile and preserve baseline tests: rc=%d err=%v", rc, err)
			}
			if rc, err := runOracle(context.Background(), dir, c.OracleCommand); err != nil || rc == 0 {
				t.Fatalf("oracle accepted %s regression: rc=%d err=%v", mutation.name, rc, err)
			}
		})
	}

	t.Run("protected acceptance cannot be removed", func(t *testing.T) {
		write(c.ProtectedFiles[0], "//go:build acceptance\n\npackage catalog_test\n")
		intact, err := protectedFilesIntact(dir, c)
		if err != nil || intact {
			t.Fatalf("modified acceptance test was accepted: intact=%t err=%v", intact, err)
		}
	})
}
