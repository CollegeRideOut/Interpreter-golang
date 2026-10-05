package workspace

import (
	"os"
	"path/filepath"
	"testing"

	"interpreter/explorer"
)

func TestInquiryRowsGrowIndependently(t *testing.T) {
	workspace := New()
	state, err := workspace.OpenProgram("../TestProgram")
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Rows) != 1 || len(state.Rows[0].Tiles) != 1 {
		t.Fatalf("initial state = %+v", state)
	}

	root := state.Rows[0].Tiles[0]
	state, err = workspace.OpenPackage(state.Rows[0].ID, root.ID, "", "main")
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Rows[0].Tiles) != 2 || state.Rows[0].Edges[0].ToTileID != state.Rows[0].Tiles[1].ID {
		t.Fatalf("grown row = %+v", state.Rows[0])
	}

	state, err = workspace.StartInquiry("second question")
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Rows) != 2 || len(state.Rows[1].Tiles) != 1 {
		t.Fatalf("new row = %+v", state.Rows)
	}

	current, err := workspace.CurrentState()
	if err != nil {
		t.Fatal(err)
	}
	if current.Revision != state.Revision || len(current.Rows) != 2 {
		t.Fatalf("current state = %+v", current)
	}
}

func TestOpenComparisonFileCreatesIndependentFileInquiry(t *testing.T) {
	workspace := New()
	state, err := workspace.OpenProgram("../TestProgram")
	if err != nil {
		t.Fatal(err)
	}
	file := explorer.File{Name: "main.tsx", Path: "src/main.tsx"}
	state, err = workspace.OpenComparisonFile("src/main.tsx", "src", "", file, "export function App() {}\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(state.Rows))
	}
	row := state.Rows[1]
	if row.Title != "src/main.tsx" || row.Tiles[0].Target.Kind != "file" || row.Tiles[0].Text.Content == "" {
		t.Fatalf("comparison inquiry = %+v", row)
	}
}

func TestComparisonFileNavigationUsesActiveTileMetadata(t *testing.T) {
	root := t.TempDir()
	filePath := filepath.Join(root, "server", "src", "index.ts")
	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filePath, []byte("const app = express();\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	workspace := New()
	state, err := workspace.OpenProgram(root)
	if err != nil {
		t.Fatal(err)
	}
	file := explorer.File{
		Name:         "index.ts",
		Path:         "server/src/index.ts",
		Declarations: []explorer.Declaration{{Kind: "const", Name: "app", Line: 1, EndLine: 1}},
	}
	state, err = workspace.OpenComparisonFile(file.Path, "server/src", "src", file, "const app = express();\n")
	if err != nil {
		t.Fatal(err)
	}
	row := state.Rows[1]
	if err := os.RemoveAll(workspace.programRoot); err != nil {
		t.Fatal(err)
	}
	state, err = workspace.OpenDeclaration(row.ID, row.Tiles[0].ID, "server/src", "src", file.Path, "app", 1)
	if err != nil {
		t.Fatalf("open declaration from comparison file: %v", err)
	}
	if len(state.Rows[1].Tiles) != 2 || state.Rows[1].Tiles[1].Target.DeclarationName != "app" {
		t.Fatalf("comparison navigation state = %+v", state.Rows[1])
	}
}

func TestTilePanesAreIndependent(t *testing.T) {
	workspace := New()
	state, err := workspace.OpenProgram("../TestProgram")
	if err != nil {
		t.Fatal(err)
	}
	tile := state.Rows[0].Tiles[0]
	state, err = workspace.SetPane(state.Rows[0].ID, tile.ID, "overview", true)
	if err != nil {
		t.Fatal(err)
	}
	if !state.Rows[0].Tiles[0].Panes.OverviewCollapsed || state.Rows[0].Tiles[0].Panes.TextCollapsed {
		t.Fatalf("pane state = %+v", state.Rows[0].Tiles[0].Panes)
	}
}

func TestTileCanCollapseWithoutLeavingTheInquiry(t *testing.T) {
	workspace := New()
	state, err := workspace.OpenProgram("../TestProgram")
	if err != nil {
		t.Fatal(err)
	}
	tile := state.Rows[0].Tiles[0]
	state, err = workspace.SetTileCollapsed(state.Rows[0].ID, tile.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	if !state.Rows[0].Tiles[0].Collapsed || state.Active.TileID != tile.ID {
		t.Fatalf("collapsed tile state = %+v", state.Rows[0].Tiles[0])
	}
}

func TestRepeatedTargetIsMarkedWithoutHidingTheLineage(t *testing.T) {
	workspace := New()
	state, err := workspace.OpenProgram("../TestProgram")
	if err != nil {
		t.Fatal(err)
	}
	root := state.Rows[0].Tiles[0]
	state, err = workspace.OpenPackage(state.Rows[0].ID, root.ID, "", "main")
	if err != nil {
		t.Fatal(err)
	}
	packageTile := state.Rows[0].Tiles[1]
	state, err = workspace.OpenPackage(state.Rows[0].ID, packageTile.ID, "", "main")
	if err != nil {
		t.Fatal(err)
	}
	repeated := state.Rows[0].Tiles[2]
	if !repeated.PreviouslyOpened || repeated.ExistingTileID != packageTile.ID {
		t.Fatalf("repeated tile = %+v", repeated)
	}
}

func TestNavigationReplacesTheCurrentColumn(t *testing.T) {
	workspace := New()
	state, err := workspace.OpenProgram("../TestProgram")
	if err != nil {
		t.Fatal(err)
	}
	row := state.Rows[0]
	state, err = workspace.NavigatePackage(row.ID, row.Tiles[0].ID, "", "main")
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Rows[0].Tiles) != 1 || state.Rows[0].Tiles[0].Target.Kind != "package" || state.Rows[0].Tiles[0].Column != 0 {
		t.Fatalf("navigated state = %+v", state.Rows[0])
	}
}

func TestBackRestoresNavigationHistory(t *testing.T) {
	workspace := New()
	state, err := workspace.OpenProgram("../TestProgram")
	if err != nil {
		t.Fatal(err)
	}
	row := state.Rows[0]
	state, err = workspace.NavigatePackage(row.ID, row.Tiles[0].ID, "", "main")
	if err != nil {
		t.Fatal(err)
	}
	if !state.Rows[0].Tiles[0].CanGoBack {
		t.Fatal("navigated tile should be able to go back")
	}

	state, err = workspace.Back(row.ID, row.Tiles[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if state.Rows[0].Tiles[0].Target.Kind != "program" || state.Rows[0].Tiles[0].CanGoBack {
		t.Fatalf("back state = %+v", state.Rows[0].Tiles[0])
	}
}

func TestOpeningFromAnAncestorTruncatesDescendants(t *testing.T) {
	workspace := New()
	state, err := workspace.OpenProgram("../TestProgram")
	if err != nil {
		t.Fatal(err)
	}
	row := state.Rows[0]
	state, err = workspace.OpenPackage(row.ID, row.Tiles[0].ID, "", "main")
	if err != nil {
		t.Fatal(err)
	}
	state, err = workspace.OpenFile(row.ID, state.Rows[0].Tiles[1].ID, "", "main", "main.go")
	if err != nil {
		t.Fatal(err)
	}
	root := state.Rows[0].Tiles[0]
	state, err = workspace.OpenFile(row.ID, root.ID, "", "main", "main.go")
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Rows[0].Tiles) != 2 || state.Rows[0].Tiles[1].Target.Kind != "file" || state.Rows[0].Tiles[1].Column != 1 {
		t.Fatalf("branched state = %+v", state.Rows[0])
	}
}

func TestClosingTheLeftmostTileKeepsTheRemainingColumns(t *testing.T) {
	workspace := New()
	state, err := workspace.OpenProgram("../TestProgram")
	if err != nil {
		t.Fatal(err)
	}
	row := state.Rows[0]
	state, err = workspace.OpenPackage(row.ID, row.Tiles[0].ID, "", "main")
	if err != nil {
		t.Fatal(err)
	}
	leftID := state.Rows[0].Tiles[0].ID
	state, err = workspace.CloseTile(row.ID, leftID)
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Rows[0].Tiles) != 1 || state.Rows[0].Tiles[0].Column != 0 || state.Active.TileID != state.Rows[0].Tiles[0].ID {
		t.Fatalf("closed leftmost state = %+v", state.Rows[0])
	}
}

func TestDeclarationOverviewIncludesReferences(t *testing.T) {
	workspace := New()
	state, err := workspace.OpenProgram("../TestProgramCalorieApp")
	if err != nil {
		t.Fatal(err)
	}
	row := state.Rows[0]
	state, err = workspace.OpenDeclaration(row.ID, row.Tiles[0].ID, "controllers", "controllers", "controllers/calories.go", "CaloriesController", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Rows[0].Tiles[1].Overview.References) == 0 {
		t.Fatal("declaration overview has no references")
	}
	reference := state.Rows[0].Tiles[1].Overview.References[0]
	if reference.PackageName == "" || reference.PackageDir != "controllers" {
		t.Fatalf("reference package metadata = %+v", reference)
	}
}

func TestOpenDeclarationLeftInsertsBeforeFirstColumn(t *testing.T) {
	workspace := New()
	state, err := workspace.OpenProgram("../TestProgramCalorieApp")
	if err != nil {
		t.Fatal(err)
	}
	row := state.Rows[0]
	state, err = workspace.OpenDeclarationLeft(row.ID, row.Tiles[0].ID, "controllers", "controllers", "controllers/calories.go", "CaloriesController", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Rows[0].Tiles) != 2 || state.Rows[0].Tiles[0].Target.Kind != "declaration" || state.Rows[0].Tiles[0].Column != 0 || state.Rows[0].Tiles[1].Column != 1 {
		t.Fatalf("left declaration state = %+v", state.Rows[0].Tiles)
	}
}

func TestFileOverviewIncludesAllImportsAndLocalImportMetadata(t *testing.T) {
	workspace := New()
	state, err := workspace.OpenProgram("../TestProgramCalorieApp")
	if err != nil {
		t.Fatal(err)
	}
	root := state.Rows[0].Tiles[0]
	state, err = workspace.OpenFile(state.Rows[0].ID, root.ID, "server", "server", "server/server.go")
	if err != nil {
		t.Fatal(err)
	}
	overview := state.Rows[0].Tiles[1].Overview
	if len(overview.ImportPaths) != 2 || overview.ImportPaths[0] != "calorieapp/controllers" || overview.ImportPaths[1] != "net/http" {
		t.Fatalf("import paths = %+v", overview.ImportPaths)
	}
	if len(overview.Imports) != 1 || overview.Imports[0].Path != "calorieapp/controllers" {
		t.Fatalf("local import summaries = %+v", overview.Imports)
	}
}

func TestInspectFileKeepsPackageTileAndUpdatesItsSource(t *testing.T) {
	workspace := New()
	state, err := workspace.OpenProgram("../TestProgramCalorieApp")
	if err != nil {
		t.Fatal(err)
	}
	row := state.Rows[0]
	state, err = workspace.OpenPackage(row.ID, row.Tiles[0].ID, "server", "server")
	if err != nil {
		t.Fatal(err)
	}
	packageTile := state.Rows[0].Tiles[1]
	state, err = workspace.InspectFile(row.ID, packageTile.ID, "server", "server", "server/server.go")
	if err != nil {
		t.Fatal(err)
	}
	inspected := state.Rows[0].Tiles[1]
	if len(state.Rows[0].Tiles) != 2 || inspected.Target.Kind != "package" || inspected.Text.Filename != "server/server.go" || inspected.Text.Content == "" {
		t.Fatalf("inspected package tile = %+v", inspected)
	}
}

func TestCloseColumnTruncatesTheRightHandLineage(t *testing.T) {
	workspace := New()
	state, err := workspace.OpenProgram("../TestProgram")
	if err != nil {
		t.Fatal(err)
	}
	row := state.Rows[0]
	state, err = workspace.OpenPackage(row.ID, row.Tiles[0].ID, "", "main")
	if err != nil {
		t.Fatal(err)
	}
	row = state.Rows[0]
	state, err = workspace.OpenFile(row.ID, row.Tiles[1].ID, "", "main", "main.go")
	if err != nil {
		t.Fatal(err)
	}
	row = state.Rows[0]
	state, err = workspace.CloseColumn(row.ID, row.Tiles[1].ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Rows[0].Tiles) != 1 || state.Rows[0].Tiles[0].Column != 0 {
		t.Fatalf("closed state = %+v", state.Rows[0])
	}
	if state.Active.TileID != state.Rows[0].Tiles[0].ID {
		t.Fatalf("active selection = %+v", state.Active)
	}
}

func TestCurrentStateDoesNotShareNestedProgramData(t *testing.T) {
	workspace := New()
	state, err := workspace.OpenProgram("../TestProgram")
	if err != nil {
		t.Fatal(err)
	}
	state.Program.Packages[0].Files[0].Name = "changed outside workspace"
	current, err := workspace.CurrentState()
	if err != nil {
		t.Fatal(err)
	}
	if current.Program.Packages[0].Files[0].Name == "changed outside workspace" {
		t.Fatal("workspace state shares nested package data")
	}
}

func TestOpenProgramSupportsTypeScriptProjects(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(`{"name":"ticket-api"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "src", "main.ts"), []byte("export const main = true;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "src", "App.tsx"), []byte("export function App() { return <main />; }\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	state, err := New().OpenProgram(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Program.Packages) != 1 || state.Program.Packages[0].Name != "ticket-api" {
		t.Fatalf("program = %+v", state.Program)
	}
	if len(state.Program.Packages[0].Children) != 1 || state.Program.Packages[0].Children[0].Name != "src" {
		t.Fatalf("project tree = %+v", state.Program.Packages[0])
	}
	workspace := New()
	opened, err := workspace.OpenProgram(root)
	if err != nil {
		t.Fatal(err)
	}
	state, err = workspace.OpenPackage(opened.Rows[0].ID, opened.Rows[0].Tiles[0].ID, "src", "src")
	if err != nil || len(state.Rows[0].Tiles) != 2 || len(state.Rows[0].Tiles[1].Overview.Files) != 2 {
		t.Fatalf("opened folder = %+v, err = %v", state, err)
	}
}
