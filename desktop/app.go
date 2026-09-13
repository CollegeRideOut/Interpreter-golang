package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"interpreter/engine"
)

// App struct
type App struct {
	ctx         context.Context
	programPath string
	source      []byte
	target      []byte
	state       *engine.WorkingState
}

type ProgramSnapshot struct {
	Path              string              `json:"path"`
	Source            string              `json:"source"`
	Target            string              `json:"target"`
	Working           *engine.Node        `json:"working"`
	WorkingCode       string              `json:"workingCode"`
	Edits             []engine.Edit       `json:"edits"`
	Status            []engine.EditStatus `json:"status"`
	Valid             bool                `json:"valid"`
	Diagnostics       []string            `json:"diagnostics,omitempty"`
	RenderDiagnostics []string            `json:"renderDiagnostics,omitempty"`
	Candidates        []ProgramCandidate  `json:"candidates,omitempty"`
}

type ProgramCandidate struct {
	AncestorID string `json:"ancestorId"`
	engine.ReconciliationCandidate
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called at application startup
func (a *App) startup(ctx context.Context) {
	// Perform your setup here
	a.ctx = ctx
}

// OpenProgram loads main.go and its previous Git version into a working
// session. The frontend provides the directory, so no program is hardcoded.
func (a *App) OpenProgram(directory string) (ProgramSnapshot, error) {
	absolute, err := filepath.Abs(directory)
	if err != nil {
		return ProgramSnapshot{}, err
	}
	filePath := filepath.Join(absolute, "main.go")
	target, err := os.ReadFile(filePath)
	if err != nil {
		return ProgramSnapshot{}, fmt.Errorf("read %s: %w", filePath, err)
	}
	source, err := previousVersion(absolute)
	if err != nil {
		return ProgramSnapshot{}, err
	}
	state, err := engine.NewWorkingStateFromSource(source, target)
	if err != nil {
		return ProgramSnapshot{}, err
	}
	a.programPath = absolute
	a.source = source
	a.target = target
	a.state = state
	return a.snapshot(), nil
}

func previousVersion(directory string) ([]byte, error) {
	command := exec.Command("git", "-C", directory, "show", "HEAD~1:main.go")
	output, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("read previous Git version: %w", err)
	}
	return output, nil
}

// ApplyEdit applies one edit to the current working session.
func (a *App) ApplyEdit(index int) (ProgramSnapshot, error) {
	return a.ApplyEditWithOptions(index, true)
}

// ApplyEditWithOptions applies one edit with optional automatic reconciliation.
func (a *App) ApplyEditWithOptions(index int, reconcile bool) (ProgramSnapshot, error) {
	if a.state == nil {
		return ProgramSnapshot{}, fmt.Errorf("no program is open")
	}
	if err := a.state.ApplyWithOptions(index, engine.ApplyOptions{Reconcile: reconcile}); err != nil {
		return ProgramSnapshot{}, err
	}
	return a.snapshot(), nil
}

// ApplyEditSubtree applies an edit and all of its dependent child edits.
func (a *App) ApplyEditSubtree(index int) (ProgramSnapshot, error) {
	if a.state == nil {
		return ProgramSnapshot{}, fmt.Errorf("no program is open")
	}
	if err := a.state.ApplySubtree(index); err != nil {
		return ProgramSnapshot{}, err
	}
	return a.snapshot(), nil
}

// ApplyEditSiblings applies the selected edit and its same-parent peers.
func (a *App) ApplyEditSiblings(index int) (ProgramSnapshot, error) {
	if a.state == nil {
		return ProgramSnapshot{}, fmt.Errorf("no program is open")
	}
	if err := a.state.ApplySiblings(index); err != nil {
		return ProgramSnapshot{}, err
	}
	return a.snapshot(), nil
}

// RemoveEdit removes one edit and rebuilds the working session.
func (a *App) RemoveEdit(index int) (ProgramSnapshot, error) {
	if a.state == nil {
		return ProgramSnapshot{}, fmt.Errorf("no program is open")
	}
	if err := a.state.Remove(index); err != nil {
		return ProgramSnapshot{}, err
	}
	return a.snapshot(), nil
}

// RemoveEditSubtree removes an edit and all of its dependent child edits.
func (a *App) RemoveEditSubtree(index int) (ProgramSnapshot, error) {
	if a.state == nil {
		return ProgramSnapshot{}, fmt.Errorf("no program is open")
	}
	if err := a.state.RemoveSubtree(index); err != nil {
		return ProgramSnapshot{}, err
	}
	return a.snapshot(), nil
}

// RemoveEditSiblings removes the selected edit and its same-parent peers.
func (a *App) RemoveEditSiblings(index int) (ProgramSnapshot, error) {
	if a.state == nil {
		return ProgramSnapshot{}, fmt.Errorf("no program is open")
	}
	if err := a.state.RemoveSiblings(index); err != nil {
		return ProgramSnapshot{}, err
	}
	return a.snapshot(), nil
}

// GetReconciliationCandidates returns nodes whose current parent differs from
// the target parent below the requested ancestor.
func (a *App) GetReconciliationCandidates(ancestorID string) ([]engine.ReconciliationCandidate, error) {
	if a.state == nil {
		return nil, fmt.Errorf("no program is open")
	}
	return a.state.Candidates(ancestorID), nil
}

// Reconcile applies a selected reconciliation candidate.
func (a *App) Reconcile(candidate engine.ReconciliationCandidate) (ProgramSnapshot, error) {
	if a.state == nil {
		return ProgramSnapshot{}, fmt.Errorf("no program is open")
	}
	if err := a.state.Reconcile(candidate); err != nil {
		return ProgramSnapshot{}, err
	}
	return a.snapshot(), nil
}

// ReconcileGroup applies every currently available candidate below an ancestor.
func (a *App) ReconcileGroup(ancestorID string) (ProgramSnapshot, error) {
	if a.state == nil {
		return ProgramSnapshot{}, fmt.Errorf("no program is open")
	}
	selected := make([]string, 0)
	for _, candidate := range a.state.Candidates(ancestorID) {
		selected = append(selected, candidate.NodeGlobalID)
	}
	return a.reconcileSelected(selected)
}

// ReconcileSelected applies only the candidates identified by global ID.
func (a *App) ReconcileSelected(nodeGlobalIDs []string) (ProgramSnapshot, error) {
	if a.state == nil {
		return ProgramSnapshot{}, fmt.Errorf("no program is open")
	}
	return a.reconcileSelected(nodeGlobalIDs)
}

func (a *App) reconcileSelected(nodeGlobalIDs []string) (ProgramSnapshot, error) {
	selected := make(map[string]bool, len(nodeGlobalIDs))
	for _, globalID := range nodeGlobalIDs {
		selected[globalID] = true
	}
	seen := make(map[string]bool)
	for _, edit := range a.state.Snapshot().Edits {
		for _, candidate := range a.state.Candidates(edit.NodeID) {
			if !selected[candidate.NodeGlobalID] || seen[candidate.NodeGlobalID] || !candidate.CanReconcile {
				continue
			}
			seen[candidate.NodeGlobalID] = true
			if err := a.state.Reconcile(candidate); err != nil {
				return ProgramSnapshot{}, err
			}
		}
	}
	return a.snapshot(), nil
}

func (a *App) snapshot() ProgramSnapshot {
	snapshot := a.state.Snapshot()
	validation := a.state.ValidateGo()
	candidates := make([]ProgramCandidate, 0)
	seenCandidates := make(map[string]bool)
	for _, edit := range snapshot.Edits {
		for _, candidate := range a.state.Candidates(edit.NodeID) {
			key := candidate.NodeGlobalID + "|" + candidate.NodeID
			if seenCandidates[key] {
				continue
			}
			seenCandidates[key] = true
			candidates = append(candidates, ProgramCandidate{AncestorID: edit.NodeID, ReconciliationCandidate: candidate})
		}
	}
	return ProgramSnapshot{
		Path:              a.programPath,
		Source:            string(a.source),
		Target:            string(a.target),
		Working:           snapshot.Root,
		WorkingCode:       snapshot.RenderedCode,
		Edits:             snapshot.Edits,
		Status:            snapshot.Status,
		Valid:             validation.Valid,
		Diagnostics:       validation.Diagnostics,
		RenderDiagnostics: snapshot.RenderDiagnostics,
		Candidates:        candidates,
	}
}

// domReady is called after front-end resources have been loaded
func (a App) domReady(ctx context.Context) {
	// Add your action here
}

// beforeClose is called when the application is about to quit,
// either by clicking the window close button or calling runtime.Quit.
// Returning true will cause the application to continue, false will continue shutdown as normal.
func (a *App) beforeClose(ctx context.Context) (prevent bool) {
	return false
}

// shutdown is called at application termination
func (a *App) shutdown(ctx context.Context) {
	// Perform your teardown here
}

// Greet returns a greeting for the given name
func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's show time!", name)
}
