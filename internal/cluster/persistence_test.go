package cluster

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestFilePersister_SaveLoadRaftState(t *testing.T) {
	dir := t.TempDir()
	p, err := NewFilePersister(dir)
	if err != nil {
		t.Fatalf("NewFilePersister: %v", err)
	}

	// Load from empty — should return zero state
	state, err := p.LoadRaftState()
	if err != nil {
		t.Fatalf("LoadRaftState (empty): %v", err)
	}
	if state.Term != 0 || state.VotedFor != "" || state.CommitIndex != 0 {
		t.Errorf("expected zero state, got %+v", state)
	}

	// Save a state
	want := RaftStateV1{Term: 5, VotedFor: "node1", CommitIndex: 100, LastApplied: 99}
	if err := p.SaveRaftState(want); err != nil {
		t.Fatalf("SaveRaftState: %v", err)
	}

	// Load it back
	got, err := p.LoadRaftState()
	if err != nil {
		t.Fatalf("LoadRaftState: %v", err)
	}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestFilePersister_SaveLoadLogEntries(t *testing.T) {
	dir := t.TempDir()
	p, err := NewFilePersister(dir)
	if err != nil {
		t.Fatalf("NewFilePersister: %v", err)
	}

	// Load from empty — should return nil
	entries, err := p.LoadLogEntries()
	if err != nil {
		t.Fatalf("LoadLogEntries (empty): %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}

	// Save some entries
	want := []*LogEntry{
		{Index: 1, Term: 1, Command: []byte(`{"type":"set_config"}`)},
		{Index: 2, Term: 1, Command: []byte(`{"type":"update_backend"}`)},
		{Index: 3, Term: 2, Command: []byte(`{"type":"update_route"}`)},
	}
	if err := p.SaveLogEntries(want); err != nil {
		t.Fatalf("SaveLogEntries: %v", err)
	}

	// Load back
	got, err := p.LoadLogEntries()
	if err != nil {
		t.Fatalf("LoadLogEntries: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(got))
	}
	for i, e := range got {
		if e.Index != want[i].Index {
			t.Errorf("entry %d: Index = %d, want %d", i, e.Index, want[i].Index)
		}
		if e.Term != want[i].Term {
			t.Errorf("entry %d: Term = %d, want %d", i, e.Term, want[i].Term)
		}
		var cmd1, cmd2 map[string]any
		json.Unmarshal(e.Command, &cmd1)
		json.Unmarshal(want[i].Command, &cmd2)
		if cmd1["type"] != cmd2["type"] {
			t.Errorf("entry %d: Command type mismatch", i)
		}
	}
}

func TestFilePersister_SaveSingleLogEntry(t *testing.T) {
	dir := t.TempDir()
	p, err := NewFilePersister(dir)
	if err != nil {
		t.Fatalf("NewFilePersister: %v", err)
	}

	entry := &LogEntry{Index: 5, Term: 1, Command: []byte("cmd5")}
	if err := p.SaveLogEntry(entry); err != nil {
		t.Fatalf("SaveLogEntry: %v", err)
	}

	entries, err := p.LoadLogEntries()
	if err != nil {
		t.Fatalf("LoadLogEntries: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Index != 5 {
		t.Errorf("Index = %d, want 5", entries[0].Index)
	}
}

func TestFilePersister_DeleteLogBefore(t *testing.T) {
	dir := t.TempDir()
	p, err := NewFilePersister(dir)
	if err != nil {
		t.Fatalf("NewFilePersister: %v", err)
	}

	entries := []*LogEntry{
		{Index: 1, Term: 1, Command: []byte("a")},
		{Index: 2, Term: 1, Command: []byte("b")},
		{Index: 3, Term: 1, Command: []byte("c")},
		{Index: 4, Term: 1, Command: []byte("d")},
	}
	if err := p.SaveLogEntries(entries); err != nil {
		t.Fatalf("SaveLogEntries: %v", err)
	}

	// Delete entries with index <= 2
	if err := p.DeleteLogBefore(2); err != nil {
		t.Fatalf("DeleteLogBefore: %v", err)
	}

	got, err := p.LoadLogEntries()
	if err != nil {
		t.Fatalf("LoadLogEntries: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 entries after compaction, got %d", len(got))
	}
	if got[0].Index != 3 || got[1].Index != 4 {
		t.Errorf("remaining entries: %+v", got)
	}
}

func TestFilePersister_DirCreated(t *testing.T) {
	dir := t.TempDir()
	_, err := NewFilePersister(dir)
	if err != nil {
		t.Fatalf("NewFilePersister: %v", err)
	}

	// Verify directory was created
	if _, err := os.Stat(filepath.Join(dir, "log")); os.IsNotExist(err) {
		t.Error("log directory was not created")
	}
}

func TestFilePersister_CorruptedStateFile(t *testing.T) {
	dir := t.TempDir()
	p, err := NewFilePersister(dir)
	if err != nil {
		t.Fatalf("NewFilePersister: %v", err)
	}

	// Write corrupted data
	if err := os.WriteFile(filepath.Join(dir, "raft_state.json"), []byte("not json"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err = p.LoadRaftState()
	if err == nil {
		t.Error("expected error loading corrupted state file")
	}
}

func TestFilePersister_CorruptedLogEntry(t *testing.T) {
	dir := t.TempDir()
	p, err := NewFilePersister(dir)
	if err != nil {
		t.Fatalf("NewFilePersister: %v", err)
	}

	// Write a valid and a corrupted entry
	validEntry := &LogEntry{Index: 1, Term: 1, Command: []byte("a")}
	p.SaveLogEntry(validEntry)

	logDir := filepath.Join(dir, "log")
	os.WriteFile(filepath.Join(logDir, "entry_0000000002.json"), []byte("bad json"), 0644)

	// Should load only the valid entry
	got, err := p.LoadLogEntries()
	if err != nil {
		t.Fatalf("LoadLogEntries: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("expected 1 valid entry, got %d", len(got))
	}
}

func TestFilePersister_ConcurrentSave(t *testing.T) {
	dir := t.TempDir()
	p, err := NewFilePersister(dir)
	if err != nil {
		t.Fatalf("NewFilePersister: %v", err)
	}

	var wg sync.WaitGroup
	errCh := make(chan error, 20)

	// Concurrent saves of raft state
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			state := RaftStateV1{Term: uint64(i), VotedFor: "node1", CommitIndex: uint64(i * 10)}
			if err := p.SaveRaftState(state); err != nil {
				errCh <- err
			}
		}(i)
	}

	// Concurrent saves of log entries
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			entry := &LogEntry{Index: uint64(i + 1), Term: 1, Command: []byte("cmd")}
			if err := p.SaveLogEntry(entry); err != nil {
				errCh <- err
			}
		}(i)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("concurrent save error: %v", err)
	}

	// Verify final state is readable
	state, err := p.LoadRaftState()
	if err != nil {
		t.Fatalf("LoadRaftState after concurrent saves: %v", err)
	}
	if state.Term > 9 {
		t.Errorf("Term = %d, expected <= 9", state.Term)
	}

	entries, err := p.LoadLogEntries()
	if err != nil {
		t.Fatalf("LoadLogEntries after concurrent saves: %v", err)
	}
	if len(entries) == 0 {
		t.Error("expected at least 1 log entry")
	}
}

func TestFilePersister_NilLogEntry(t *testing.T) {
	dir := t.TempDir()
	p, err := NewFilePersister(dir)
	if err != nil {
		t.Fatalf("NewFilePersister: %v", err)
	}

	// Saving nil entry should be a no-op
	if err := p.SaveLogEntry(nil); err != nil {
		t.Errorf("SaveLogEntry(nil) returned error: %v", err)
	}
}

func TestFilePersister_EmptyDirPath(t *testing.T) {
	_, err := NewFilePersister("")
	if err == nil {
		t.Error("expected error for empty dir")
	}
}

func TestFilePersister_Dir(t *testing.T) {
	dir := t.TempDir()
	p, err := NewFilePersister(dir)
	if err != nil {
		t.Fatalf("NewFilePersister: %v", err)
	}
	if p.Dir() != dir {
		t.Errorf("Dir() = %q, want %q", p.Dir(), dir)
	}
}
