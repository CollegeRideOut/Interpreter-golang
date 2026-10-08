package main

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/creack/pty"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"interpreter/engine"
	"interpreter/explorer"
	headless "interpreter/workspace"
)

// App struct
type App struct {
	ctx              context.Context
	workspace        *headless.Workspace
	programPath      string
	source           []byte
	target           []byte
	state            *engine.WorkingState
	revisionMu       sync.Mutex
	revisionRoot     string
	comparisonStates map[string]*engine.WorkingState
	proposalSessions map[string]*proposalSession
	terminalMu       sync.Mutex
	terminal         *os.File
	terminalCommand  *exec.Cmd
	terminalDone     chan struct{}
}

type ProgramSnapshot struct {
	Path              string                   `json:"path"`
	Source            string                   `json:"source"`
	Target            string                   `json:"target"`
	Working           *engine.Node             `json:"working"`
	WorkingCode       string                   `json:"workingCode"`
	Edits             []engine.Edit            `json:"edits"`
	Status            []engine.EditStatus      `json:"status"`
	Valid             bool                     `json:"valid"`
	Diagnostics       []string                 `json:"diagnostics,omitempty"`
	RenderDiagnostics []string                 `json:"renderDiagnostics,omitempty"`
	Candidates        []ProgramCandidate       `json:"candidates,omitempty"`
	EditViews         []engine.EditView        `json:"editViews"`
	Exploration       bool                     `json:"exploration"`
	Packages          []explorer.Package       `json:"packages,omitempty"`
	PackageName       string                   `json:"packageName,omitempty"`
	PackageDirectory  string                   `json:"packageDirectory"`
	FileName          string                   `json:"fileName,omitempty"`
	FilePath          string                   `json:"filePath,omitempty"`
	Declarations      []explorer.Declaration   `json:"declarations,omitempty"`
	FileSource        string                   `json:"fileSource,omitempty"`
	DeclarationName   string                   `json:"declarationName,omitempty"`
	DeclarationSource string                   `json:"declarationSource,omitempty"`
	LocalImports      []string                 `json:"localImports,omitempty"`
	Imports           []explorer.ImportSummary `json:"imports,omitempty"`
	ImportedFileName  string                   `json:"importedFileName,omitempty"`
	ImportedSource    string                   `json:"importedSource,omitempty"`
	ImportedName      string                   `json:"importedName,omitempty"`
	Files             []explorer.File          `json:"files,omitempty"`
}

type ProgramCandidate struct {
	AncestorID string `json:"ancestorId"`
	engine.ReconciliationCandidate
}

type RevisionOption struct {
	Kind      string `json:"kind"`
	Ref       string `json:"ref"`
	Hash      string `json:"hash"`
	ShortHash string `json:"shortHash"`
	Date      string `json:"date"`
	Author    string `json:"author"`
	Subject   string `json:"subject"`
}

type RevisionContext struct {
	Branch        string           `json:"branch"`
	CurrentCommit string           `json:"currentCommit"`
	Options       []RevisionOption `json:"options"`
}

type ComparisonFile struct {
	PackageDirectory string        `json:"packageDirectory"`
	PackageName      string        `json:"packageName"`
	File             explorer.File `json:"file"`
}

type EditSummary struct {
	Index          int                   `json:"index"`
	Kind           string                `json:"kind"`
	NodeID         string                `json:"nodeId"`
	NodeGlobalID   string                `json:"nodeGlobalId,omitempty"`
	SourceGlobalID string                `json:"sourceGlobalId,omitempty"`
	ParentGlobalID string                `json:"parentGlobalId,omitempty"`
	NodeKind       string                `json:"nodeKind"`
	ParentID       string                `json:"parentId,omitempty"`
	ParentKind     string                `json:"parentKind,omitempty"`
	AncestorIDs    []string              `json:"ancestorIds,omitempty"`
	Ancestors      []engine.EditAncestor `json:"ancestors,omitempty"`
	Field          string                `json:"field,omitempty"`
	Position       int                   `json:"position"`
	Value          string                `json:"value,omitempty"`
	StartLine      int                   `json:"startLine,omitempty"`
	EndLine        int                   `json:"endLine,omitempty"`
	Status         engine.EditStatus     `json:"status"`
}

type FileEditState struct {
	Edits                []EditSummary `json:"edits"`
	LiftedEdits          []EditSummary `json:"liftedEdits,omitempty"`
	WorkingCode          string        `json:"workingCode"`
	TargetCode           string        `json:"targetCode,omitempty"`
	TargetDiagnostics    []string      `json:"targetDiagnostics,omitempty"`
	RenderDiagnostics    []string      `json:"renderDiagnostics,omitempty"`
	Diagnostics          []string      `json:"diagnostics,omitempty"`
	Valid                bool          `json:"valid"`
	BranchID             string        `json:"branchId,omitempty"`
	WorkingAuthoritative bool          `json:"workingAuthoritative,omitempty"`
}

type ProposalBranchSummary struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Kind           string `json:"kind"`
	ParentID       string `json:"parentId,omitempty"`
	BaseRevision   string `json:"baseRevision"`
	SourceRevision string `json:"sourceRevision,omitempty"`
	Active         bool   `json:"active"`
}

type ProposalWorkspace struct {
	ActiveBranchID      string                  `json:"activeBranchId"`
	SelectedProposalIDs []string                `json:"selectedProposalIds,omitempty"`
	Branches            []ProposalBranchSummary `json:"branches"`
}

type proposalBranch struct {
	ProposalBranchSummary
	states map[string]*engine.WorkingState
	stable bool
}

type proposalSession struct {
	activeBranchID  string
	selected        map[string]bool
	branches        map[string]*proposalBranch
	nextBranch      int
	rebaseProposals bool
	humanStates     map[string]*engine.WorkingState
}

func liftOptions(hiddenKinds []string) engine.LiftOptions {
	hidden := make(map[string]bool, len(hiddenKinds))
	for _, kind := range hiddenKinds {
		hidden[kind] = true
	}
	return engine.LiftOptions{HiddenKinds: hidden}
}

func defaultHiddenKinds() []string {
	return []string{
		"*ast.BlockStmt", "*ast.ExprStmt", "*ast.DeclStmt", "*ast.ImportSpec", "*ast.FieldList", "*ast.Field",
		"typescript:program", "typescript:formal_parameters", "typescript:statement_block", "typescript:interface_body",
		"typescript:class_body", "typescript:named_imports", "typescript:import_clause", "typescript:namespace_import",
		"typescript:type_arguments", "typescript:arguments", "typescript:object", "typescript:array", "typescript:object_type",
		"typescript:type_annotation",
		"typescript:(", "typescript:)", "typescript:{", "typescript:}", "typescript:[", "typescript:]", "typescript:,", "typescript:;",
		"typescript::", "typescript:.", "typescript:?.", "typescript:=>", "typescript:...", "typescript:from",
		"typescript:import", "typescript:export", "typescript:const", "typescript:let", "typescript:var", "typescript:async",
		"typescript:await", "typescript:new", "typescript:if", "typescript:else", "typescript:throw", "typescript:return",
	}
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{workspace: headless.New(), comparisonStates: make(map[string]*engine.WorkingState), proposalSessions: make(map[string]*proposalSession)}
}

// startup is called at application startup
func (a *App) startup(ctx context.Context) {
	// Perform your setup here
	a.ctx = ctx
}

// OpenProgram opens the current working tree in read-only package exploration
// mode. It does not require Git or create an edit session.
func (a *App) OpenProgram(directory string) (ProgramSnapshot, error) {
	a.revisionMu.Lock()
	defer a.revisionMu.Unlock()
	absolute, err := filepath.Abs(directory)
	if err != nil {
		return ProgramSnapshot{}, err
	}
	a.clearRevisionRoot()
	if _, err := a.headlessWorkspace().OpenProgram(absolute); err != nil {
		return ProgramSnapshot{}, err
	}
	packages, err := explorer.DiscoverPackages(absolute)
	if err != nil {
		return ProgramSnapshot{}, err
	}
	a.programPath = absolute
	a.source = nil
	a.target = nil
	a.state = nil
	a.comparisonStates = make(map[string]*engine.WorkingState)
	a.proposalSessions = make(map[string]*proposalSession)
	imports := make(map[string]bool)
	for _, pkg := range packages {
		for _, imported := range pkg.LocalImports {
			imports[imported] = true
		}
	}
	localImports := make([]string, 0, len(imports))
	for imported := range imports {
		localImports = append(localImports, imported)
	}
	sort.Strings(localImports)
	return ProgramSnapshot{Path: absolute, Exploration: true, Packages: packages, LocalImports: localImports}, nil
}

// ChooseDirectory opens the native directory picker and returns the selected
// folder. An empty path means the picker was cancelled.
func (a *App) ChooseDirectory() (string, error) {
	if a.ctx == nil {
		return "", fmt.Errorf("application is not ready")
	}
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		DefaultDirectory: a.programPath,
		Title:            "Choose a program folder",
	})
}

// StartOpenCode starts the real OpenCode terminal in the selected repository.
func (a *App) StartOpenCode(directory string) error {
	if a.ctx == nil {
		return fmt.Errorf("application is not ready")
	}
	absolute, err := filepath.Abs(directory)
	if err != nil {
		return err
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return fmt.Errorf("open folder: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("open folder: %s is not a directory", absolute)
	}

	a.terminalMu.Lock()
	defer a.terminalMu.Unlock()
	if a.terminal != nil {
		return nil
	}
	command := exec.Command("opencode")
	command.Dir = absolute
	command.Env = append(os.Environ(), "TERM=xterm-256color", "COLORTERM=true")
	terminal, err := pty.Start(command)
	if err != nil {
		return fmt.Errorf("start opencode: %w", err)
	}
	a.terminal = terminal
	a.terminalCommand = command
	a.terminalDone = make(chan struct{})
	runtime.EventsEmit(a.ctx, "opencode:status", map[string]any{"running": true, "directory": absolute})

	go a.readOpenCodeOutput(terminal)
	go a.waitForOpenCode(command, terminal, a.terminalDone)
	return nil
}

func (a *App) readOpenCodeOutput(terminal *os.File) {
	buffer := make([]byte, 32*1024)
	for {
		count, err := terminal.Read(buffer)
		if count > 0 && a.ctx != nil {
			runtime.EventsEmit(a.ctx, "opencode:output", string(buffer[:count]))
		}
		if err != nil {
			return
		}
	}
}

func (a *App) waitForOpenCode(command *exec.Cmd, terminal *os.File, done chan struct{}) {
	err := command.Wait()
	_ = terminal.Close()
	a.terminalMu.Lock()
	if a.terminal == terminal {
		a.terminal = nil
		a.terminalCommand = nil
		a.terminalDone = nil
	}
	a.terminalMu.Unlock()
	close(done)
	if a.ctx != nil {
		status := map[string]any{"running": false}
		if err != nil {
			status["error"] = err.Error()
		}
		runtime.EventsEmit(a.ctx, "opencode:status", status)
	}
}

// WriteOpenCodeInput forwards keyboard input to the OpenCode PTY.
func (a *App) WriteOpenCodeInput(input string) error {
	a.terminalMu.Lock()
	terminal := a.terminal
	a.terminalMu.Unlock()
	if terminal == nil {
		return fmt.Errorf("opencode is not running")
	}
	_, err := terminal.Write([]byte(input))
	return err
}

// ResizeOpenCode updates the PTY dimensions used by the OpenCode TUI.
func (a *App) ResizeOpenCode(columns, rows uint16) error {
	a.terminalMu.Lock()
	terminal := a.terminal
	a.terminalMu.Unlock()
	if terminal == nil {
		return fmt.Errorf("opencode is not running")
	}
	return pty.Setsize(terminal, &pty.Winsize{Cols: columns, Rows: rows})
}

// StopOpenCode stops the embedded OpenCode process.
func (a *App) StopOpenCode() error {
	a.terminalMu.Lock()
	command := a.terminalCommand
	done := a.terminalDone
	a.terminalMu.Unlock()
	if command == nil || command.Process == nil {
		return nil
	}
	killErr := command.Process.Kill()
	if done != nil {
		<-done
	}
	if killErr != nil && !errors.Is(killErr, os.ErrProcessDone) {
		return killErr
	}
	return nil
}

// SelectRevision reloads the explorer from the working tree or an immutable
// Git snapshot. It never checks out or changes the user's repository.
func (a *App) SelectRevision(directory, revision string) (headless.State, error) {
	a.revisionMu.Lock()
	defer a.revisionMu.Unlock()
	absolute, err := filepath.Abs(directory)
	if err != nil {
		return headless.State{}, err
	}
	previousRoot := a.revisionRoot
	if revision == "working-tree" || revision == "" {
		state, openErr := a.headlessWorkspace().OpenProgram(absolute)
		if openErr != nil {
			return headless.State{}, openErr
		}
		a.revisionRoot = ""
		if previousRoot != "" {
			_ = os.RemoveAll(previousRoot)
		}
		return state, nil
	}
	root, err := os.MkdirTemp("", "contuts-revision-")
	if err != nil {
		return headless.State{}, err
	}
	if err := materializeGitRevision(absolute, revision, root); err != nil {
		os.RemoveAll(root)
		return headless.State{}, err
	}
	a.programPath = absolute
	a.source = nil
	a.target = nil
	a.state = nil
	a.comparisonStates = make(map[string]*engine.WorkingState)
	a.proposalSessions = make(map[string]*proposalSession)
	state, err := a.headlessWorkspace().OpenProgramAt(absolute, root)
	if err != nil {
		os.RemoveAll(root)
		return headless.State{}, err
	}
	a.revisionRoot = root
	if previousRoot != "" && previousRoot != root {
		_ = os.RemoveAll(previousRoot)
	}
	return state, nil
}

// GetFileEdits returns structural edits needed to transform currentRevision
// into compareRevision for one file. It does not apply or persist anything.
func (a *App) GetFileEdits(directory, currentRevision, compareRevision, packageDirectory, packageName, filePath string) ([]EditSummary, error) {
	a.revisionMu.Lock()
	defer a.revisionMu.Unlock()
	absolute, err := filepath.Abs(directory)
	if err != nil {
		return nil, err
	}
	if _, err := gitOutput(absolute, "rev-parse", "--show-toplevel"); err != nil {
		return nil, err
	}
	state, err := a.comparisonState(absolute, currentRevision, compareRevision, packageDirectory, packageName, filePath)
	if err != nil {
		return nil, err
	}
	return summarizeComparisonState(state).Edits, nil
}

func (a *App) GetFileEditState(directory, currentRevision, compareRevision, packageDirectory, packageName, filePath string) (FileEditState, error) {
	a.revisionMu.Lock()
	defer a.revisionMu.Unlock()
	absolute, err := filepath.Abs(directory)
	if err != nil {
		return FileEditState{}, err
	}
	state, err := a.comparisonState(absolute, currentRevision, compareRevision, packageDirectory, packageName, filePath)
	if err != nil {
		return FileEditState{}, err
	}
	return a.summarizeActiveComparisonState(absolute, currentRevision, compareRevision, state), nil
}

// GetComparisonFiles returns the union of supported files present in either
// revision. A file may exist only in one revision and still needs comparison.
func (a *App) GetComparisonFiles(directory, currentRevision, compareRevision string) ([]ComparisonFile, error) {
	absolute, err := filepath.Abs(directory)
	if err != nil {
		return nil, err
	}
	files := make(map[string]ComparisonFile)
	roots := make([]string, 0, 2)
	cleanup := make([]string, 0, 2)
	for _, revision := range []string{currentRevision, compareRevision} {
		root := absolute
		if revision != "working-tree" && revision != "" {
			root, err = os.MkdirTemp("", "contuts-comparison-")
			if err != nil {
				return nil, err
			}
			cleanup = append(cleanup, root)
			if err := materializeGitRevision(absolute, revision, root); err != nil {
				return nil, err
			}
		}
		roots = append(roots, root)
	}
	for _, root := range roots {
		packages, discoverErr := explorer.DiscoverPackages(root)
		if discoverErr != nil {
			for _, path := range cleanup {
				_ = os.RemoveAll(path)
			}
			return nil, discoverErr
		}
		collectComparisonFiles(packages, files)
	}
	for _, path := range cleanup {
		_ = os.RemoveAll(path)
	}
	result := make([]ComparisonFile, 0, len(files))
	for _, file := range files {
		result = append(result, file)
	}
	sort.Slice(result, func(left, right int) bool { return result[left].File.Path < result[right].File.Path })
	return result, nil
}

func collectComparisonFiles(packages []explorer.Package, files map[string]ComparisonFile) {
	for _, pkg := range packages {
		for _, file := range pkg.Files {
			key := strings.Join([]string{pkg.Directory, pkg.Name, file.Path}, "\x00")
			files[key] = ComparisonFile{PackageDirectory: pkg.Directory, PackageName: pkg.Name, File: file}
		}
		collectComparisonFiles(pkg.Children, files)
	}
}

func comparisonKey(directory, currentRevision, compareRevision, packageDirectory, packageName, filePath string) string {
	return strings.Join([]string{directory, currentRevision, compareRevision, packageDirectory, packageName, filePath}, "\x00")
}

func proposalSessionKey(directory, currentRevision, compareRevision string) string {
	// Proposals belong to a shared base, not to one linear Compare to choice.
	return strings.Join([]string{directory, currentRevision}, "\x00")
}

func (a *App) ensureProposalSession(directory, currentRevision, compareRevision string) *proposalSession {
	if a.proposalSessions == nil {
		a.proposalSessions = make(map[string]*proposalSession)
	}
	key := proposalSessionKey(directory, currentRevision, compareRevision)
	if session := a.proposalSessions[key]; session != nil {
		return session
	}
	session := &proposalSession{
		activeBranchID: "base",
		branches:       make(map[string]*proposalBranch),
		selected:       make(map[string]bool),
	}
	session.branches["base"] = &proposalBranch{
		ProposalBranchSummary: ProposalBranchSummary{ID: "base", Name: "Base diff", Kind: "base", BaseRevision: currentRevision, SourceRevision: compareRevision, Active: true},
		states:                make(map[string]*engine.WorkingState),
	}
	session.branches["human"] = &proposalBranch{
		ProposalBranchSummary: ProposalBranchSummary{ID: "human", Name: "Human build", Kind: "human", ParentID: "base", BaseRevision: currentRevision, SourceRevision: currentRevision},
		states:                make(map[string]*engine.WorkingState),
	}
	a.proposalSessions[key] = session
	return session
}

func proposalWorkspace(session *proposalSession) ProposalWorkspace {
	branches := make([]ProposalBranchSummary, 0, len(session.branches))
	for _, branch := range session.branches {
		branch.Active = branch.ID == session.activeBranchID
		branches = append(branches, branch.ProposalBranchSummary)
	}
	sort.Slice(branches, func(left, right int) bool {
		if branches[left].ID == "base" {
			return true
		}
		if branches[right].ID == "base" {
			return false
		}
		if branches[left].ID == "human" {
			return true
		}
		if branches[right].ID == "human" {
			return false
		}
		return branches[left].ID < branches[right].ID
	})
	selected := make([]string, 0, len(session.selected))
	for id := range session.selected {
		selected = append(selected, id)
	}
	sort.Strings(selected)
	return ProposalWorkspace{ActiveBranchID: session.activeBranchID, SelectedProposalIDs: selected, Branches: branches}
}

// GetProposalBranches returns the base and proposal branches for a comparison.
func (a *App) GetProposalBranches(directory, currentRevision, compareRevision string) (ProposalWorkspace, error) {
	a.revisionMu.Lock()
	defer a.revisionMu.Unlock()
	absolute, err := filepath.Abs(directory)
	if err != nil {
		return ProposalWorkspace{}, err
	}
	return proposalWorkspace(a.ensureProposalSession(absolute, currentRevision, compareRevision)), nil
}

// CreateProposalBranch forks a new proposal from the immutable base branch.
func (a *App) CreateProposalBranch(directory, currentRevision, compareRevision, proposalRevision, name string) (ProposalWorkspace, error) {
	a.revisionMu.Lock()
	defer a.revisionMu.Unlock()
	absolute, err := filepath.Abs(directory)
	if err != nil {
		return ProposalWorkspace{}, err
	}
	session := a.ensureProposalSession(absolute, currentRevision, compareRevision)
	name = strings.TrimSpace(name)
	if name == "" {
		return ProposalWorkspace{}, fmt.Errorf("proposal name is required")
	}
	if proposalRevision == "" || proposalRevision == currentRevision {
		return ProposalWorkspace{}, fmt.Errorf("proposal source revision is required")
	}
	session.nextBranch++
	id := fmt.Sprintf("proposal-%d", session.nextBranch)
	branch := &proposalBranch{
		ProposalBranchSummary: ProposalBranchSummary{ID: id, Name: name, Kind: "proposal", ParentID: "base", BaseRevision: currentRevision, SourceRevision: proposalRevision},
		states:                make(map[string]*engine.WorkingState),
	}
	session.branches[id] = branch
	session.activeBranchID = id
	return proposalWorkspace(session), nil
}

// SelectProposalBranch switches the active branch without changing the base.
func (a *App) SelectProposalBranch(directory, currentRevision, compareRevision, branchID string) (ProposalWorkspace, error) {
	a.revisionMu.Lock()
	defer a.revisionMu.Unlock()
	absolute, err := filepath.Abs(directory)
	if err != nil {
		return ProposalWorkspace{}, err
	}
	session := a.ensureProposalSession(absolute, currentRevision, compareRevision)
	if _, ok := session.branches[branchID]; !ok {
		return ProposalWorkspace{}, fmt.Errorf("proposal branch %q does not exist", branchID)
	}
	session.activeBranchID = branchID
	return proposalWorkspace(session), nil
}

// SelectProposalBranches selects independent proposal targets for review.
func (a *App) SelectProposalBranches(directory, baseRevision string, branchIDs []string) (ProposalWorkspace, error) {
	a.revisionMu.Lock()
	defer a.revisionMu.Unlock()
	absolute, err := filepath.Abs(directory)
	if err != nil {
		return ProposalWorkspace{}, err
	}
	session := a.ensureProposalSession(absolute, baseRevision, "")
	session.selected = make(map[string]bool)
	for _, branchID := range branchIDs {
		branch := session.branches[branchID]
		if branch == nil || branch.Kind != "proposal" {
			return ProposalWorkspace{}, fmt.Errorf("proposal branch %q does not exist", branchID)
		}
		session.selected[branchID] = true
	}
	if len(branchIDs) > 0 {
		session.activeBranchID = branchIDs[0]
	}
	return proposalWorkspace(session), nil
}

// GetProposalFileEditState returns one proposal's diff against the shared base.
func (a *App) GetProposalFileEditState(directory, baseRevision, proposalID, packageDirectory, packageName, filePath string) (FileEditState, error) {
	a.revisionMu.Lock()
	defer a.revisionMu.Unlock()
	absolute, err := filepath.Abs(directory)
	if err != nil {
		return FileEditState{}, err
	}
	state, err := a.comparisonStateForBranch(absolute, baseRevision, "", packageDirectory, packageName, filePath, proposalID)
	if err != nil {
		return FileEditState{}, err
	}
	result := summarizeComparisonState(state)
	result.BranchID = proposalID
	return result, nil
}

// GetHumanFileEditState returns the current human build against the shared base.
func (a *App) GetHumanFileEditState(directory, baseRevision, packageDirectory, packageName, filePath string) (FileEditState, error) {
	a.revisionMu.Lock()
	defer a.revisionMu.Unlock()
	absolute, err := filepath.Abs(directory)
	if err != nil {
		return FileEditState{}, err
	}
	state, err := a.comparisonStateForBranch(absolute, baseRevision, "", packageDirectory, packageName, filePath, "human")
	if err != nil {
		return FileEditState{}, err
	}
	result := summarizeComparisonState(state)
	result.BranchID = "human"
	result.WorkingAuthoritative = true
	return result, nil
}

// CopyProposalEdit transfers one proposal edit into the human build branch.
func (a *App) CopyProposalEdit(directory, currentRevision, compareRevision, proposalID, packageDirectory, packageName, filePath string, index int) (FileEditState, error) {
	a.revisionMu.Lock()
	defer a.revisionMu.Unlock()
	absolute, err := filepath.Abs(directory)
	if err != nil {
		return FileEditState{}, err
	}
	source, err := a.comparisonStateForBranch(absolute, currentRevision, compareRevision, packageDirectory, packageName, filePath, proposalID)
	if err != nil {
		return FileEditState{}, err
	}
	human, err := a.comparisonStateForBranch(absolute, currentRevision, compareRevision, packageDirectory, packageName, filePath, "human")
	if err != nil {
		return FileEditState{}, err
	}
	operations, err := source.ProposalOperationsForSubtree(index)
	if err != nil {
		return FileEditState{}, err
	}
	if err := human.ApplyProposalOperations(operations); err != nil {
		return FileEditState{}, err
	}
	session := a.ensureProposalSession(absolute, currentRevision, compareRevision)
	session.rebaseProposals = true
	if session.humanStates == nil {
		session.humanStates = make(map[string]*engine.WorkingState)
	}
	session.humanStates[strings.Join([]string{packageDirectory, packageName, filePath}, "\x00")] = human
	session.branches[proposalID].stable = true
	for _, branch := range session.branches {
		if (branch.Kind == "proposal" && !branch.stable) || branch.Kind == "human" {
			branch.states = make(map[string]*engine.WorkingState)
		}
	}
	result := summarizeComparisonState(human)
	result.BranchID = "human"
	result.WorkingAuthoritative = true
	return result, nil
}

func languageForFile(filePath string) string {
	switch strings.ToLower(filepath.Ext(filePath)) {
	case ".ts":
		return "typescript"
	case ".tsx":
		return "tsx"
	case ".html", ".htm":
		return "html"
	}
	return "go"
}

func (a *App) baseComparisonState(directory, currentRevision, compareRevision, packageDirectory, packageName, filePath string) (*engine.WorkingState, error) {
	if a.comparisonStates == nil {
		a.comparisonStates = make(map[string]*engine.WorkingState)
	}
	key := comparisonKey(directory, currentRevision, compareRevision, packageDirectory, packageName, filePath)
	if state := a.comparisonStates[key]; state != nil {
		return state, nil
	}
	language := languageForFile(filePath)
	current, currentExists, err := revisionFileBytesOptional(directory, currentRevision, filePath)
	if err != nil {
		return nil, err
	}
	compare, compareExists, err := revisionFileBytesOptional(directory, compareRevision, filePath)
	if err != nil {
		return nil, err
	}
	if (!currentExists || !compareExists) && language == "go" {
		return nil, fmt.Errorf("cannot compare missing Go file %s", filePath)
	}
	state, err := engine.NewWorkingStateFromLanguage(language, current, compare)
	if err != nil {
		return nil, fmt.Errorf("compare %s: %w", filePath, err)
	}
	a.comparisonStates[key] = state
	return state, nil
}

func (a *App) comparisonState(directory, currentRevision, compareRevision, packageDirectory, packageName, filePath string) (*engine.WorkingState, error) {
	session := a.ensureProposalSession(directory, currentRevision, compareRevision)
	return a.comparisonStateForBranch(directory, currentRevision, compareRevision, packageDirectory, packageName, filePath, session.activeBranchID)
}

func (a *App) comparisonStateForBranch(directory, currentRevision, compareRevision, packageDirectory, packageName, filePath, branchID string) (*engine.WorkingState, error) {
	session := a.ensureProposalSession(directory, currentRevision, compareRevision)
	key := comparisonKey(directory, currentRevision, compareRevision, packageDirectory, packageName, filePath)
	branch := session.branches[branchID]
	if branch == nil {
		return nil, fmt.Errorf("proposal branch %q does not exist", branchID)
	}
	if branch.Kind == "base" {
		return a.baseComparisonState(directory, currentRevision, compareRevision, packageDirectory, packageName, filePath)
	}
	if state := branch.states[key]; state != nil {
		return state, nil
	}
	if branch.Kind == "human" {
		if session.rebaseProposals {
			humanKey := strings.Join([]string{packageDirectory, packageName, filePath}, "\x00")
			if human := session.humanStates[humanKey]; human != nil {
				branch.states[key] = human
				return human, nil
			}
		}
		current, exists, err := revisionFileBytesOptional(directory, currentRevision, filePath)
		if err != nil {
			return nil, err
		}
		if !exists && languageForFile(filePath) == "go" {
			return nil, fmt.Errorf("cannot compare missing Go file %s", filePath)
		}
		branch.states[key], err = engine.NewWorkingStateFromLanguage(languageForFile(filePath), current, current)
		if err != nil {
			return nil, fmt.Errorf("create human proposal state %s: %w", filePath, err)
		}
		return branch.states[key], nil
	}
	var base []byte
	var exists bool
	var err error
	if session.rebaseProposals && !branch.stable {
		humanKey := strings.Join([]string{packageDirectory, packageName, filePath}, "\x00")
		human := session.humanStates[humanKey]
		if human == nil {
			var humanErr error
			human, humanErr = a.comparisonStateForBranch(directory, currentRevision, compareRevision, packageDirectory, packageName, filePath, "human")
			if humanErr != nil {
				return nil, humanErr
			}
		}
		humanSnapshot := human.Snapshot()
		base = []byte(humanSnapshot.RenderedCode)
		exists = true
	} else {
		base, exists, err = revisionFileBytesOptional(directory, currentRevision, filePath)
		if err != nil {
			return nil, err
		}
	}
	proposal, proposalExists, err := revisionFileBytesOptional(directory, branch.SourceRevision, filePath)
	if err != nil {
		return nil, err
	}
	if (!exists || !proposalExists) && languageForFile(filePath) == "go" {
		return nil, fmt.Errorf("cannot compare missing Go file %s", filePath)
	}
	branch.states[key], err = engine.NewWorkingStateFromLanguage(languageForFile(filePath), base, proposal)
	if err != nil {
		return nil, fmt.Errorf("compare proposal %s: %w", filePath, err)
	}
	return branch.states[key], nil
}

func revisionFileBytesOptional(directory, revision, filePath string) ([]byte, bool, error) {
	if revision == "working-tree" || revision == "" {
		contents, err := os.ReadFile(filepath.Join(directory, filepath.FromSlash(filePath)))
		if os.IsNotExist(err) {
			return []byte{}, false, nil
		}
		return contents, err == nil, err
	}
	exists := exec.Command("git", "-C", directory, "cat-file", "-e", revision+":"+filePath).Run() == nil
	if !exists {
		return []byte{}, false, nil
	}
	contents, err := exec.Command("git", "-C", directory, "show", revision+":"+filePath).Output()
	return contents, err == nil, err
}

func summarizeComparisonState(state *engine.WorkingState) FileEditState {
	snapshot := state.Snapshot()
	target := state.TargetCode()
	validation := state.Validate()
	views := engine.ProjectEdits(snapshot.Edits, liftOptions(defaultHiddenKinds()))
	summarize := func(index int) EditSummary {
		edit := snapshot.Edits[index]
		item := EditSummary{Index: edit.Index, Kind: edit.Kind, NodeID: edit.NodeID, NodeGlobalID: edit.NodeGlobalID, SourceGlobalID: edit.SourceGlobalID, ParentGlobalID: edit.ParentGlobalID, NodeKind: edit.NodeKind, ParentID: edit.ParentID, ParentKind: edit.ParentKind, AncestorIDs: append([]string(nil), edit.AncestorIDs...), Ancestors: append([]engine.EditAncestor(nil), edit.Ancestors...), Field: edit.Field, Position: edit.Position, Value: edit.Value, Status: snapshot.Status[index]}
		if edit.Node != nil {
			item.StartLine = edit.Node.StartLine
			item.EndLine = edit.Node.EndLine
		}
		if item.EndLine == 0 {
			item.EndLine = item.StartLine
		}
		return item
	}
	all := make([]EditSummary, 0, len(snapshot.Edits))
	for index := range snapshot.Edits {
		if snapshot.Edits[index].Hidden {
			continue
		}
		all = append(all, summarize(index))
	}
	lifted := make([]EditSummary, 0, len(views))
	for _, view := range views {
		lifted = append(lifted, summarize(view.EditIndex))
	}
	return FileEditState{Edits: all, LiftedEdits: lifted, WorkingCode: snapshot.RenderedCode, TargetCode: target.Code, TargetDiagnostics: target.Diagnostics, RenderDiagnostics: snapshot.RenderDiagnostics, Diagnostics: validation.Diagnostics, Valid: validation.Valid}
}

func (a *App) summarizeActiveComparisonState(directory, currentRevision, compareRevision string, state *engine.WorkingState) FileEditState {
	result := summarizeComparisonState(state)
	session := a.ensureProposalSession(directory, currentRevision, compareRevision)
	result.BranchID = session.activeBranchID
	result.WorkingAuthoritative = session.activeBranchID == "human"
	return result
}

func (a *App) ApplyFileEdit(directory, currentRevision, compareRevision, packageDirectory, packageName, filePath string, index int) (FileEditState, error) {
	a.revisionMu.Lock()
	defer a.revisionMu.Unlock()
	absolute, err := filepath.Abs(directory)
	if err != nil {
		return FileEditState{}, err
	}
	state, err := a.comparisonState(absolute, currentRevision, compareRevision, packageDirectory, packageName, filePath)
	if err != nil {
		return FileEditState{}, err
	}
	if err := state.ApplyProjected(index, engine.ApplyOptions{Reconcile: true}, liftOptions(defaultHiddenKinds())); err != nil {
		return FileEditState{}, err
	}
	return a.summarizeActiveComparisonState(absolute, currentRevision, compareRevision, state), nil
}

// ApplyFileEditSubtree applies an edit and all of its dependent child edits.
func (a *App) ApplyFileEditSubtree(directory, currentRevision, compareRevision, packageDirectory, packageName, filePath string, index int) (FileEditState, error) {
	a.revisionMu.Lock()
	defer a.revisionMu.Unlock()
	absolute, err := filepath.Abs(directory)
	if err != nil {
		return FileEditState{}, err
	}
	state, err := a.comparisonState(absolute, currentRevision, compareRevision, packageDirectory, packageName, filePath)
	if err != nil {
		return FileEditState{}, err
	}
	// Full-line actions must leave a valid projected tree. Reconciliation keeps
	// structural children attached when the sequence applies adjacent edits.
	if err := state.ApplyProjectedSubtree(index, engine.ApplyOptions{Reconcile: true}, liftOptions(defaultHiddenKinds())); err != nil {
		return FileEditState{}, err
	}
	return a.summarizeActiveComparisonState(absolute, currentRevision, compareRevision, state), nil
}

func (a *App) RemoveFileEdit(directory, currentRevision, compareRevision, packageDirectory, packageName, filePath string, index int) (FileEditState, error) {
	a.revisionMu.Lock()
	defer a.revisionMu.Unlock()
	absolute, err := filepath.Abs(directory)
	if err != nil {
		return FileEditState{}, err
	}
	state, err := a.comparisonState(absolute, currentRevision, compareRevision, packageDirectory, packageName, filePath)
	if err != nil {
		return FileEditState{}, err
	}
	if err := state.RemoveProjected(index, liftOptions(defaultHiddenKinds())); err != nil {
		return FileEditState{}, err
	}
	return a.summarizeActiveComparisonState(absolute, currentRevision, compareRevision, state), nil
}

// RemoveFileEditSubtree removes an edit and all of its dependent child edits.
func (a *App) RemoveFileEditSubtree(directory, currentRevision, compareRevision, packageDirectory, packageName, filePath string, index int) (FileEditState, error) {
	a.revisionMu.Lock()
	defer a.revisionMu.Unlock()
	absolute, err := filepath.Abs(directory)
	if err != nil {
		return FileEditState{}, err
	}
	state, err := a.comparisonState(absolute, currentRevision, compareRevision, packageDirectory, packageName, filePath)
	if err != nil {
		return FileEditState{}, err
	}
	if err := state.RemoveProjectedSubtree(index, liftOptions(defaultHiddenKinds())); err != nil {
		return FileEditState{}, err
	}
	return a.summarizeActiveComparisonState(absolute, currentRevision, compareRevision, state), nil
}

func revisionFileBytes(directory, revision, filePath string) ([]byte, error) {
	if revision == "working-tree" || revision == "" {
		return os.ReadFile(filepath.Join(directory, filepath.FromSlash(filePath)))
	}
	output, err := exec.Command("git", "-C", directory, "show", revision+":"+filePath).Output()
	if err != nil {
		return nil, fmt.Errorf("read %s at %s: %w", filePath, revision, err)
	}
	return output, nil
}

func (a *App) clearRevisionRoot() {
	if a.revisionRoot != "" {
		_ = os.RemoveAll(a.revisionRoot)
		a.revisionRoot = ""
	}
}

func materializeGitRevision(directory, revision, destination string) error {
	archive, err := exec.Command("git", "-C", directory, "archive", revision).Output()
	if err != nil {
		return fmt.Errorf("read Git revision %s: %w", revision, err)
	}
	reader := tar.NewReader(bytes.NewReader(archive))
	for {
		header, err := reader.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read Git archive: %w", err)
		}
		cleanName := filepath.Clean(filepath.FromSlash(header.Name))
		if cleanName == "." || filepath.IsAbs(cleanName) || cleanName == ".." || strings.HasPrefix(cleanName, ".."+string(filepath.Separator)) {
			return fmt.Errorf("unsafe path in Git archive: %s", header.Name)
		}
		path := filepath.Join(destination, cleanName)
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(path, os.FileMode(header.Mode)); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return err
			}
			file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(file, reader)
			closeErr := file.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
		}
	}
}

// GetRevisionContext returns branch and commit choices without changing the
// checked-out worktree or generating a comparison.
func (a *App) GetRevisionContext(directory string) (RevisionContext, error) {
	absolute, err := filepath.Abs(directory)
	if err != nil {
		return RevisionContext{}, err
	}
	return a.revisionContext(absolute)
}

func (a *App) revisionContext(absolute string) (RevisionContext, error) {
	gitRoot, err := gitOutput(absolute, "rev-parse", "--show-toplevel")
	if err != nil || filepath.Clean(gitRoot) != filepath.Clean(absolute) {
		return RevisionContext{}, fmt.Errorf("program directory is not an independent Git repository")
	}
	branch, err := gitOutput(absolute, "symbolic-ref", "--short", "-q", "HEAD")
	if err != nil {
		branch = "HEAD"
	}
	commit, commitErr := gitOutput(absolute, "rev-parse", "HEAD")
	if commitErr != nil {
		// An initialized repository may not have its first commit yet.
		commit = ""
	}
	branchRows, err := gitOutput(absolute, "for-each-ref", "--format=%(refname:short)\t%(objectname)\t%(committerdate:iso8601)\t%(subject)", "refs/heads", "refs/remotes")
	if err != nil {
		return RevisionContext{}, fmt.Errorf("read Git branches: %w", err)
	}
	logRows, err := gitOutput(absolute, "log", "--all", "--format=%H\t%h\t%cI\t%an\t%s", "--date-order")
	if err != nil {
		return RevisionContext{}, fmt.Errorf("read Git commits: %w", err)
	}

	options := make([]RevisionOption, 0)
	seen := make(map[string]bool)
	for _, row := range strings.Split(branchRows, "\n") {
		parts := strings.SplitN(row, "\t", 4)
		if len(parts) != 4 || parts[0] == "" || seen[parts[1]] {
			continue
		}
		seen[parts[1]] = true
		options = append(options, RevisionOption{Kind: "branch", Ref: parts[0], Hash: parts[1], ShortHash: parts[1][:minInt(7, len(parts[1]))], Date: parts[2], Subject: parts[3]})
	}
	for _, row := range strings.Split(logRows, "\n") {
		parts := strings.SplitN(row, "\t", 5)
		if len(parts) != 5 || parts[0] == "" || seen[parts[0]] {
			continue
		}
		seen[parts[0]] = true
		options = append(options, RevisionOption{Kind: "commit", Ref: parts[0], Hash: parts[0], ShortHash: parts[1], Date: parts[2], Author: parts[3], Subject: parts[4]})
	}
	return RevisionContext{Branch: strings.TrimSpace(branch), CurrentCommit: strings.TrimSpace(commit), Options: options}, nil
}

// PromoteHumanBuild writes the in-memory Human build into the repository and
// records it as a commit on the currently checked-out branch.
func (a *App) PromoteHumanBuild(directory, currentRevision, message string) (RevisionContext, error) {
	a.revisionMu.Lock()
	defer a.revisionMu.Unlock()
	absolute, err := filepath.Abs(directory)
	if err != nil {
		return RevisionContext{}, err
	}
	if _, err := gitOutput(absolute, "rev-parse", "--show-toplevel"); err != nil {
		return RevisionContext{}, fmt.Errorf("commit Human build: %w", err)
	}
	branch, err := gitOutput(absolute, "symbolic-ref", "--short", "-q", "HEAD")
	if err != nil || strings.TrimSpace(branch) == "" {
		return RevisionContext{}, fmt.Errorf("commit Human build: repository is not on a branch")
	}
	session := a.ensureProposalSession(absolute, currentRevision, "")
	human := session.branches["human"]
	if human == nil {
		return RevisionContext{}, fmt.Errorf("Human build is not initialized")
	}
	files := make(map[string]*engine.WorkingState)
	for key, state := range session.humanStates {
		parts := strings.Split(key, "\x00")
		if len(parts) == 3 {
			files[parts[2]] = state
		}
	}
	for key, state := range human.states {
		parts := strings.Split(key, "\x00")
		if len(parts) >= 6 {
			files[parts[5]] = state
		}
	}
	if len(files) == 0 {
		return RevisionContext{}, fmt.Errorf("no Human build edits are ready to commit")
	}
	paths := make([]string, 0, len(files))
	for path, state := range files {
		if state == nil {
			continue
		}
		validation := state.Validate()
		if !validation.Valid {
			return RevisionContext{}, fmt.Errorf("cannot commit Human build: %s is invalid", path)
		}
		absolutePath := filepath.Join(absolute, filepath.FromSlash(path))
		if err := os.WriteFile(absolutePath, []byte(state.Snapshot().RenderedCode), 0o644); err != nil {
			return RevisionContext{}, fmt.Errorf("write Human file %s: %w", path, err)
		}
		paths = append(paths, path)
	}
	sort.Strings(paths)
	if len(paths) == 0 {
		return RevisionContext{}, fmt.Errorf("no Human build files are ready to commit")
	}
	if _, err := gitOutput(absolute, append([]string{"add", "--"}, paths...)...); err != nil {
		return RevisionContext{}, fmt.Errorf("stage Human build: %w", err)
	}
	message = strings.TrimSpace(message)
	if message == "" {
		message = "contuts: commit Human build"
	}
	if _, err := gitOutput(absolute, "commit", "-m", message); err != nil {
		return RevisionContext{}, fmt.Errorf("commit Human build: %w", err)
	}
	return a.revisionContext(absolute)
}

func gitOutput(directory string, args ...string) (string, error) {
	commandArgs := append([]string{"-C", directory}, args...)
	output, err := exec.Command("git", commandArgs...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, output)
	}
	return strings.TrimSpace(string(output)), nil
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}

func (a *App) headlessWorkspace() *headless.Workspace {
	if a.workspace == nil {
		a.workspace = headless.New()
	}
	return a.workspace
}

// GetCurrentState returns the complete renderable inquiry state.
func (a *App) GetCurrentState() (headless.State, error) {
	return a.headlessWorkspace().CurrentState()
}

// StartInquiry adds a new independent inquiry row at the bottom.
func (a *App) StartInquiry(title string) (headless.State, error) {
	return a.headlessWorkspace().StartInquiry(title)
}

// OpenComparisonFile starts a new inquiry for a file that may only exist in
// one side of the selected revision comparison. The diff baseline is the
// source shown to the user; compare-to is only a fallback for missing files.
func (a *App) OpenComparisonFile(directory, currentRevision, compareRevision, packageDirectory, packageName, filePath string) (headless.State, error) {
	absolute, err := filepath.Abs(directory)
	if err != nil {
		return headless.State{}, err
	}
	files, err := a.GetComparisonFiles(directory, currentRevision, compareRevision)
	if err != nil {
		return headless.State{}, err
	}
	for _, comparisonFile := range files {
		if comparisonFile.PackageDirectory != packageDirectory || comparisonFile.PackageName != packageName || comparisonFile.File.Path != filePath {
			continue
		}
		source, exists, err := revisionFileBytesOptional(absolute, currentRevision, filePath)
		if err != nil {
			return headless.State{}, err
		}
		if !exists {
			source, _, err = revisionFileBytesOptional(absolute, compareRevision, filePath)
			if err != nil {
				return headless.State{}, err
			}
		}
		return a.headlessWorkspace().OpenComparisonFile(filePath, packageDirectory, packageName, comparisonFile.File, string(source))
	}
	return headless.State{}, fmt.Errorf("comparison file %s was not found", filePath)
}

// OpenInquiryPackage appends a package tile to an inquiry row.
func (a *App) OpenInquiryPackage(rowID, tileID, packageDirectory, packageName string) (headless.State, error) {
	return a.headlessWorkspace().OpenPackage(rowID, tileID, packageDirectory, packageName)
}

// NavigateInquiryPackage enters a package in the current column.
func (a *App) NavigateInquiryPackage(rowID, tileID, packageDirectory, packageName string) (headless.State, error) {
	return a.headlessWorkspace().NavigatePackage(rowID, tileID, packageDirectory, packageName)
}

// BackInquiry restores the previous target in the selected column.
func (a *App) BackInquiry(rowID, tileID string) (headless.State, error) {
	return a.headlessWorkspace().Back(rowID, tileID)
}

// InspectInquiryFile loads a file into a tile's source pane without changing
// the tile's package target.
func (a *App) InspectInquiryFile(rowID, tileID, packageDirectory, packageName, filePath string) (headless.State, error) {
	return a.headlessWorkspace().InspectFile(rowID, tileID, packageDirectory, packageName, filePath)
}

// InspectInquiryDeclaration loads a declaration's containing file into the
// current tile's source pane without changing lineage.
func (a *App) InspectInquiryDeclaration(rowID, tileID, packageDirectory, packageName, filePath, name string, line int) (headless.State, error) {
	return a.headlessWorkspace().InspectDeclaration(rowID, tileID, packageDirectory, packageName, filePath, name, line)
}

// OpenInquiryFile appends a file tile to an inquiry row.
func (a *App) OpenInquiryFile(rowID, tileID, packageDirectory, packageName, filePath string) (headless.State, error) {
	return a.headlessWorkspace().OpenFile(rowID, tileID, packageDirectory, packageName, filePath)
}

// NavigateInquiryFile enters a file in the current column.
func (a *App) NavigateInquiryFile(rowID, tileID, packageDirectory, packageName, filePath string) (headless.State, error) {
	return a.headlessWorkspace().NavigateFile(rowID, tileID, packageDirectory, packageName, filePath)
}

// OpenInquiryDeclaration appends a declaration tile to an inquiry row.
func (a *App) OpenInquiryDeclaration(rowID, tileID, packageDirectory, packageName, filePath, name string, line int) (headless.State, error) {
	return a.headlessWorkspace().OpenDeclaration(rowID, tileID, packageDirectory, packageName, filePath, name, line)
}

// OpenInquiryDeclarationLeft inserts a declaration tile to the left of the source tile.
func (a *App) OpenInquiryDeclarationLeft(rowID, tileID, packageDirectory, packageName, filePath, name string, line int) (headless.State, error) {
	return a.headlessWorkspace().OpenDeclarationLeft(rowID, tileID, packageDirectory, packageName, filePath, name, line)
}

// NavigateInquiryDeclaration enters a declaration in the current column.
func (a *App) NavigateInquiryDeclaration(rowID, tileID, packageDirectory, packageName, filePath, name string, line int) (headless.State, error) {
	return a.headlessWorkspace().NavigateDeclaration(rowID, tileID, packageDirectory, packageName, filePath, name, line)
}

// SetInquiryPane controls one of the two independently collapsible tile views.
func (a *App) SetInquiryPane(rowID, tileID, pane string, collapsed bool) (headless.State, error) {
	return a.headlessWorkspace().SetPane(rowID, tileID, pane, collapsed)
}

// SetInquiryTileCollapsed controls the contents of one inquiry tile.
func (a *App) SetInquiryTileCollapsed(rowID, tileID string, collapsed bool) (headless.State, error) {
	return a.headlessWorkspace().SetTileCollapsed(rowID, tileID, collapsed)
}

// CloseInquiryColumn removes the selected column and its descendants from a
// row, returning focus to the preceding tile.
func (a *App) CloseInquiryColumn(rowID, tileID string) (headless.State, error) {
	return a.headlessWorkspace().CloseColumn(rowID, tileID)
}

// CloseInquiryTile removes one tile while preserving the other columns.
func (a *App) CloseInquiryTile(rowID, tileID string) (headless.State, error) {
	return a.headlessWorkspace().CloseTile(rowID, tileID)
}

// OpenPackage enters the file view for one discovered package.
func (a *App) OpenPackage(directory, packageDirectory, packageName string) (ProgramSnapshot, error) {
	absolute, err := filepath.Abs(directory)
	if err != nil {
		return ProgramSnapshot{}, err
	}
	packages, err := explorer.DiscoverPackages(absolute)
	if err != nil {
		return ProgramSnapshot{}, err
	}
	for _, pkg := range packages {
		if pkg.Directory == packageDirectory && pkg.Name == packageName {
			a.programPath = absolute
			a.source = nil
			a.target = nil
			a.state = nil
			return ProgramSnapshot{Path: absolute, Exploration: true, PackageName: pkg.Name, PackageDirectory: pkg.Directory, LocalImports: pkg.LocalImports, Files: pkg.Files}, nil
		}
	}
	return ProgramSnapshot{}, fmt.Errorf("package %s in %s was not found", packageName, packageDirectory)
}

// OpenFile enters the declaration view for one discovered file.
func (a *App) OpenFile(directory, packageDirectory, packageName, filePath string) (ProgramSnapshot, error) {
	absolute, err := filepath.Abs(directory)
	if err != nil {
		return ProgramSnapshot{}, err
	}
	packages, err := explorer.DiscoverPackages(absolute)
	if err != nil {
		return ProgramSnapshot{}, err
	}
	for _, pkg := range packages {
		if pkg.Directory != packageDirectory || pkg.Name != packageName {
			continue
		}
		for _, file := range pkg.Files {
			if file.Path != filePath {
				continue
			}
			source, err := os.ReadFile(filepath.Join(absolute, filepath.FromSlash(file.Path)))
			if err != nil {
				return ProgramSnapshot{}, fmt.Errorf("read %s: %w", file.Path, err)
			}
			a.programPath = absolute
			a.source = nil
			a.target = nil
			a.state = nil
			return ProgramSnapshot{Path: absolute, Exploration: true, PackageName: pkg.Name, PackageDirectory: pkg.Directory, LocalImports: pkg.LocalImports, Imports: localImportSummaries(absolute, file, packages), Files: pkg.Files, FileName: file.Name, FilePath: file.Path, Declarations: file.Declarations, FileSource: string(source)}, nil
		}
	}
	return ProgramSnapshot{}, fmt.Errorf("file %s was not found in package %s", filePath, packageName)
}

func localImportSummaries(root string, file explorer.File, packages []explorer.Package) []explorer.ImportSummary {
	module := explorer.ModulePath(root)
	result := make([]explorer.ImportSummary, 0)
	for _, imported := range file.Imports {
		if module == "" || !strings.HasPrefix(imported, module) {
			continue
		}
		for _, pkg := range packages {
			packagePath := module
			if pkg.Directory != "" {
				packagePath += "/" + filepath.ToSlash(pkg.Directory)
			}
			if packagePath == imported {
				declarations := make([]explorer.Declaration, 0)
				for _, candidate := range pkg.Files {
					for _, declaration := range candidate.Declarations {
						if declaration.Exported || len(declaration.Children) > 0 {
							declarations = append(declarations, declaration)
						}
					}
				}
				result = append(result, explorer.ImportSummary{Path: imported, Name: pkg.Name, Directory: pkg.Directory, Files: pkg.Files, Declarations: declarations})
			}
		}
	}
	return result
}

// OpenImportedFile loads a local imported file without replacing the current file.
func (a *App) OpenImportedFile(directory, importPath, filePath string) (ProgramSnapshot, error) {
	absolute, err := filepath.Abs(directory)
	if err != nil {
		return ProgramSnapshot{}, err
	}
	module := explorer.ModulePath(absolute)
	packages, err := explorer.DiscoverPackages(absolute)
	if err != nil {
		return ProgramSnapshot{}, err
	}
	for _, pkg := range packages {
		packagePath := module
		if pkg.Directory != "" {
			packagePath += "/" + filepath.ToSlash(pkg.Directory)
		}
		if packagePath != importPath {
			continue
		}
		for _, file := range pkg.Files {
			if file.Path != filePath {
				continue
			}
			source, err := os.ReadFile(filepath.Join(absolute, filepath.FromSlash(file.Path)))
			if err != nil {
				return ProgramSnapshot{}, err
			}
			return ProgramSnapshot{Path: absolute, Exploration: true, PackageName: pkg.Name, PackageDirectory: pkg.Directory, FileName: file.Name, FilePath: file.Path, Declarations: file.Declarations, FileSource: string(source)}, nil
		}
	}
	return ProgramSnapshot{}, fmt.Errorf("imported file %s was not found in %s", filePath, importPath)
}

// OpenImportedDeclaration loads an imported local symbol without replacing the current file.
func (a *App) OpenImportedDeclaration(directory, importPath, name string, line int) (ProgramSnapshot, error) {
	absolute, err := filepath.Abs(directory)
	if err != nil {
		return ProgramSnapshot{}, err
	}
	module := explorer.ModulePath(absolute)
	packages, err := explorer.DiscoverPackages(absolute)
	if err != nil {
		return ProgramSnapshot{}, err
	}
	for _, pkg := range packages {
		packagePath := module
		if pkg.Directory != "" {
			packagePath += "/" + filepath.ToSlash(pkg.Directory)
		}
		if packagePath != importPath {
			continue
		}
		for _, file := range pkg.Files {
			declaration := findDeclaration(file.Declarations, name, line)
			if declaration == nil {
				continue
			}
			source, err := os.ReadFile(filepath.Join(absolute, filepath.FromSlash(file.Path)))
			if err != nil {
				return ProgramSnapshot{}, err
			}
			lines := strings.Split(string(source), "\n")
			return ProgramSnapshot{Path: absolute, Exploration: true, ImportedFileName: file.Name, ImportedSource: strings.Join(lines[declaration.Line-1:declaration.EndLine], "\n"), ImportedName: name}, nil
		}
	}
	return ProgramSnapshot{}, fmt.Errorf("imported declaration %s was not found in %s", name, importPath)
}

func findDeclaration(declarations []explorer.Declaration, name string, line int) *explorer.Declaration {
	for index := range declarations {
		if declarations[index].Name == name && declarations[index].Line == line {
			return &declarations[index]
		}
		if match := findDeclaration(declarations[index].Children, name, line); match != nil {
			return match
		}
	}
	return nil
}

// OpenDeclaration shows the exact source range for a top-level declaration.
func (a *App) OpenDeclaration(directory, packageDirectory, packageName, filePath, name string, line int) (ProgramSnapshot, error) {
	snapshot, err := a.OpenFile(directory, packageDirectory, packageName, filePath)
	if err != nil {
		return ProgramSnapshot{}, err
	}
	var match *explorer.Declaration
	var find func([]explorer.Declaration)
	find = func(declarations []explorer.Declaration) {
		for index := range declarations {
			declaration := &declarations[index]
			if declaration.Name == name && declaration.Line == line {
				match = declaration
				return
			}
			find(declaration.Children)
			if match != nil {
				return
			}
		}
	}
	find(snapshot.Declarations)
	if match == nil {
		return ProgramSnapshot{}, fmt.Errorf("declaration %s was not found in %s", name, filePath)
	}
	lines := strings.Split(snapshot.FileSource, "\n")
	start := match.Line - 1
	end := match.EndLine
	if start < 0 {
		start = 0
	}
	if end > len(lines) {
		end = len(lines)
	}
	snapshot.DeclarationName = match.Name
	snapshot.DeclarationSource = strings.Join(lines[start:end], "\n")
	return snapshot, nil
}

// ExplorePreviousCommitEdits opens the previous-vs-current structural edit
// workflow that was formerly performed by OpenProgram.
func (a *App) ExplorePreviousCommitEdits(directory string) (ProgramSnapshot, error) {
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

// ApplyEditView applies a projected row and prepares hidden ancestors in the
// engine rather than in the frontend.
func (a *App) ApplyEditView(index int, reconcile bool, hiddenKinds []string) (ProgramSnapshot, error) {
	if a.state == nil {
		return ProgramSnapshot{}, fmt.Errorf("no program is open")
	}
	if err := a.state.ApplyProjected(index, engine.ApplyOptions{Reconcile: reconcile}, liftOptions(hiddenKinds)); err != nil {
		return ProgramSnapshot{}, err
	}
	return a.snapshot(), nil
}

// RemoveEditView removes a projected row and empty hidden ancestors.
func (a *App) RemoveEditView(index int, hiddenKinds []string) (ProgramSnapshot, error) {
	if a.state == nil {
		return ProgramSnapshot{}, fmt.Errorf("no program is open")
	}
	if err := a.state.RemoveProjected(index, liftOptions(hiddenKinds)); err != nil {
		return ProgramSnapshot{}, err
	}
	return a.snapshot(), nil
}

// ProjectEdits returns the canonical edits projected with hidden shells.
func (a *App) ProjectEdits(hiddenKinds []string) ([]engine.EditView, error) {
	if a.state == nil {
		return nil, fmt.Errorf("no program is open")
	}
	return engine.ProjectEdits(a.state.Snapshot().Edits, liftOptions(hiddenKinds)), nil
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
	if a.state == nil {
		return ProgramSnapshot{Path: a.programPath, Target: string(a.target), Exploration: true}
	}
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
		EditViews:         engine.ProjectEdits(snapshot.Edits, liftOptions(defaultHiddenKinds())),
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
	_ = a.StopOpenCode()
}

// Greet returns a greeting for the given name
func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's show time!", name)
}
