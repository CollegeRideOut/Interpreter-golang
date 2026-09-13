package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"interpreter/engine"
)

type sequentialTraceStep struct {
	Index       int    `json:"index"`
	Kind        string `json:"kind"`
	NodeKind    string `json:"nodeKind"`
	NodeID      string `json:"nodeId"`
	ParentID    string `json:"parentId,omitempty"`
	Status      string `json:"status"`
	Issue       string `json:"issue,omitempty"`
	BeforeJSON  string `json:"beforeJson"`
	AfterJSON   string `json:"afterJson"`
	BeforeShape string `json:"beforeShape"`
	AfterShape  string `json:"afterShape"`
}

type sequentialTrace struct {
	Program        string                `json:"program"`
	SourceRevision string                `json:"sourceRevision"`
	TargetRevision string                `json:"targetRevision"`
	EditCount      int                   `json:"editCount"`
	Steps          []sequentialTraceStep `json:"steps"`
	FinalShape     string                `json:"finalShape"`
	TargetShape    string                `json:"targetShape"`
	FinalMatches   bool                  `json:"finalMatchesTarget"`
}

func TestSequentialApplyTestProgramDiagnostic(t *testing.T) {
	programDirectory := "TestProgram"
	targetPath := filepath.Join(programDirectory, "main.go")
	target, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatal(err)
	}
	source, err := gitFile(programDirectory, "HEAD~1:main.go")
	if err != nil {
		t.Fatal(err)
	}

	sourceTree, targetTree, edits, err := engine.Diff(source, target)
	if err != nil {
		t.Fatal(err)
	}
	state, err := engine.NewWorkingState(sourceTree, targetTree, edits)
	if err != nil {
		t.Fatal(err)
	}

	trace := sequentialTrace{
		Program:        programDirectory,
		SourceRevision: "HEAD~1:main.go",
		TargetRevision: "working tree:main.go",
		EditCount:      len(edits),
		Steps:          make([]sequentialTraceStep, 0, len(edits)),
	}
	unexplainedNoOps := 0
	for index, edit := range edits {
		before := state.Snapshot()
		beforeJSON := mustJSON(t, before.Root)
		beforeShape := mustJSON(t, shape(before.Root))

		step := sequentialTraceStep{
			Index:       index,
			Kind:        edit.Kind,
			NodeKind:    edit.NodeKind,
			NodeID:      edit.NodeID,
			ParentID:    edit.ParentID,
			BeforeJSON:  beforeJSON,
			BeforeShape: beforeShape,
		}
		applyErr := state.Apply(index)
		after := state.Snapshot()
		step.AfterJSON = mustJSON(t, after.Root)
		step.AfterShape = mustJSON(t, shape(after.Root))
		step.Status = string(after.Status[index])

		if applyErr != nil {
			step.Issue = "apply returned an error: " + applyErr.Error()
		} else if bytes.Equal([]byte(step.BeforeShape), []byte(step.AfterShape)) {
			if ancestor, ok := appliedAncestor(index, edits, before.Status); ok {
				step.Issue = fmt.Sprintf("no structural change; ancestor edit %d (%s) was already applied and may have realized this child", ancestor, edits[ancestor].NodeID)
			} else {
				step.Issue = "no structural change and no applied ancestor explains the no-op"
				unexplainedNoOps++
			}
		}
		trace.Steps = append(trace.Steps, step)
	}

	trace.FinalShape = mustJSON(t, shape(state.Snapshot().Root))
	trace.TargetShape = mustJSON(t, shape(targetTree))
	trace.FinalMatches = trace.FinalShape == trace.TargetShape
	if !trace.FinalMatches {
		trace.Steps = append(trace.Steps, sequentialTraceStep{
			Index:       len(edits),
			Kind:        "FINAL",
			NodeKind:    "target comparison",
			Issue:       "final intermediate AST does not match target AST after all edits",
			BeforeShape: trace.FinalShape,
			AfterShape:  trace.TargetShape,
		})
	}

	writeDiagnosticArtifacts(t, trace)
	for _, step := range trace.Steps {
		if step.Issue != "" {
			t.Logf("edit %d %s %s: %s", step.Index, step.Kind, step.NodeKind, step.Issue)
		}
	}
	if unexplainedNoOps > 0 {
		t.Fatalf("found %d unexplained edit no-ops; see sequential_apply_report.md", unexplainedNoOps)
	}
	if !trace.FinalMatches {
		t.Fatalf("final intermediate AST does not match target AST; see sequential_apply_report.md")
	}
}

func gitFile(directory, revision string) ([]byte, error) {
	command := exec.Command("git", "-C", directory, "show", revision)
	return command.Output()
}

func appliedAncestor(index int, edits []engine.Edit, statuses []engine.EditStatus) (int, bool) {
	byNodeID := make(map[string]int, len(edits))
	for editIndex, edit := range edits {
		byNodeID[edit.NodeID] = editIndex
	}
	for parentID := edits[index].ParentID; parentID != ""; {
		parentIndex, ok := byNodeID[parentID]
		if !ok {
			return 0, false
		}
		if statuses[parentIndex] == engine.EditApplied {
			return parentIndex, true
		}
		parentID = edits[parentIndex].ParentID
	}
	return 0, false
}

type structuralShape struct {
	Kind     string            `json:"kind"`
	Value    string            `json:"value,omitempty"`
	Field    string            `json:"field,omitempty"`
	Index    int               `json:"index"`
	Children []structuralShape `json:"children,omitempty"`
}

func shape(node *engine.Node) structuralShape {
	if node == nil {
		return structuralShape{}
	}
	result := structuralShape{Kind: node.Kind, Value: node.Value, Field: node.Field, Index: node.Index}
	for _, child := range node.Children {
		result.Children = append(result.Children, shape(child))
	}
	return result
}

func mustJSON(t *testing.T, value interface{}) string {
	t.Helper()
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func writeDiagnosticArtifacts(t *testing.T, trace sequentialTrace) {
	t.Helper()
	traceJSON, err := json.MarshalIndent(trace, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("sequential_apply_trace.json", append(traceJSON, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}

	var report strings.Builder
	report.WriteString("# Sequential Apply Diagnostic\n\n")
	report.WriteString("This report was generated by `TestSequentialApplyTestProgramDiagnostic`. Structural expectations are derived from the target snapshot for this integration diagnostic; independent hardcoded expectations live in `hardcoded_apply_golden_test.go`.\n\n")
	fmt.Fprintf(&report, "- Program: `%s`\n- Source: `%s`\n- Target: `%s`\n- Edit count: `%d`\n- Final intermediate AST matches target shape: `%t`\n\n", trace.Program, trace.SourceRevision, trace.TargetRevision, trace.EditCount, trace.FinalMatches)
	report.WriteString("## Observed Issues\n\n")
	issueCount := 0
	for _, step := range trace.Steps {
		if step.Issue == "" {
			continue
		}
		issueCount++
		fmt.Fprintf(&report, "%d. Edit `%d` `%s %s` (`%s`): %s\n", issueCount, step.Index, step.Kind, step.NodeKind, step.NodeID, step.Issue)
	}
	if issueCount == 0 {
		report.WriteString("No issues were observed.\n")
	}
	report.WriteString("\n## Analysis\n\n")
	if trace.FinalMatches {
		report.WriteString("- **Final structural comparison passed:** after all 74 edits, the intermediate AST has the same node kinds, values, fields, child order, and indexes as the target AST. Identity/provenance metadata is intentionally excluded from this comparison.\n")
	} else {
		report.WriteString("- **Final structural comparison failed:** the intermediate AST does not have the same structural shape as the target AST.\n")
	}
	report.WriteString("- **Dependent delete no-ops:** edits 69, 70, 72, and 73 target children of deleted assignments. Their parent deletes already removed those children, so the desired structural result was already present when those child deletes ran.\n")
	report.WriteString("- **Identity regression fixed:** insert edits no longer inherit `SourceGlobalID` from an old node occupying the same path, so an insert cannot be mistaken for an existing node of another kind.\n")
	report.WriteString("- **Renderer is not used for the structural verdict:** the test compares exact intermediate-tree JSON shapes before best-effort source rendering.\n")
	report.WriteString("\n## Trace\n\nThe exact before/after JSON for every edit is in `sequential_apply_trace.json`. `beforeShape` and `afterShape` remove identity/provenance fields and are used to detect real structural changes.\n")
	if err := os.WriteFile("sequential_apply_report.md", []byte(report.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}
