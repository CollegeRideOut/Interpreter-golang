package main

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

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
	revisionRoot     string
	comparisonStates map[string]*engine.WorkingState
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
	Edits             []EditSummary `json:"edits"`
	LiftedEdits       []EditSummary `json:"liftedEdits,omitempty"`
	WorkingCode       string        `json:"workingCode"`
	RenderDiagnostics []string      `json:"renderDiagnostics,omitempty"`
	Diagnostics       []string      `json:"diagnostics,omitempty"`
	Valid             bool          `json:"valid"`
}

func liftOptions(hiddenKinds []string) engine.LiftOptions {
	hidden := make(map[string]bool, len(hiddenKinds))
	for _, kind := range hiddenKinds {
		hidden[kind] = true
	}
	return engine.LiftOptions{HiddenKinds: hidden}
}

func defaultHiddenKinds() []string {
	return []string{"*ast.BlockStmt", "*ast.ExprStmt", "*ast.DeclStmt", "*ast.ImportSpec", "*ast.FieldList", "*ast.Field"}
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{workspace: headless.New(), comparisonStates: make(map[string]*engine.WorkingState)}
}

// startup is called at application startup
func (a *App) startup(ctx context.Context) {
	// Perform your setup here
	a.ctx = ctx
}

// OpenProgram opens the current working tree in read-only package exploration
// mode. It does not require Git or create an edit session.
func (a *App) OpenProgram(directory string) (ProgramSnapshot, error) {
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

// SelectRevision reloads the explorer from the working tree or an immutable
// Git snapshot. It never checks out or changes the user's repository.
func (a *App) SelectRevision(directory, revision string) (headless.State, error) {
	absolute, err := filepath.Abs(directory)
	if err != nil {
		return headless.State{}, err
	}
	a.clearRevisionRoot()
	if revision == "working-tree" || revision == "" {
		return a.headlessWorkspace().OpenProgram(absolute)
	}
	root, err := os.MkdirTemp("", "contuts-revision-")
	if err != nil {
		return headless.State{}, err
	}
	if err := materializeGitRevision(absolute, revision, root); err != nil {
		os.RemoveAll(root)
		return headless.State{}, err
	}
	a.revisionRoot = root
	a.programPath = absolute
	a.source = nil
	a.target = nil
	a.state = nil
	a.comparisonStates = make(map[string]*engine.WorkingState)
	return a.headlessWorkspace().OpenProgramAt(absolute, root)
}

// GetFileEdits returns structural edits needed to transform currentRevision
// into compareRevision for one file. It does not apply or persist anything.
func (a *App) GetFileEdits(directory, currentRevision, compareRevision, packageDirectory, packageName, filePath string) ([]EditSummary, error) {
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
	absolute, err := filepath.Abs(directory)
	if err != nil {
		return FileEditState{}, err
	}
	state, err := a.comparisonState(absolute, currentRevision, compareRevision, packageDirectory, packageName, filePath)
	if err != nil {
		return FileEditState{}, err
	}
	return summarizeComparisonState(state), nil
}

func comparisonKey(directory, currentRevision, compareRevision, packageDirectory, packageName, filePath string) string {
	return strings.Join([]string{directory, currentRevision, compareRevision, packageDirectory, packageName, filePath}, "\x00")
}

func (a *App) comparisonState(directory, currentRevision, compareRevision, packageDirectory, packageName, filePath string) (*engine.WorkingState, error) {
	if a.comparisonStates == nil {
		a.comparisonStates = make(map[string]*engine.WorkingState)
	}
	key := comparisonKey(directory, currentRevision, compareRevision, packageDirectory, packageName, filePath)
	if state := a.comparisonStates[key]; state != nil {
		return state, nil
	}
	current, err := revisionFileBytes(directory, currentRevision, filePath)
	if err != nil {
		return nil, err
	}
	compare, err := revisionFileBytes(directory, compareRevision, filePath)
	if err != nil {
		return nil, err
	}
	state, err := engine.NewWorkingStateFromSource(current, compare)
	if err != nil {
		return nil, fmt.Errorf("compare %s: %w", filePath, err)
	}
	a.comparisonStates[key] = state
	return state, nil
}

func summarizeComparisonState(state *engine.WorkingState) FileEditState {
	snapshot := state.Snapshot()
	validation := state.ValidateGo()
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
		all = append(all, summarize(index))
	}
	lifted := make([]EditSummary, 0, len(views))
	for _, view := range views {
		lifted = append(lifted, summarize(view.EditIndex))
	}
	return FileEditState{Edits: all, LiftedEdits: lifted, WorkingCode: snapshot.RenderedCode, RenderDiagnostics: snapshot.RenderDiagnostics, Diagnostics: validation.Diagnostics, Valid: validation.Valid}
}

func (a *App) ApplyFileEdit(directory, currentRevision, compareRevision, packageDirectory, packageName, filePath string, index int) (FileEditState, error) {
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
	return summarizeComparisonState(state), nil
}

func (a *App) RemoveFileEdit(directory, currentRevision, compareRevision, packageDirectory, packageName, filePath string, index int) (FileEditState, error) {
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
	return summarizeComparisonState(state), nil
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
	gitRoot, err := gitOutput(absolute, "rev-parse", "--show-toplevel")
	if err != nil || filepath.Clean(gitRoot) != filepath.Clean(absolute) {
		return RevisionContext{}, fmt.Errorf("program directory is not an independent Git repository")
	}
	branch, err := gitOutput(absolute, "symbolic-ref", "--short", "-q", "HEAD")
	if err != nil {
		branch = "HEAD"
	}
	commit, err := gitOutput(absolute, "rev-parse", "HEAD")
	if err != nil {
		return RevisionContext{}, fmt.Errorf("read current Git commit: %w", err)
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
	// Perform your teardown here
}

// Greet returns a greeting for the given name
func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's show time!", name)
}
