package tunnel

import (
	"fmt"
	"net"
	"os/exec"
	"time"

	"github.com/neonwatty/fleet/internal/config"
)

func AllocatePort(rangeStart, rangeEnd int, used map[int]bool) (int, error) {
	for p := rangeStart; p <= rangeEnd; p++ {
		if !used[p] && IsPortAvailable(p) {
			return p, nil
		}
	}
	return 0, fmt.Errorf("no available ports in range %d-%d", rangeStart, rangeEnd)
}

func AllocatePortPinned(port int, used map[int]bool) (int, error) {
	if used[port] {
		return 0, fmt.Errorf("pinned port %d is already in use by another session", port)
	}
	if !IsPortAvailable(port) {
		return 0, fmt.Errorf("pinned port %d is already in use by another process", port)
	}
	return port, nil
}

func IsPortAvailable(port int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return false
	}
	_ = ln.Close()
	return true
}

type Tunnel struct {
	LocalPort  int
	RemotePort int
	Machine    config.Machine
	Cmd        *exec.Cmd
	done       chan error
}

var sshCommandPath = "ssh"

func Start(m config.Machine, localPort, remotePort int) (*Tunnel, error) {
	if m.IsLocal() {
		return &Tunnel{
			LocalPort:  remotePort,
			RemotePort: remotePort,
			Machine:    m,
		}, nil
	}

	arg := fmt.Sprintf("%d:localhost:%d", localPort, remotePort)
	cmd := exec.Command(sshCommandPath, buildSSHForwardArgs(m, arg)...)

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start tunnel: %w", err)
	}
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()
	if err := verifyStarted(done, localPort); err != nil {
		_ = cmd.Process.Kill()
		select {
		case <-done:
		default:
		}
		return nil, err
	}

	return &Tunnel{
		LocalPort:  localPort,
		RemotePort: remotePort,
		Machine:    m,
		Cmd:        cmd,
		done:       done,
	}, nil
}

func verifyStarted(done <-chan error, localPort int) error {
	deadline := time.Now().Add(700 * time.Millisecond)
	for time.Now().Before(deadline) {
		select {
		case err := <-done:
			if err != nil {
				return fmt.Errorf("tunnel exited during startup: %w", err)
			}
			return fmt.Errorf("tunnel exited during startup")
		default:
		}
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", localPort), 50*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return nil
}

func buildSSHForwardArgs(m config.Machine, forwardArg string) []string {
	return []string{
		"-N",
		"-L", forwardArg,
		"-o", "ExitOnForwardFailure=yes",
		"-o", "ConnectTimeout=5",
		m.SSHTarget(),
	}
}

func (t *Tunnel) Stop() error {
	if t.Cmd == nil || t.Cmd.Process == nil {
		return nil
	}
	if t.Cmd.ProcessState != nil {
		return nil
	}
	err := t.Cmd.Process.Kill()
	if t.done != nil {
		<-t.done
	} else {
		_ = t.Cmd.Wait()
	}
	return err
}
