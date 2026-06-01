package runner

import (
	"testing"
	"time"

	"github.com/fabiang/go-xyrun/internal/models"
)

// buildApp creates a minimal App suitable for metrics tests.
func buildApp(worker *Worker) *App {
	return &App{
		activeJobs: make(map[string]*models.Job),
		kids:       make(map[int]*Worker),
		connCache:  make(map[string]*ConnCacheInfo),
		worker:     worker,
		numCPUs:    1,
	}
}

func TestMeasureJobResources_NoProcForRootPID(t *testing.T) {
	// When the root PID is not in the pid map, job CPU/Mem should stay nil.
	app := buildApp(&Worker{PID: 99999})
	job := &models.Job{}
	pids := map[int]*models.ProcInfo{}

	app.measureJobResources(job, pids)

	if job.CPU != nil {
		t.Error("expected CPU to remain nil when root PID not in pids")
	}
	if job.Mem != nil {
		t.Error("expected Mem to remain nil when root PID not in pids")
	}
}

func TestMeasureJobResources_UnmatchedPid_AccumulatorsUnchanged(t *testing.T) {
	// Pre-seed CPU/Mem with values; when no root PID matches, they must stay unchanged.
	app := buildApp(&Worker{PID: 1})
	job := &models.Job{
		CPU: &models.StatAccumulator{Min: 10, Max: 10, Total: 10, Count: 1, Current: 10},
		Mem: &models.StatAccumulator{Min: 512, Max: 512, Total: 512, Count: 1, Current: 512},
	}

	pids := map[int]*models.ProcInfo{}
	app.measureJobResources(job, pids)

	if job.CPU.Count != 1 {
		t.Errorf("expected CPU count to remain 1, got %d", job.CPU.Count)
	}
	if job.Mem.Count != 1 {
		t.Errorf("expected Mem count to remain 1, got %d", job.Mem.Count)
	}
}

func TestGetProcsCached_ReturnsCachedResult(t *testing.T) {
	app := buildApp(&Worker{PID: 1})

	cached := &ProcInfoResult{List: []*models.ProcInfo{
		{PID: 1, Command: "test"},
	}}
	app.procCache = &ProcCache{
		Data:    cached,
		Expires: time.Now().Add(1 * time.Hour),
	}

	var got *ProcInfoResult
	app.getProcsCached(func(data *ProcInfoResult) {
		got = data
	})

	if got == nil {
		t.Fatal("callback was not called")
	}
	if got != cached {
		t.Error("expected the cached result to be returned")
	}
}

func TestGetProcsCached_InitializesCacheWhenNil(t *testing.T) {
	app := buildApp(&Worker{PID: 1})

	called := false
	app.getProcsCached(func(data *ProcInfoResult) {
		called = true
		if data == nil {
			t.Error("callback received nil data")
		}
	})

	if !called {
		t.Error("callback was not called")
	}
	if app.procCache == nil {
		t.Error("expected procCache to be initialized after first call")
	}
}

func TestGetProcsCached_ExpiredCache_RefetchesData(t *testing.T) {
	app := buildApp(&Worker{PID: 1})

	old := &ProcInfoResult{List: []*models.ProcInfo{{PID: 99, Command: "old"}}}
	app.procCache = &ProcCache{
		Data:    old,
		Expires: time.Now().Add(-1 * time.Second), // already expired
	}

	var got *ProcInfoResult
	app.getProcsCached(func(data *ProcInfoResult) {
		got = data
	})

	if got == nil {
		t.Fatal("callback was not called")
	}
	// Fresh data should differ from the expired cached pointer
	if got == old {
		t.Error("expected fresh data to be fetched, not the old cached result")
	}
}

func TestTick_NilJob_DoesNotPanic(t *testing.T) {
	app := buildApp(nil)
	app.job = nil

	// Should return immediately without panicking.
	app.tick()
}

func TestTick_TickLocked_DoesNotPanic(t *testing.T) {
	app := buildApp(nil)
	app.tickLock = true

	// Should return immediately without panicking.
	app.tick()
}
