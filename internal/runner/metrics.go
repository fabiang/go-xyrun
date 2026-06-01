package runner

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/fabiang/go-xyrun/internal/models"
)

type ProcCache struct {
	Data    *ProcInfoResult
	Date    time.Time
	Elapsed time.Duration
	Expires time.Time
}

type ProcInfoResult struct {
	List []*models.ProcInfo
}

type ConnCacheInfo struct {
	Type       string
	State      string
	LocalAddr  string
	RemoteAddr string
	PID        int
	Bytes      int64
	Delta      int64
	Started    int64
}

func (a *App) tick() {
	if a.job == nil {
		return
	}

	if a.tickLock {
		return
	}
	a.tickLock = true
	defer func() { a.tickLock = false }()

	a.jobTick()
}

func (a *App) jobTick() {
	job := a.job

	a.getProcsCached(func(data *ProcInfoResult) {
		if a.job == nil {
			return
		}

		pids := make(map[int]*models.ProcInfo)
		for _, proc := range data.List {
			pids[proc.PID] = proc
		}

		for _, ajob := range a.activeJobs {
			a.measureJobResources(ajob, pids)
		}

		a.measureJobDiskIO()
		a.measureJobNetworkIO()

		if a.job == nil {
			return
		}

		res := map[string]interface{}{
			"xy":    1,
			"rpid":  os.Getpid(),
			"procs": job.Procs,
			"conns": job.Conns,
			"cpu":   job.CPU,
			"mem":   job.Mem,
		}

		b, _ := json.Marshal(res)
		fmt.Println(string(b))
	})
}

func (a *App) measureJobResources(job *models.Job, pids map[int]*models.ProcInfo) {
	if job.Procs == nil {
		job.Procs = make(map[int]*models.ProcInfo)
	}

	newProcs := make(map[int]*models.ProcInfo)
	rootPID := a.worker.PID

	rootPID = os.Getpid()

	if info, exists := pids[rootPID]; exists {
		newProcs[rootPID] = info

		cpu := info.CPU
		mem := info.MemRSS

		family := make(map[int]bool)
		family[rootPID] = true

		levels := 0
		for len(family) > 0 && levels <= 100 {
			levels++
			nextFamily := make(map[int]bool)
			for fpid := range family {
				for cpid, cinfo := range pids {
					if cinfo.ParentPID == fpid {
						nextFamily[cpid] = true
						cpu += cinfo.CPU
						mem += cinfo.MemRSS
						newProcs[cpid] = cinfo
					}
				}
			}
			family = nextFamily
		}

		job.Procs = newProcs

		if job.CPU == nil {
			job.CPU = &models.StatAccumulator{Min: cpu, Max: cpu, Total: cpu, Count: 1, Current: cpu}
		} else {
			if cpu < job.CPU.Min {
				job.CPU.Min = cpu
			}
			if cpu > job.CPU.Max {
				job.CPU.Max = cpu
			}
			job.CPU.Total += cpu
			job.CPU.Count++
			job.CPU.Current = cpu
		}

		fMem := float64(mem)
		if job.Mem == nil {
			job.Mem = &models.StatAccumulator{Min: fMem, Max: fMem, Total: fMem, Count: 1, Current: fMem}
		} else {
			if fMem < job.Mem.Min {
				job.Mem.Min = fMem
			}
			if fMem > job.Mem.Max {
				job.Mem.Max = fMem
			}
			job.Mem.Total += fMem
			job.Mem.Count++
			job.Mem.Current = fMem
		}
	}
}

func (a *App) getProcsCached(callback func(*ProcInfoResult)) {
	now := time.Now()

	if a.procCache == nil {
		a.procCache = &ProcCache{}
	}

	if a.procCache.Data != nil && now.Before(a.procCache.Expires) {
		callback(a.procCache.Data)
		return
	}

	a.getProcsFast(func(data *ProcInfoResult) {
		a.procCache.Data = data
		a.procCache.Date = time.Now()
		a.procCache.Elapsed = a.procCache.Date.Sub(now)
		a.procCache.Expires = a.procCache.Date.Add(a.procCache.Elapsed * 5)
		callback(data)
	})
}
