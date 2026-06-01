package runner

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/fabiang/go-xyrun/internal/models"
)

type Worker struct {
	PID         int
	Child       *exec.Cmd
	ChildExited bool
	KillTimer   *time.Timer
}

type App struct {
	job        *models.Job
	worker     *Worker
	activeJobs map[string]*models.Job
	kids       map[int]*Worker

	procCache *ProcCache
	connCache map[string]*ConnCacheInfo

	ticker   *time.Ticker
	tickStop chan bool
	tickLock bool // simplistic mutex for ticker

	platform string
	aborted  bool
	numCPUs  int
	version  string
}

func NewApp(job *models.Job, versionStr string) *App {
	return &App{
		job:        job,
		activeJobs: make(map[string]*models.Job),
		kids:       make(map[int]*Worker),
		connCache:  make(map[string]*ConnCacheInfo),
		tickStop:   make(chan bool),
		numCPUs:    runtime.NumCPU(),
		platform:   runtime.GOOS,
		version:    versionStr,
	}
}

func (a *App) Run() {
	a.activeJobs[a.job.ID] = a.job // register it up front theoretically

	// Prepare job launch (download files, setup temp dir)
	a.prepLaunchJob()

	// Start ticker
	a.ticker = time.NewTicker(1 * time.Second)
	go func() {
		for {
			select {
			case <-a.ticker.C:
				a.tick()
			case <-a.tickStop:
				return
			}
		}
	}()

	// Wait mechanism - wait until finishJob closes App
	<-a.tickStop
}

func (a *App) prepLaunchJob() {
	job := a.job

	// No input files? Skip prep, no temp dir needed
	if len(job.Input.Files) == 0 {
		a.launchJob()
		return
	}

	// Create temp dir
	if err := os.MkdirAll(job.Cwd, 0777); err != nil {
		a.failJob(fmt.Sprintf("Failed to create job dir: %v", err))
		return
	}
	if err := os.Chmod(job.Cwd, 0777); err != nil {
		// Log but keep going, maybe not fatal
	}

	// Download input files sequentially
	for _, file := range job.Input.Files {
		destFile := filepath.Join(job.Cwd, file.Filename)
		url := job.BaseURL + "/" + file.Path

		fmt.Printf("xyRun: Downloading file: %s (%d bytes)\n", destFile, file.Size)

		if err := a.downloadFile(url, destFile, job); err != nil {
			a.failJob(fmt.Sprintf("Failed to download job file: %s: %v", file.Filename, err))
			return
		}
	}

	// Launch job for real
	a.launchJob()
}

func (a *App) failJob(reason string) {
	a.job.PID = 0
	a.job.Code = 1
	a.job.Description = "xyRun: " + reason
	fmt.Fprintf(os.Stderr, "%s\n", a.job.Description)
	a.activeJobs[a.job.ID] = a.job
	a.finishJob()
}

func (a *App) launchJob() {
	job := a.job

	// Setup environment
	childArgs := os.Args[1:] // process.argv.slice(2)

	var childCmd string
	if len(childArgs) > 0 {
		childCmd = childArgs[0]
		childArgs = childArgs[1:]
	}

	envVars := os.Environ()
	envMap := make(map[string]string)
	for _, e := range envVars {
		pair := strings.SplitN(e, "=", 2)
		if len(pair) == 2 {
			envMap[pair[0]] = pair[1]
		}
	}

	// Add secrets
	for k, v := range job.Secrets {
		envMap[k] = v
	}

	envMap["XYOPS"] = "runner-" + a.version
	envMap["JOB_ID"] = job.ID
	envMap["JOB_NOW"] = fmt.Sprintf("%d", job.Now)

	if childCmd == "" && job.Params != nil && job.Params["script"] != nil {
		scriptStr, ok := job.Params["script"].(string)
		if ok {
			scriptFile := filepath.Join(job.Cwd, fmt.Sprintf("xyops-script-temp-%s.sh", job.ID))
			childCmd = scriptFile
			childArgs = []string{}

			os.MkdirAll(job.Cwd, 0777)
			scriptStr = strings.ReplaceAll(scriptStr, "\r\n", "\n")
			err := os.WriteFile(scriptFile, []byte(scriptStr), 0775)
			if err != nil {
				a.failJob(fmt.Sprintf("Failed to write script: %v", err))
				return
			}
		}
	}

	if childCmd == "" {
		a.failJob("No command specified.")
		return
	}
	if !job.Runner {
		a.failJob("Job not launched in runner mode (Set runner flag in event plugin).")
		return
	}

	// add plugin params as env vars, expand $INLINE vars
	if job.Params != nil {
		for key, val := range job.Params {
			safeKey := safeEnvKey(key)
			valStr := fmt.Sprintf("%v", val)
			for envK, envV := range envMap {
				valStr = strings.ReplaceAll(valStr, "$"+envK, envV)
			}
			envMap[safeKey] = valStr
		}
	}

	// add workflow params
	if job.Workflow.Params != nil {
		for key, val := range job.Workflow.Params {
			switch val.(type) {
			case map[string]interface{}, []interface{}:
			default:
				safeKey := "workflow_" + safeEnvKey(key)
				valStr := fmt.Sprintf("%v", val)
				for envK, envV := range envMap {
					valStr = strings.ReplaceAll(valStr, "$"+envK, envV)
				}
				envMap[safeKey] = valStr
			}
		}
	}

	finalEnv := []string{}
	for k, v := range envMap {
		finalEnv = append(finalEnv, fmt.Sprintf("%s=%s", k, v))
	}

	cmd := exec.Command(childCmd, childArgs...)
	cmd.Env = finalEnv
	if job.Cwd != "" {
		cmd.Dir = job.Cwd
	}

	// Apply OS specific child creation configuration
	if err := a.applyOSChildOpts(cmd, job, envMap); err != nil {
		a.failJob(err.Error())
		return
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		a.failJob(fmt.Sprintf("StdoutPipe error: %v", err))
		return
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		a.failJob(fmt.Sprintf("StdinPipe error: %v", err))
		return
	}

	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		a.failJob(fmt.Sprintf("Child spawn error: %s: %v", childCmd, err))
		return
	}

	job.PID = cmd.Process.Pid

	a.worker = &Worker{
		PID:   job.PID,
		Child: cmd,
	}
	a.kids[job.PID] = a.worker
	a.activeJobs[job.ID] = job

	workerStreamPipe(a, stdout, stdin)

	jobBytes, _ := json.Marshal(job)
	stdin.Write(jobBytes)
	stdin.Write([]byte("\n"))
	stdin.Close()

	go func() {
		err := cmd.Wait()
		code := 0
		if err != nil {
			if exiterr, ok := err.(*exec.ExitError); ok {
				code = exiterr.ExitCode()
			} else {
				code = 1
			}
			fmt.Fprintf(os.Stderr, "xyRun: Child process exited with code: %d; error: %v\n", code, err)
		}
		a.worker.ChildExited = true
		a.finishJob()
	}()
}

func safeEnvKey(key string) string {
	return strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			return r
		}
		return '_'
	}, key)
}

func (a *App) finishJob() {
	job := a.job
	worker := a.worker

	if job == nil || worker == nil {
		a.shutdown()
		return
	}

	if worker.KillTimer != nil {
		worker.KillTimer.Stop()
	}

	a.job = nil
	a.worker = nil
	a.activeJobs = make(map[string]*models.Job)
	a.kids = make(map[int]*Worker)

	a.prepUploadJobFiles(job, func(err error) {
		if err != nil {
			job.Code = 1
			job.Description = fmt.Sprintf("xyRun: %v", err)
		}

		if len(job.Files) > 0 {
			res := map[string]interface{}{"xy": 1, "files": job.Files}
			b, _ := json.Marshal(res)
			fmt.Println(string(b))
		}

		code := 0
		if job.Code != nil {
			switch v := job.Code.(type) {
			case int:
				code = v
			case float64:
				code = int(v)
			}
		}

		res := map[string]interface{}{
			"xy":          1,
			"complete":    true,
			"code":        code,
			"description": job.Description,
		}
		b, _ := json.Marshal(res)
		fmt.Println(string(b))

		if job.Cwd != "" {
			os.RemoveAll(job.Cwd)
		}
		a.shutdown()
	})
}

func (a *App) AbortJob() {
	if a.aborted {
		return
	}
	a.aborted = true
	fmt.Fprintf(os.Stderr, "xyRun: Caught abort signal, shutting down\n")

	job := a.job
	worker := a.worker

	if job == nil || worker == nil {
		os.Exit(0)
	}

	if worker.Child != nil {
		if job.Kill == "none" {
			a.finishJob()
			return
		}

		worker.KillTimer = time.AfterFunc(10*time.Second, func() {
			if job.Kill == "all" && job.Procs != nil {
				fmt.Fprintf(os.Stderr, "xyRun: Children did not exit, killing harder\n")
				for pid := range job.Procs {
					proc, _ := os.FindProcess(pid)
					if proc != nil {
						proc.Kill()
					}
				}
			} else {
				fmt.Fprintf(os.Stderr, "xyRun: Child did not exit, killing harder: %d\n", job.PID)
				worker.Child.Process.Kill()
			}
		})

		if job.Kill == "all" && job.Procs != nil {
			fmt.Printf("xyRun: Killing all job processes\n")
			for pid := range job.Procs {
				a.sendSigTerm(pid)
			}
		} else {
			fmt.Printf("xyRun: Killing job process: %d\n", job.PID)
			a.sendSigTerm(job.PID)
		}
	} else {
		os.Exit(0)
	}
}

func (a *App) shutdown() {
	if a.ticker != nil {
		a.ticker.Stop()
	}
	close(a.tickStop)
}
