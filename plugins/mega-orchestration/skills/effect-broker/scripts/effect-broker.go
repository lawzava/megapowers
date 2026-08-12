package main

import (
	"fmt"
	"os"
	"strings"
)

func ebUsage() {
	fmt.Fprintln(os.Stderr, "usage: effect-broker <reversible|staged|irreversible> [--level autonomous|on-the-loop|in-the-loop]")
}

func main() {
	os.Exit(runEffectBroker(os.Args[1:]))
}

func runEffectBroker(args []string) int {
	level := "on-the-loop"
	class := ""
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch a {
		case "--level":
			if i+1 >= len(args) {
				ebUsage()
				return 2
			}
			i++
			level = args[i]
		case "-h", "--help":
			ebUsage()
			return 0
		default:
			if strings.HasPrefix(a, "-") {
				ebUsage()
				return 2
			}
			if class != "" {
				fmt.Fprintf(os.Stderr, "effect-broker: unexpected extra argument '%s'\n", a)
				ebUsage()
				return 2
			}
			class = a
		}
	}
	switch class {
	case "reversible", "staged", "irreversible":
	default:
		fmt.Fprintln(os.Stderr, "effect-broker: class must be reversible|staged|irreversible")
		return 2
	}
	switch level {
	case "autonomous", "on-the-loop", "in-the-loop":
	default:
		fmt.Fprintf(os.Stderr, "effect-broker: unknown level '%s'\n", level)
		return 2
	}
	fmt.Printf("CLASS=%s\n", class)
	fmt.Printf("LEVEL=%s\n", level)
	switch class {
	case "reversible":
		fmt.Print("DRY_RUN=n/a\nIDEMPOTENCY=n/a\nJOURNAL=optional\nAPPROVAL=none\nPROCEED=yes\n")
	case "staged":
		approval := "none"
		if level == "in-the-loop" {
			approval = "required"
		}
		fmt.Printf("DRY_RUN=required\nIDEMPOTENCY=recommended-if-supported\nBLAST_RADIUS=caller-enforced-vs-charter\nJOURNAL=required\nAPPROVAL=%s\nPROCEED=after-dry-run-diff\n", approval)
	case "irreversible":
		fmt.Print("DRY_RUN=required\nIDEMPOTENCY=required-if-supported\nBLAST_RADIUS=caller-enforced-vs-charter\nJOURNAL=required\nAPPROVAL=required\nPROCEED=after-staged-approval\n")
	}
	return 0
}
