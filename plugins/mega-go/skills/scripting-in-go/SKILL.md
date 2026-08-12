---
name: scripting-in-go
description: >
  Use when writing a script, glue, probe, transform, one-off tool call, or
  go run helper, or when tempted to write bash, Python, or Node for agent glue.
license: MIT
---

# Scripting in Go

Agent-authored glue is Go. Application code still matches the project. A
Python or TypeScript repo does not make the next helper Python or Node.

## When not

- Invoking an existing CLI as-is (`git`, `rg`, `go test`).
- Editing a file that is already bash or JavaScript because a hook protocol
  or plugin runtime requires that language.
- Building a real Go module or service: mega-go:greenfield-go-stack.

## Pattern

One stdlib `package main` in scratch. No `go.mod` unless a dependency is
required. Delete the file before the turn ends.

```bash
scratch="${TMPDIR:-/tmp}/agent-$$" && mkdir -p "$scratch"
```

```go
package main

import (
	"bufio"
	"fmt"
	"os"
)

func main() {
	n := 0
	sc := bufio.NewScanner(os.Stdin)
	for sc.Scan() {
		n++
	}
	fmt.Println(n)
}
```

```text
go run "$scratch/count.go" < events.jsonl
```

## Common mistakes

- Writing Python or Node because the surrounding project is Python or Node.
- A multi-line bash script for JSON, CSV, or file rewriting.
- Leaving the `.go` file in the working tree.
- Scaffolding a module for a twenty-line helper.
