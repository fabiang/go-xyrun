package runner

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/fabiang/go-xyrun/internal/models"
)

func (a *App) measureJobDiskIO() {
	if runtime.GOOS != "linux" {
		return
	}

	for _, job := range a.activeJobs {
		if job.Procs != nil {
			for _, proc := range job.Procs {
				proc.Disk = 0
				b, err := os.ReadFile(fmt.Sprintf("/proc/%d/io", proc.PID))
				if err == nil {
					text := string(b)
					var rchar, wchar int64

					lines := strings.Split(text, "\n")
					for _, line := range lines {
						if strings.HasPrefix(line, "rchar:") {
							rchar, _ = strconv.ParseInt(strings.TrimSpace(strings.TrimPrefix(line, "rchar:")), 10, 64)
						} else if strings.HasPrefix(line, "wchar:") {
							wchar, _ = strconv.ParseInt(strings.TrimSpace(strings.TrimPrefix(line, "wchar:")), 10, 64)
						}
					}
					proc.Disk = rchar + wchar
				}
			}
		}
	}
}

func (a *App) measureJobNetworkIO() {
	if runtime.GOOS != "linux" {
		return
	}

	for _, job := range a.activeJobs {
		if job.Procs != nil {
			for _, proc := range job.Procs {
				proc.Conns = 0
				proc.Net = 0
			}
		}
	}

	ssBin, err := exec.LookPath("ss")
	if err != nil {
		return
	}

	cmd := exec.Command(ssBin, "-nutipaO")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return
	}

	now := time.Now().Unix()
	lines := strings.Split(stdout.String(), "\n")
	ids := make(map[string]bool)

	ssRegex := regexp.MustCompile(`^(tcp|tcp4|tcp6|udp|udp4|udp6)\s+(\w+)\s+(\d+)\s+(\d+)\s+(\S+)\s+(\S+)\s+.+pid=(\d+)`)
	bytesRegex := regexp.MustCompile(`bytes_(?:acked|received):(\d+)`)

	for _, line := range lines {
		matches := ssRegex.FindStringSubmatch(line)
		if len(matches) > 0 {
			typ := matches[1]
			state := matches[2]
			localAddr := matches[5]
			remoteAddr := matches[6]
			pidStr := matches[7]

			pid, _ := strconv.Atoi(pidStr)
			if state == "ESTAB" {
				state = "ESTABLISHED"
			}
			if state == "UNCONN" {
				state = "UNCONNECTED"
			}

			id := localAddr + "|" + remoteAddr

			if a.connCache[id] == nil {
				a.connCache[id] = &ConnCacheInfo{Started: now}
			}
			conn := a.connCache[id]

			conn.Type = typ
			conn.State = state
			conn.LocalAddr = localAddr
			conn.RemoteAddr = remoteAddr
			conn.PID = pid

			var bytesAcc int64
			byteMatches := bytesRegex.FindAllStringSubmatch(line, -1)
			for _, bm := range byteMatches {
				if len(bm) > 1 {
					v, _ := strconv.ParseInt(bm[1], 10, 64)
					bytesAcc += v
				}
			}

			conn.Delta = bytesAcc - conn.Bytes
			conn.Bytes = bytesAcc
			ids[id] = true
		}
	}

	for id := range a.connCache {
		if !ids[id] {
			delete(a.connCache, id)
		}
	}

	for _, job := range a.activeJobs {
		if job.Procs == nil {
			continue
		}

		job.Conns = []*models.ConnInfo{}
		for _, conn := range a.connCache {
			if _, exists := job.Procs[conn.PID]; exists {
				cinfo := &models.ConnInfo{
					Type:       conn.Type,
					State:      conn.State,
					LocalAddr:  conn.LocalAddr,
					RemoteAddr: conn.RemoteAddr,
					PID:        conn.PID,
					Bytes:      conn.Bytes,
					Delta:      conn.Delta,
					Started:    int(conn.Started),
				}
				job.Conns = append(job.Conns, cinfo)
				job.Procs[conn.PID].Conns++
				job.Procs[conn.PID].Net += conn.Delta
			}
		}
	}
}

func (a *App) getProcsFast(callback func(*ProcInfoResult)) {
	info := &ProcInfoResult{List: make([]*models.ProcInfo, 0)}

	if runtime.GOOS == "windows" {
		callback(info)
		return
	}

	psBin, err := exec.LookPath("ps")
	if err != nil {
		callback(info)
		return
	}

	var psArgs []string
	if runtime.GOOS == "linux" {
		psArgs = []string{"-eo", "pid,ppid,user,%cpu,rss,etimes,state,pri,nice,vsz,tty,%mem,class,group,thcount,times,args"}
	} else if runtime.GOOS == "darwin" {
		psArgs = []string{"-axro", "pid,ppid,%cpu,%mem,pri,vsz,rss,nice,etime,state,tty,user,group,args"}
	} else {
		callback(info)
		return
	}

	out, err := exec.Command(psBin, psArgs...).Output()
	if err != nil {
		callback(info)
		return
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) < 2 {
		callback(info)
		return
	}

	headerLine := lines[0]
	headers := strings.Fields(strings.ToLower(headerLine))

	colMap := map[string]string{
		"ppid": "parentPid", "rss": "memRss", "vsz": "memVsz", "tt": "tty",
		"thcnt": "threads", "pri": "priority", "ni": "nice", "s": "state",
		"stat": "state", "elapsed": "age", "cls": "class", "gid": "group", "args": "command",
		"%cpu": "cpu", "%mem": "mem",
	}

	for i, h := range headers {
		h = strings.ReplaceAll(h, "%", "")
		if mapped, ok := colMap[h]; ok {
			headers[i] = mapped
		} else {
			headers[i] = h
		}
	}

	numCPUs := float64(a.numCPUs)
	if numCPUs == 0 {
		numCPUs = 1
	}
	now := int(time.Now().Unix())

	for _, line := range lines[1:] {
		cols := strings.Fields(line)
		if len(cols) == 0 {
			continue
		}

		if len(cols) > len(headers) {
			cmdParts := cols[len(headers)-1:]
			cols = cols[:len(headers)-1]
			cols = append(cols, strings.Join(cmdParts, " "))
		}

		proc := &models.ProcInfo{}
		for i, h := range headers {
			if i >= len(cols) {
				continue
			}
			val := cols[i]

			switch h {
			case "pid":
				proc.PID, _ = strconv.Atoi(val)
			case "parentPid":
				proc.ParentPID, _ = strconv.Atoi(val)
			case "cpu":
				cpuF, _ := strconv.ParseFloat(val, 64)
				proc.CPU = cpuF / numCPUs
			case "memRss":
				v, _ := strconv.ParseInt(val, 10, 64)
				proc.MemRSS = v * 1024
			case "memVsz":
				v, _ := strconv.ParseInt(val, 10, 64)
				proc.MemVSZ = v * 1024
			case "state":
				if len(val) > 0 {
					switch val[0] {
					case 'I':
						proc.State = "Idle"
					case 'S', 'D', 'U':
						proc.State = "Sleeping"
					case 'R':
						proc.State = "Running"
					case 'Z':
						proc.State = "Zombie"
					case 'T', 't':
						proc.State = "Stopped"
					case 'W':
						proc.State = "Paged"
					case 'X':
						proc.State = "Dead"
					default:
						proc.State = "Unknown"
					}
				}
			case "age":
				if v, err := strconv.Atoi(val); err == nil {
					proc.Age = v
				} else {
					proc.Age = 0
				}
			case "command":
				proc.Command = val
			}
		}

		proc.Started = max(0, now-proc.Age)

		if proc.ParentPID == os.Getpid() && strings.Contains(proc.Command, "ps") {
			continue
		}

		info.List = append(info.List, proc)
	}

	callback(info)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
