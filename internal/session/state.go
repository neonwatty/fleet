package session

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type State struct {
	Sessions      []Session                 `json:"sessions"`
	MachineLabels map[string][]MachineLabel `json:"machine_labels,omitempty"`
}

type Session struct {
	ID           string     `json:"id"`
	Project      string     `json:"project"`
	Machine      string     `json:"machine"`
	Branch       string     `json:"branch"`
	Account      string     `json:"account,omitempty"`
	WorktreePath string     `json:"worktree_path"`
	BareRepoPath string     `json:"bare_repo_path,omitempty"`
	Tunnel       TunnelInfo `json:"tunnel"`
	StartedAt    time.Time  `json:"started_at"`
	OwnerPID     int        `json:"pid"` // fleet CLI PID for signal cleanup, NOT the remote claude PID
}

type MachineLabel struct {
	Name        string    `json:"name"`
	SessionID   string    `json:"session_id,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	LastSeenPID int       `json:"last_seen_pid,omitempty"`
}

type TunnelInfo struct {
	LocalPort  int `json:"local_port"`
	RemotePort int `json:"remote_port"`
}

func DefaultStatePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".fleet", "state.json")
}

func LoadState(path string) (*State, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &State{}, nil
		}
		return nil, fmt.Errorf("read state: %w", err)
	}

	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse state: %w", err)
	}
	return &s, nil
}

func Save(path string, s *State) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}

	tmp, err := os.CreateTemp(dir, ".state-*.json")
	if err != nil {
		return fmt.Errorf("create temp state: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath) //nolint:errcheck

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp state: %w", err)
	}
	if err := tmp.Chmod(0644); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod temp state: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync temp state: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp state: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("replace state: %w", err)
	}
	syncDir(dir)
	return nil
}

func AddSession(path string, sess Session) error {
	s, err := LoadState(path)
	if err != nil {
		return err
	}
	s.Sessions = append(s.Sessions, sess)
	return Save(path, s)
}

func RemoveSession(path string, id string) error {
	s, err := LoadState(path)
	if err != nil {
		return err
	}

	filtered := make([]Session, 0, len(s.Sessions))
	for _, sess := range s.Sessions {
		if sess.ID != id {
			filtered = append(filtered, sess)
		}
	}
	s.Sessions = filtered
	return Save(path, s)
}

func (s *State) UsedPorts() map[int]bool {
	ports := make(map[int]bool)
	for _, sess := range s.Sessions {
		if sess.Tunnel.LocalPort > 0 {
			ports[sess.Tunnel.LocalPort] = true
		}
	}
	return ports
}

func GenerateID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("generate session id: %v", err))
	}
	return hex.EncodeToString(b)
}

func syncDir(path string) {
	dir, err := os.Open(path)
	if err != nil {
		return
	}
	defer dir.Close() //nolint:errcheck
	_ = dir.Sync()
}
