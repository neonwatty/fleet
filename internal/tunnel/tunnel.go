package tunnel

import (
	"fmt"
	"os/exec"

	"github.com/neonwatty/fleet/internal/config"
)

func AllocatePort(rangeStart, rangeEnd int, used map[int]bool) (int, error) {
	for p := rangeStart; p <= rangeEnd; p++ {
		if !used[p] {
			return p, nil
		}
	}
	return 0, fmt.Errorf("no available ports in range %d-%d", rangeStart, rangeEnd)
}

func AllocatePortPinned(port int, used map[int]bool) (int, error) {
	if used[port] {
		return 0, fmt.Errorf("pinned port %d is already in use by another session", port)
	}
	return port, nil
}

type Tunnel struct {
	LocalPort  int
	RemotePort int
	Machine    config.Machine
	Cmd        *exec.Cmd
}

func Start(m config.Machine, localPort, remotePort int) (*Tunnel, error) {
	if m.IsLocal() {
		return &Tunnel{
			LocalPort:  remotePort,
			RemotePort: remotePort,
			Machine:    m,
		}, nil
	}

	arg := fmt.Sprintf("%d:localhost:%d", localPort, remotePort)
	cmd := exec.Command("ssh",
		"-N",
		"-L", arg,
		"-o", "ExitOnForwardFailure=yes",
		"-o", "ConnectTimeout=5",
		m.Host,
	)

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start tunnel: %w", err)
	}

	return &Tunnel{
		LocalPort:  localPort,
		RemotePort: remotePort,
		Machine:    m,
		Cmd:        cmd,
	}, nil
}

func (t *Tunnel) Stop() error {
	if t.Cmd == nil || t.Cmd.Process == nil {
		return nil
	}
	if t.Cmd.ProcessState != nil {
		return nil
	}
	err := t.Cmd.Process.Kill()
	_ = t.Cmd.Wait()
	return err
}
