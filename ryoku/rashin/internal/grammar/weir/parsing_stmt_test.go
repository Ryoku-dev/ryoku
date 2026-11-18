package weir

import (
	"reflect"
	"testing"
)

func assertStatements(t *testing.T, code string, want []AstStmtNode) *Ast {
	t.Helper()
	a, err := Parse(code)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a.Stmts, want) {
		t.Fatalf("got %#v, want %#v", a.Stmts, want)
	}
	return a
}
func checkTestVariable(t *testing.T, a *Ast, want AstVariable) {
	t.Helper()
	got, ok := a.GetVariableValue("test")
	if !ok || !reflect.DeepEqual(got, want) {
		t.Fatalf("test variable = %#v,%v; want %#v", got, ok, want)
	}
}
func TestParsesSingleVarStmt(t *testing.T) {
	v := AstVariable{String: "to be this"}
	a := assertStatements(t, `let test "to be this"`, []AstStmtNode{{Kind: DeclareVariable, Name: "test", Value: v}})
	checkTestVariable(t, a, v)
}
func TestParsesSingleVarStmtWithLotsOfSpace(t *testing.T) {
	v := AstVariable{String: "to be this"}
	a := assertStatements(t, `let            test "to be this"`, []AstStmtNode{{Kind: DeclareVariable, Name: "test", Value: v}})
	checkTestVariable(t, a, v)
}
func TestParsesSingleVarStmtArray(t *testing.T) {
	v := AstVariable{IsArray: true, Array: []AstVariable{{String: "to be this"}, {String: "and this"}}}
	a := assertStatements(t, `let test ["to be this", "and this"]`, []AstStmtNode{{Kind: DeclareVariable, Name: "test", Value: v}})
	checkTestVariable(t, a, v)
}
func TestParsesSingleExprStmt(t *testing.T) {
	assertStatements(t, "expr main word", []AstStmtNode{{Kind: SetExpr, Name: "main", Expr: AstExprNode{Kind: Word, Text: "word"}}})
}
func TestParsesSingleCommentStmt(t *testing.T) {
	assertStatements(t, "# this is a comment", []AstStmtNode{{Kind: Comment, Name: "# this is a comment"}})
}
func TestParsesSingleCommentStmtWithSpacePrefix(t *testing.T) {
	assertStatements(t, "    # this is a comment", []AstStmtNode{{Kind: Comment, Name: "# this is a comment"}})
}
func TestParsesTests(t *testing.T) {
	assertStatements(t, `test "this is the case" "this is the solution"`, []AstStmtNode{{Kind: Test, Expect: "this is the case", ToBe: "this is the solution"}})
}
func TestParsesMultipleSpacesInTests(t *testing.T) {
	assertStatements(t, `test "this is the case"           "this is the solution"`, []AstStmtNode{{Kind: Test, Expect: "this is the case", ToBe: "this is the solution"}})
}
func TestParsesAllows(t *testing.T) {
	assertStatements(t, `allows "this is the case"`, []AstStmtNode{{Kind: Allows, Expect: "this is the case"}})
}
func TestParsesCommentExprVarTogether(t *testing.T) {
	v := AstVariable{String: "to be this"}
	e := AstExprNode{Kind: Word, Text: "word"}
	a := assertStatements(t, "let test \"to be this\"\nexpr main word\n# this is a comment", []AstStmtNode{{Kind: DeclareVariable, Name: "test", Value: v}, {Kind: SetExpr, Name: "main", Expr: e}, {Kind: Comment, Name: "# this is a comment"}})
	checkTestVariable(t, a, v)
	got, ok := a.GetExpr("main")
	if !ok || !reflect.DeepEqual(got, e) {
		t.Fatalf("main=%#v,%v", got, ok)
	}
}
func TestParsesCommentInMiddle(t *testing.T) {
	v := AstVariable{String: "to be this"}
	e := AstExprNode{Kind: Word, Text: "word"}
	a := assertStatements(t, "let test \"to be this\"\n# this is a comment\nexpr main word", []AstStmtNode{{Kind: DeclareVariable, Name: "test", Value: v}, {Kind: Comment, Name: "# this is a comment"}, {Kind: SetExpr, Name: "main", Expr: e}})
	checkTestVariable(t, a, v)
	got, ok := a.GetExpr("main")
	if !ok || !reflect.DeepEqual(got, e) {
		t.Fatalf("main=%#v,%v", got, ok)
	}
}
func assertParseError(t *testing.T, code string) {
	t.Helper()
	if _, err := ParseStr(code, false); err == nil {
		t.Fatalf("expected parse error for %q", code)
	}
}
func TestCatchesNonWhitespaceAfterExpr(t *testing.T)   { assertParseError(t, `expr+this is a test`) }
func TestCatchesNonWhitespaceAfterTest(t *testing.T)   { assertParseError(t, `test+"" ""`) }
func TestCatchesNonWhitespaceBetweenTest(t *testing.T) { assertParseError(t, `test ""+""`) }
func TestCatchesNonWhitespaceAfterLet(t *testing.T)    { assertParseError(t, `let+var ""`) }
func TestCatchesNonWhitespaceAfterLetVar(t *testing.T) { assertParseError(t, `let var+""`) }
func TestCatchesNonWhitespaceAfterAllows(t *testing.T) { assertParseError(t, `allows+""`) }
func FuzzCatchesAnythingAfterTest(f *testing.F) {
	for _, s := range []string{" ", "x", "\"", "\\", "\t", ";"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		if s != "" && s[0] != '\n' {
			assertParseError(t, `test "" ""`+s)
		}
	})
}
func FuzzCatchesAnythingAfterAllows(f *testing.F) {
	for _, s := range []string{" ", "x", "\"", "\\", "\t", ";"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		if s != "" && s[0] != '\n' {
			assertParseError(t, `allows "" `+s)
		}
	})
}
