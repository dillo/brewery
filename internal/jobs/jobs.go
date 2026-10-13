// Package jobs tracks the brew commands brewery runs. They run one at a
// time, in the order they were asked for, because brew holds a global lock.
package jobs

import (
	"strings"
	"time"
)

// State is where a job is in its life.
type State int

const (
	Queued   State = iota
	Running        // started and not yet exited
	Waiting        // exited because another brew held the lock; will retry
	Done           // exited 0
	Failed         // exited non-zero, or could not start
	Canceled       // stopped by the user
)

func (s State) String() string {
	return [...]string{"queued", "running", "waiting", "done", "failed", "canceled"}[s]
}

// Job is one brew command.
type Job struct {
	ID       int
	Title    string   // what the user asked for, e.g. "upgrade fd"
	Args     []string // brew's arguments
	State    State
	ExitCode int
	Attempts int // starts so far, counting retries after a lock
	Started  time.Time
	Ended    time.Time
}

// Finished reports whether the job has stopped for good.
func (j *Job) Finished() bool { return j.State >= Done }

// Queue holds the session's jobs, oldest first.
type Queue struct {
	jobs   []*Job
	nextID int
}

// Add queues a job and returns it.
func (q *Queue) Add(title string, args []string) *Job {
	q.nextID++
	j := &Job{ID: q.nextID, Title: title, Args: args}
	q.jobs = append(q.jobs, j)
	return j
}

// Current is the running or lock-waiting job, or nil.
func (q *Queue) Current() *Job {
	for _, j := range q.jobs {
		if j.State == Running || j.State == Waiting {
			return j
		}
	}
	return nil
}

// Next starts the oldest queued job and returns it. It returns nil while
// another job is current or when nothing is queued.
func (q *Queue) Next(now time.Time) *Job {
	if q.Current() != nil {
		return nil
	}
	for _, j := range q.jobs {
		if j.State == Queued {
			j.State, j.Started, j.Attempts = Running, now, 1
			return j
		}
	}
	return nil
}

// Queued counts jobs waiting their turn.
func (q *Queue) Queued() int {
	n := 0
	for _, j := range q.jobs {
		if j.State == Queued {
			n++
		}
	}
	return n
}

// Finish records how a job's run ended. If brew refused to run because
// another brew process held the lock, the job waits to retry instead of
// failing, until MaxAttempts runs have been tried. It reports whether the
// job will be retried.
func (q *Queue) Finish(j *Job, code int, canceled bool, output string, now time.Time) (retry bool) {
	j.ExitCode = code
	switch {
	case canceled:
		j.State = Canceled
	case code == 0:
		j.State = Done
	case IsLockError(output) && j.Attempts < MaxAttempts:
		j.State = Waiting
		return true
	default:
		j.State = Failed
	}
	j.Ended = now
	return false
}

// Retry runs a waiting job again.
func (q *Queue) Retry(j *Job) {
	if j.State == Waiting {
		j.State = Running
		j.Attempts++
	}
}

// MaxAttempts is how many times a job runs before a held lock fails it.
const MaxAttempts = 5

// Backoff is how long to wait before retry number attempt (1-based):
// 2s, 4s, 8s, 16s, then 30s.
func Backoff(attempt int) time.Duration {
	return min(2*time.Second<<max(attempt-1, 0), 30*time.Second)
}

// IsLockError reports whether brew's output says another brew process
// holds the lock it needs ("A `brew …` process has already locked …").
func IsLockError(output string) bool {
	return strings.Contains(output, "has already locked")
}
