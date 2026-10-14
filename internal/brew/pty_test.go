package brew

import (
	"io"
	"strings"
	"testing"
	"time"
)

// sh starts a shell script in a pseudo-terminal.
func sh(t *testing.T, script string, env ...string) *Proc {
	t.Helper()
	p, err := StartPTY("/bin/sh", []string{"-c", script}, env, 80, 24)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// finish reads all output, then waits, failing the test if that takes
// longer than 5s.
func finish(t *testing.T, p *Proc) (string, int) {
	t.Helper()
	type result struct {
		out  string
		code int
	}
	done := make(chan result)
	go func() {
		b, _ := io.ReadAll(p)
		code, err := p.Wait()
		if err != nil {
			t.Error(err)
		}
		done <- result{string(b), code}
	}()
	select {
	case r := <-done:
		return r.out, r.code
	case <-time.After(5 * time.Second):
		t.Fatal("command did not finish")
		return "", 0
	}
}

func TestPTYOutputAndExitCode(t *testing.T) {
	out, code := finish(t, sh(t, `printf 'hello\nworld'; exit 3`))
	if !strings.Contains(out, "hello\r\nworld") {
		t.Errorf("output %q: a terminal turns \\n into \\r\\n", out)
	}
	if code != 3 {
		t.Errorf("exit code %d, want 3", code)
	}
}

func TestPTYIsATerminalOfTheRightSize(t *testing.T) {
	out, _ := finish(t, sh(t, `[ -t 1 ] && echo tty; stty size`))
	if !strings.Contains(out, "tty") || !strings.Contains(out, "24 80") {
		t.Errorf("want a 80x24 terminal, got %q", out)
	}
}

func TestPTYResize(t *testing.T) {
	p := sh(t, `sleep 0.3; stty size`)
	if err := p.Resize(100, 30); err != nil {
		t.Fatal(err)
	}
	if out, _ := finish(t, p); !strings.Contains(out, "30 100") {
		t.Errorf("want 100x30 after resize, got %q", out)
	}
}

func TestPTYInput(t *testing.T) {
	p := sh(t, `read answer; echo "got:$answer"`)
	if _, err := p.Write([]byte("yes\r")); err != nil {
		t.Fatal(err)
	}
	if out, _ := finish(t, p); !strings.Contains(out, "got:yes") {
		t.Errorf("input not delivered: %q", out)
	}
}

func TestPTYEnv(t *testing.T) {
	if out, _ := finish(t, sh(t, `echo "v=$BREWERY_TEST"`, "BREWERY_TEST=42")); !strings.Contains(out, "v=42") {
		t.Errorf("env not passed: %q", out)
	}
}

func TestPTYInterruptReachesChildren(t *testing.T) {
	p := sh(t, `sleep 30 & wait`)
	time.Sleep(100 * time.Millisecond)
	if err := p.Interrupt(); err != nil {
		t.Fatal(err)
	}
	if _, code := finish(t, p); code == 0 {
		t.Error("an interrupted command should not report success")
	}
}

func TestPTYKill(t *testing.T) {
	p := sh(t, `trap '' INT; sleep 30`)
	time.Sleep(100 * time.Millisecond)
	if err := p.Kill(); err != nil {
		t.Fatal(err)
	}
	if _, code := finish(t, p); code != -1 {
		t.Errorf("a killed command reports -1, got %d", code)
	}
}

// A process brew leaves running in the background (a daemon a post-install
// step started) keeps the terminal open, but on macOS the output still ends
// when brew exits, so the job finishes then.
func TestPTYEndsWhenTheCommandExitsDespiteABackgroundChild(t *testing.T) {
	p := sh(t, `(trap '' HUP; sleep 3) & echo started`)
	start := time.Now()
	out, code := finish(t, p)
	if !strings.Contains(out, "started") || code != 0 {
		t.Errorf("got %q, exit %d", out, code)
	}
	if d := time.Since(start); d > 2*time.Second {
		t.Errorf("output ended after %v: it waited for the background child", d)
	}
}
