package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/lawzava/megapowers/scripts/internal/maintain"
)

func main() {
	root := os.Getenv("MEGAPOWERS_ROOT")
	if root == "" {
		wd, err := os.Getwd()
		if err != nil {
			fmt.Fprintln(os.Stderr, "maintainer: cannot determine repository root")
			os.Exit(2)
		}
		root = wd
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "maintainer: cannot resolve repository root")
		os.Exit(2)
	}
	args := os.Args[1:]
	if caller := os.Getenv("MEGAPOWERS_CALLER_CWD"); caller != "" && len(args) > 0 && args[0] == "install-smoke" {
		args = resolveSmokePaths(caller, args)
	}
	os.Exit(maintain.Main(context.Background(), absolute, args, os.Stdout, os.Stderr))
}

func resolveSmokePaths(caller string, args []string) []string {
	out := append([]string(nil), args...)
	for i := 1; i+1 < len(out); i++ {
		if out[i] != "--out" && out[i] != "--repo" {
			continue
		}
		if !filepath.IsAbs(out[i+1]) {
			out[i+1] = filepath.Join(caller, out[i+1])
		}
		i++
	}
	return out
}
