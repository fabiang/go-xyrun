//go:build integration

package runner_test

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// binaryPath returns the path to the xyrun binary to test against.
// It prefers a pre-built binary in the repo root; if absent it builds one on
// the fly into a temp dir so the tests are self-contained.
func binaryPath(t *testing.T) string {
	t.Helper()

	root := moduleRoot(t)
	candidate := filepath.Join(root, "xyrun")
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}

	tmp := t.TempDir()
	out := filepath.Join(tmp, "xyrun")
	cmd := exec.Command("go", "build", "-o", out, "./cmd/xyrun")
	cmd.Dir = root
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build xyrun: %v\n%s", err, b)
	}
	return out
}

// moduleRoot walks upward until go.mod is found.
func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not locate go.mod (module root)")
		}
		dir = parent
	}
}

// runBinary pipes jobJSON to the binary and returns combined stdout+stderr,
// stdout alone, and the exit code.
func runBinary(t *testing.T, bin, jobJSON string, extraArgs ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	cmd := exec.Command(bin, extraArgs...)
	cmd.Stdin = strings.NewReader(jobJSON)

	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err := cmd.Run()
	exitCode = 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			exitCode = ee.ExitCode()
		}
	}
	return outBuf.String(), errBuf.String(), exitCode
}

// lastJSON returns the last JSON object found in s (searches stdout lines).
func lastJSON(t *testing.T, s string) map[string]interface{} {
	t.Helper()
	var last map[string]interface{}
	for _, line := range strings.Split(strings.TrimSpace(s), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "{") && strings.HasSuffix(line, "}") {
			var m map[string]interface{}
			if err := json.Unmarshal([]byte(line), &m); err == nil {
				last = m
			}
		}
	}
	return last
}

// ─── Tests ───────────────────────────────────────────────────────────────────

// TestIntegration_InvalidJSON: garbage on stdin → binary exits non-zero.
func TestIntegration_InvalidJSON(t *testing.T) {
	bin := binaryPath(t)
	_, _, code := runBinary(t, bin, "THIS IS NOT JSON")
	if code == 0 {
		t.Error("expected non-zero exit code for invalid JSON input")
	}
}

// TestIntegration_NoRunnerFlag: runner=false → failJob → complete JSON with
// code=1 on stdout because finishJob always emits the complete message.
// However when worker==nil, finishJob short-circuits to shutdown without
// printing. We therefore check stderr for the expected error message instead.
func TestIntegration_NoRunnerFlag(t *testing.T) {
	bin := binaryPath(t)
	// Pass a command so the runner-mode check is reached (without args it
	// short-circuits at "No command specified" first).
	job := `{"id":"test-no-runner","runner":false,"cwd":"/tmp"}`
	_, stderr, _ := runBinary(t, bin, job, "echo", "hi")

	if !strings.Contains(stderr, "runner mode") {
		t.Errorf("expected stderr to mention 'runner mode', got:\n%s", stderr)
	}
}

// TestIntegration_NoCommand: runner=true but no command arg and no
// params.script → same fail path as NoRunnerFlag.
func TestIntegration_NoCommand(t *testing.T) {
	bin := binaryPath(t)
	job := `{"id":"test-no-cmd","runner":true,"cwd":"/tmp"}`
	_, stderr, _ := runBinary(t, bin, job)

	if !strings.Contains(stderr, "No command") {
		t.Errorf("expected stderr to mention 'No command', got:\n%s", stderr)
	}
}

// TestIntegration_SimpleEcho: a successful echo job.
// The child echoes a line, which is forwarded to stdout by the runner, and
// the runner emits a final complete JSON with code=0.
func TestIntegration_SimpleEcho(t *testing.T) {
	bin := binaryPath(t)
	cwd := t.TempDir()
	job := fmt.Sprintf(`{"id":"test-echo","runner":true,"cwd":%q}`, cwd)
	stdout, _, _ := runBinary(t, bin, job, "echo", "hello-from-xyrun")

	if !strings.Contains(stdout, "hello-from-xyrun") {
		t.Errorf("expected stdout to contain 'hello-from-xyrun', got:\n%s", stdout)
	}

	m := lastJSON(t, stdout)
	if m == nil {
		t.Fatal("expected a JSON complete message in stdout")
	}
	if complete, _ := m["complete"].(bool); !complete {
		t.Errorf("expected complete=true, got %v", m["complete"])
	}
	code, _ := m["code"].(float64)
	if int(code) != 0 {
		t.Errorf("expected code=0, got %v", m["code"])
	}
}

// TestIntegration_ChildExitNonZero: a child that exits with code 1.
// The runner always emits code=0 in the complete message (the child's own exit
// code is not forwarded unless the child plugin itself outputs {"code":N}).
// We verify the runner exits cleanly and emits a complete message.
func TestIntegration_ChildExitNonZero(t *testing.T) {
	bin := binaryPath(t)
	cwd := t.TempDir()
	job := fmt.Sprintf(`{"id":"test-exit","runner":true,"cwd":%q}`, cwd)
	stdout, stderr, _ := runBinary(t, bin, job, "bash", "-c", "exit 3")

	// The runner logs child non-zero exit to stderr.
	if !strings.Contains(stderr, "exited with code: 3") {
		t.Errorf("expected stderr to mention exit code 3, got:\n%s", stderr)
	}
	// A complete message must still be emitted.
	m := lastJSON(t, stdout)
	if m == nil {
		t.Fatal("expected a JSON complete message in stdout even after child failure")
	}
	if complete, _ := m["complete"].(bool); !complete {
		t.Errorf("expected complete=true, got %v", m["complete"])
	}
}

// TestIntegration_ScriptParam: inline script via params.script.
func TestIntegration_ScriptParam(t *testing.T) {
	bin := binaryPath(t)
	cwd := t.TempDir()
	job := fmt.Sprintf(
		`{"id":"test-script","runner":true,"cwd":%q,"params":{"script":"#!/bin/bash\necho script-output-ok\n"}}`,
		cwd,
	)
	stdout, _, _ := runBinary(t, bin, job)

	if !strings.Contains(stdout, "script-output-ok") {
		t.Errorf("expected stdout to contain 'script-output-ok', got:\n%s", stdout)
	}

	m := lastJSON(t, stdout)
	if m == nil {
		t.Fatal("expected a JSON complete message in stdout")
	}
	if complete, _ := m["complete"].(bool); !complete {
		t.Errorf("expected complete=true, got %v", m["complete"])
	}
}
