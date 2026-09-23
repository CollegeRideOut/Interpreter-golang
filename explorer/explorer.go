package explorer

import (
	"fmt"
	goAst "go/ast"
	"go/format"
	goParser "go/parser"
	goToken "go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Package describes one Go package found in a working tree. It intentionally
// stops at package level; files and declarations are future explorer views.
type Package struct {
	Name         string   `json:"name"`
	Directory    string   `json:"directory"`
	FileCount    int      `json:"fileCount"`
	Files        []File   `json:"files,omitempty"`
	LocalImports []string `json:"localImports,omitempty"`
}

type File struct {
	Name                  string        `json:"name"`
	Path                  string        `json:"path"`
	Declarations          []Declaration `json:"declarations,omitempty"`
	Imports               []string      `json:"imports,omitempty"`
	Occurrences           []Occurrence  `json:"occurrences,omitempty"`
	SamePackageReferences []Reference   `json:"samePackageReferences,omitempty"`
}

type Occurrence struct {
	SymbolID    string `json:"symbolId"`
	Name        string `json:"name"`
	StartLine   int    `json:"startLine"`
	StartColumn int    `json:"startColumn"`
	EndLine     int    `json:"endLine"`
	EndColumn   int    `json:"endColumn"`
}

type Reference struct {
	SymbolID      string `json:"symbolId,omitempty"`
	Name          string `json:"name"`
	Kind          string `json:"kind"`
	PackagePath   string `json:"packagePath"`
	PackageName   string `json:"packageName,omitempty"`
	PackageDir    string `json:"packageDirectory,omitempty"`
	FilePath      string `json:"filePath"`
	Declaration   string `json:"declaration"`
	Line          int    `json:"line"`
	ReferenceLine int    `json:"referenceLine,omitempty"`
}

type ImportSummary struct {
	Path         string        `json:"path"`
	Name         string        `json:"name"`
	Directory    string        `json:"directory"`
	Files        []File        `json:"files,omitempty"`
	Declarations []Declaration `json:"declarations,omitempty"`
}

type Declaration struct {
	SymbolID       string        `json:"symbolId,omitempty"`
	Kind           string        `json:"kind"`
	Name           string        `json:"name"`
	Receiver       string        `json:"receiver,omitempty"`
	Line           int           `json:"line"`
	EndLine        int           `json:"endLine"`
	Exported       bool          `json:"exported"`
	Parameters     []Parameter   `json:"parameters,omitempty"`
	Results        []Parameter   `json:"results,omitempty"`
	Type           string        `json:"type,omitempty"`
	TypeReferences []Reference   `json:"typeReferences,omitempty"`
	Children       []Declaration `json:"children,omitempty"`
}

type Parameter struct {
	Name string `json:"name,omitempty"`
	Type string `json:"type"`
}

// DiscoverPackages reads Go source files without compiling or type checking.
func DiscoverPackages(root string) ([]Package, error) {
	type packageFiles struct {
		name    string
		files   []File
		imports map[string]bool
	}
	found := make(map[string]packageFiles)
	module := modulePath(root)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if path != root && (entry.Name() == "vendor" || strings.HasPrefix(entry.Name(), ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		fileSet := goToken.NewFileSet()
		file, err := goParser.ParseFile(fileSet, path, nil, 0)
		if err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
		directory, err := filepath.Rel(root, filepath.Dir(path))
		if err != nil {
			return err
		}
		if directory == "." {
			directory = ""
		}
		key := filepath.ToSlash(directory) + "\x00" + file.Name.Name
		item := found[key]
		item.name = file.Name.Name
		if item.imports == nil {
			item.imports = make(map[string]bool)
		}
		for _, imported := range file.Imports {
			if module != "" {
				path, err := strconv.Unquote(imported.Path.Value)
				if err == nil && strings.HasPrefix(path, module+"/") {
					item.imports[path] = true
				}
			}
		}
		imports := make([]string, 0, len(file.Imports))
		for _, imported := range file.Imports {
			if importPath, err := strconv.Unquote(imported.Path.Value); err == nil {
				imports = append(imports, importPath)
			}
		}
		sort.Strings(imports)
		item.files = append(item.files, File{Name: filepath.Base(path), Path: filepath.ToSlash(filepath.Join(directory, filepath.Base(path))), Declarations: declarations(file, fileSet), Imports: imports})
		found[key] = item
		return nil
	})
	if err != nil {
		return nil, err
	}
	packages := make([]Package, 0, len(found))
	for key, item := range found {
		directory := strings.SplitN(key, "\x00", 2)[0]
		sort.Slice(item.files, func(i, j int) bool { return item.files[i].Name < item.files[j].Name })
		imports := make([]string, 0, len(item.imports))
		for imported := range item.imports {
			imports = append(imports, imported)
		}
		sort.Strings(imports)
		pkg := Package{Name: item.name, Directory: directory, FileCount: len(item.files), Files: item.files, LocalImports: imports}
		annotateReferences(root, module, &pkg)
		packages = append(packages, pkg)
	}
	sort.Slice(packages, func(i, j int) bool {
		if packages[i].Name == "main" && packages[j].Name != "main" {
			return true
		}
		if packages[i].Name != "main" && packages[j].Name == "main" {
			return false
		}
		return packages[i].Directory < packages[j].Directory
	})
	for index := range packages {
		assignSymbolIDs(module, &packages[index])
	}
	for index := range packages {
		annotateOccurrences(root, module, packages, &packages[index])
	}
	return packages, nil
}

func packagePath(module string, pkg Package) string {
	if pkg.Directory == "" {
		return module
	}
	return module + "/" + filepath.ToSlash(pkg.Directory)
}

func assignSymbolIDs(module string, pkg *Package) {
	path := packagePath(module, *pkg)
	for fileIndex := range pkg.Files {
		assignDeclarationIDs(path, pkg.Files[fileIndex].Declarations, "")
	}
}

func assignDeclarationIDs(packagePath string, declarations []Declaration, parent string) {
	for index := range declarations {
		declaration := &declarations[index]
		declaration.SymbolID = packagePath + "::"
		if parent != "" {
			declaration.SymbolID += parent + "."
		}
		declaration.SymbolID += declaration.Name
		assignDeclarationIDs(packagePath, declaration.Children, declaration.SymbolID)
	}
}

func annotateOccurrences(root, module string, packages []Package, pkg *Package) {
	packageSymbols := make(map[string]string)
	for _, file := range pkg.Files {
		for _, declaration := range file.Declarations {
			collectDeclarationSymbols(declaration, packageSymbols)
		}
	}
	for index := range pkg.Files {
		file := &pkg.Files[index]
		path := filepath.Join(root, filepath.FromSlash(file.Path))
		fileSet := goToken.NewFileSet()
		parsed, err := goParser.ParseFile(fileSet, path, nil, 0)
		if err != nil {
			continue
		}
		file.Occurrences = occurrencesForFile(parsed, fileSet, packageSymbols)
	}
}

func collectDeclarationSymbols(declaration Declaration, symbols map[string]string) {
	symbols[declaration.Name] = declaration.SymbolID
	for _, child := range declaration.Children {
		collectDeclarationSymbols(child, symbols)
	}
}

func occurrencesForFile(file *goAst.File, fileSet *goToken.FileSet, packageSymbols map[string]string) []Occurrence {
	declarationSymbols := make(map[goToken.Pos]string)
	for _, declaration := range file.Decls {
		collectDeclarationPositions(declaration, declarationSymbols, packageSymbols)
	}
	imports := make(map[string]string)
	for _, imported := range file.Imports {
		path, err := strconv.Unquote(imported.Path.Value)
		if err != nil {
			continue
		}
		name := filepath.Base(path)
		if imported.Name != nil {
			name = imported.Name.Name
		}
		if name != "_" && name != "." {
			imports[name] = path
		}
	}
	occurrences := make([]Occurrence, 0)
	goAst.Inspect(file, func(node goAst.Node) bool {
		identifier, ok := node.(*goAst.Ident)
		if !ok {
			return true
		}
		if symbolID, exists := declarationSymbols[identifier.Pos()]; exists {
			occurrences = append(occurrences, occurrence(fileSet, identifier, symbolID))
			return true
		}
		if selector, isSelector := parentSelector(file, identifier); isSelector && selector.Sel == identifier {
			if packageIdent, isPackage := selector.X.(*goAst.Ident); isPackage {
				if importedPath, exists := imports[packageIdent.Name]; exists {
					occurrences = append(occurrences, occurrence(fileSet, identifier, importedPath+"::"+identifier.Name))
				}
				return true
			}
		}
		if symbolID, exists := packageSymbols[identifier.Name]; exists {
			occurrences = append(occurrences, occurrence(fileSet, identifier, symbolID))
		}
		return true
	})
	return occurrences
}

func collectDeclarationPositions(node goAst.Node, positions map[goToken.Pos]string, symbols map[string]string) {
	switch declaration := node.(type) {
	case *goAst.FuncDecl:
		if symbolID, exists := symbols[declaration.Name.Name]; exists {
			positions[declaration.Name.Pos()] = symbolID
		}
	case *goAst.GenDecl:
		for _, specification := range declaration.Specs {
			switch specification := specification.(type) {
			case *goAst.TypeSpec:
				if symbolID, exists := symbols[specification.Name.Name]; exists {
					positions[specification.Name.Pos()] = symbolID
				}
			case *goAst.ValueSpec:
				for _, name := range specification.Names {
					if symbolID, exists := symbols[name.Name]; exists {
						positions[name.Pos()] = symbolID
					}
				}
			}
		}
	}
}

func parentSelector(root goAst.Node, identifier *goAst.Ident) (*goAst.SelectorExpr, bool) {
	var result *goAst.SelectorExpr
	goAst.Inspect(root, func(node goAst.Node) bool {
		selector, ok := node.(*goAst.SelectorExpr)
		if ok && selector.Sel == identifier {
			result = selector
			return false
		}
		return result == nil
	})
	return result, result != nil
}

func occurrence(fileSet *goToken.FileSet, identifier *goAst.Ident, symbolID string) Occurrence {
	start := fileSet.Position(identifier.Pos())
	end := fileSet.Position(identifier.End())
	return Occurrence{SymbolID: symbolID, Name: identifier.Name, StartLine: start.Line, StartColumn: start.Column, EndLine: end.Line, EndColumn: end.Column}
}

func annotateReferences(root, module string, pkg *Package) {
	type symbol struct {
		file string
		decl string
		kind string
		line int
	}
	symbols := make(map[string]symbol)
	type parsedFile struct {
		fileSet *goToken.FileSet
		file    *goAst.File
	}
	asts := make(map[string]parsedFile)
	for index := range pkg.Files {
		file := &pkg.Files[index]
		path := filepath.Join(root, filepath.FromSlash(file.Path))
		set := goToken.NewFileSet()
		parsed, err := goParser.ParseFile(set, path, nil, 0)
		if err != nil {
			continue
		}
		asts[file.Path] = parsedFile{fileSet: set, file: parsed}
		for _, declaration := range file.Declarations {
			symbols[declaration.Name] = symbol{file: file.Path, decl: declaration.Name, kind: declaration.Kind, line: declaration.Line}
			for _, child := range declaration.Children {
				symbols[child.Name] = symbol{file: file.Path, decl: child.Name, kind: child.Kind, line: child.Line}
			}
		}
	}
	packagePath := module
	if pkg.Directory != "" {
		packagePath += "/" + filepath.ToSlash(pkg.Directory)
	}
	for index := range pkg.Files {
		file := &pkg.Files[index]
		parsed := asts[file.Path]
		if parsed.file == nil {
			continue
		}
		declarationPositions := make(map[goToken.Pos]bool)
		for _, declaration := range parsed.file.Decls {
			switch declaration := declaration.(type) {
			case *goAst.FuncDecl:
				declarationPositions[declaration.Name.Pos()] = true
			case *goAst.GenDecl:
				for _, specification := range declaration.Specs {
					switch specification := specification.(type) {
					case *goAst.TypeSpec:
						declarationPositions[specification.Name.Pos()] = true
					case *goAst.ValueSpec:
						for _, name := range specification.Names {
							declarationPositions[name.Pos()] = true
						}
					}
				}
			}
		}
		seen := make(map[string]bool)
		goAst.Inspect(parsed.file, func(node goAst.Node) bool {
			identifier, ok := node.(*goAst.Ident)
			if !ok || declarationPositions[identifier.Pos()] {
				return true
			}
			target, ok := symbols[identifier.Name]
			if !ok || seen[target.file+"\x00"+target.decl] {
				return true
			}
			seen[target.file+"\x00"+target.decl] = true
			position := parsed.fileSet.Position(identifier.Pos())
			file.SamePackageReferences = append(file.SamePackageReferences, Reference{Name: identifier.Name, Kind: target.kind, PackagePath: packagePath, FilePath: target.file, Declaration: target.decl, Line: target.line, ReferenceLine: position.Line})
			return true
		})
		for _, targetDeclaration := range declarationTypeExpressions(parsed.file.Decls, parsed.fileSet) {
			name, line, expressions := targetDeclaration.name, targetDeclaration.line, targetDeclaration.expressions
			for _, expression := range expressions {
				goAst.Inspect(expression, func(node goAst.Node) bool {
					identifier, ok := node.(*goAst.Ident)
					if !ok {
						return true
					}
					target, ok := symbols[identifier.Name]
					if !ok || (target.kind != "type" && target.kind != "struct") {
						return true
					}
					for declarationIndex := range file.Declarations {
						candidate := &file.Declarations[declarationIndex]
						if candidate.Name == name && candidate.Line == line {
							candidate.TypeReferences = append(candidate.TypeReferences, Reference{Name: identifier.Name, Kind: "type", PackagePath: packagePath, FilePath: target.file, Declaration: target.decl, Line: target.line})
						}
					}
					return true
				})
			}
		}
		sort.Slice(file.SamePackageReferences, func(i, j int) bool {
			return file.SamePackageReferences[i].FilePath < file.SamePackageReferences[j].FilePath
		})
	}
}

type declarationTypeTarget struct {
	name        string
	line        int
	expressions []goAst.Expr
}

func declarationTypeExpressions(declarations []goAst.Decl, fileSet *goToken.FileSet) []declarationTypeTarget {
	result := make([]declarationTypeTarget, 0)
	for _, syntax := range declarations {
		switch declaration := syntax.(type) {
		case *goAst.FuncDecl:
			expressions := make([]goAst.Expr, 0)
			if declaration.Recv != nil {
				for _, field := range declaration.Recv.List {
					expressions = append(expressions, field.Type)
				}
			}
			if declaration.Type.Params != nil {
				for _, field := range declaration.Type.Params.List {
					expressions = append(expressions, field.Type)
				}
			}
			if declaration.Type.Results != nil {
				for _, field := range declaration.Type.Results.List {
					expressions = append(expressions, field.Type)
				}
			}
			result = append(result, declarationTypeTarget{name: declaration.Name.Name, line: fileSet.Position(declaration.Pos()).Line, expressions: expressions})
		case *goAst.GenDecl:
			for _, specification := range declaration.Specs {
				switch specification := specification.(type) {
				case *goAst.TypeSpec:
					result = append(result, declarationTypeTarget{name: specification.Name.Name, line: fileSet.Position(specification.Pos()).Line, expressions: []goAst.Expr{specification.Type}})
				case *goAst.ValueSpec:
					for _, name := range specification.Names {
						result = append(result, declarationTypeTarget{name: name.Name, line: fileSet.Position(specification.Pos()).Line, expressions: []goAst.Expr{specification.Type}})
					}
				}
			}
		}
	}
	return result
}

func ModulePath(root string) string {
	return modulePath(root)
}

func modulePath(root string) string {
	contents, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(contents), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "module" {
			return fields[1]
		}
	}
	return ""
}

func declarations(file *goAst.File, fileSet *goToken.FileSet) []Declaration {
	result := make([]Declaration, 0)
	types := make(map[string]int)
	methods := make([]Declaration, 0)
	for _, declaration := range file.Decls {
		line := fileSet.Position(declaration.Pos()).Line
		endLine := fileSet.Position(declaration.End()).Line
		switch declaration := declaration.(type) {
		case *goAst.FuncDecl:
			kind := "function"
			receiver := ""
			if declaration.Recv != nil && len(declaration.Recv.List) > 0 {
				kind = "method"
				receiver = formatReceiver(declaration.Recv.List[0].Type)
			}
			method := Declaration{Kind: kind, Name: declaration.Name.Name, Receiver: receiver, Line: line, EndLine: endLine, Exported: goAst.IsExported(declaration.Name.Name), Parameters: fieldParameters(declaration.Type.Params, fileSet), Results: fieldParameters(declaration.Type.Results, fileSet)}
			if kind == "method" {
				methods = append(methods, method)
			} else {
				result = append(result, method)
			}
		case *goAst.GenDecl:
			kind := strings.ToLower(declaration.Tok.String())
			for _, specification := range declaration.Specs {
				specLine := fileSet.Position(specification.Pos()).Line
				specEndLine := fileSet.Position(specification.End()).Line
				switch specification := specification.(type) {
				case *goAst.TypeSpec:
					declarationKind := kind
					if _, ok := specification.Type.(*goAst.StructType); ok {
						declarationKind = "struct"
					}
					declaration := Declaration{Kind: declarationKind, Name: specification.Name.Name, Line: specLine, EndLine: specEndLine, Exported: goAst.IsExported(specification.Name.Name), Type: expressionString(specification.Type)}
					types[specification.Name.Name] = len(result)
					result = append(result, declaration)
				case *goAst.ValueSpec:
					for _, name := range specification.Names {
						result = append(result, Declaration{Kind: kind, Name: name.Name, Line: specLine, EndLine: specEndLine, Exported: goAst.IsExported(name.Name), Type: expressionString(specification.Type)})
					}
				}
			}
		}
	}
	for _, method := range methods {
		if index, ok := types[strings.TrimPrefix(method.Receiver, "*")]; ok {
			result[index].Children = append(result[index].Children, method)
		} else {
			result = append(result, method)
		}
	}
	for index := range result {
		sort.SliceStable(result[index].Children, func(i, j int) bool { return result[index].Children[i].Line < result[index].Children[j].Line })
	}
	return result
}

func fieldParameters(fields *goAst.FieldList, fileSet *goToken.FileSet) []Parameter {
	if fields == nil {
		return nil
	}
	result := make([]Parameter, 0)
	for _, field := range fields.List {
		typeName := expressionString(field.Type)
		if len(field.Names) == 0 {
			result = append(result, Parameter{Type: typeName})
			continue
		}
		for _, name := range field.Names {
			result = append(result, Parameter{Name: name.Name, Type: typeName})
		}
	}
	return result
}

func expressionString(expression goAst.Expr) string {
	if expression == nil {
		return ""
	}
	var output strings.Builder
	if err := format.Node(&output, goToken.NewFileSet(), expression); err != nil {
		return ""
	}
	return output.String()
}

func formatReceiver(expression goAst.Expr) string {
	var receiver string
	if identifier, ok := expression.(*goAst.Ident); ok {
		receiver = identifier.Name
	}
	if pointer, ok := expression.(*goAst.StarExpr); ok {
		if identifier, ok := pointer.X.(*goAst.Ident); ok {
			receiver = "*" + identifier.Name
		}
	}
	return receiver
}
