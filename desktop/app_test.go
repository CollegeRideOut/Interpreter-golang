package main

import "testing"

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
