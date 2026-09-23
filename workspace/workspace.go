package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"interpreter/explorer"
)

// State is the complete renderable state of the headless inquiry workspace.
// Clients should replace their local state with the result of every command.
type State struct {
	Revision uint64        `json:"revision"`
	Program  *ProgramState `json:"program,omitempty"`
	Rows     []InquiryRow  `json:"rows"`
	Active   Selection     `json:"active"`
}

type ProgramState struct {
	Path         string             `json:"path"`
	Module       string             `json:"module,omitempty"`
	Packages     []explorer.Package `json:"packages,omitempty"`
	LocalImports []string           `json:"localImports,omitempty"`
}

type InquiryRow struct {
	ID    string `json:"id"`
	Title string `json:"title,omitempty"`
	Tiles []Tile `json:"tiles"`
	Edges []Edge `json:"edges,omitempty"`
}

type Tile struct {
	ID               string    `json:"id"`
	Column           int       `json:"column"`
	Target           Target    `json:"target"`
	OpenedBy         *EdgeRef  `json:"openedBy,omitempty"`
	PreviouslyOpened bool      `json:"previouslyOpened,omitempty"`
	ExistingTileID   string    `json:"existingTileId,omitempty"`
	Overview         Overview  `json:"overview"`
	Text             TextView  `json:"text"`
	Panes            PaneState `json:"panes"`
	Collapsed        bool      `json:"collapsed"`
	CanGoBack        bool      `json:"canGoBack"`
}

type Target struct {
	Kind            string `json:"kind"`
	PackagePath     string `json:"packagePath,omitempty"`
	PackageName     string `json:"packageName,omitempty"`
	FilePath        string `json:"filePath,omitempty"`
	DeclarationName string `json:"declarationName,omitempty"`
	Line            int    `json:"line,omitempty"`
	EndLine         int    `json:"endLine,omitempty"`
}

type Overview struct {
	Kind                string                   `json:"kind"`
	Title               string                   `json:"title"`
	Subtitle            string                   `json:"subtitle,omitempty"`
	Packages            []explorer.Package       `json:"packages,omitempty"`
	Files               []explorer.File          `json:"files,omitempty"`
	Declarations        []explorer.Declaration   `json:"declarations,omitempty"`
	ImportPaths         []string                 `json:"importPaths,omitempty"`
	Imports             []explorer.ImportSummary `json:"imports,omitempty"`
	References          []explorer.Reference     `json:"references,omitempty"`
	SelectedDeclaration *explorer.Declaration    `json:"selectedDeclaration,omitempty"`
}

type TextView struct {
	Language        string                `json:"language,omitempty"`
	Filename        string                `json:"filename,omitempty"`
	Content         string                `json:"content,omitempty"`
	SourceStartLine int                   `json:"sourceStartLine,omitempty"`
	Occurrences     []explorer.Occurrence `json:"occurrences,omitempty"`
}

type PaneState struct {
	OverviewCollapsed bool `json:"overviewCollapsed"`
	TextCollapsed     bool `json:"textCollapsed"`
}

type Selection struct {
	RowID  string `json:"rowId,omitempty"`
	TileID string `json:"tileId,omitempty"`
}

type Edge struct {
	ID         string `json:"id"`
	FromTileID string `json:"fromTileId"`
	ToTileID   string `json:"toTileId"`
	Kind       string `json:"kind"`
	Label      string `json:"label"`
}

type EdgeRef struct {
	EdgeID       string `json:"edgeId"`
	Relationship string `json:"relationship"`
	FromTileID   string `json:"fromTileId"`
}

type Workspace struct {
	mu          sync.RWMutex
	state       State
	programRoot string
	nextID      uint64
	history     map[string][]Tile
}

func New() *Workspace {
	return &Workspace{history: make(map[string][]Tile)}
}

// CurrentState returns an independent copy of the current renderable state.
func (w *Workspace) CurrentState() (State, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if w.state.Program == nil {
		return State{}, fmt.Errorf("no program is open")
	}
	return cloneState(w.state), nil
}

// OpenProgram creates the first inquiry row for a directory.
func (w *Workspace) OpenProgram(directory string) (State, error) {
	absolute, err := filepath.Abs(directory)
	if err != nil {
		return State{}, err
	}
	return w.OpenProgramAt(absolute, absolute)
}

// OpenProgramAt explores sourceRoot while keeping displayPath as the program
// identity shown to clients. This lets callers inspect a Git snapshot without
// changing the user's checked-out working tree.
func (w *Workspace) OpenProgramAt(displayPath, sourceRoot string) (State, error) {
	absolute, err := filepath.Abs(sourceRoot)
	if err != nil {
		return State{}, err
	}
	packages, err := explorer.DiscoverPackages(absolute)
	if err != nil {
		return State{}, err
	}
	localImports := make([]string, 0)
	seenImports := make(map[string]bool)
	for _, pkg := range packages {
		for _, imported := range pkg.LocalImports {
			if !seenImports[imported] {
				seenImports[imported] = true
				localImports = append(localImports, imported)
			}
		}
	}
	program := &ProgramState{Path: displayPath, Module: explorer.ModulePath(absolute), Packages: packages, LocalImports: localImports}

	w.mu.Lock()
	defer w.mu.Unlock()
	w.programRoot = absolute
	w.nextID = 0
	w.history = make(map[string][]Tile)
	w.state = State{Program: program}
	rowID := w.id("row")
	tileID := w.id("tile")
	w.state.Rows = []InquiryRow{{
		ID:    rowID,
		Title: "Program inquiry",
		Tiles: []Tile{{
			ID:       tileID,
			Column:   0,
			Target:   Target{Kind: "program"},
			Overview: Overview{Kind: "program", Title: filepath.Base(absolute), Subtitle: absolute, Packages: packages},
			Text:     TextView{Language: "text", Filename: absolute},
			Panes:    PaneState{},
		}},
	}}
	w.state.Active = Selection{RowID: rowID, TileID: tileID}
	w.bump()
	return cloneState(w.state), nil
}

// StartInquiry adds an independent inquiry row at the bottom.
func (w *Workspace) StartInquiry(title string) (State, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.state.Program == nil {
		return State{}, fmt.Errorf("no program is open")
	}
	rowID := w.id("row")
	tileID := w.id("tile")
	if strings.TrimSpace(title) == "" {
		title = fmt.Sprintf("Inquiry %d", len(w.state.Rows)+1)
	}
	w.state.Rows = append(w.state.Rows, InquiryRow{ID: rowID, Title: title, Tiles: []Tile{{
		ID: tileID, Column: 0, Target: Target{Kind: "program"},
		Overview: Overview{Kind: "program", Title: filepath.Base(w.programRoot), Subtitle: w.programRoot, Packages: w.state.Program.Packages},
		Text:     TextView{Language: "text", Filename: w.programRoot},
	}}})
	w.state.Active = Selection{RowID: rowID, TileID: tileID}
	w.bump()
	return cloneState(w.state), nil
}

func (w *Workspace) OpenPackage(rowID, tileID, packageDirectory, packageName string) (State, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	pkg, err := w.packageLocked(packageDirectory, packageName)
	if err != nil {
		return State{}, err
	}
	return w.appendTileLocked(rowID, tileID, "package", "opens package", Target{Kind: "package", PackagePath: packageDirectory, PackageName: packageName}, Overview{Kind: "package", Title: "package " + packageName, Subtitle: packageDirectory, Files: pkg.Files}, TextView{Language: "go", Filename: packageDirectory})
}

// NavigatePackage replaces the current tile with a package in the same
// column. Use OpenPackage when the caller wants to preserve the current tile.
func (w *Workspace) NavigatePackage(rowID, tileID, packageDirectory, packageName string) (State, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	pkg, err := w.packageLocked(packageDirectory, packageName)
	if err != nil {
		return State{}, err
	}
	return w.replaceTileLocked(rowID, tileID, Target{Kind: "package", PackagePath: packageDirectory, PackageName: packageName}, Overview{Kind: "package", Title: "package " + packageName, Subtitle: packageDirectory, Files: pkg.Files}, TextView{Language: "go", Filename: packageDirectory})
}

// InspectFile loads a file into the current tile's text pane without changing
// the tile target or inquiry lineage.
func (w *Workspace) InspectFile(rowID, tileID, packageDirectory, packageName, filePath string) (State, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	pkg, err := w.packageLocked(packageDirectory, packageName)
	if err != nil {
		return State{}, err
	}
	for _, file := range pkg.Files {
		if file.Path != filePath {
			continue
		}
		source, readErr := os.ReadFile(filepath.Join(w.programRoot, filepath.FromSlash(file.Path)))
		if readErr != nil {
			return State{}, readErr
		}
		row, tile, tileErr := w.tileLocked(rowID, tileID)
		if tileErr != nil {
			return State{}, tileErr
		}
		tile.Text = textViewForFile(file, string(source))
		tile.Overview.SelectedDeclaration = nil
		tile.Overview.References = nil
		row.Tiles[tile.Column] = *tile
		w.state.Active = Selection{RowID: rowID, TileID: tileID}
		w.bump()
		return cloneState(w.state), nil
	}
	return State{}, fmt.Errorf("file %s was not found in package %s", filePath, packageName)
}

// InspectDeclaration loads its containing file and keeps the package tile in
// place. The declaration identity and occurrence ranges let clients select it.
func (w *Workspace) InspectDeclaration(rowID, tileID, packageDirectory, packageName, filePath, name string, line int) (State, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	pkg, err := w.packageLocked(packageDirectory, packageName)
	if err != nil {
		return State{}, err
	}
	for _, file := range pkg.Files {
		if file.Path != filePath {
			continue
		}
		declaration := findDeclaration(file.Declarations, name, line)
		if declaration == nil {
			return State{}, fmt.Errorf("declaration %s was not found in %s", name, filePath)
		}
		source, readErr := os.ReadFile(filepath.Join(w.programRoot, filepath.FromSlash(file.Path)))
		if readErr != nil {
			return State{}, readErr
		}
		row, tile, tileErr := w.tileLocked(rowID, tileID)
		if tileErr != nil {
			return State{}, tileErr
		}
		tile.Text = textViewForFile(file, string(source))
		tile.Overview.SelectedDeclaration = declaration
		tile.Overview.References = referencesForDeclaration(w.state.Program, packageDirectory, filePath, *declaration)
		row.Tiles[tile.Column] = *tile
		w.state.Active = Selection{RowID: rowID, TileID: tileID}
		w.bump()
		return cloneState(w.state), nil
	}
	return State{}, fmt.Errorf("file %s was not found in package %s", filePath, packageName)
}

func (w *Workspace) OpenFile(rowID, tileID, packageDirectory, packageName, filePath string) (State, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	pkg, err := w.packageLocked(packageDirectory, packageName)
	if err != nil {
		return State{}, err
	}
	for _, file := range pkg.Files {
		if file.Path != filePath {
			continue
		}
		source, readErr := os.ReadFile(filepath.Join(w.programRoot, filepath.FromSlash(file.Path)))
		if readErr != nil {
			return State{}, readErr
		}
		imports := localImportSummaries(w.programRoot, file, w.state.Program.Packages)
		return w.appendTileLocked(rowID, tileID, "file", "opens file", Target{Kind: "file", PackagePath: packageDirectory, PackageName: packageName, FilePath: filePath}, Overview{Kind: "file", Title: file.Name, Subtitle: file.Path, Declarations: file.Declarations, ImportPaths: file.Imports, Imports: imports}, textViewForFile(file, string(source)))
	}
	return State{}, fmt.Errorf("file %s was not found in package %s", filePath, packageName)
}

// NavigateFile replaces the current tile with a file in the same column.
func (w *Workspace) NavigateFile(rowID, tileID, packageDirectory, packageName, filePath string) (State, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	pkg, err := w.packageLocked(packageDirectory, packageName)
	if err != nil {
		return State{}, err
	}
	for _, file := range pkg.Files {
		if file.Path != filePath {
			continue
		}
		source, readErr := os.ReadFile(filepath.Join(w.programRoot, filepath.FromSlash(file.Path)))
		if readErr != nil {
			return State{}, readErr
		}
		imports := localImportSummaries(w.programRoot, file, w.state.Program.Packages)
		return w.replaceTileLocked(rowID, tileID, Target{Kind: "file", PackagePath: packageDirectory, PackageName: packageName, FilePath: filePath}, Overview{Kind: "file", Title: file.Name, Subtitle: file.Path, Declarations: file.Declarations, ImportPaths: file.Imports, Imports: imports}, textViewForFile(file, string(source)))
	}
	return State{}, fmt.Errorf("file %s was not found in package %s", filePath, packageName)
}

func (w *Workspace) OpenDeclaration(rowID, tileID, packageDirectory, packageName, filePath, name string, line int) (State, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	pkg, err := w.packageLocked(packageDirectory, packageName)
	if err != nil {
		return State{}, err
	}
	for _, file := range pkg.Files {
		if file.Path != filePath {
			continue
		}
		declaration := findDeclaration(file.Declarations, name, line)
		if declaration == nil {
			return State{}, fmt.Errorf("declaration %s was not found in %s", name, filePath)
		}
		source, readErr := os.ReadFile(filepath.Join(w.programRoot, filepath.FromSlash(file.Path)))
		if readErr != nil {
			return State{}, readErr
		}
		lines := strings.Split(string(source), "\n")
		start, end := declaration.Line-1, declaration.EndLine
		if start < 0 {
			start = 0
		}
		if end > len(lines) {
			end = len(lines)
		}
		content := ""
		if start <= end {
			content = strings.Join(lines[start:end], "\n")
		}
		return w.appendTileLocked(rowID, tileID, "declaration", "opens declaration", Target{Kind: "declaration", PackagePath: packageDirectory, PackageName: packageName, FilePath: filePath, DeclarationName: name, Line: declaration.Line, EndLine: declaration.EndLine}, Overview{Kind: "declaration", Title: name, Subtitle: declaration.Kind, Declarations: []explorer.Declaration{*declaration}, References: referencesForDeclaration(w.state.Program, packageDirectory, filePath, *declaration)}, textViewForDeclaration(file, declaration, content))
	}
	return State{}, fmt.Errorf("file %s was not found in package %s", filePath, packageName)
}

// NavigateDeclaration replaces the current tile with a declaration in the
// same column.
func (w *Workspace) NavigateDeclaration(rowID, tileID, packageDirectory, packageName, filePath, name string, line int) (State, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	pkg, err := w.packageLocked(packageDirectory, packageName)
	if err != nil {
		return State{}, err
	}
	for _, file := range pkg.Files {
		if file.Path != filePath {
			continue
		}
		declaration := findDeclaration(file.Declarations, name, line)
		if declaration == nil {
			return State{}, fmt.Errorf("declaration %s was not found in %s", name, filePath)
		}
		source, readErr := os.ReadFile(filepath.Join(w.programRoot, filepath.FromSlash(file.Path)))
		if readErr != nil {
			return State{}, readErr
		}
		lines := strings.Split(string(source), "\n")
		start, end := declaration.Line-1, declaration.EndLine
		if start < 0 {
			start = 0
		}
		if end > len(lines) {
			end = len(lines)
		}
		content := ""
		if start <= end {
			content = strings.Join(lines[start:end], "\n")
		}
		return w.replaceTileLocked(rowID, tileID, Target{Kind: "declaration", PackagePath: packageDirectory, PackageName: packageName, FilePath: filePath, DeclarationName: name, Line: declaration.Line, EndLine: declaration.EndLine}, Overview{Kind: "declaration", Title: name, Subtitle: declaration.Kind, Declarations: []explorer.Declaration{*declaration}, References: referencesForDeclaration(w.state.Program, packageDirectory, filePath, *declaration)}, textViewForDeclaration(file, declaration, content))
	}
	return State{}, fmt.Errorf("file %s was not found in package %s", filePath, packageName)
}

func (w *Workspace) SetPane(rowID, tileID, pane string, collapsed bool) (State, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	row, tile, err := w.tileLocked(rowID, tileID)
	if err != nil {
		return State{}, err
	}
	switch pane {
	case "overview":
		tile.Panes.OverviewCollapsed = collapsed
	case "text":
		tile.Panes.TextCollapsed = collapsed
	default:
		return State{}, fmt.Errorf("unknown pane %q", pane)
	}
	row.Tiles[tile.Column] = *tile
	w.state.Active = Selection{RowID: rowID, TileID: tileID}
	w.bump()
	return cloneState(w.state), nil
}

// SetTileCollapsed controls the visibility of a tile's contents without
// removing the tile from the inquiry lineage.
func (w *Workspace) SetTileCollapsed(rowID, tileID string, collapsed bool) (State, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	row, tile, err := w.tileLocked(rowID, tileID)
	if err != nil {
		return State{}, err
	}
	tile.Collapsed = collapsed
	row.Tiles[tile.Column] = *tile
	w.state.Active = Selection{RowID: rowID, TileID: tileID}
	w.bump()
	return cloneState(w.state), nil
}

// CloseColumn removes the selected column and everything opened after it in
// the same inquiry row. The preceding tile becomes active.
func (w *Workspace) CloseColumn(rowID, tileID string) (State, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	row, tile, err := w.tileLocked(rowID, tileID)
	if err != nil {
		return State{}, err
	}
	if tile.Column == 0 {
		return State{}, fmt.Errorf("the program column cannot be closed")
	}
	keep := tile.Column
	removed := make(map[string]bool)
	for _, candidate := range row.Tiles[keep:] {
		removed[candidate.ID] = true
	}
	row.Tiles = row.Tiles[:keep]
	for index := range row.Tiles {
		row.Tiles[index].Column = index
	}
	filteredEdges := row.Edges[:0]
	for _, edge := range row.Edges {
		if !removed[edge.FromTileID] && !removed[edge.ToTileID] {
			filteredEdges = append(filteredEdges, edge)
		}
	}
	row.Edges = filteredEdges
	active := row.Tiles[len(row.Tiles)-1]
	w.state.Active = Selection{RowID: rowID, TileID: active.ID}
	w.bump()
	return cloneState(w.state), nil
}

// CloseTile removes one tile and keeps the other columns in the inquiry.
func (w *Workspace) CloseTile(rowID, tileID string) (State, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	row, tile, err := w.tileLocked(rowID, tileID)
	if err != nil {
		return State{}, err
	}
	if len(row.Tiles) == 1 {
		return State{}, fmt.Errorf("the only inquiry tile cannot be closed")
	}
	removedID := tile.ID
	removedColumn := tile.Column
	delete(w.history, removedID)
	kept := make([]Tile, 0, len(row.Tiles)-1)
	for _, candidate := range row.Tiles {
		if candidate.ID != removedID {
			candidate.Column = len(kept)
			if candidate.OpenedBy != nil && candidate.OpenedBy.FromTileID == removedID {
				candidate.OpenedBy = nil
			}
			kept = append(kept, candidate)
		}
	}
	row.Tiles = kept
	filtered := row.Edges[:0]
	for _, edge := range row.Edges {
		if edge.FromTileID != removedID && edge.ToTileID != removedID {
			filtered = append(filtered, edge)
		}
	}
	row.Edges = filtered
	activeColumn := removedColumn
	if activeColumn >= len(row.Tiles) {
		activeColumn = len(row.Tiles) - 1
	}
	active := row.Tiles[activeColumn]
	w.state.Active = Selection{RowID: rowID, TileID: active.ID}
	w.bump()
	return cloneState(w.state), nil
}

// Back restores the previous target in the selected column.
func (w *Workspace) Back(rowID, tileID string) (State, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	_, tile, err := w.tileLocked(rowID, tileID)
	if err != nil {
		return State{}, err
	}
	stack := w.history[tileID]
	if len(stack) == 0 {
		return State{}, fmt.Errorf("tile %s has no previous target", tileID)
	}
	previous := stack[len(stack)-1]
	w.history[tileID] = stack[:len(stack)-1]
	previous.ID = tile.ID
	previous.Column = tile.Column
	previous.CanGoBack = len(w.history[tileID]) > 0
	*tile = previous
	w.state.Active = Selection{RowID: rowID, TileID: tileID}
	w.bump()
	return cloneState(w.state), nil
}

func (w *Workspace) packageLocked(directory, name string) (explorer.Package, error) {
	if w.state.Program == nil {
		return explorer.Package{}, fmt.Errorf("no program is open")
	}
	for _, pkg := range w.state.Program.Packages {
		if pkg.Directory == directory && pkg.Name == name {
			return pkg, nil
		}
	}
	return explorer.Package{}, fmt.Errorf("package %s in %s was not found", name, directory)
}

func (w *Workspace) appendTileLocked(rowID, fromTileID, kind, relation string, target Target, overview Overview, text TextView) (State, error) {
	row, from, err := w.tileLocked(rowID, fromTileID)
	if err != nil {
		return State{}, err
	}
	w.truncateDescendantsLocked(row, from.Column)
	tileID := w.id("tile")
	edgeID := w.id("edge")
	tile := Tile{ID: tileID, Column: len(row.Tiles), Target: target, OpenedBy: &EdgeRef{EdgeID: edgeID, Relationship: relation, FromTileID: from.ID}, Overview: overview, Text: text}
	for _, existing := range row.Tiles {
		if existing.Target == target {
			tile.PreviouslyOpened = true
			tile.ExistingTileID = existing.ID
			break
		}
	}
	row.Tiles = append(row.Tiles, tile)
	row.Edges = append(row.Edges, Edge{ID: edgeID, FromTileID: from.ID, ToTileID: tileID, Kind: kind, Label: relation})
	w.state.Active = Selection{RowID: rowID, TileID: tileID}
	w.bump()
	return cloneState(w.state), nil
}

func (w *Workspace) truncateDescendantsLocked(row *InquiryRow, column int) {
	if column+1 >= len(row.Tiles) {
		return
	}
	for _, tile := range row.Tiles[column+1:] {
		delete(w.history, tile.ID)
	}
	row.Tiles = row.Tiles[:column+1]
	filtered := row.Edges[:0]
	for _, edge := range row.Edges {
		if containsTile(row.Tiles, edge.FromTileID) && containsTile(row.Tiles, edge.ToTileID) {
			filtered = append(filtered, edge)
		}
	}
	row.Edges = filtered
}

func containsTile(tiles []Tile, id string) bool {
	for _, tile := range tiles {
		if tile.ID == id {
			return true
		}
	}
	return false
}

func referencesForDeclaration(program *ProgramState, packageDirectory, declarationFile string, declaration explorer.Declaration) []explorer.Reference {
	result := make([]explorer.Reference, 0)
	if program == nil {
		return result
	}
	targetSymbol := declaration.SymbolID
	if targetSymbol == "" {
		targetSymbol = program.Module
		if packageDirectory != "" {
			targetSymbol += "/" + packageDirectory
		}
		targetSymbol += "::" + declaration.Name
	}
	for _, pkg := range program.Packages {
		packagePath := program.Module
		if pkg.Directory != "" {
			packagePath += "/" + pkg.Directory
		}
		for _, file := range pkg.Files {
			for _, occurrence := range file.Occurrences {
				if occurrence.SymbolID != targetSymbol || (file.Path == declarationFile && occurrence.StartLine == declaration.Line) {
					continue
				}
				result = append(result, explorer.Reference{SymbolID: targetSymbol, Name: declaration.Name, Kind: declaration.Kind, PackagePath: packagePath, PackageName: pkg.Name, PackageDir: pkg.Directory, FilePath: file.Path, Declaration: declaration.Name, Line: declaration.Line, ReferenceLine: occurrence.StartLine})
			}
		}
	}
	return result
}

func (w *Workspace) replaceTileLocked(rowID, tileID string, target Target, overview Overview, text TextView) (State, error) {
	row, tile, err := w.tileLocked(rowID, tileID)
	if err != nil {
		return State{}, err
	}
	currentID := tile.ID
	column := tile.Column
	if w.history == nil {
		w.history = make(map[string][]Tile)
	}
	previous := cloneTile(*tile)
	previous.CanGoBack = len(w.history[currentID]) > 0
	w.history[currentID] = append(w.history[currentID], previous)
	*tile = Tile{ID: currentID, Column: column, Target: target, Overview: overview, Text: text}
	tile.CanGoBack = true
	for _, existing := range row.Tiles {
		if existing.ID != currentID && existing.Target == target {
			tile.PreviouslyOpened = true
			tile.ExistingTileID = existing.ID
			break
		}
	}
	w.state.Active = Selection{RowID: rowID, TileID: tileID}
	w.bump()
	return cloneState(w.state), nil
}

func (w *Workspace) tileLocked(rowID, tileID string) (*InquiryRow, *Tile, error) {
	for rowIndex := range w.state.Rows {
		row := &w.state.Rows[rowIndex]
		if row.ID != rowID {
			continue
		}
		for tileIndex := range row.Tiles {
			if row.Tiles[tileIndex].ID == tileID {
				return row, &row.Tiles[tileIndex], nil
			}
		}
		return nil, nil, fmt.Errorf("tile %s was not found", tileID)
	}
	return nil, nil, fmt.Errorf("inquiry row %s was not found", rowID)
}

func (w *Workspace) id(prefix string) string {
	w.nextID++
	return fmt.Sprintf("%s-%d", prefix, w.nextID)
}

func (w *Workspace) bump() { w.state.Revision++ }

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
			if packagePath != imported {
				continue
			}
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
	return result
}

func textViewForFile(file explorer.File, source string) TextView {
	return TextView{Language: "go", Filename: file.Path, Content: source, SourceStartLine: 1, Occurrences: append([]explorer.Occurrence(nil), file.Occurrences...)}
}

func textViewForDeclaration(file explorer.File, declaration *explorer.Declaration, source string) TextView {
	occurrences := make([]explorer.Occurrence, 0)
	if declaration != nil {
		for _, occurrence := range file.Occurrences {
			if occurrence.StartLine >= declaration.Line && occurrence.StartLine <= declaration.EndLine {
				occurrences = append(occurrences, occurrence)
			}
		}
	}
	return TextView{Language: "go", Filename: file.Path, Content: source, SourceStartLine: declaration.Line, Occurrences: occurrences}
}

func cloneState(state State) State {
	program := *state.Program
	program.Packages = clonePackages(state.Program.Packages)
	program.LocalImports = append([]string(nil), state.Program.LocalImports...)
	clone := State{Revision: state.Revision, Program: &program, Active: state.Active}
	clone.Rows = append([]InquiryRow(nil), state.Rows...)
	for rowIndex := range clone.Rows {
		clone.Rows[rowIndex].Tiles = make([]Tile, len(state.Rows[rowIndex].Tiles))
		for tileIndex, sourceTile := range state.Rows[rowIndex].Tiles {
			tile := sourceTile
			tile.Text.Occurrences = append([]explorer.Occurrence(nil), sourceTile.Text.Occurrences...)
			tile.Overview.Packages = clonePackages(sourceTile.Overview.Packages)
			tile.Overview.Files = cloneFiles(sourceTile.Overview.Files)
			tile.Overview.Declarations = cloneDeclarations(sourceTile.Overview.Declarations)
			tile.Overview.Imports = cloneImports(sourceTile.Overview.Imports)
			tile.Overview.References = append([]explorer.Reference(nil), sourceTile.Overview.References...)
			if sourceTile.Overview.SelectedDeclaration != nil {
				selected := *sourceTile.Overview.SelectedDeclaration
				tile.Overview.SelectedDeclaration = &selected
			}
			clone.Rows[rowIndex].Tiles[tileIndex] = tile
		}
		clone.Rows[rowIndex].Edges = append([]Edge(nil), state.Rows[rowIndex].Edges...)
		for tileIndex := range clone.Rows[rowIndex].Tiles {
			tile := &clone.Rows[rowIndex].Tiles[tileIndex]
			if tile.OpenedBy != nil {
				openedBy := *tile.OpenedBy
				tile.OpenedBy = &openedBy
			}
		}
	}
	return clone
}

func cloneTile(source Tile) Tile {
	clone := source
	clone.Text.Occurrences = append([]explorer.Occurrence(nil), source.Text.Occurrences...)
	clone.Overview.Packages = clonePackages(source.Overview.Packages)
	clone.Overview.Files = cloneFiles(source.Overview.Files)
	clone.Overview.Declarations = cloneDeclarations(source.Overview.Declarations)
	clone.Overview.Imports = cloneImports(source.Overview.Imports)
	clone.Overview.References = append([]explorer.Reference(nil), source.Overview.References...)
	if source.Overview.SelectedDeclaration != nil {
		selected := *source.Overview.SelectedDeclaration
		clone.Overview.SelectedDeclaration = &selected
	}
	if source.OpenedBy != nil {
		openedBy := *source.OpenedBy
		clone.OpenedBy = &openedBy
	}
	return clone
}

func clonePackages(packages []explorer.Package) []explorer.Package {
	clone := make([]explorer.Package, len(packages))
	for index, pkg := range packages {
		clone[index] = pkg
		clone[index].Files = make([]explorer.File, len(pkg.Files))
		for fileIndex, file := range pkg.Files {
			clone[index].Files[fileIndex] = file
			clone[index].Files[fileIndex].Imports = append([]string(nil), file.Imports...)
			clone[index].Files[fileIndex].Occurrences = append([]explorer.Occurrence(nil), file.Occurrences...)
			clone[index].Files[fileIndex].Declarations = cloneDeclarations(file.Declarations)
			clone[index].Files[fileIndex].SamePackageReferences = append([]explorer.Reference(nil), file.SamePackageReferences...)
		}
		clone[index].LocalImports = append([]string(nil), pkg.LocalImports...)
	}
	return clone
}

func cloneFiles(files []explorer.File) []explorer.File {
	clone := make([]explorer.File, len(files))
	for index, file := range files {
		clone[index] = file
		clone[index].Imports = append([]string(nil), file.Imports...)
		clone[index].Occurrences = append([]explorer.Occurrence(nil), file.Occurrences...)
		clone[index].Declarations = cloneDeclarations(file.Declarations)
		clone[index].SamePackageReferences = append([]explorer.Reference(nil), file.SamePackageReferences...)
	}
	return clone
}

func cloneImports(imports []explorer.ImportSummary) []explorer.ImportSummary {
	clone := make([]explorer.ImportSummary, len(imports))
	for index, imported := range imports {
		clone[index] = imported
		clone[index].Files = cloneFiles(imported.Files)
		clone[index].Declarations = cloneDeclarations(imported.Declarations)
	}
	return clone
}

func cloneDeclarations(declarations []explorer.Declaration) []explorer.Declaration {
	clone := make([]explorer.Declaration, len(declarations))
	for index, declaration := range declarations {
		clone[index] = declaration
		clone[index].Parameters = append([]explorer.Parameter(nil), declaration.Parameters...)
		clone[index].Results = append([]explorer.Parameter(nil), declaration.Results...)
		clone[index].TypeReferences = append([]explorer.Reference(nil), declaration.TypeReferences...)
		clone[index].Children = cloneDeclarations(declaration.Children)
	}
	return clone
}
