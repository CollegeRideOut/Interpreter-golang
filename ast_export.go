package main

import (
	"encoding/json"
	"fmt"
	goast "go/ast"
	"go/token"
	"os"
	"reflect"
	"strings"
)

type astJSONNode struct {
	Kind   string                    `json:"kind"`
	Value  string                    `json:"value,omitempty"`
	Pos    int                       `json:"pos,omitempty"`
	End    int                       `json:"end,omitempty"`
	Fields map[string]*astJSONNode   `json:"fields,omitempty"`
	Lists  map[string][]*astJSONNode `json:"lists,omitempty"`
	Values map[string]interface{}    `json:"values,omitempty"`
}

type editScriptJSON struct {
	Index       int    `json:"index"`
	Kind        string `json:"kind"`
	ParentIndex int    `json:"parentIndex"`
	SourceID    string `json:"sourceId,omitempty"`
	SourceKind  string `json:"sourceKind,omitempty"`
	TargetID    string `json:"targetId,omitempty"`
	TargetKind  string `json:"targetKind,omitempty"`
	Start       int    `json:"start"`
	End         int    `json:"end"`
	TargetStart int    `json:"targetStart"`
	TargetEnd   int    `json:"targetEnd"`
	Description string `json:"description"`
	Original    string `json:"original,omitempty"`
	Replacement string `json:"replacement,omitempty"`
}

func exportASTDiff(directory string) error {
	diff, err := prepareDirectoryDiff(directory)
	if err != nil {
		return err
	}

	sourceAST, sourceFileSet, err := parseGoAST(diff.sourceSource)
	if err != nil {
		return fmt.Errorf("parse previous source: %w", err)
	}
	targetAST, targetFileSet, err := parseGoAST(diff.targetSource)
	if err != nil {
		return fmt.Errorf("parse current source: %w", err)
	}

	if err := writeJSON("previousCommitAst.json", astJSONNodeFromGoAST(sourceAST, sourceFileSet)); err != nil {
		return err
	}
	if err := writeJSON("currentCommitAst.json", astJSONNodeFromGoAST(targetAST, targetFileSet)); err != nil {
		return err
	}
	if err := writeJSON("editScript.json", simpleASTEditScripts(sourceAST, targetAST, sourceFileSet, targetFileSet)); err != nil {
		return err
	}

	fmt.Println("wrote previousCommitAst.json")
	fmt.Println("wrote currentCommitAst.json")
	fmt.Println("wrote editScript.json")
	return nil
}

func writeJSON(path string, value interface{}) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", path, err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func astJSONNodeFromGoAST(node goast.Node, fileSet *token.FileSet) *astJSONNode {
	if node == nil {
		return nil
	}
	result := &astJSONNode{
		Kind:   fmt.Sprintf("%T", node),
		Fields: make(map[string]*astJSONNode),
		Lists:  make(map[string][]*astJSONNode),
		Values: make(map[string]interface{}),
	}
	if fileSet != nil {
		result.Pos = fileSet.Position(node.Pos()).Offset
		result.End = fileSet.Position(node.End()).Offset
	}
	if value := goNodeValue(node); value != "" {
		result.Value = value
	}

	value := reflect.ValueOf(node)
	if value.Kind() == reflect.Pointer {
		value = value.Elem()
	}
	typeOfNode := reflect.TypeOf((*goast.Node)(nil)).Elem()
	for index := 0; index < value.NumField(); index++ {
		fieldInfo := value.Type().Field(index)
		field := value.Field(index)
		if !fieldInfo.IsExported() || fieldInfo.Name == "Obj" || fieldInfo.Name == "Scope" || fieldInfo.Name == "Unresolved" {
			continue
		}
		if field.Type().Implements(typeOfNode) {
			if field.IsNil() {
				continue
			}
			result.Fields[fieldInfo.Name] = astJSONNodeFromGoAST(field.Interface().(goast.Node), fileSet)
			continue
		}
		if field.Kind() == reflect.Slice && field.Type().Elem().Implements(typeOfNode) {
			items := make([]*astJSONNode, 0, field.Len())
			for item := 0; item < field.Len(); item++ {
				items = append(items, astJSONNodeFromGoAST(field.Index(item).Interface().(goast.Node), fileSet))
			}
			result.Lists[fieldInfo.Name] = items
			continue
		}
		if scalar := astJSONScalar(field); scalar != nil {
			result.Values[fieldInfo.Name] = scalar
		}
	}
	if len(result.Fields) == 0 {
		result.Fields = nil
	}
	if len(result.Lists) == 0 {
		result.Lists = nil
	}
	if len(result.Values) == 0 {
		result.Values = nil
	}
	return result
}

func astJSONScalar(value reflect.Value) interface{} {
	if !value.IsValid() || !value.CanInterface() {
		return nil
	}
	if value.Type() == reflect.TypeOf(token.Token(0)) {
		return token.Token(value.Int()).String()
	}
	switch value.Kind() {
	case reflect.String, reflect.Bool:
		return value.Interface()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if strings.HasSuffix(value.Type().String(), ".Pos") {
			return nil
		}
		return value.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return value.Uint()
	}
	return nil
}

func editScriptsJSON(scripts []editScript) []editScriptJSON {
	result := make([]editScriptJSON, 0, len(scripts))
	for index, script := range scripts {
		item := editScriptJSON{
			Index:       index,
			Kind:        string(script.kind),
			ParentIndex: script.parentIndex,
			Start:       script.start,
			End:         script.end,
			TargetStart: script.targetStart,
			TargetEnd:   script.targetEnd,
			Description: script.description,
			Original:    string(script.original),
			Replacement: string(script.replacement),
		}
		if script.source != nil {
			item.SourceID = string(script.source.Id)
			item.SourceKind = string(script.source.Label)
		}
		if script.target != nil {
			item.TargetID = string(script.target.Id)
			item.TargetKind = string(script.target.Label)
		}
		result = append(result, item)
	}
	return result
}
