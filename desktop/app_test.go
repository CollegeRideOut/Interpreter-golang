package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenMainFileFromRootPackage(t *testing.T) {
	app := NewApp()
	packageSnapshot, err := app.OpenPackage("../TestProgram", "", "main")
	if err != nil {
		t.Fatal(err)
	}
	if len(packageSnapshot.Files) != 1 || packageSnapshot.Files[0].Path != "main.go" {
		t.Fatalf("files = %+v", packageSnapshot.Files)
	}
	fileSnapshot, err := app.OpenFile("../TestProgram", packageSnapshot.PackageDirectory, packageSnapshot.PackageName, packageSnapshot.Files[0].Path)
	if err != nil {
		t.Fatal(err)
	}
	if fileSnapshot.FileName != "main.go" || len(fileSnapshot.Declarations) != 1 || fileSnapshot.Declarations[0].Name != "main" {
		t.Fatalf("file snapshot = %+v", fileSnapshot)
	}
}

func TestGetCurrentStateReturnsInquiryRows(t *testing.T) {
	app := NewApp()
	if _, err := app.OpenProgram("../TestProgram"); err != nil {
		t.Fatal(err)
	}
	state, err := app.GetCurrentState()
	if err != nil {
		t.Fatal(err)
	}
	if state.Program == nil || len(state.Rows) != 1 || len(state.Rows[0].Tiles) != 1 {
		t.Fatalf("state = %+v", state)
	}
}

func TestGetRevisionContextListsDatedGitChoices(t *testing.T) {
	app := NewApp()
	context, err := app.GetRevisionContext("..")
	if err != nil {
		t.Fatal(err)
	}
	if context.Branch == "" || context.CurrentCommit == "" || len(context.Options) == 0 {
		t.Fatalf("revision context = %+v", context)
	}
	if context.Options[0].Hash == "" || context.Options[0].Date == "" || context.Options[0].Subject == "" {
		t.Fatalf("revision option = %+v", context.Options[0])
	}
}

func TestGetRevisionContextDoesNotUseAParentRepository(t *testing.T) {
	app := NewApp()
	root := t.TempDir()
	program := filepath.Join(root, "program")
	if err := os.Mkdir(program, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := app.GetRevisionContext(program); err == nil {
		t.Fatal("expected a non-repository program directory to have no revision context")
	}
}

func TestSelectRevisionLoadsTheProgramCommitWithoutCheckout(t *testing.T) {
	app := NewApp()
	context, err := app.GetRevisionContext("../TestProgramCalorieApp")
	if err != nil {
		t.Fatal(err)
	}
	state, err := app.SelectRevision("../TestProgramCalorieApp", context.CurrentCommit)
	if err != nil {
		t.Fatal(err)
	}
	if state.Program == nil || state.Program.Path == "" || len(state.Program.Packages) == 0 {
		t.Fatalf("selected revision state = %+v", state)
	}
}

func TestComparisonFileEditsCanBeAppliedAndRemoved(t *testing.T) {
	app := NewApp()
	edits, err := app.GetFileEdits("../TestProgram", "working-tree", "aaf2399", "", "main", "main.go")
	if err != nil {
		t.Fatal(err)
	}
	if len(edits) == 0 {
		t.Fatal("expected comparison edits")
	}
	if edits[0].Status != "unapplied" {
		t.Fatalf("initial status = %q", edits[0].Status)
	}
	applied, err := app.ApplyFileEdit("../TestProgram", "working-tree", "aaf2399", "", "main", "main.go", edits[0].Index)
	if err != nil {
		t.Fatal(err)
	}
	if applied.Edits[0].Status != "applied" {
		t.Fatalf("applied status = %q", applied.Edits[0].Status)
	}
	removed, err := app.RemoveFileEdit("../TestProgram", "working-tree", "aaf2399", "", "main", "main.go", edits[0].Index)
	if err != nil {
		t.Fatal(err)
	}
	if removed.Edits[0].Status != "removed" {
		t.Fatalf("removed status = %q", removed.Edits[0].Status)
	}
}

func TestProgramComparisonDiscoversChangedFiles(t *testing.T) {
	app := NewApp()
	if _, err := app.OpenProgram("../TestProgram"); err != nil {
		t.Fatal(err)
	}
	state, err := app.GetCurrentState()
	if err != nil {
		t.Fatal(err)
	}
	pkg := state.Rows[0].Tiles[0].Overview.Packages[0]
	files := pkg.Files
	if len(files) == 0 {
		t.Fatal("program tile has no files")
	}
	editState, err := app.GetFileEditState("../TestProgram", "working-tree", "aaf2399", pkg.Directory, pkg.Name, files[0].Path)
	if err != nil {
		t.Fatal(err)
	}
	if len(editState.Edits) == 0 {
		t.Fatalf("no edits for %s/%s/%s", pkg.Directory, pkg.Name, files[0].Path)
	}
}

func TestComparisonStateKeepsCanonicalAndLiftedEdits(t *testing.T) {
	app := NewApp()
	state, err := app.GetFileEditState("../TestProgram", "f0d86c9e9bb21d15ecbe78a4b1d42c4ea2785dc8", "3f2d37d", "", "main", "main.go")
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Edits) <= len(state.LiftedEdits) {
		t.Fatalf("canonical edits = %d, lifted edits = %d; expected the complete list", len(state.Edits), len(state.LiftedEdits))
	}
	foundEntertainment := false
	for _, edit := range state.Edits {
		if edit.Value == "entertainment" {
			foundEntertainment = true
			break
		}
	}
	if !foundEntertainment {
		t.Fatal("canonical comparison edits omitted the entertainment assignment")
	}
}
