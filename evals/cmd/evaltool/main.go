package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/lawzava/megapowers/evals/internal/evaltool"
)

func main() {
	root := os.Getenv("MEGAPOWERS_ROOT")
	if root == "" {
		wd, err := os.Getwd()
		if err != nil {
			fmt.Fprintln(os.Stderr, "evaltool: cannot determine repository root")
			os.Exit(2)
		}
		root = wd
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "evaltool: cannot resolve repository root")
		os.Exit(2)
	}
	args := os.Args[1:]
	if caller := os.Getenv("MEGAPOWERS_CALLER_CWD"); caller != "" {
		args = resolvePaths(caller, args)
	}
	os.Exit(evaltool.Main(context.Background(), absolute, args, os.Stdout, os.Stderr))
}

func resolvePaths(caller string, args []string) []string {
	out := append([]string(nil), args...)
	for i := 1; i < len(out); i++ {
		pathArg := (out[0] == "run-all" && out[i] == "--json" && i+1 < len(out)) ||
			(out[0] == "check-portability-boundary" && i == 1)
		if !pathArg {
			continue
		}
		index := i
		if out[i] == "--json" {
			index = i + 1
		}
		if !filepath.IsAbs(out[index]) {
			out[index] = filepath.Join(caller, out[index])
		}
		if index != i {
			i++
		}
	}
	return out
}
