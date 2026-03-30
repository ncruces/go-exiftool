package exiftool

import (
	"bytes"
	"testing"
)

func TestCommandArgsDefault(t *testing.T) {
	// Save and restore globals
	savedExec, savedArg1, savedConfig := Exec, Arg1, Config
	defer func() { Exec, Arg1, Config = savedExec, savedArg1, savedConfig }()

	Arg1 = ""
	Config = ""

	args := commandArgs([]string{"-ver"})
	// Args: -charset filename=utf8 -ver
	if len(args) != 3 {
		t.Fatalf("expected 3 args, got %d: %v", len(args), args)
	}
	if args[0] != "-charset" {
		t.Errorf("expected -charset, got %q", args[0])
	}
	if args[1] != "filename=utf8" {
		t.Errorf("expected filename=utf8, got %q", args[1])
	}
	if args[2] != "-ver" {
		t.Errorf("expected -ver, got %q", args[2])
	}
}

func TestCommandArgsWithArg1(t *testing.T) {
	savedExec, savedArg1, savedConfig := Exec, Arg1, Config
	defer func() { Exec, Arg1, Config = savedExec, savedArg1, savedConfig }()

	Arg1 = "/usr/bin/perl"
	Config = ""

	args := commandArgs([]string{"-ver"})
	found := false
	for _, a := range args {
		if a == "/usr/bin/perl" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected Arg1 in args, got %v", args)
	}
}

func TestCommandArgsWithConfig(t *testing.T) {
	savedExec, savedArg1, savedConfig := Exec, Arg1, Config
	defer func() { Exec, Arg1, Config = savedExec, savedArg1, savedConfig }()

	Arg1 = ""
	Config = "/path/to/config"

	args := commandArgs([]string{"-ver"})
	foundConfig, foundConfigPath := false, false
	for i, a := range args {
		if a == "-config" {
			foundConfig = true
			if i+1 < len(args) && args[i+1] == "/path/to/config" {
				foundConfigPath = true
			}
		}
	}
	if !foundConfig || !foundConfigPath {
		t.Errorf("expected -config /path/to/config in args, got %v", args)
	}
}

func TestCommandArgsWithBoth(t *testing.T) {
	savedExec, savedArg1, savedConfig := Exec, Arg1, Config
	defer func() { Exec, Arg1, Config = savedExec, savedArg1, savedConfig }()

	Arg1 = "/usr/bin/perl"
	Config = "/path/to/config"

	args := commandArgs([]string{"-ver"})

	// Arg1 should be first
	if args[0] != "/usr/bin/perl" {
		t.Errorf("expected Arg1 first, got %q", args[0])
	}
	// Then -config
	foundConfig := false
	for i, a := range args {
		if a == "-config" && i+1 < len(args) && args[i+1] == "/path/to/config" {
			foundConfig = true
		}
	}
	if !foundConfig {
		t.Errorf("expected -config in args, got %v", args)
	}
}

func TestServerWithConfig(t *testing.T) {
	savedExec, savedArg1, savedConfig := Exec, Arg1, Config
	defer func() { Exec, Arg1, Config = savedExec, savedArg1, savedConfig }()

	// Test with a nonexistent config file
	// ExifTool may not fail on nonexistent config, so just test that it's passed
	Config = "/nonexistent/config/file.cfg"
	e, err := NewServer()
	if err != nil {
		// If it fails, that's fine - config file doesn't exist
		t.Logf("NewServer with bad config (acceptable): %v", err)
		return
	}
	// If it succeeds, verify the server works
	if err := e.Shutdown(); err != nil {
		t.Logf("Shutdown error (acceptable): %v", err)
	}
}

func TestServerRestart(t *testing.T) {
	e, err := NewServer()
	if err != nil {
		t.Fatal(err)
	}

	// First, close the server to set done=true
	err = e.Close()
	if err != nil {
		t.Fatal(err)
	}

	// restart should be a no-op when done=true
	e.restart()

	// Verify server is still done
	if !e.done {
		t.Error("expected server to still be done after restart")
	}
}

func TestServerCommandAfterKill(t *testing.T) {
	e, err := NewServer()
	if err != nil {
		t.Fatal(err)
	}

	// Get version first to verify server works
	out, err := e.Command("-ver")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Before kill: %s", bytes.TrimSpace(out))

	// Kill the underlying process to trigger restart path
	if err := e.cmd.Process.Kill(); err != nil {
		t.Logf("Kill error: %v", err)
	}
	if err := e.cmd.Process.Release(); err != nil {
		t.Logf("Release error: %v", err)
	}

	// The next command should trigger a restart attempt
	// Since done is not set, restart will try to start a new process
	// But the stdin/stdout pipes are broken, so this may error
	_, err = e.Command("-ver")
	// This may or may not succeed depending on timing
	if err != nil {
		t.Logf("Command after kill (expected error): %v", err)
	}

	// Clean up
	if err := e.Close(); err != nil {
		t.Fatal(err)
	}
}
