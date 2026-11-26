package outbound

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// bannedHTTP are net/http names that build or use a client without a redirect policy.
var bannedHTTP = map[string]bool{
	"DefaultClient": true, "DefaultTransport": true,
	"Get": true, "Head": true, "Post": true, "PostForm": true,
}

// TestOutboundClientsUseThisPackage fails when production code outside this package builds
// an http.Client or uses the default client, the default transport, or the package-level
// request helpers. Use NewClient (and NewTransport for a custom transport) instead.
func TestOutboundClientsUseThisPackage(t *testing.T) {
	root := filepath.Join("..", "..")
	var found []string
	for _, dir := range []string{"internal", "cmd"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if d.Name() == "outbound" || d.Name() == "testdata" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			found = append(found, bannedUses(t, path)...)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range found {
		t.Errorf("%s: build outbound clients with outbound.NewClient", f)
	}
}

func bannedUses(t *testing.T, path string) []string {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	name := ""
	for _, imp := range file.Imports {
		if p, _ := strconv.Unquote(imp.Path.Value); p == "net/http" {
			name = "http"
			if imp.Name != nil {
				name = imp.Name.Name
			}
		}
	}
	if name == "" {
		return nil
	}
	isHTTP := func(e ast.Expr, sel func(string) bool) bool {
		s, ok := e.(*ast.SelectorExpr)
		if !ok {
			return false
		}
		id, ok := s.X.(*ast.Ident)
		return ok && id.Name == name && sel(s.Sel.Name)
	}
	var out []string
	report := func(n ast.Node, what string) {
		out = append(out, filepath.ToSlash(fset.Position(n.Pos()).String())+": "+what)
	}
	ast.Inspect(file, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.CompositeLit:
			if isHTTP(n.Type, func(s string) bool { return s == "Client" }) {
				report(n, "http.Client literal")
			}
		case *ast.CallExpr:
			// new(http.Client)
			if id, ok := n.Fun.(*ast.Ident); ok && id.Name == "new" && len(n.Args) == 1 &&
				isHTTP(n.Args[0], func(s string) bool { return s == "Client" }) {
				report(n, "new(http.Client)")
			}
		case *ast.SelectorExpr:
			if isHTTP(n, func(s string) bool { return bannedHTTP[s] }) {
				report(n, "http."+n.Sel.Name)
			}
		}
		return true
	})
	return out
}
