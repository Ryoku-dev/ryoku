// Package weir implements Harper's Weir rule language and Weirpack archives.
package weir

import (
	"fmt"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/pos"
	"ryoku-rashin/internal/grammar/text"
)

type Ast struct{ Stmts []AstStmtNode }
type AstStmtKind uint8

const (
	DeclareVariable AstStmtKind = iota
	SetExpr
	Comment
	Test
	Allows
)

type AstStmtNode struct {
	Kind         AstStmtKind
	Name         string
	Value        AstVariable
	Expr         AstExprNode
	Expect, ToBe string
}
type AstVariable struct {
	String  string
	Array   []AstVariable
	IsArray bool
}
type AstExprKind uint8

const (
	Whitespace AstExprKind = iota
	Progressive
	UPOSSet
	Word
	DerivativeOf
	Punctuation
	Not
	Seq
	Arr
	Filter
	ExprRef
	Anything
)

type AstExprNode struct {
	Kind        AstExprKind
	Text        string
	UPOS        []pos.UPOS
	Punctuation text.Punctuation
	Children    []AstExprNode
}
type TestCase struct {
	Input, Expected string
	Allows          bool
}

func (a *Ast) GetVariableValue(name string) (AstVariable, bool) {
	for i := len(a.Stmts) - 1; i >= 0; i-- {
		s := a.Stmts[i]
		if s.Kind == DeclareVariable && s.Name == name {
			return s.Value, true
		}
	}
	return AstVariable{}, false
}
func (a *Ast) GetExpr(name string) (AstExprNode, bool) {
	for i := len(a.Stmts) - 1; i >= 0; i-- {
		s := a.Stmts[i]
		if s.Kind == SetExpr && s.Name == name {
			return s.Expr, true
		}
	}
	return AstExprNode{}, false
}
func (a *Ast) Tests() []TestCase {
	var out []TestCase
	for _, s := range a.Stmts {
		if s.Kind == Test {
			out = append(out, TestCase{s.Expect, s.ToBe, false})
		} else if s.Kind == Allows {
			out = append(out, TestCase{s.Expect, s.Expect, true})
		}
	}
	return out
}
func (n AstExprNode) ToExpr(ctx map[string]expr.Expr) (expr.Expr, error) {
	switch n.Kind {
	case Anything:
		return expr.NewAnyPattern(), nil
	case Progressive:
		return expr.NewSequenceExpr().ThenKind(doc.TokenKind.IsVerbProgressiveForm), nil
	case Whitespace:
		return expr.NewWhitespacePattern(), nil
	case UPOSSet:
		return expr.NewUPOSSet(n.UPOS), nil
	case Word:
		return expr.NewWord(n.Text), nil
	case DerivativeOf:
		return expr.NewDerivedFromStr(n.Text), nil
	case Punctuation:
		p := n.Punctuation
		return expr.NewSequenceExpr().Then(func(t doc.Token, _ []rune) bool { return t.Kind.IsPunctuation() && t.Kind.Punctuation == p }), nil
	case ExprRef:
		if e, ok := ctx[n.Text]; ok {
			return e, nil
		}
		return nil, Error{Kind: UnableToResolveExpr, Value: n.Text}
	}
	children := make([]expr.Expr, 0, len(n.Children))
	for _, c := range n.Children {
		e, err := c.ToExpr(ctx)
		if err != nil {
			return nil, err
		}
		children = append(children, e)
	}
	switch n.Kind {
	case Not:
		if len(children) != 1 {
			return nil, fmt.Errorf("invalid negation")
		}
		return expr.NewUnlessStep(children[0], func(doc.Token, []rune) bool { return true }), nil
	case Seq:
		s := expr.NewSequenceExpr()
		for _, c := range children {
			s = s.Then(c)
		}
		return s, nil
	case Arr:
		return expr.NewLongestMatchOf(children), nil
	case Filter:
		return expr.NewFilter(children), nil
	}
	return nil, fmt.Errorf("invalid expression kind %d", n.Kind)
}
func OptimizeExpr(n *AstExprNode) bool {
	edited := false
	switch n.Kind {
	case Not:
		if len(n.Children) > 0 {
			return OptimizeExpr(&n.Children[0])
		}
	case Seq, Arr:
		if len(n.Children) == 1 {
			*n = n.Children[0]
			return true
		}
		if n.Kind == Arr && len(n.Children) > 0 {
			all := true
			for _, c := range n.Children {
				if c.Kind != UPOSSet {
					all = false
				}
			}
			if all {
				var up []pos.UPOS
				for _, c := range n.Children {
					up = append(up, c.UPOS...)
				}
				*n = AstExprNode{Kind: UPOSSet, UPOS: up}
				return true
			}
		}
		for i := range n.Children {
			if OptimizeExpr(&n.Children[i]) {
				edited = true
			}
		}
	}
	return edited
}
func Optimize(a *Ast) bool {
	edited := false
	for i := range a.Stmts {
		if a.Stmts[i].Kind == SetExpr && OptimizeExpr(&a.Stmts[i].Expr) {
			edited = true
		}
	}
	return edited
}
