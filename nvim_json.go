package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"interpreter/engine"
)

type nvimEdit struct {
	Index       int      `json:"index"`
	Kind        string   `json:"kind"`
	NodeKind    string   `json:"nodeKind"`
	NodeID      string   `json:"nodeId"`
	ParentID    string   `json:"parentId,omitempty"`
	AncestorIDs []string `json:"ancestorIds,omitempty"`
	Field       string   `json:"field,omitempty"`
	Value       string   `json:"value,omitempty"`
	StartLine   int      `json:"startLine,omitempty"`
	EndLine     int      `json:"endLine,omitempty"`
	Hidden      bool     `json:"hidden,omitempty"`
}

type nvimFile struct {
	Path  string     `json:"path"`
	Edits []nvimEdit `json:"edits"`
}

type nvimResponse struct {
	Schema    string     `json:"schema"`
	Directory string     `json:"directory"`
	Base      string     `json:"base"`
	Compare   string     `json:"compare"`
	Files     []nvimFile `json:"files"`
}

func runNvimJSON(arguments []string) error {
	if len(arguments) != 3 {
		return fmt.Errorf("usage: interpreter --nvim-json <directory> <base> <compare>")
	}
	directory, err := filepath.Abs(arguments[0])
	if err != nil {
		return err
	}
	paths, err := nvimChangedPaths(directory, arguments[1], arguments[2])
	if err != nil {
		return err
	}
	response := nvimResponse{Schema: "contuts.nvim.v1", Directory: directory, Base: arguments[1], Compare: arguments[2], Files: make([]nvimFile, 0, len(paths))}
	for _, path := range paths {
		source, err := nvimRevisionBytes(directory, arguments[1], path)
		if err != nil {
			return err
		}
		target, err := nvimRevisionBytes(directory, arguments[2], path)
		if err != nil {
			return err
		}
		state, err := engine.NewWorkingStateFromLanguage(nvimLanguage(path), source, target)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		snapshot := state.Snapshot()
		file := nvimFile{Path: path, Edits: make([]nvimEdit, 0, len(snapshot.Edits))}
		for _, edit := range snapshot.Edits {
			startLine, endLine := 0, 0
			if edit.Node != nil {
				startLine, endLine = edit.Node.StartLine, edit.Node.EndLine
			}
			file.Edits = append(file.Edits, nvimEdit{
				Index: edit.Index, Kind: edit.Kind, NodeKind: edit.NodeKind, NodeID: edit.NodeID,
				ParentID: edit.ParentID, AncestorIDs: edit.AncestorIDs, Field: edit.Field,
				Value: edit.Value, StartLine: startLine, EndLine: endLine, Hidden: edit.Hidden,
			})
		}
		response.Files = append(response.Files, file)
	}
	return json.NewEncoder(os.Stdout).Encode(response)
}

func nvimChangedPaths(directory, base, compare string) ([]string, error) {
	args := []string{"-C", directory, "diff", "--name-only"}
	if base == "working-tree" {
		args = append(args, compare)
	} else if compare == "working-tree" {
		args = append(args, base)
	} else {
		args = append(args, base, compare)
	}
	output, err := exec.Command("git", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("list changed files: %w", err)
	}
	paths := make([]string, 0)
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if line != "" && nvimLanguage(line) != "" {
			paths = append(paths, filepath.ToSlash(line))
		}
	}
	return paths, nil
}

func nvimRevisionBytes(directory, revision, path string) ([]byte, error) {
	if revision == "working-tree" {
		contents, err := os.ReadFile(filepath.Join(directory, filepath.FromSlash(path)))
		if os.IsNotExist(err) {
			return []byte{}, nil
		}
		return contents, err
	}
	contents, err := exec.Command("git", "-C", directory, "show", revision+":"+path).Output()
	if err != nil {
		if _, statErr := os.Stat(filepath.Join(directory, filepath.FromSlash(path))); os.IsNotExist(statErr) {
			return []byte{}, nil
		}
		return []byte{}, fmt.Errorf("read %s at %s: %w", path, revision, err)
	}
	return contents, nil
}

func nvimLanguage(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".ts":
		return "typescript"
	case ".tsx":
		return "tsx"
	case ".html", ".htm":
		return "html"
	case ".go":
		return "go"
	default:
		return ""
	}
}
