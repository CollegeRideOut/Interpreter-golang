package workspace

import "testing"

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
