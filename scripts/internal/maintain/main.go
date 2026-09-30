package maintain

import (
	"context"
	"fmt"
	"io"

	"github.com/lawzava/megapowers/internal/gocache"
)

// Main runs the maintainer command and returns its process exit status.
func Main(ctx context.Context, root string, args []string, stdout, stderr io.Writer) int {
	if err := gocache.Ensure(); err != nil {
		fmt.Fprintln(stderr, "maintainer: no writable Go cache")
		return 2
	}
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: maintainer <validate|release|install-smoke|codex-install-smoke> [args]")
		return 2
	}
	switch args[0] {
	case "validate":
		return runValidate(ctx, root, args[1:], stdout, stderr)
	case "release":
		return runRelease(ctx, root, args[1:], stdout, stderr)
	case "install-smoke":
		return runInstallSmoke(ctx, root, args[1:], stdout, stderr)
	case "codex-install-smoke":
		return runCodexInstallSmoke(ctx, root, args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "maintainer: unknown command: %s\n", args[0])
		return 2
	}
}
