package brew

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"syscall"

	"github.com/creack/pty"
)

// JobEnv is added to the environment of commands started from a pane: an
// action should do what it says without running `brew update` first.
var JobEnv = []string{"HOMEBREW_NO_AUTO_UPDATE=1", "HOMEBREW_NO_ENV_HINTS=1"}

// Proc is a brew command running in a pseudo-terminal, so it behaves as it
// would in a shell: colors, progress bars and password prompts all work.
type Proc struct {
	cmd *exec.Cmd
	pty *os.File
}

// StartPTY starts bin with args in a new pseudo-terminal of cols×rows.
// env is appended to the current environment. The command leads its own
// process group, so Interrupt and Kill reach everything it started.
func StartPTY(bin string, args, env []string, cols, rows int) (*Proc, error) {
	cmd := exec.Command(bin, args...)
	cmd.Env = append(os.Environ(), env...)
	f, err := pty.StartWithSize(cmd, winsize(cols, rows))
	if err != nil {
		return nil, err
	}
	return &Proc{cmd: cmd, pty: f}, nil
}

func winsize(cols, rows int) *pty.Winsize {
	return &pty.Winsize{Cols: uint16(max(cols, 1)), Rows: uint16(max(rows, 1))}
}

// Read returns the command's output. Once the command has exited and its
// output is drained, it returns io.EOF.
func (p *Proc) Read(b []byte) (int, error) {
	n, err := p.pty.Read(b)
	if errors.Is(err, syscall.EIO) {
		err = io.EOF // the terminal's other side closed: the command is gone
	}
	return n, err
}

// Write sends keystrokes to the command, as typing in a terminal would.
func (p *Proc) Write(b []byte) (int, error) { return p.pty.Write(b) }

// Resize tells the command its terminal is now cols×rows.
func (p *Proc) Resize(cols, rows int) error { return pty.Setsize(p.pty, winsize(cols, rows)) }

// Interrupt sends SIGINT to the command's process group, like ctrl+c.
func (p *Proc) Interrupt() error { return syscall.Kill(-p.cmd.Process.Pid, syscall.SIGINT) }

// Kill ends the command's process group immediately.
func (p *Proc) Kill() error { return syscall.Kill(-p.cmd.Process.Pid, syscall.SIGKILL) }

// Wait waits for the command to exit and returns its exit code, or -1 if a
// signal ended it. Call it after Read has returned io.EOF.
func (p *Proc) Wait() (int, error) {
	err := p.cmd.Wait()
	p.pty.Close()
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		return exitErr.ExitCode(), nil
	}
	if err != nil {
		return -1, err
	}
	return 0, nil
}
