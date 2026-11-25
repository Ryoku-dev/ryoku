package extract

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
)

func extractGo(source []byte) (Result, error) {
	set := token.NewFileSet()
	file, err := parser.ParseFile(set, "source.go", source, parser.ParseComments)
	if err != nil && file == nil {
		return Result{}, fmt.Errorf("parse Go source: %w", err)
	}
	result := Result{}
	for _, spec := range file.Imports {
		target, _ := strconv.Unquote(spec.Path.Value)
		result.Imports = append(result.Imports, Import{Target: target, Line: set.Position(spec.Pos()).Line})
	}
	for _, declaration := range file.Decls {
		switch node := declaration.(type) {
		case *ast.FuncDecl:
			kind, parent, name := "function", "", node.Name.Name
			if node.Recv != nil && len(node.Recv.List) > 0 {
				kind = "method"
				parent = receiverName(node.Recv.List[0].Type)
				if parent != "" {
					name = parent + "." + name
				}
			}
			depth := 0
			if parent != "" {
				depth = 1
			}
			result.Symbols = append(result.Symbols, Symbol{Name: name, Kind: kind, Signature: sourceLine(source, set.Position(node.Pos()).Line), Start: set.Position(node.Pos()).Line, End: set.Position(node.End()).Line, Parent: parent, Doc: commentText(node.Doc), Depth: depth})
		case *ast.GenDecl:
			for _, spec := range node.Specs {
				switch item := spec.(type) {
				case *ast.TypeSpec:
					start := set.Position(node.Pos()).Line
					if node.Lparen.IsValid() {
						start = set.Position(item.Pos()).Line
					}
					result.Symbols = append(result.Symbols, Symbol{Name: item.Name.Name, Kind: goTypeKind(item.Type), Signature: sourceLine(source, start), Start: start, End: set.Position(item.End()).Line, Doc: firstComment(item.Doc, node.Doc)})
				case *ast.ValueSpec:
					kind := strings.ToLower(node.Tok.String())
					for _, name := range item.Names {
						start := set.Position(node.Pos()).Line
						if node.Lparen.IsValid() {
							start = set.Position(name.Pos()).Line
						}
						result.Symbols = append(result.Symbols, Symbol{Name: name.Name, Kind: kind, Signature: sourceLine(source, start), Start: start, End: set.Position(item.End()).Line, Doc: firstComment(item.Doc, node.Doc)})
					}
				}
			}
		}
	}
	return result, err
}

func receiverName(expression ast.Expr) string {
	switch value := expression.(type) {
	case *ast.Ident:
		return value.Name
	case *ast.StarExpr:
		return receiverName(value.X)
	case *ast.IndexExpr:
		return receiverName(value.X)
	case *ast.IndexListExpr:
		return receiverName(value.X)
	}
	return ""
}

func goTypeKind(expression ast.Expr) string {
	switch expression.(type) {
	case *ast.StructType:
		return "struct"
	case *ast.InterfaceType:
		return "interface"
	}
	return "type"
}

func commentText(group *ast.CommentGroup) string {
	if group == nil {
		return ""
	}
	return strings.TrimSpace(group.Text())
}
func firstComment(groups ...*ast.CommentGroup) string {
	for _, group := range groups {
		if text := commentText(group); text != "" {
			return text
		}
	}
	return ""
}
func sourceLine(source []byte, line int) string {
	lines := strings.Split(string(source), "\n")
	if line < 1 || line > len(lines) {
		return ""
	}
	return strings.TrimSpace(lines[line-1])
}
