package session

import (
	"fmt"
	"path/filepath"
	"sync"
	"testing"
)

func TestConcurrentAddSessionDoesNotDropEntries(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	const count = 25
	var wg sync.WaitGroup
	errCh := make(chan error, count)
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errCh <- AddSession(path, Session{ID: fmt.Sprintf("s-%02d", i)})
		}(i)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatalf("AddSession() error: %v", err)
		}
	}

	loaded, err := LoadState(path)
	if err != nil {
		t.Fatalf("LoadState() error: %v", err)
	}
	if len(loaded.Sessions) != count {
		t.Fatalf("len(Sessions) = %d, want %d", len(loaded.Sessions), count)
	}
}

func TestWithStateLockReleasesAfterCallbackError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	errBoom := fmt.Errorf("boom")
	if err := WithStateLock(path, func(s *State) error {
		s.Sessions = append(s.Sessions, Session{ID: "not-saved"})
		return errBoom
	}); err != errBoom {
		t.Fatalf("WithStateLock() error = %v, want %v", err, errBoom)
	}

	if err := AddSession(path, Session{ID: "saved"}); err != nil {
		t.Fatalf("AddSession after callback error: %v", err)
	}
	loaded, err := LoadState(path)
	if err != nil {
		t.Fatalf("LoadState() error: %v", err)
	}
	if len(loaded.Sessions) != 1 || loaded.Sessions[0].ID != "saved" {
		t.Fatalf("loaded sessions = %+v, want only saved session", loaded.Sessions)
	}
}
