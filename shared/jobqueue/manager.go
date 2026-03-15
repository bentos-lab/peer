package jobqueue

import (
	"fmt"
	"sync"
	"sync/atomic"
)

// Job defines a unit of work for the queue.
type Job struct {
	Name      string
	DependsOn []string
	Run       func() error
}

type jobState int

const (
	jobPending jobState = iota
	jobRunning
	jobCompleted
)

type queuedJob struct {
	id    string
	name  string
	deps  []string
	run   func() error
	err   error
	state jobState
}

// Manager executes jobs with dependency tracking and bounded concurrency.
type Manager struct {
	maxWorkers int

	mu    sync.Mutex
	cond  *sync.Cond
	queue []string
	jobs  map[string]*queuedJob

	nextID uint64
}

// NewManager creates a job manager and starts worker goroutines.
func NewManager(maxWorkers int) *Manager {
	if maxWorkers <= 0 {
		maxWorkers = 1
	}
	manager := &Manager{
		maxWorkers: maxWorkers,
		jobs:       make(map[string]*queuedJob),
	}
	manager.cond = sync.NewCond(&manager.mu)
	for i := 0; i < maxWorkers; i++ {
		go manager.worker()
	}
	return manager
}

// Enqueue registers a job and returns its ID.
func (m *Manager) Enqueue(job Job) (string, error) {
	if m == nil {
		return "", fmt.Errorf("job manager is nil")
	}
	if job.Run == nil {
		return "", fmt.Errorf("job run function is required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	id := m.nextJobID()
	m.jobs[id] = &queuedJob{
		id:    id,
		name:  job.Name,
		deps:  append([]string(nil), job.DependsOn...),
		run:   job.Run,
		state: jobPending,
	}
	m.queue = append(m.queue, id)
	m.cond.Signal()
	return id, nil
}

func (m *Manager) nextJobID() string {
	value := atomic.AddUint64(&m.nextID, 1)
	return fmt.Sprintf("job-%d", value)
}

func (m *Manager) worker() {
	for {
		job := m.nextReadyJob()
		if job == nil {
			continue
		}
		job.err = job.run()
		m.finishJob(job)
	}
}

func (m *Manager) nextReadyJob() *queuedJob {
	m.mu.Lock()
	defer m.mu.Unlock()

	for {
		for idx, id := range m.queue {
			job := m.jobs[id]
			if job == nil || job.state != jobPending {
				continue
			}
			if !m.dependenciesComplete(job) {
				continue
			}

			job.state = jobRunning
			m.queue = append(m.queue[:idx], m.queue[idx+1:]...)
			return job
		}

		m.cond.Wait()
	}
}

func (m *Manager) dependenciesComplete(job *queuedJob) bool {
	for _, depID := range job.deps {
		dep := m.jobs[depID]
		if dep == nil {
			continue
		}
		if dep.state != jobCompleted {
			return false
		}
	}
	return true
}

func (m *Manager) finishJob(job *queuedJob) {
	m.mu.Lock()
	job.state = jobCompleted
	m.mu.Unlock()
	m.cond.Broadcast()
}
