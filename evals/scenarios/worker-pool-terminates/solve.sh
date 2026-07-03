#!/usr/bin/env bash
# Extract the worker-pool pattern shipped in golang-patterns and run it in the
# natural synchronous shape (call workerPool, then range results). If the fix
# regressed to an inline wg.Wait, this deadlocks and the timeout trips.
command -v go >/dev/null 2>&1 || { echo "GO_ABSENT"; exit 0; }
cat > go.mod <<'EOF'
module wptest
go 1.22
EOF
cat > main.go <<'EOF'
package main

import ("fmt"; "sync")

type Job int
type Result int
func processJob(j Job) Result { return Result(j * 2) }

// mirrors plugins/mega-go/skills/golang-patterns/SKILL.md
func workerPool(jobs <-chan Job, results chan<- Result, workers int) {
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				results <- processJob(job)
			}
		}()
	}
	go func() { wg.Wait(); close(results) }()
}

func main() {
	jobs := make(chan Job)
	results := make(chan Result)
	go func() { for i := 1; i <= 5; i++ { jobs <- Job(i) }; close(jobs) }()
	workerPool(jobs, results, 3)
	sum := 0
	for r := range results { sum += int(r) }
	fmt.Printf("SUM=%d\n", sum)
}
EOF
if timeout 40 go run main.go > run.out 2>&1; then
  cat run.out
else
  echo "RUN_TIMEOUT_OR_ERROR"; cat run.out
fi
