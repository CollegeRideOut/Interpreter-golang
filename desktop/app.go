package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"interpreter/engine"
	"interpreter/explorer"
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
	return &App{}
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
	packages, err := explorer.DiscoverPackages(absolute)
	if err != nil {
		return ProgramSnapshot{}, err
	}
	a.programPath = absolute
	a.source = nil
	a.target = nil
	a.state = nil
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
