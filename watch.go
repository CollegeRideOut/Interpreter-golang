package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
)

type watchOptions struct {
	directory string
	command   string
	debounce  time.Duration
	once      bool
}

type checkpointFile struct {
	Path string `json:"path"`
	Hash string `json:"hash"`
	Size int64  `json:"size"`
}

type checkpoint struct {
	ID         string           `json:"id"`
	CapturedAt time.Time        `json:"capturedAt"`
	Directory  string           `json:"directory"`
	GitCommit  string           `json:"gitCommit,omitempty"`
	Files      []checkpointFile `json:"files"`
}

type executionRecord struct {
	ID               string    `json:"id"`
	StartedAt        time.Time `json:"startedAt"`
	EndedAt          time.Time `json:"endedAt"`
	Directory        string    `json:"directory"`
	Command          string    `json:"command"`
	BeforeCheckpoint string    `json:"beforeCheckpoint"`
	AfterCheckpoint  string    `json:"afterCheckpoint,omitempty"`
	ExitCode         int       `json:"exitCode"`
	RestartReason    string    `json:"restartReason"`
	Stdout           string    `json:"stdout"`
	Stderr           string    `json:"stderr"`
}

type watchedProcess struct {
	command *exec.Cmd
	stdout  *bytes.Buffer
	stderr  *bytes.Buffer
	done    chan error
}

func parseWatchArguments(arguments []string) (watchOptions, error) {
	options := watchOptions{directory: ".", debounce: 350 * time.Millisecond}
	for index := 0; index < len(arguments); index++ {
		switch arguments[index] {
		case "--directory":
			index++
			if index >= len(arguments) {
				return options, fmt.Errorf("--directory requires a value")
			}
			options.directory = arguments[index]
		case "--command":
			index++
			if index >= len(arguments) {
				return options, fmt.Errorf("--command requires a value")
			}
			options.command = arguments[index]
		case "--debounce-ms":
			index++
			if index >= len(arguments) {
				return options, fmt.Errorf("--debounce-ms requires a value")
			}
			var milliseconds int
			if _, err := fmt.Sscanf(arguments[index], "%d", &milliseconds); err != nil || milliseconds < 0 {
				return options, fmt.Errorf("invalid debounce duration %q", arguments[index])
			}
			options.debounce = time.Duration(milliseconds) * time.Millisecond
		case "--once":
			options.once = true
		default:
			return options, fmt.Errorf("unknown watch option %q", arguments[index])
		}
	}
	if strings.TrimSpace(options.command) == "" {
		return options, errors.New("watch requires --command, for example --command 'pnpm dev:web'")
	}
	return options, nil
}

func runWatch(arguments []string) error {
	options, err := parseWatchArguments(arguments)
	if err != nil {
		return err
	}
	directory, err := filepath.Abs(options.directory)
	if err != nil {
		return fmt.Errorf("resolve watch directory: %w", err)
	}
	if info, err := os.Stat(directory); err != nil || !info.IsDir() {
		return fmt.Errorf("watch directory %q does not exist", directory)
	}

	before, err := captureCheckpoint(directory)
	if err != nil {
		return err
	}
	if err := saveCheckpoint(directory, before); err != nil {
		return err
	}

	process, err := startWatchedProcess(directory, options.command)
	if err != nil {
		return err
	}
	started := time.Now().UTC()
	if options.once {
		runErr := <-process.done
		after, captureErr := captureCheckpoint(directory)
		if captureErr == nil {
			_ = saveCheckpoint(directory, after)
		}
		recordErr := saveExecution(directory, executionRecordFromProcess(process, directory, options.command, before.ID, checkpointID(after), started, "completed"))
		if runErr != nil {
			return runErr
		}
		return recordErr
	}

	return watchProcess(directory, options, process, before, started)
}

func watchProcess(directory string, options watchOptions, process *watchedProcess, before checkpoint, started time.Time) error {
	previousState, err := workspaceSignature(directory)
	if err != nil {
		return err
	}
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-signals:
			stopWatchedProcess(process)
			return saveExecution(directory, executionRecordFromProcess(process, directory, options.command, before.ID, "", started, "interrupted"))
		case runErr := <-process.done:
			saveErr := saveExecution(directory, executionRecordFromProcess(process, directory, options.command, before.ID, "", started, "exited"))
			if saveErr != nil {
				return saveErr
			}
			return runErr
		case <-ticker.C:
			currentState, signatureErr := workspaceSignature(directory)
			if signatureErr != nil {
				return signatureErr
			}
			if currentState == previousState {
				continue
			}
			time.Sleep(options.debounce)
			stableState, stableErr := workspaceSignature(directory)
			if stableErr != nil {
				return stableErr
			}
			previousState = stableState
			after, captureErr := captureCheckpoint(directory)
			if captureErr != nil {
				return captureErr
			}
			if saveErr := saveCheckpoint(directory, after); saveErr != nil {
				return saveErr
			}
			stopWatchedProcess(process)
			if saveErr := saveExecution(directory, executionRecordFromProcess(process, directory, options.command, before.ID, after.ID, started, "source-changed")); saveErr != nil {
				return saveErr
			}
			before = after
			started = time.Now().UTC()
			process, err = startWatchedProcess(directory, options.command)
			if err != nil {
				return err
			}
		}
	}
}

func startWatchedProcess(directory, command string) (*watchedProcess, error) {
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = directory
	cmd.Stdout = io.MultiWriter(os.Stdout, stdout)
	cmd.Stderr = io.MultiWriter(os.Stderr, stderr)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start watched command: %w", err)
	}
	process := &watchedProcess{command: cmd, stdout: stdout, stderr: stderr, done: make(chan error, 1)}
	go func() { process.done <- cmd.Wait() }()
	return process, nil
}

func stopWatchedProcess(process *watchedProcess) {
	if process == nil || process.command == nil || process.command.Process == nil {
		return
	}
	_ = syscall.Kill(-process.command.Process.Pid, syscall.SIGTERM)
	select {
	case <-process.done:
	case <-time.After(2 * time.Second):
		_ = syscall.Kill(-process.command.Process.Pid, syscall.SIGKILL)
		<-process.done
	}
}

func executionRecordFromProcess(process *watchedProcess, directory, command, before, after string, started time.Time, reason string) executionRecord {
	return executionRecord{
		ID:               fmt.Sprintf("run-%d", started.UnixNano()),
		StartedAt:        started,
		EndedAt:          time.Now().UTC(),
		Directory:        directory,
		Command:          command,
		BeforeCheckpoint: before,
		AfterCheckpoint:  after,
		ExitCode:         processExitCode(process),
		RestartReason:    reason,
		Stdout:           process.stdout.String(),
		Stderr:           process.stderr.String(),
	}
}

func processExitCode(process *watchedProcess) int {
	if process == nil || process.command == nil || process.command.ProcessState == nil {
		return -1
	}
	return process.command.ProcessState.ExitCode()
}

func captureCheckpoint(directory string) (checkpoint, error) {
	files := make([]checkpointFile, 0)
	err := filepath.WalkDir(directory, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if ignoredWatchDirectory(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !watchFile(entry.Name(), path) {
			return nil
		}
		contents, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		info, statErr := entry.Info()
		if statErr != nil {
			return statErr
		}
		hash := sha256.Sum256(contents)
		relative, relativeErr := filepath.Rel(directory, path)
		if relativeErr != nil {
			return relativeErr
		}
		files = append(files, checkpointFile{Path: filepath.ToSlash(relative), Hash: hex.EncodeToString(hash[:]), Size: info.Size()})
		return nil
	})
	if err != nil {
		return checkpoint{}, fmt.Errorf("capture checkpoint: %w", err)
	}
	sort.Slice(files, func(left, right int) bool { return files[left].Path < files[right].Path })
	gitCommit := commandOutput(directory, "git", "rev-parse", "HEAD")
	identity, _ := json.Marshal(struct {
		GitCommit string           `json:"gitCommit"`
		Files     []checkpointFile `json:"files"`
	}{gitCommit, files})
	hash := sha256.Sum256(identity)
	return checkpoint{ID: "checkpoint-" + hex.EncodeToString(hash[:8]), CapturedAt: time.Now().UTC(), Directory: directory, GitCommit: gitCommit, Files: files}, nil
}

func workspaceSignature(directory string) (string, error) {
	checkpoint, err := captureCheckpoint(directory)
	if err != nil {
		return "", err
	}
	return checkpoint.ID, nil
}

func saveCheckpoint(directory string, value checkpoint) error {
	path := filepath.Join(directory, ".contuts", "checkpoints", value.ID+".json")
	return writeWatchJSON(path, value)
}

func saveExecution(directory string, value executionRecord) error {
	path := filepath.Join(directory, ".contuts", "runs", value.ID+".json")
	return writeWatchJSON(path, value)
}

func writeWatchJSON(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	contents, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(contents, '\n'), 0o644)
}

func commandOutput(directory, name string, arguments ...string) string {
	command := exec.Command(name, arguments...)
	command.Dir = directory
	output, err := command.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

func checkpointID(value checkpoint) string {
	return value.ID
}

func ignoredWatchDirectory(name string) bool {
	switch name {
	case ".git", ".contuts", "node_modules", "dist", "build", ".next", "coverage":
		return true
	default:
		return false
	}
}

func watchFile(name, path string) bool {
	extension := strings.ToLower(filepath.Ext(name))
	if extension != ".ts" && extension != ".tsx" && extension != ".js" && extension != ".jsx" && extension != ".html" && extension != ".css" && extension != ".json" {
		return false
	}
	return !strings.HasSuffix(path, ".lock")
}
