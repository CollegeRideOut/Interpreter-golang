package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseWatchArguments(t *testing.T) {
	options, err := parseWatchArguments([]string{"--directory", "web", "--command", "pnpm dev:web", "--debounce-ms", "100", "--once"})
	if err != nil {
		t.Fatal(err)
	}
	if options.directory != "web" || options.command != "pnpm dev:web" || options.debounce.Milliseconds() != 100 || !options.once {
		t.Fatalf("unexpected options: %+v", options)
	}
}

func TestCaptureCheckpointTracksTypeScriptFilesAndIgnoresBuildOutput(t *testing.T) {
	directory := t.TempDir()
	if err := os.MkdirAll(filepath.Join(directory, "node_modules"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "App.tsx"), []byte("export const App = 1;"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "README.md"), []byte("ignored"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "node_modules", "ignored.ts"), []byte("ignored"), 0o644); err != nil {
		t.Fatal(err)
	}

	value, err := captureCheckpoint(directory)
	if err != nil {
		t.Fatal(err)
	}
	if len(value.Files) != 1 || value.Files[0].Path != "App.tsx" {
		t.Fatalf("unexpected checkpoint files: %+v", value.Files)
	}
}
