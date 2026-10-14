package jobs

import (
	"testing"
	"time"
)

var t0 = time.Date(2026, 10, 13, 10, 0, 0, 0, time.UTC)

func TestJobsRunOneAtATimeInOrder(t *testing.T) {
	var q Queue
	a := q.Add("upgrade fd", []string{"upgrade", "fd"})
	b := q.Add("pin git", []string{"pin", "git"})
	if a.ID == b.ID {
		t.Fatal("job IDs must be unique")
	}
	if got := q.Next(t0); got != a || a.State != Running || a.Attempts != 1 || !a.Started.Equal(t0) {
		t.Fatalf("first Next should start a: %+v", got)
	}
	if q.Next(t0) != nil {
		t.Error("a second job must not start while one is running")
	}
	if q.Queued() != 1 || q.Current() != a {
		t.Errorf("queued %d, current %+v", q.Queued(), q.Current())
	}
	if retry := q.Finish(a, 0, false, "", t0.Add(time.Second)); retry || a.State != Done || !a.Finished() {
		t.Errorf("exit 0 is done: %+v", a)
	}
	if q.Next(t0) != b || q.Queued() != 0 {
		t.Error("b should start after a finishes")
	}
}

func TestFinishStates(t *testing.T) {
	tests := []struct {
		code     int
		canceled bool
		output   string
		want     State
	}{
		{0, false, "", Done},
		{1, false, "Error: No such keg", Failed},
		{-1, true, "", Canceled},
		{130, true, "", Canceled},
		{0, true, "", Done}, // brew took the ctrl+c as an answer and finished anyway
		{1, false, "Error: A `brew upgrade` process has already locked /opt/homebrew/Cellar/fd.lock.", Waiting},
	}
	for _, tt := range tests {
		var q Queue
		j := q.Add("x", nil)
		q.Next(t0)
		retry := q.Finish(j, tt.code, tt.canceled, tt.output, t0)
		if j.State != tt.want || retry != (tt.want == Waiting) || j.ExitCode != tt.code {
			t.Errorf("Finish(%d, %v, %q) -> %v retry=%v", tt.code, tt.canceled, tt.output, j.State, retry)
		}
	}
}

func TestLockedJobRetriesThenFails(t *testing.T) {
	var q Queue
	j := q.Add("upgrade", []string{"upgrade"})
	other := q.Add("pin git", nil)
	q.Next(t0)
	locked := "Error: A `brew update` process has already locked /opt/homebrew/var/homebrew/locks/update."
	for attempt := 1; attempt < MaxAttempts; attempt++ {
		if !q.Finish(j, 1, false, locked, t0) {
			t.Fatalf("attempt %d should retry", attempt)
		}
		if q.Next(t0) != nil || q.Current() != j {
			t.Fatal("nothing else may start while a job waits for the lock")
		}
		q.Retry(j)
		if j.State != Running || j.Attempts != attempt+1 {
			t.Fatalf("after retry: %+v", j)
		}
	}
	if q.Finish(j, 1, false, locked, t0) || j.State != Failed {
		t.Errorf("after %d attempts the job fails: %+v", MaxAttempts, j)
	}
	if q.Next(t0) != other {
		t.Error("the next job starts once the locked one gives up")
	}
}

func TestIsLockError(t *testing.T) {
	for _, tt := range []struct {
		output string
		want   bool
	}{
		{"Error: A `brew upgrade` process has already locked /opt/homebrew/Cellar/fd.lock.", true},
		{"==> Upgrading fd\r\n\x1b[4;31mError\x1b[0m: A `brew upgrade fd` process has already locked /opt/homebrew/Cellar/fd.lock.\r\nPlease wait for it to finish or terminate it to continue.\r\n", true},
		{"A `brew update` process has already locked /x.", false},                         // not brew's error line
		{"==> Caveats\nThis tool says: Error: A `x` process has already locked y", false}, // mid-line
		{"Error: No such keg: /opt/homebrew/Cellar/fd", false},
	} {
		if got := IsLockError(tt.output); got != tt.want {
			t.Errorf("IsLockError(%q) = %v, want %v", tt.output, got, tt.want)
		}
	}
}

func TestBackoff(t *testing.T) {
	want := []time.Duration{2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second, 30 * time.Second, 30 * time.Second}
	for i, w := range want {
		if got := Backoff(i + 1); got != w {
			t.Errorf("Backoff(%d) = %v, want %v", i+1, got, w)
		}
	}
}

func TestStateString(t *testing.T) {
	if Waiting.String() != "waiting" || Canceled.String() != "canceled" {
		t.Error("state names wrong")
	}
}
