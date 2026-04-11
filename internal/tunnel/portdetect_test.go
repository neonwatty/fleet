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

func TestDetectPortFallback(t *testing.T) {
	dir := t.TempDir()
	devPort, _ := DetectPorts(dir)
	if devPort != 3000 {
		t.Errorf("devPort = %d, want 3000 (fallback)", devPort)
	}
}
