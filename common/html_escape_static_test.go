package common

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCommonUtilitiesDoNotBypassTemplateEscaping(t *testing.T) {
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, "utils.go", nil, 0)
	require.NoError(t, err)

	ast.Inspect(parsed, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || !isTemplateHTMLSelector(call.Fun) {
			return true
		}

		require.Failf(t, "template.HTML bypass is not allowed",
			"utils.go wraps data with template.HTML at %s", fset.Position(call.Pos()))
		return true
	})
}

func isTemplateHTMLSelector(expr ast.Expr) bool {
	selector, ok := expr.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "HTML" {
		return false
	}
	ident, ok := selector.X.(*ast.Ident)
	return ok && ident.Name == "template"
}
