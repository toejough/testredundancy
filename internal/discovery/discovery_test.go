package discovery_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/toejough/testredundancy/internal/discovery"
)

func TestHasParallelCall(t *testing.T) {
	tests := []struct {
		name string
		code string
		want bool
	}{
		{
			name: "has t.Parallel",
			code: `func TestFoo(t *testing.T) {
				t.Parallel()
				// test code
			}`,
			want: true,
		},
		{
			name: "no parallel",
			code: `func TestFoo(t *testing.T) {
				// test code
			}`,
			want: false,
		},
		{
			name: "parallel in subtest",
			code: `func TestFoo(t *testing.T) {
				t.Run("sub", func(t *testing.T) {
					t.Parallel()
				})
			}`,
			want: true,
		},
		{
			name: "other method call",
			code: `func TestFoo(t *testing.T) {
				t.Helper()
				t.Log("hello")
			}`,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Wrap the function in a package for parsing
			src := "package test\n\nimport \"testing\"\n\n" + tt.code

			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, "test.go", src, 0)
			if err != nil {
				t.Fatalf("failed to parse: %v", err)
			}

			// Find the function body
			var body *ast.BlockStmt
			for _, decl := range f.Decls {
				if fn, ok := decl.(*ast.FuncDecl); ok {
					body = fn.Body
					break
				}
			}

			if body == nil {
				t.Fatal("no function body found")
			}

			got := discovery.HasParallelCall(body)
			if got != tt.want {
				t.Errorf("HasParallelCall() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTestInfoQualifiedName(t *testing.T) {
	info := discovery.TestInfo{Pkg: "github.com/foo/bar", Name: "TestSomething"}
	got := info.QualifiedName()
	want := "github.com/foo/bar:TestSomething"
	if got != want {
		t.Errorf("QualifiedName() = %q, want %q", got, want)
	}
}
