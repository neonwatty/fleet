package tunnel

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectPortFromFleetToml(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".fleet.toml"), []byte(`
dev_port = 3001
tunnel_local_port = 3001
`), 0644)

	devPort, localPort := DetectPorts(dir)
	if devPort != 3001 {
		t.Errorf("devPort = %d, want 3001", devPort)
	}
	if localPort != 3001 {
		t.Errorf("localPort = %d, want 3001 (pinned)", localPort)
	}
}

func TestDetectPortFromFleetTomlNoPinning(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".fleet.toml"), []byte(`
dev_port = 5173
`), 0644)

	devPort, localPort := DetectPorts(dir)
	if devPort != 5173 {
		t.Errorf("devPort = %d, want 5173", devPort)
	}
	if localPort != 0 {
		t.Errorf("localPort = %d, want 0 (no pin)", localPort)
	}
}

func TestDetectProjectConfigFromFleetTomlLaunchCommand(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".fleet.toml"), []byte(`
launch_command = "zsh -lc 'claude --dangerously-skip-permissions'"
`), 0644)

	cfg := DetectProjectConfig(dir)
	if cfg.DevPort != 3000 {
		t.Errorf("DevPort = %d, want 3000 fallback", cfg.DevPort)
	}
	if cfg.LaunchCommand != "zsh -lc 'claude --dangerously-skip-permissions'" {
		t.Errorf("LaunchCommand = %q", cfg.LaunchCommand)
	}
}

func TestDetectProjectConfigFromFleetTomlPortsAndLaunchCommand(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".fleet.toml"), []byte(`
dev_port = 4321
tunnel_local_port = 4322
launch_command = "npm run agent"
`), 0644)

	cfg := DetectProjectConfig(dir)
	if cfg.DevPort != 4321 {
		t.Errorf("DevPort = %d, want 4321", cfg.DevPort)
	}
	if cfg.TunnelLocalPort != 4322 {
		t.Errorf("TunnelLocalPort = %d, want 4322", cfg.TunnelLocalPort)
	}
	if cfg.LaunchCommand != "npm run agent" {
		t.Errorf("LaunchCommand = %q", cfg.LaunchCommand)
	}
}

func TestDetectPortFromPackageJSON(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{
  "scripts": {
    "dev": "next dev -p 3002"
  }
}`), 0644)

	devPort, localPort := DetectPorts(dir)
	if devPort != 3002 {
		t.Errorf("devPort = %d, want 3002", devPort)
	}
	if localPort != 0 {
		t.Errorf("localPort = %d, want 0", localPort)
	}
}

func TestDetectPortFromPackageJSONVite(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{
  "scripts": {
    "dev": "vite --port 5173"
  }
}`), 0644)

	devPort, _ := DetectPorts(dir)
	if devPort != 5173 {
		t.Errorf("devPort = %d, want 5173", devPort)
	}
}

func TestDetectPortFromPackageJSONPortEnv(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{
  "scripts": {
    "dev": "PORT=4321 next dev"
  }
}`), 0644)

	devPort, _ := DetectPorts(dir)
	if devPort != 4321 {
		t.Errorf("devPort = %d, want 4321", devPort)
	}
}

func TestDetectPortFromPackageJSONViteDefault(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{
  "scripts": {
    "dev": "vite --host 0.0.0.0"
  }
}`), 0644)

	devPort, _ := DetectPorts(dir)
	if devPort != 5173 {
		t.Errorf("devPort = %d, want 5173", devPort)
	}
}

func TestDetectPortFromPackageJSONNextDefault(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{
  "scripts": {
    "dev": "next dev"
  }
}`), 0644)

	devPort, _ := DetectPorts(dir)
	if devPort != 3000 {
		t.Errorf("devPort = %d, want 3000", devPort)
	}
}

func TestDetectPortFallback(t *testing.T) {
	dir := t.TempDir()
	devPort, _ := DetectPorts(dir)
	if devPort != 3000 {
		t.Errorf("devPort = %d, want 3000 (fallback)", devPort)
	}
}
