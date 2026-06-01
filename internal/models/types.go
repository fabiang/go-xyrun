package models

// Job represents the incoming job configuration parsed from stdin
type Job struct {
	ID           string                 `json:"id"`
	Now          int64                  `json:"now"`
	BaseURL      string                 `json:"base_url"`
	Server       string                 `json:"server"`
	AuthToken    string                 `json:"auth_token"`
	Runner       bool                   `json:"runner"`
	Kill         string                 `json:"kill"` // 'none', 'all', or default
	UID          interface{}            `json:"uid"`  // Can be string or int
	GID          interface{}            `json:"gid"`  // Can be string or int
	Cwd          string                 `json:"cwd"`
	Secrets      map[string]string      `json:"secrets"`
	Params       map[string]interface{} `json:"params"`
	Workflow     Workflow               `json:"workflow"`
	Input        Input                  `json:"input"`
	Files        []interface{}          `json:"files,omitempty"` // For incoming mixed type and final outgoing
	HTTPFileOpts *HTTPFileOpts          `json:"http_file_opts,omitempty"`
	SocketOpts   map[string]interface{} `json:"socket_opts,omitempty"`

	// Real-time tracking
	PID         int               `json:"pid,omitempty"`
	Code        interface{}       `json:"code,omitempty"`
	Description string            `json:"description,omitempty"`
	Procs       map[int]*ProcInfo `json:"procs,omitempty"`
	Conns       []*ConnInfo       `json:"conns,omitempty"`
	CPU         *StatAccumulator  `json:"cpu,omitempty"`
	Mem         *StatAccumulator  `json:"mem,omitempty"`
	Complete    bool              `json:"complete,omitempty"`
	Xy          int               `json:"xy,omitempty"` // Flag to indicate runner-to-base system messages
}

type Workflow struct {
	Params map[string]interface{} `json:"params"`
}

type Input struct {
	Files []InputFile `json:"files"`
}

type InputFile struct {
	Filename string `json:"filename"`
	Path     string `json:"path,omitempty"`
	Size     int64  `json:"size,omitempty"`
}

type UploadFileDef struct {
	Path     string
	Filename string
	Delete   bool
}

type UploadFileResult struct {
	ID       string `json:"id"`
	Date     int    `json:"date"`
	Filename string `json:"filename"`
	Path     string `json:"path"`
	Size     int64  `json:"size"`
	Server   string `json:"server"`
	Job      string `json:"job"`
}

type HTTPFileOpts struct {
	Timeout        int `json:"timeout"`
	IdleTimeout    int `json:"idleTimeout"`
	ConnectTimeout int `json:"connectTimeout"`
	Retries        int `json:"retries"`
	RetryDelay     int `json:"retryDelay"`
	RetryDelayMax  int `json:"retryDelayMax"`
}

type StatAccumulator struct {
	Min     float64 `json:"min"`
	Max     float64 `json:"max"`
	Total   float64 `json:"total"`
	Count   int     `json:"count"`
	Current float64 `json:"current"`
}

type ProcInfo struct {
	PID       int     `json:"pid"`
	ParentPID int     `json:"parentPid"`
	Command   string  `json:"command"`
	CPU       float64 `json:"cpu"`
	MemRSS    int64   `json:"memRss"`
	MemVSZ    int64   `json:"memVsz"`
	TTY       string  `json:"tty,omitempty"`
	Threads   int     `json:"threads"`
	Priority  int     `json:"priority"`
	Nice      int     `json:"nice"`
	State     string  `json:"state"`
	Age       int     `json:"age"`
	Class     string  `json:"class,omitempty"`
	Group     string  `json:"group,omitempty"`
	Started   int     `json:"started"`
	Disk      int64   `json:"disk"`
	Net       int64   `json:"net"`
	Conns     int     `json:"conns"`
}

type ConnInfo struct {
	Type       string `json:"type"`
	State      string `json:"state"`
	LocalAddr  string `json:"local_addr"`
	RemoteAddr string `json:"remote_addr"`
	PID        int    `json:"pid"`
	Bytes      int64  `json:"bytes"`
	Delta      int64  `json:"delta"`
	Started    int    `json:"started"`
}
