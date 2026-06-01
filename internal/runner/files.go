package runner

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/fabiang/go-xyrun/internal/models"
)

func (a *App) getHTTPClient(job *models.Job) *http.Client {
	timeout := 300 * time.Second
	if job.HTTPFileOpts != nil && job.HTTPFileOpts.Timeout > 0 {
		timeout = time.Duration(job.HTTPFileOpts.Timeout) * time.Millisecond
	}

	return &http.Client{
		Timeout: timeout,
	}
}

func (a *App) downloadFile(url, destFile string, job *models.Job) error {
	client := a.getHTTPClient(job)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		return fmt.Errorf("HTTP status %d", res.StatusCode)
	}

	f, err := os.Create(destFile)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, res.Body)
	return err
}

func (a *App) prepUploadJobFiles(job *models.Job, callback func(error)) {
	if len(job.Files) == 0 {
		callback(nil)
		return
	}

	var toUpload []models.UploadFileDef

	for _, fi := range job.Files {
		switch v := fi.(type) {
		case string:
			toUpload = append(toUpload, models.UploadFileDef{Path: v})
		case []interface{}:
			if len(v) >= 1 {
				def := models.UploadFileDef{Path: fmt.Sprintf("%v", v[0])}
				if len(v) >= 2 {
					def.Filename = fmt.Sprintf("%v", v[1])
				}
				if len(v) >= 3 {
					if deleteBool, ok := v[2].(bool); ok && deleteBool {
						def.Delete = true
					}
				}
				toUpload = append(toUpload, def)
			}
		case map[string]interface{}:
			def := models.UploadFileDef{}
			if path, ok := v["path"].(string); ok {
				def.Path = path
			}
			if filename, ok := v["filename"].(string); ok {
				def.Filename = filename
			}
			if del, ok := v["delete"].(bool); ok {
				def.Delete = del
			}
			toUpload = append(toUpload, def)
		}
	}

	var expandedUpload []models.UploadFileDef
	for _, def := range toUpload {
		if def.Filename != "" || !stringsContainsWildcard(def.Path) {
			expandedUpload = append(expandedUpload, def)
		} else {
			matches, err := filepath.Glob(def.Path)
			if err == nil {
				for _, match := range matches {
					expandedUpload = append(expandedUpload, models.UploadFileDef{Path: match, Delete: def.Delete})
				}
			}
		}
	}

	a.uploadJobFiles(job, expandedUpload, callback)
}

func stringsContainsWildcard(path string) bool {
	for i := 0; i < len(path); i++ {
		if path[i] == '*' || path[i] == '?' {
			return true
		}
	}
	return false
}

func (a *App) uploadJobFiles(job *models.Job, files []models.UploadFileDef, callback func(error)) {
	if len(files) == 0 {
		callback(nil)
		return
	}

	job.Files = []interface{}{}

	url := job.BaseURL + "/api/app/upload_job_file"
	client := a.getHTTPClient(job)

	for _, file := range files {
		fn := file.Filename
		if fn == "" {
			fn = filepath.Base(file.Path)
		}

		fmt.Printf("xyRun: Uploading file: %s\n", fn)

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		writer.WriteField("id", job.ID)
		writer.WriteField("server", job.Server)
		writer.WriteField("auth", job.AuthToken)

		f, err := os.Open(file.Path)
		if err != nil {
			callback(err)
			return
		}

		part, err := writer.CreateFormFile("file1", fn)
		if err != nil {
			f.Close()
			callback(err)
			return
		}

		_, err = io.Copy(part, f)
		f.Close()
		if err != nil {
			callback(err)
			return
		}

		err = writer.Close()
		if err != nil {
			callback(err)
			return
		}

		req, err := http.NewRequest("POST", url, body)
		if err != nil {
			callback(err)
			return
		}
		req.Header.Set("Content-Type", writer.FormDataContentType())

		res, err := client.Do(req)
		if err != nil {
			callback(err)
			return
		}

		resBody, _ := io.ReadAll(res.Body)
		res.Body.Close()

		var result map[string]interface{}
		if err := json.Unmarshal(resBody, &result); err != nil {
			callback(fmt.Errorf("Failed to parse upload JSON: %v", err))
			return
		}

		if code, ok := result["code"].(string); ok && code != "" && code != "0" {
			desc, _ := result["description"].(string)
			callback(fmt.Errorf("Failed to upload job file: %s: %s", fn, desc))
			return
		}

		sizeFloat, _ := result["size"].(float64)
		pathStr, _ := result["key"].(string)

		finalFile := map[string]interface{}{
			"id":       time.Now().UnixNano(),
			"date":     time.Now().Unix(),
			"filename": fn,
			"path":     pathStr,
			"size":     int64(sizeFloat),
			"server":   job.Server,
			"job":      job.ID,
		}

		job.Files = append(job.Files, finalFile)

		if file.Delete {
			os.Remove(file.Path)
		}
	}

	callback(nil)
}
