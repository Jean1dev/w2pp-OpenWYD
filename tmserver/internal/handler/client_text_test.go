package handler

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestClientTextIsWindows1252 guards issue #412: the 7662 client reads every
// text field as Windows-1252, and the protocol encoders copy string bytes as
// is. A literal typed with a raw accent ("Você") is UTF-8 in the Go source and
// reaches the client as two bytes ("VocÃª"), so non-ASCII text must be written
// as cp1252 escapes ("Voc\xea"). Any non-ASCII byte in a literal's source text
// is therefore a mistake, whether it ends up in a packet or only in a log.
func TestClientTextIsWindows1252(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		f, err := parser.ParseFile(fset, name, src, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			for i := 0; i < len(lit.Value); i++ {
				if lit.Value[i] >= 0x80 {
					t.Errorf("%s: literal %s has a raw non-ASCII byte; write it as a cp1252 \\x escape",
						fset.Position(lit.Pos()), lit.Value)
					break
				}
			}
			return true
		})
	}
}
