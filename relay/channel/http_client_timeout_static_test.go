package channel_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProviderHTTPClientsDeclareTimeout(t *testing.T) {
	files := []string{
		filepath.Join("ali", "image.go"),
		filepath.Join("ollama", "relay-ollama.go"),
	}

	for _, file := range files {
		t.Run(file, func(t *testing.T) {
			fset := token.NewFileSet()
			parsed, err := parser.ParseFile(fset, file, nil, 0)
			require.NoError(t, err)

			ast.Inspect(parsed, func(node ast.Node) bool {
				literal, ok := node.(*ast.CompositeLit)
				if !ok || !isHTTPClientLiteral(literal.Type) {
					return true
				}

				require.Truef(t, compositeLiteralHasKey(literal, "Timeout"),
					"%s uses http.Client without Timeout at %s", file, fset.Position(literal.Pos()))
				return true
			})
		})
	}
}

func isHTTPClientLiteral(expr ast.Expr) bool {
	selector, ok := expr.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "Client" {
		return false
	}
	ident, ok := selector.X.(*ast.Ident)
	return ok && ident.Name == "http"
}

func compositeLiteralHasKey(literal *ast.CompositeLit, key string) bool {
	for _, element := range literal.Elts {
		kv, ok := element.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		ident, ok := kv.Key.(*ast.Ident)
		if ok && ident.Name == key {
			return true
		}
	}
	return false
}
