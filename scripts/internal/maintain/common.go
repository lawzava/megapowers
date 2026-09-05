package maintain

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

func command(ctx context.Context, root string, stdout, stderr io.Writer, env []string, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = root
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.WaitDelay = 5 * time.Second
	if env != nil {
		cmd.Env = mergedEnv(env)
	}
	return cmd.Run()
}

func output(ctx context.Context, root string, env []string, name string, args ...string) ([]byte, error) {
	buffer := cappedBuffer{limit: 10 << 20}
	if err := command(ctx, root, &buffer, io.Discard, env, name, args...); err != nil {
		return nil, err
	}
	if buffer.overflow {
		return nil, errors.New("subprocess output exceeded 10 MiB")
	}
	return buffer.Bytes(), nil
}

type cappedBuffer struct {
	bytes.Buffer
	limit    int
	overflow bool
}

func (b *cappedBuffer) Write(data []byte) (int, error) {
	written := len(data)
	remaining := b.limit - b.Len()
	if len(data) > remaining {
		b.overflow = true
	}
	if remaining > 0 {
		if len(data) > remaining {
			data = data[:remaining]
		}
		_, _ = b.Buffer.Write(data)
	}
	return written, nil
}

func mergedEnv(overrides []string) []string {
	keys := make(map[string]bool, len(overrides))
	for _, override := range overrides {
		if key, _, ok := strings.Cut(override, "="); ok {
			keys[key] = true
		}
	}
	env := make([]string, 0, len(os.Environ())+len(overrides))
	for _, current := range os.Environ() {
		key, _, ok := strings.Cut(current, "=")
		if ok && keys[key] {
			continue
		}
		env = append(env, current)
	}
	return append(env, overrides...)
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode()
	}
	return 125
}

func privateTempDir(prefix string) (string, error) {
	base := os.Getenv("TMPDIR")
	if base == "" {
		base = os.TempDir()
	}
	dir, err := os.MkdirTemp(base, prefix+"-")
	if err != nil {
		return "", err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		os.RemoveAll(dir)
		return "", err
	}
	return dir, nil
}

func validReleaseVersion(version string) bool {
	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		return false
	}
	for _, part := range parts {
		if part == "" || (len(part) > 1 && part[0] == '0') {
			return false
		}
		for _, r := range part {
			if r < '0' || r > '9' {
				return false
			}
		}
	}
	return true
}

func versionAtLeast(version, minimum string) bool {
	parse := func(value string) ([]int, bool) {
		parts := strings.Split(value, ".")
		if len(parts) < 2 {
			return nil, false
		}
		out := make([]int, len(parts))
		for i, part := range parts {
			if part == "" {
				return nil, false
			}
			value, err := strconv.Atoi(part)
			if err != nil || value < 0 {
				return nil, false
			}
			out[i] = value
		}
		return out, true
	}
	got, ok := parse(version)
	if !ok {
		return false
	}
	want, ok := parse(minimum)
	if !ok {
		return false
	}
	n := len(got)
	if len(want) > n {
		n = len(want)
	}
	for i := 0; i < n; i++ {
		var a, b int
		if i < len(got) {
			a = got[i]
		}
		if i < len(want) {
			b = want[i]
		}
		if a > b {
			return true
		}
		if a < b {
			return false
		}
	}
	return true
}

type treeEntry struct {
	mode fs.FileMode
	data []byte
}

func readTree(root string) (map[string]treeEntry, error) {
	entries := make(map[string]treeEntry)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("non-regular tree entry: %s", path)
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		entries[filepath.ToSlash(rel)] = treeEntry{mode: info.Mode().Perm(), data: data}
		return nil
	})
	return entries, err
}

func compareTrees(source, installed string) error {
	want, err := readTree(source)
	if err != nil {
		return err
	}
	got, err := readTree(installed)
	if err != nil {
		return err
	}
	if len(want) != len(got) {
		return fmt.Errorf("file count differs: source=%d installed=%d", len(want), len(got))
	}
	for name, sourceEntry := range want {
		installedEntry, ok := got[name]
		if !ok {
			return fmt.Errorf("installed tree is missing %s", name)
		}
		if sourceEntry.mode != installedEntry.mode {
			return fmt.Errorf("mode differs for %s", name)
		}
		if !bytes.Equal(sourceEntry.data, installedEntry.data) {
			return fmt.Errorf("bytes differ for %s", name)
		}
	}
	return nil
}

func resultsOK(data []byte, strict bool) bool {
	hasPass := false
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		fields := strings.SplitN(line, "\t", 3)
		if len(fields) < 2 {
			continue
		}
		switch fields[1] {
		case "FAIL":
			return false
		case "SKIP":
			if strict {
				return false
			}
		case "PASS":
			hasPass = true
		}
	}
	return hasPass
}

func regularFiles(root string, suffixes ...string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		for _, suffix := range suffixes {
			if strings.HasSuffix(entry.Name(), suffix) {
				files = append(files, path)
				break
			}
		}
		return nil
	})
	sort.Strings(files)
	return files, err
}

func executable(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func deadline(parent context.Context, duration time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, duration)
}
