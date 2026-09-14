package explorer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverPackagesReadsSourceWithoutTests(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"main.go":                      "package main\nfunc main() {}\n",
		"ignored_test.go":              "package main\n",
		filepath.Join("sub", "lib.go"): "package lib\ntype Controller struct{}\nfunc (controller *Controller) Run() {}\nfunc NewController() {}\n",
		filepath.Join("sub", "use.go"): "package lib\nfunc Use() Controller { return Controller{} }\n",
	}
	for path, content := range files {
		if err := os.WriteFile(filepath.Join(root, path), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	packages, err := DiscoverPackages(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(packages) != 2 || packages[0].Name != "main" || packages[0].Directory != "" || packages[0].FileCount != 1 {
		t.Fatalf("packages = %+v", packages)
	}
	if len(packages[0].Files) != 1 || packages[0].Files[0].Name != "main.go" {
		t.Fatalf("main files = %+v", packages[0].Files)
	}
	declarations := packages[0].Files[0].Declarations
	if len(declarations) != 1 || declarations[0].Kind != "function" || declarations[0].Name != "main" || declarations[0].Line != 2 {
		t.Fatalf("main declarations = %+v", declarations)
	}
	if packages[1].Name != "lib" || packages[1].Directory != "sub" {
		t.Fatalf("subpackage = %+v", packages[1])
	}
	if len(packages[1].Files[0].Declarations) != 2 || len(packages[1].Files[0].Declarations[0].Children) != 1 {
		t.Fatalf("grouped declarations = %+v", packages[1].Files[0].Declarations)
	}
	if packages[1].Files[0].Declarations[0].Children[0].Receiver != "*Controller" || !packages[1].Files[0].Declarations[1].Exported {
		t.Fatalf("method or export metadata = %+v", packages[1].Files[0].Declarations)
	}
	var use File
	for _, file := range packages[1].Files {
		if file.Name == "use.go" {
			use = file
		}
	}
	if len(use.SamePackageReferences) != 1 || use.SamePackageReferences[0].Name != "Controller" || use.SamePackageReferences[0].FilePath != "sub/lib.go" {
		t.Fatalf("same-package references = %+v", use.SamePackageReferences)
	}
}
