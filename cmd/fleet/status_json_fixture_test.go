package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestSwiftFixtureMatchesGoSchema decodes the fixture used by the native
// menu bar app's Swift tests into the same Go type that buildStatusJSON
// emits. If either side drifts, this test fails loudly.
func TestSwiftFixtureMatchesGoSchema(t *testing.T) {
	path := filepath.Join("..", "..", "menubar", "Tests", "FleetMenuBarTests", "Fixtures", "status.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	var doc statusDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if doc.Version != "1" {
		t.Errorf("version = %q, want \"1\"", doc.Version)
	}
	if len(doc.Machines) != 3 {
		t.Errorf("machines len = %d, want 3", len(doc.Machines))
	}
	if doc.Thresholds.SwapWarnMB != 1024 || doc.Thresholds.SwapHighMB != 4096 {
		t.Errorf("thresholds = %+v, want swap_warn_mb=1024 swap_high_mb=4096", doc.Thresholds)
	}
	if len(doc.Sessions) != 2 {
		t.Errorf("sessions len = %d, want 2", len(doc.Sessions))
	}
	if doc.Sessions[0].LaunchCommand != "npm run agent" {
		t.Errorf("first session launch_command = %q, want npm run agent", doc.Sessions[0].LaunchCommand)
	}
}
