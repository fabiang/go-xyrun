package runner

import (
	"testing"

	"github.com/fabiang/go-xyrun/internal/models"
)

func TestHandleChildResponse_NoXyKey_ReturnsFalse(t *testing.T) {
	job := &models.Job{}
	data := map[string]interface{}{
		"message": "hello",
	}

	got := handleChildResponse(job, data)
	if got {
		t.Error("expected false when 'xy' key is missing")
	}
}

func TestHandleChildResponse_XyKeyOnly_ReturnsTrue(t *testing.T) {
	// Only the "xy" key remains after stripping code/description/complete → len==1 → true
	job := &models.Job{}
	data := map[string]interface{}{
		"xy":          1,
		"code":        0,
		"description": "",
		"complete":    false,
	}

	got := handleChildResponse(job, data)
	if !got {
		t.Error("expected true when only 'xy' key remains after stripping")
	}
}

func TestHandleChildResponse_StripsSentinelKeys(t *testing.T) {
	job := &models.Job{}
	data := map[string]interface{}{
		"xy":          1,
		"code":        42,
		"description": "some error",
		"complete":    true,
		"extra":       "value",
	}

	handleChildResponse(job, data)

	if _, ok := data["code"]; ok {
		t.Error("expected 'code' to be removed from data")
	}
	if _, ok := data["description"]; ok {
		t.Error("expected 'description' to be removed from data")
	}
	if _, ok := data["complete"]; ok {
		t.Error("expected 'complete' to be removed from data")
	}
}

func TestHandleChildResponse_FilesSlice_AppendedToJob(t *testing.T) {
	job := &models.Job{}
	data := map[string]interface{}{
		"xy": 1,
		"files": []interface{}{
			map[string]interface{}{"filename": "a.txt"},
			map[string]interface{}{"filename": "b.txt"},
		},
	}

	handleChildResponse(job, data)

	if len(job.Files) != 2 {
		t.Errorf("expected 2 files on job, got %d", len(job.Files))
	}
	if _, ok := data["files"]; ok {
		t.Error("expected 'files' to be removed from data")
	}
}

func TestHandleChildResponse_FilesSingleValue_AppendedToJob(t *testing.T) {
	job := &models.Job{}
	// Non-slice files value
	data := map[string]interface{}{
		"xy":    1,
		"files": "single-file.txt",
	}

	handleChildResponse(job, data)

	if len(job.Files) != 1 {
		t.Errorf("expected 1 file on job, got %d", len(job.Files))
	}
}

func TestHandleChildResponse_PushFilesSlice_AppendedToJob(t *testing.T) {
	job := &models.Job{}
	data := map[string]interface{}{
		"xy": 1,
		"push": map[string]interface{}{
			"files": []interface{}{
				map[string]interface{}{"filename": "pushed.txt"},
			},
		},
	}

	handleChildResponse(job, data)

	if len(job.Files) != 1 {
		t.Errorf("expected 1 file on job from push, got %d", len(job.Files))
	}
}

func TestHandleChildResponse_PushFilesAndOtherKeys_PushRemainsInData(t *testing.T) {
	job := &models.Job{}
	data := map[string]interface{}{
		"xy": 1,
		"push": map[string]interface{}{
			"files":   []interface{}{"f.txt"},
			"message": "hello",
		},
	}

	handleChildResponse(job, data)

	// push still in data because it has other keys
	if _, ok := data["push"]; !ok {
		t.Error("expected 'push' to remain in data when it has other keys besides 'files'")
	}
}

func TestHandleChildResponse_PushFilesOnly_PushRemovedFromData(t *testing.T) {
	job := &models.Job{}
	data := map[string]interface{}{
		"xy": 1,
		"push": map[string]interface{}{
			"files": []interface{}{"f.txt"},
		},
	}

	handleChildResponse(job, data)

	// push is removed from data because only "files" was in push map
	if _, ok := data["push"]; ok {
		t.Error("expected 'push' to be removed from data when it only contained 'files'")
	}
}

func TestHandleChildResponse_ExtraKeys_ReturnsFalse(t *testing.T) {
	// xy + extra key → len > 1 → false (data is forwarded to output)
	job := &models.Job{}
	data := map[string]interface{}{
		"xy":    1,
		"extra": "value",
	}

	got := handleChildResponse(job, data)
	if got {
		t.Error("expected false when additional keys exist beyond 'xy'")
	}
}

func TestHandleChildResponse_FilesAppendedToExistingJobFiles(t *testing.T) {
	existing := map[string]interface{}{"filename": "existing.txt"}
	job := &models.Job{
		Files: []interface{}{existing},
	}
	data := map[string]interface{}{
		"xy": 1,
		"files": []interface{}{
			map[string]interface{}{"filename": "new.txt"},
		},
	}

	handleChildResponse(job, data)

	if len(job.Files) != 2 {
		t.Errorf("expected 2 files (1 existing + 1 new), got %d", len(job.Files))
	}
}
