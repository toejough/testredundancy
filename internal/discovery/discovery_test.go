package discovery_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/onsi/gomega"
	"github.com/toejough/testredundancy/internal/discovery"
)

func TestHasParallelCall_DetectsDirectCall(t *testing.T) {
	t.Parallel()
	expect := gomega.NewWithT(t)

	src := `package test
func TestFoo(t *testing.T) {
	t.Parallel()
	// test body
}`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", src, 0)
	expect.Expect(err).NotTo(gomega.HaveOccurred())

	// Get the function body
	fn := f.Decls[0].(*ast.FuncDecl)
	result := discovery.HasParallelCall(fn.Body)

	expect.Expect(result).To(gomega.BeTrue())
}

func TestHasParallelCall_DetectsNestedCall(t *testing.T) {
	t.Parallel()
	expect := gomega.NewWithT(t)

	src := `package test
func TestFoo(t *testing.T) {
	if true {
		t.Parallel()
	}
}`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", src, 0)
	expect.Expect(err).NotTo(gomega.HaveOccurred())

	fn := f.Decls[0].(*ast.FuncDecl)
	result := discovery.HasParallelCall(fn.Body)

	expect.Expect(result).To(gomega.BeTrue())
}

func TestHasParallelCall_ReturnsFalseWithoutCall(t *testing.T) {
	t.Parallel()
	expect := gomega.NewWithT(t)

	src := `package test
func TestFoo(t *testing.T) {
	t.Log("not parallel")
}`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", src, 0)
	expect.Expect(err).NotTo(gomega.HaveOccurred())

	fn := f.Decls[0].(*ast.FuncDecl)
	result := discovery.HasParallelCall(fn.Body)

	expect.Expect(result).To(gomega.BeFalse())
}

func TestTestInfo_QualifiedName(t *testing.T) {
	t.Parallel()
	expect := gomega.NewWithT(t)

	info := discovery.TestInfo{
		Pkg:  "github.com/user/repo",
		Name: "TestFoo",
	}

	expect.Expect(info.QualifiedName()).To(gomega.Equal("github.com/user/repo:TestFoo"))
}
