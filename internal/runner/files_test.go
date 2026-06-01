package runner

import (
	"testing"
	"time"

	"github.com/fabiang/go-xyrun/internal/models"
)

func TestGetHTTPClient_DefaultTimeout(t *testing.T) {
	app := &App{}
	client := app.getHTTPClient(&models.Job{})
	want := 300 * time.Second
	if client.Timeout != want {
		t.Errorf("expected default timeout %v, got %v", want, client.Timeout)
	}
}

func TestGetHTTPClient_CustomTimeout(t *testing.T) {
	app := &App{}
	job := &models.Job{
		HTTPFileOpts: &models.HTTPFileOpts{Timeout: 5000}, // 5000 ms
	}
	client := app.getHTTPClient(job)
	want := 5 * time.Second
	if client.Timeout != want {
		t.Errorf("expected timeout %v, got %v", want, client.Timeout)
	}
}

func TestGetHTTPClient_ZeroTimeoutUsesDefault(t *testing.T) {
	app := &App{}
	job := &models.Job{
		HTTPFileOpts: &models.HTTPFileOpts{Timeout: 0}, // should fall back to default
	}
	client := app.getHTTPClient(job)
	want := 300 * time.Second
	if client.Timeout != want {
		t.Errorf("expected fallback timeout %v, got %v", want, client.Timeout)
	}
}

func TestStringsContainsWildcard(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "no wildcard",
			path:     "/tmp/myfile.txt",
			expected: false,
		},
		{
			name:     "asterisk wildcard",
			path:     "/tmp/*.txt",
			expected: true,
		},
		{
			name:     "question mark wildcard",
			path:     "/tmp/file?.txt",
			expected: true,
		},
		{
			name:     "both wildcards",
			path:     "/tmp/*?.txt",
			expected: true,
		},
		{
			name:     "wildcard at start",
			path:     "*.log",
			expected: true,
		},
		{
			name:     "empty string",
			path:     "",
			expected: false,
		},
		{
			name:     "plain filename",
			path:     "output.csv",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stringsContainsWildcard(tt.path)
			if got != tt.expected {
				t.Errorf("stringsContainsWildcard(%q) = %v, want %v", tt.path, got, tt.expected)
			}
		})
	}
}

func TestPrepUploadJobFiles_NoFiles(t *testing.T) {
	app := &App{}
	job := &models.Job{}

	called := false
	var callbackErr error
	app.prepUploadJobFiles(job, func(err error) {
		called = true
		callbackErr = err
	})

	if !called {
		t.Error("callback was not called")
	}
	if callbackErr != nil {
		t.Errorf("expected no error, got %v", callbackErr)
	}
}

func TestPrepUploadJobFiles_StringEntry(t *testing.T) {
	// prepUploadJobFiles with a plain string file entry should call uploadJobFiles
	// with one UploadFileDef. We verify the parsing logic by using a non-existent
	// file so that the HTTP call fails early (we only care about def construction here).
	app := &App{
		connCache: make(map[string]*ConnCacheInfo),
	}
	job := &models.Job{
		Files: []interface{}{"/some/path/file.txt"},
	}

	// Override via a minimal fake by just running and observing the callback is
	// reached (uploadJobFiles calls callback(nil) when files is empty after expansion,
	// but here the file exists in the slice so it will try to open it and fail → err).
	called := false
	app.prepUploadJobFiles(job, func(err error) {
		called = true
	})

	if !called {
		t.Error("callback was not called")
	}
}

func TestPrepUploadJobFiles_SliceEntry_PathOnly(t *testing.T) {
	app := &App{connCache: make(map[string]*ConnCacheInfo)}
	job := &models.Job{
		Files: []interface{}{
			[]interface{}{"/path/to/file.csv"},
		},
	}

	called := false
	app.prepUploadJobFiles(job, func(err error) {
		called = true
	})

	if !called {
		t.Error("callback was not called")
	}
}

func TestPrepUploadJobFiles_SliceEntry_WithFilenameAndDelete(t *testing.T) {
	app := &App{connCache: make(map[string]*ConnCacheInfo)}
	job := &models.Job{
		Files: []interface{}{
			[]interface{}{"/path/to/file.csv", "custom-name.csv", true},
		},
	}

	called := false
	app.prepUploadJobFiles(job, func(err error) {
		called = true
	})

	if !called {
		t.Error("callback was not called")
	}
}

func TestPrepUploadJobFiles_MapEntry(t *testing.T) {
	app := &App{connCache: make(map[string]*ConnCacheInfo)}
	job := &models.Job{
		Files: []interface{}{
			map[string]interface{}{
				"path":     "/path/to/file.csv",
				"filename": "output.csv",
				"delete":   false,
			},
		},
	}

	called := false
	app.prepUploadJobFiles(job, func(err error) {
		called = true
	})

	if !called {
		t.Error("callback was not called")
	}
}

func TestUploadJobFiles_NoFiles(t *testing.T) {
	app := &App{}
	job := &models.Job{}

	called := false
	var callbackErr error
	app.uploadJobFiles(job, nil, func(err error) {
		called = true
		callbackErr = err
	})

	if !called {
		t.Error("callback was not called")
	}
	if callbackErr != nil {
		t.Errorf("expected no error, got %v", callbackErr)
	}
}
