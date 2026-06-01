package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/fabiang/go-xyrun/internal/models"
	"github.com/fabiang/go-xyrun/internal/runner"
)

var version = "1.0.0" // Will match roughly with XyOps runner expectations

func main() {
	// Read job from stdin
	stdinBytes, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "xyRun Error: Failed to read from stdin: %v\n", err)
		os.Exit(1)
	}

	var job models.Job
	if err := json.Unmarshal(stdinBytes, &job); err != nil {
		fmt.Fprintf(os.Stderr, "xyRun Error: Failed to parse job JSON: %v\n", err)
		os.Exit(1)
	}

	app := runner.NewApp(&job, version)

	// Setup signal catching
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)
	go func() {
		<-sigs // wait for a signal
		app.AbortJob()
	}()

	app.Run()
}
