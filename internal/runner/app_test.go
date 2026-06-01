package runner

import (
	"runtime"
	"strings"
	"testing"

	"github.com/fabiang/go-xyrun/internal/models"
)

func TestSafeEnvKey(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "lowercase letters are kept", input: "hello", expected: "hello"},
		{name: "uppercase letters are kept", input: "HELLO", expected: "HELLO"},
		{name: "digits are kept", input: "abc123", expected: "abc123"},
		{name: "hyphens are replaced with underscores", input: "my-key", expected: "my_key"},
		{name: "dots are replaced with underscores", input: "my.key", expected: "my_key"},
		{name: "spaces are replaced with underscores", input: "my key", expected: "my_key"},
		{name: "special characters are replaced with underscores", input: "key!@#$%^&*()", expected: "key__________"},
		{name: "already valid key is unchanged", input: "MY_ENV_VAR_123", expected: "MY_ENV_VAR_123"},
		{name: "empty string stays empty", input: "", expected: ""},
		{name: "mixed valid and invalid characters", input: "plugin-param.value", expected: "plugin_param_value"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := safeEnvKey(tt.input)
			if got != tt.expected {
				t.Errorf("safeEnvKey(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestNewApp_InitializesFields(t *testing.T) {
	job := &models.Job{ID: "test-job-1"}
	app := NewApp(job, "2.0.0")

	if app.job != job {
		t.Error("expected job to be set")
	}
	if app.version != "2.0.0" {
		t.Errorf("expected version 2.0.0, got %q", app.version)
	}
	if app.activeJobs == nil {
		t.Error("expected activeJobs map to be initialised")
	}
	if app.kids == nil {
		t.Error("expected kids map to be initialised")
	}
	if app.connCache == nil {
		t.Error("expected connCache map to be initialised")
	}
	if app.tickStop == nil {
		t.Error("expected tickStop channel to be initialised")
	}
	if app.numCPUs <= 0 {
		t.Errorf("expected numCPUs > 0, got %d", app.numCPUs)
	}
	if app.platform != runtime.GOOS {
		t.Errorf("expected platform %q, got %q", runtime.GOOS, app.platform)
	}
}

func TestNewApp_NumCPUs_MatchesRuntime(t *testing.T) {
	app := NewApp(&models.Job{}, "1.0.0")
	if app.numCPUs != runtime.NumCPU() {
		t.Errorf("expected numCPUs=%d, got %d", runtime.NumCPU(), app.numCPUs)
	}
}

// TestFailJob_MutatesJob verifies that failJob sets the expected fields on the
// job before calling finishJob. Shutdown is detected by waiting on tickStop.
func TestFailJob_MutatesJob(t *testing.T) {
	job := &models.Job{
		ID:  "fail-test",
		PID: 42,
	}
	app := NewApp(job, "1.0.0")

	// failJob → finishJob → shutdown closes tickStop.
	go app.failJob("something went wrong")

	<-app.tickStop // wait for shutdown

	if job.PID != 0 {
		t.Errorf("expected PID=0 after failJob, got %d", job.PID)
	}
	if job.Code != 1 {
		t.Errorf("expected Code=1 after failJob, got %v", job.Code)
	}
	if !strings.Contains(job.Description, "something went wrong") {
		t.Errorf("expected Description to contain the reason, got %q", job.Description)
	}
	if !strings.HasPrefix(job.Description, "xyRun: ") {
		t.Errorf("expected Description to start with 'xyRun: ', got %q", job.Description)
	}
}
