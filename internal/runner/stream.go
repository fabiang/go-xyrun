package runner

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/fabiang/go-xyrun/internal/models"
)

func workerStreamPipe(app *App, stdout io.Reader, stdin io.Writer) {
	reader := bufio.NewReader(stdout)

	go func() {
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				break
			}

			origLine := line
			line = strings.TrimRight(line, "\r\n")
			trimmed := strings.TrimSpace(line)

			if strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}") {
				var data map[string]interface{}
				if err := json.Unmarshal([]byte(trimmed), &data); err == nil {
					if handleChildResponse(app.job, data) {
						continue
					}
					b, _ := json.Marshal(data)
					fmt.Println(string(b))
					continue
				}
			}

			if app.platform == "windows" {
				fmt.Print(strings.TrimSuffix(origLine, "\r\n") + "\n")
			} else {
				fmt.Print(origLine)
			}
		}
	}()
}

func handleChildResponse(job *models.Job, data map[string]interface{}) bool {
	_, hasXy := data["xy"]
	if !hasXy {
		return false
	}

	delete(data, "code")
	delete(data, "description")
	delete(data, "complete")

	if filesRaw, ok := data["files"]; ok {
		delete(data, "files")
		if arr, ok := filesRaw.([]interface{}); ok {
			job.Files = append(job.Files, arr...)
		} else {
			job.Files = append(job.Files, filesRaw)
		}
	} else if pushRaw, ok := data["push"]; ok {
		if pushMap, isMap := pushRaw.(map[string]interface{}); isMap {
			if pushFilesRaw, ok := pushMap["files"]; ok {
				if arr, ok := pushFilesRaw.([]interface{}); ok {
					job.Files = append(job.Files, arr...)
				} else {
					job.Files = append(job.Files, pushFilesRaw)
				}
				delete(pushMap, "files")
				if len(pushMap) == 0 {
					delete(data, "push")
				}
			}
		}
	}

	if len(data) == 1 {
		return true
	}

	return false
}
