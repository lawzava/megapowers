// opencode-export creates offline session exports for opencode-mine. Repository tool only.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type options struct {
	opencodeCmd     string
	opencodeTimeout time.Duration
	output          string
	workspace       string
	all             bool
	since           time.Time
}
type scanStats struct {
	opencodeStatus      string
	opencodeEnvironment string
	partialSources      int
	skippedSources      int
	skippedSessions     int
	notes               []string
}
type openCodeInfo struct {
	ID       string `json:"id"`
	ParentID string `json:"parentID"`
	Location struct {
		Directory string `json:"directory"`
	} `json:"location"`
	Time struct {
		Updated int64 `json:"updated"`
	} `json:"time"`
}

// openCodeGET uses only the CLI's public GET interface and existing configuration.
// A regular file, not a pipe, avoids CLI stdout truncation on large exports.
// This program opens no OpenCode files directly.
func openCodeGET(parent context.Context, opt options, env []string, route string, result any) error {
	file, err := os.CreateTemp("", "session-mine-opencode-*.json")
	if err != nil {
		return fmt.Errorf("cannot create temporary API output")
	}
	defer os.Remove(file.Name())
	defer file.Close()
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, opt.opencodeCmd, "api", "get", route)
	cmd.Env, cmd.WaitDelay = env, time.Second
	cmd.Stdout = file
	// CLI diagnostics can include unredacted content. Report the failed
	// operation, not raw stdout/stderr, and leave diagnosis to a separate run.
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("API GET timed out: %w", ctx.Err())
		}
		if errors.Is(err, exec.ErrWaitDelay) {
			return fmt.Errorf("API GET pipe wait timed out: %w", exec.ErrWaitDelay)
		}
		return fmt.Errorf("API GET failed (check service status and command environment)")
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("cannot read temporary API output")
	}
	decoder := json.NewDecoder(openCodeReader{ctx, file})
	if err := decoder.Decode(result); err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("API decode timed out: %w", ctx.Err())
		}
		return fmt.Errorf("invalid or truncated API JSON")
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if ctx.Err() != nil {
			return fmt.Errorf("API decode timed out: %w", ctx.Err())
		}
		return fmt.Errorf("extra API output after JSON")
	}
	return nil
}

type openCodeReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r openCodeReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}

func openCodeTimedOut(err error) bool {
	return errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) || errors.Is(err, exec.ErrWaitDelay)
}

func openCodeStatus(parent context.Context, opt options, env []string) (bool, error) {
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, opt.opencodeCmd, "service", "status")
	cmd.Env, cmd.WaitDelay = env, time.Second
	status, err := cmd.Output()
	if ctx.Err() != nil {
		return false, fmt.Errorf("service status timed out: %w", ctx.Err())
	}
	if errors.Is(err, exec.ErrWaitDelay) {
		return false, fmt.Errorf("service status pipe wait timed out: %w", exec.ErrWaitDelay)
	}
	if err != nil {
		return false, fmt.Errorf("CLI unavailable or service status failed")
	}
	server, err := url.Parse(strings.TrimSpace(string(status)))
	return err == nil && server.Host != "" && (server.Scheme == "http" || server.Scheme == "https"), nil
}

// Retry malformed/empty responses once, but retain a valid empty snapshot if
// warm-up's retry fails. A timeout is terminal even when an empty snapshot exists.
func openCodeRead[T any](ctx context.Context, opt options, env []string, route string, check func(T) (valid, empty bool)) (T, error) {
	var kept T
	var err error
	haveValid := false
	for attempt := 0; attempt < 2; attempt++ {
		var candidate T
		err = openCodeGET(ctx, opt, env, route, &candidate)
		if err == nil {
			valid, empty := check(candidate)
			if valid {
				kept, haveValid = candidate, true
				if !empty {
					return kept, nil
				}
			} else {
				err = fmt.Errorf("missing or invalid API data")
			}
		}
		if openCodeTimedOut(err) {
			return kept, err
		}
	}
	if haveValid {
		return kept, nil
	}
	return kept, err
}

type openCodePage struct {
	Data   *[]openCodeInfo `json:"data"`
	Cursor struct {
		Next string `json:"next"`
	} `json:"cursor"`
}

type openCodeExport struct {
	Data *struct {
		Info     openCodeInfo       `json:"info"`
		Messages *[]json.RawMessage `json:"messages"`
	} `json:"data"`
}

func exportOpenCode(opt options, stats *scanStats) int {
	written := 0
	ctx, cancel := context.WithTimeout(context.Background(), opt.opencodeTimeout)
	defer cancel()
	stats.opencodeStatus, stats.opencodeEnvironment = "complete", "inherited"
	problem := func(reason string, partial bool) {
		status := "skipped"
		if partial || written > 0 || ctx.Err() != nil {
			status = "partial"
		}
		if stats.opencodeStatus == "complete" {
			if status == "partial" {
				stats.partialSources++
			} else {
				stats.skippedSources++
			}
			stats.opencodeStatus = status
		}
		stats.notes = append(stats.notes, "opencode source "+status+": "+reason+"; check service status and --opencode-cmd environment")
	}
	// Do not ask the CLI to discover/start a service when none is running.
	// The CLI needs only executable discovery and local directory selection.
	env := []string{}
	for _, name := range []string{
		"PATH", "HOME", "USERPROFILE", "APPDATA", "LOCALAPPDATA",
		"XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME", "XDG_CACHE_HOME",
		"XDG_RUNTIME_DIR", "TMPDIR",
	} {
		if value, ok := os.LookupEnv(name); ok {
			env = append(env, name+"="+value)
		}
	}
	running, err := openCodeStatus(ctx, opt, env)
	if !running && err == nil && os.Getenv("XDG_STATE_HOME") != "" {
		fallback := make([]string, 0, len(env))
		for _, value := range env {
			if key, _, _ := strings.Cut(value, "="); key != "XDG_STATE_HOME" {
				fallback = append(fallback, value)
			}
		}
		env = fallback
		stats.opencodeEnvironment = "without_xdg_state_home"
		stats.notes = append(stats.notes, "opencode environment: retrying without XDG_STATE_HOME after no running service in inherited environment")
		running, err = openCodeStatus(ctx, opt, env)
	}
	if !running || err != nil {
		reason := "no running service in this environment"
		if err != nil {
			reason = err.Error()
		}
		problem(reason, false)
		return written
	}
	query := url.Values{"limit": {"100"}, "order": {"desc"}}
	seenCursors, seenIDs := map[string]bool{}, map[string]bool{}
	for {
		// Some servers return an empty first result while warming a location.
		// Retry once, immediately, rather than silently trusting that result.
		page, err := openCodeRead(ctx, opt, env, "/api/session?"+query.Encode(), func(page openCodePage) (bool, bool) {
			return page.Data != nil, page.Data != nil && len(*page.Data) == 0
		})
		if err != nil {
			problem(err.Error(), len(seenCursors) > 0)
			break
		}
		for _, info := range *page.Data {
			if ctx.Err() != nil {
				problem("overall source deadline exceeded", true)
				return written
			}
			if info.ID == "" {
				stats.skippedSessions++
				problem("session missing ID", true)
				continue
			}
			if seenIDs[info.ID] {
				continue
			}
			seenIDs[info.ID] = true
			if info.Location.Directory == "" || (!opt.all && filepath.Clean(info.Location.Directory) != opt.workspace) {
				continue
			}
			if !opt.since.IsZero() && info.Time.Updated > 0 && time.UnixMilli(info.Time.Updated).Before(opt.since) {
				continue
			}
			exported, err := openCodeRead(ctx, opt, env, "/api/experimental/session/"+url.PathEscape(info.ID)+"/export", func(exported openCodeExport) (bool, bool) {
				valid := exported.Data != nil && exported.Data.Messages != nil && exported.Data.Info.ID == info.ID
				return valid, valid && len(*exported.Data.Messages) == 0
			})
			if err != nil {
				stats.skippedSessions++
				problem("session export unavailable: "+err.Error(), true)
				if openCodeTimedOut(err) {
					return written
				}
				continue
			}
			// The session may have moved since listing; recheck the export's scope.
			if exported.Data.Info.Location.Directory == "" || (!opt.all && filepath.Clean(exported.Data.Info.Location.Directory) != opt.workspace) {
				continue
			}
			data, err := json.Marshal(exported)
			if err == nil {
				sum := sha256.Sum256([]byte(info.ID))
				err = os.WriteFile(filepath.Join(opt.output, fmt.Sprintf("%x.json", sum)), data, 0600)
			}
			if err != nil {
				stats.skippedSessions++
				problem("cannot write export", true)
				continue
			}
			written++
		}
		next := page.Cursor.Next
		if next == "" {
			break
		}
		if seenCursors[next] {
			problem("repeated pagination cursor", true)
			break
		}
		seenCursors[next] = true
		query.Set("cursor", next)
	}
	return written
}

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }
func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("opencode-export", flag.ContinueOnError)
	fs.SetOutput(stderr)
	opt := options{}
	command := os.Getenv("SESSION_MINE_OPENCODE_CMD")
	if command == "" {
		command = "opencode"
	}
	timeout := os.Getenv("SESSION_MINE_OPENCODE_TIMEOUT")
	if timeout == "" {
		timeout = "5m"
	}
	fs.StringVar(&opt.opencodeCmd, "opencode-cmd", command, "OpenCode executable")
	fs.StringVar(&timeout, "opencode-timeout", timeout, "overall deadline (positive Go duration)")
	fs.StringVar(&opt.output, "output", "", "new or empty export directory (required)")
	fs.StringVar(&opt.workspace, "workspace", "", "session directory to export")
	fs.BoolVar(&opt.all, "all-workspaces", false, "export all directories")
	var since string
	fs.StringVar(&since, "since", "", "skip sessions updated before YYYY-MM-DD")
	if fs.Parse(args) != nil {
		return 2
	}
	var err error
	opt.opencodeTimeout, err = time.ParseDuration(timeout)
	if err != nil || opt.opencodeTimeout <= 0 {
		fmt.Fprintln(stderr, "opencode-export: timeout must be a positive duration")
		return 2
	}
	if opt.output == "" || strings.TrimSpace(opt.opencodeCmd) == "" || fs.NArg() != 0 {
		fmt.Fprintln(stderr, "usage: opencode-export --output DIR (--workspace DIR | --all-workspaces) [--since YYYY-MM-DD] [--opencode-cmd FILE] [--opencode-timeout 5m]")
		return 2
	}
	if (opt.workspace == "" && !opt.all) || (opt.workspace != "" && opt.all) {
		fmt.Fprintln(stderr, "select --workspace DIR or --all-workspaces")
		return 2
	}
	if opt.workspace != "" {
		opt.workspace, err = filepath.Abs(opt.workspace)
		if err != nil {
			fmt.Fprintln(stderr, "cannot resolve workspace")
			return 2
		}
	}
	if since != "" {
		opt.since, err = time.Parse("2006-01-02", since)
		if err != nil {
			fmt.Fprintln(stderr, "--since must use YYYY-MM-DD")
			return 2
		}
	}
	if err := validateOutput(opt.output); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if err = os.MkdirAll(opt.output, 0700); err != nil {
		fmt.Fprintln(stderr, "cannot create export directory")
		return 1
	}
	entries, err := os.ReadDir(opt.output)
	if err != nil || len(entries) != 0 {
		fmt.Fprintln(stderr, "export directory must be empty")
		return 1
	}
	stats := &scanStats{}
	// An interrupted export must never look like a complete empty snapshot.
	if err := writeStatus(opt.output, "partial", 0, "inherited"); err != nil {
		fmt.Fprintln(stderr, "cannot write export-status")
		return 1
	}
	written := exportOpenCode(opt, stats)
	for _, note := range stats.notes {
		fmt.Fprintln(stderr, note)
	}
	if err := writeStatus(opt.output, stats.opencodeStatus, stats.skippedSessions, stats.opencodeEnvironment); err != nil {
		fmt.Fprintln(stderr, "cannot write export-status")
		return 1
	}
	json.NewEncoder(stdout).Encode(map[string]any{"exports_written": written, "status": stats.opencodeStatus, "environment": stats.opencodeEnvironment, "partial_sources": stats.partialSources, "skipped_sources": stats.skippedSources, "skipped_sessions": stats.skippedSessions})
	if stats.opencodeStatus != "complete" {
		return 1
	}
	return 0
}

func writeStatus(dir, status string, skipped int, environment string) error {
	data, err := json.Marshal(map[string]any{"status": status, "skipped_sessions": skipped, "environment": environment})
	if err != nil {
		return err
	}
	// Replace the initial partial marker atomically so readers never see truncated JSON.
	file, err := os.CreateTemp(dir, ".export-status-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err = file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), filepath.Join(dir, "export-status"))
}

func validateOutput(output string) error {
	if !filepath.IsAbs(output) {
		return fmt.Errorf("--output must be an absolute path outside the checkout")
	}
	// The tool is built from this module; use its source location rather than
	// the caller's cwd, which can be elsewhere when running the compiled binary.
	_, source, _, ok := runtime.Caller(0)
	if !ok || !filepath.IsAbs(source) {
		return fmt.Errorf("cannot determine exporter module root")
	}
	root, err := filepath.EvalSymlinks(filepath.Join(filepath.Dir(source), "..", "..", ".."))
	if err != nil {
		return fmt.Errorf("cannot resolve exporter module root")
	}
	// Resolve existing ancestors too, so a symlink cannot hide an in-checkout output.
	resolved := filepath.Clean(output)
	var suffix []string
	for {
		canonical, err := filepath.EvalSymlinks(resolved)
		if err == nil {
			resolved = canonical
			for i := len(suffix) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, suffix[i])
			}
			break
		}
		if !os.IsNotExist(err) || filepath.Dir(resolved) == resolved {
			return fmt.Errorf("cannot resolve output directory")
		}
		suffix = append(suffix, filepath.Base(resolved))
		resolved = filepath.Dir(resolved)
	}
	rel, err := filepath.Rel(root, resolved)
	if err != nil || rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))) {
		return fmt.Errorf("--output must be outside the checkout")
	}
	return nil
}
