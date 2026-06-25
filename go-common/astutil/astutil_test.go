package astutil_test

import (
	"go/token"
	"os"
	"path/filepath"
	"testing"

	"github.com/byx-darwin/go-tools/go-common/astutil"
	"github.com/dave/dst"
	"github.com/stretchr/testify/require"
)

func TestFile_FindFunctions(t *testing.T) {
	file, _ := astutil.ParseSource([]byte(`package main

func foo() {}
func bar() {}
`))
	funcs := file.FindFunctions()
	require.Len(t, funcs, 2)
}

func TestFile_FindFunction(t *testing.T) {
	file, _ := astutil.ParseSource([]byte(`package main

func foo() {}
func bar() {}
`))
	fn := file.FindFunction("foo")
	require.NotNil(t, fn)
	require.Equal(t, "foo", fn.Name.Name)
}

func TestFile_FindImports(t *testing.T) {
	file, _ := astutil.ParseSource([]byte(`package main

import "fmt"
import "os"
`))
	imports := file.FindImports()
	require.Len(t, imports, 2)
}

func TestFile_AddImport(t *testing.T) {
	file, _ := astutil.ParseSource([]byte(`package main
`))
	file.Apply(astutil.AddImport("fmt"))
	imports := file.FindImports()
	require.Len(t, imports, 1)
	require.Contains(t, imports[0].Path.Value, "fmt")
}

func TestFile_AddImport_Idempotent(t *testing.T) {
	file, _ := astutil.ParseSource([]byte(`package main

import "fmt"
`))
	file.Apply(astutil.AddImport("fmt"))
	imports := file.FindImports()
	require.Len(t, imports, 1) // 不会重复添加
}

func TestFile_RemoveImport(t *testing.T) {
	file, _ := astutil.ParseSource([]byte(`package main

import "fmt"
import "os"
`))
	file.Apply(astutil.RemoveImport("fmt"))
	imports := file.FindImports()
	require.Len(t, imports, 1)
	require.Contains(t, imports[0].Path.Value, "os")
}

func TestFile_Format(t *testing.T) {
	file, _ := astutil.ParseSource([]byte(`package main
import"fmt"
func main(){fmt.Println("hello")}
`))
	out, err := file.Format()
	require.NoError(t, err)
	require.Contains(t, string(out), "package main")
	require.Contains(t, string(out), `import "fmt"`)
}

func TestParseFile(t *testing.T) {
	// 创建一个临时文件
	// 这里简化，实际测试需要创建真实文件
	file, err := astutil.ParseSource([]byte(`package main

func main() {}
`))
	require.NoError(t, err)
	require.NotNil(t, file)
}

func TestParseSource_Error(t *testing.T) {
	_, err := astutil.ParseSource([]byte(`this is not valid go code {{{`))
	require.Error(t, err)
}

func TestFile_FindFunction_NotFound(t *testing.T) {
	file, err := astutil.ParseSource([]byte(`package main

func foo() {}
`))
	require.NoError(t, err)
	fn := file.FindFunction("nonexistent")
	require.Nil(t, fn)
}

func TestFile_AddImport_WithExistingImports(t *testing.T) {
	file, err := astutil.ParseSource([]byte(`package main

import "fmt"
`))
	require.NoError(t, err)
	file.Apply(astutil.AddImport("os"))
	imports := file.FindImports()
	require.Len(t, imports, 2)
}

func TestParseFile_FromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.go")
	err := os.WriteFile(path, []byte(`package main

func hello() {}
`), 0o644)
	require.NoError(t, err)

	file, err := astutil.ParseFile(path)
	require.NoError(t, err)
	require.NotNil(t, file)
	fn := file.FindFunction("hello")
	require.NotNil(t, fn)
}

func TestFile_WriteTo(t *testing.T) {
	file, err := astutil.ParseSource([]byte(`package main

func main() {}
`))
	require.NoError(t, err)

	dir := t.TempDir()
	path := filepath.Join(dir, "output.go")
	err = file.WriteTo(path)
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Contains(t, string(data), "package main")
	require.Contains(t, string(data), "func main()")
}

func TestFile_WriteTo_Error(t *testing.T) {
	file, err := astutil.ParseSource([]byte(`package main
`))
	require.NoError(t, err)

	dir := t.TempDir()
	err = file.WriteTo(dir) // 写入目录路径应失败
	require.Error(t, err)
}

func TestFile_FindDecls_FuncDecl(t *testing.T) {
	file, err := astutil.ParseSource([]byte(`package main

func foo() {}
func bar() {}
type MyInt int
`))
	require.NoError(t, err)

	decls := file.FindDecls((*dst.FuncDecl)(nil))
	require.Len(t, decls, 2)
	require.Equal(t, "foo", decls[0].(*dst.FuncDecl).Name.Name)
	require.Equal(t, "bar", decls[1].(*dst.FuncDecl).Name.Name)
}

func TestFile_FindDecls_GenDecl(t *testing.T) {
	file, err := astutil.ParseSource([]byte(`package main

import "fmt"
type MyInt int
var x = 1
const y = 2
func foo() {}
`))
	require.NoError(t, err)

	decls := file.FindDecls((*dst.GenDecl)(nil))
	require.Len(t, decls, 4) // import, type, var, const
}

func TestFile_FindDecls_NoMatch(t *testing.T) {
	file, err := astutil.ParseSource([]byte(`package main

func foo() {}
`))
	require.NoError(t, err)

	decls := file.FindDecls((*dst.GenDecl)(nil))
	require.Len(t, decls, 0)
}

func TestFile_FindGenDecls_Import(t *testing.T) {
	file, err := astutil.ParseSource([]byte(`package main

import "fmt"
import "os"
type MyInt int
func foo() {}
`))
	require.NoError(t, err)

	decls := file.FindGenDecls(token.IMPORT)
	require.Len(t, decls, 2) // 每个单独的 import 语句是一个 GenDecl
	require.Len(t, decls[0].Specs, 1)
	require.Len(t, decls[1].Specs, 1)
}

func TestFile_FindGenDecls_ImportGrouped(t *testing.T) {
	file, err := astutil.ParseSource([]byte(`package main

import (
	"fmt"
	"os"
)
type MyInt int
`))
	require.NoError(t, err)

	decls := file.FindGenDecls(token.IMPORT)
	require.Len(t, decls, 1)
	require.Len(t, decls[0].Specs, 2)
}

func TestFile_FindGenDecls_Type(t *testing.T) {
	file, err := astutil.ParseSource([]byte(`package main

type Foo struct{}
type Bar interface{}
var x = 1
`))
	require.NoError(t, err)

	decls := file.FindGenDecls(token.TYPE)
	require.Len(t, decls, 2)
}

func TestFile_FindGenDecls_Var(t *testing.T) {
	file, err := astutil.ParseSource([]byte(`package main

var x = 1
var y = 2
type MyInt int
`))
	require.NoError(t, err)

	decls := file.FindGenDecls(token.VAR)
	require.Len(t, decls, 2)
}

func TestFile_FindGenDecls_Const(t *testing.T) {
	file, err := astutil.ParseSource([]byte(`package main

const a = 1
const b = 2
func foo() {}
`))
	require.NoError(t, err)

	decls := file.FindGenDecls(token.CONST)
	require.Len(t, decls, 2)
}

func TestFile_FindGenDecls_NoMatch(t *testing.T) {
	file, err := astutil.ParseSource([]byte(`package main

func foo() {}
`))
	require.NoError(t, err)

	decls := file.FindGenDecls(token.TYPE)
	require.Len(t, decls, 0)
}

func TestFile_InsertAfter_CommentMarker(t *testing.T) {
	file, err := astutil.ParseSource([]byte(`package main

// inject:here
func foo() {}

func bar() {}
`))
	require.NoError(t, err)

	newFn := &dst.FuncDecl{
		Name: dst.NewIdent("baz"),
		Type: &dst.FuncType{},
		Body: &dst.BlockStmt{},
	}
	file.Apply(astutil.InsertAfter("// inject:here", newFn))

	funcs := file.FindFunctions()
	require.Len(t, funcs, 3)
	require.Equal(t, "foo", funcs[0].Name.Name)
	require.Equal(t, "baz", funcs[1].Name.Name)
	require.Equal(t, "bar", funcs[2].Name.Name)
}

func TestFile_InsertAfter_FuncDeclMarker(t *testing.T) {
	file, err := astutil.ParseSource([]byte(`package main

func foo() {}
func bar() {}
`))
	require.NoError(t, err)

	fooFn := file.FindFunction("foo")
	require.NotNil(t, fooFn)

	newFn := &dst.FuncDecl{
		Name: dst.NewIdent("baz"),
		Type: &dst.FuncType{},
		Body: &dst.BlockStmt{},
	}
	file.Apply(astutil.InsertAfter(fooFn, newFn))

	funcs := file.FindFunctions()
	require.Len(t, funcs, 3)
	require.Equal(t, "foo", funcs[0].Name.Name)
	require.Equal(t, "baz", funcs[1].Name.Name)
	require.Equal(t, "bar", funcs[2].Name.Name)
}

func TestFile_InsertAfter_TypeDeclMarker(t *testing.T) {
	file, err := astutil.ParseSource([]byte(`package main

type Foo struct{}
func bar() {}
`))
	require.NoError(t, err)

	foos := file.FindDecls((*dst.GenDecl)(nil))
	require.Len(t, foos, 1)

	newType := &dst.GenDecl{
		Tok: token.TYPE,
		Specs: []dst.Spec{
			&dst.TypeSpec{
				Name: dst.NewIdent("Baz"),
				Type: &dst.StructType{},
			},
		},
	}
	file.Apply(astutil.InsertAfter(foos[0], newType))

	decls := file.FindDecls((*dst.GenDecl)(nil))
	require.Len(t, decls, 2)
	require.Equal(t, "Foo", decls[0].(*dst.GenDecl).Specs[0].(*dst.TypeSpec).Name.Name)
	require.Equal(t, "Baz", decls[1].(*dst.GenDecl).Specs[0].(*dst.TypeSpec).Name.Name)
}

func TestFile_InsertAfter_MarkerNotFound(t *testing.T) {
	file, err := astutil.ParseSource([]byte(`package main

func foo() {}
`))
	require.NoError(t, err)

	newFn := &dst.FuncDecl{
		Name: dst.NewIdent("bar"),
		Type: &dst.FuncType{},
		Body: &dst.BlockStmt{},
	}
	file.Apply(astutil.InsertAfter("// nonexistent:marker", newFn))

	funcs := file.FindFunctions()
	require.Len(t, funcs, 1)
	require.Equal(t, "foo", funcs[0].Name.Name)
}

func TestFile_InsertAfter_MultipleNodes(t *testing.T) {
	file, err := astutil.ParseSource([]byte(`package main

func foo() {}
`))
	require.NoError(t, err)

	fooFn := file.FindFunction("foo")
	require.NotNil(t, fooFn)

	barFn := &dst.FuncDecl{
		Name: dst.NewIdent("bar"),
		Type: &dst.FuncType{},
		Body: &dst.BlockStmt{},
	}
	bazFn := &dst.FuncDecl{
		Name: dst.NewIdent("baz"),
		Type: &dst.FuncType{},
		Body: &dst.BlockStmt{},
	}
	file.Apply(astutil.InsertAfter(fooFn, barFn, bazFn))

	funcs := file.FindFunctions()
	require.Len(t, funcs, 3)
	require.Equal(t, "foo", funcs[0].Name.Name)
	require.Equal(t, "bar", funcs[1].Name.Name)
	require.Equal(t, "baz", funcs[2].Name.Name)
}

func TestFile_InsertAfter_CommentEndDecoration(t *testing.T) {
	file, err := astutil.ParseSource([]byte(`package main

func foo() {} // inject:after

func bar() {}
`))
	require.NoError(t, err)

	newFn := &dst.FuncDecl{
		Name: dst.NewIdent("baz"),
		Type: &dst.FuncType{},
		Body: &dst.BlockStmt{},
	}
	file.Apply(astutil.InsertAfter("// inject:after", newFn))

	funcs := file.FindFunctions()
	require.Len(t, funcs, 3)
	require.Equal(t, "foo", funcs[0].Name.Name)
	require.Equal(t, "baz", funcs[1].Name.Name)
	require.Equal(t, "bar", funcs[2].Name.Name)
}

func TestFile_EnsureFunction_CreateNew(t *testing.T) {
	file, err := astutil.ParseSource([]byte(`package main

func foo() {}
`))
	require.NoError(t, err)

	file.Apply(astutil.EnsureFunction("bar", nil, nil, nil))

	funcs := file.FindFunctions()
	require.Len(t, funcs, 2)
	require.Equal(t, "foo", funcs[0].Name.Name)
	require.Equal(t, "bar", funcs[1].Name.Name)
}

func TestFile_EnsureFunction_AlreadyExists(t *testing.T) {
	file, err := astutil.ParseSource([]byte(`package main

func foo() {}
func bar() {}
`))
	require.NoError(t, err)

	file.Apply(astutil.EnsureFunction("foo", nil, nil, nil))

	funcs := file.FindFunctions()
	require.Len(t, funcs, 2)
	require.Equal(t, "foo", funcs[0].Name.Name)
	require.Equal(t, "bar", funcs[1].Name.Name)
}

func TestFile_EnsureFunction_WithParams(t *testing.T) {
	file, err := astutil.ParseSource([]byte(`package main
`))
	require.NoError(t, err)

	file.Apply(astutil.EnsureFunction("greet", []astutil.Field{
		{Names: []string{"name"}, Type: dst.NewIdent("string")},
		{Names: []string{"age"}, Type: dst.NewIdent("int")},
	}, nil, nil))

	fn := file.FindFunction("greet")
	require.NotNil(t, fn)
	require.Len(t, fn.Type.Params.List, 2)
	require.Equal(t, "name", fn.Type.Params.List[0].Names[0].Name)
	require.Equal(t, "string", fn.Type.Params.List[0].Type.(*dst.Ident).Name)
	require.Equal(t, "age", fn.Type.Params.List[1].Names[0].Name)
	require.Equal(t, "int", fn.Type.Params.List[1].Type.(*dst.Ident).Name)
}

func TestFile_EnsureFunction_WithResults(t *testing.T) {
	file, err := astutil.ParseSource([]byte(`package main
`))
	require.NoError(t, err)

	file.Apply(astutil.EnsureFunction("add", []astutil.Field{
		{Names: []string{"a", "b"}, Type: dst.NewIdent("int")},
	}, []astutil.Field{
		{Type: dst.NewIdent("int")},
	}, nil))

	fn := file.FindFunction("add")
	require.NotNil(t, fn)
	require.Len(t, fn.Type.Params.List, 1)
	require.Len(t, fn.Type.Params.List[0].Names, 2)
	require.Len(t, fn.Type.Results.List, 1)
	require.Equal(t, "int", fn.Type.Results.List[0].Type.(*dst.Ident).Name)
}

func TestFile_EnsureFunction_WithBody(t *testing.T) {
	file, err := astutil.ParseSource([]byte(`package main
`))
	require.NoError(t, err)

	file.Apply(astutil.EnsureFunction("hello", nil, nil, []dst.Stmt{
		&dst.ExprStmt{
			X: &dst.CallExpr{
				Fun: &dst.Ident{Name: "println"},
				Args: []dst.Expr{
					&dst.BasicLit{Kind: token.STRING, Value: `"hello"`},
				},
			},
		},
	}))

	fn := file.FindFunction("hello")
	require.NotNil(t, fn)
	require.Len(t, fn.Body.List, 1)

	out, err := file.Format()
	require.NoError(t, err)
	require.Contains(t, string(out), `println("hello")`)
}

func TestFile_ReplaceNode_FuncDecl(t *testing.T) {
	file, err := astutil.ParseSource([]byte(`package main

func foo() {}
func bar() {}
`))
	require.NoError(t, err)

	oldFn := file.FindFunction("foo")
	require.NotNil(t, oldFn)

	newFn := &dst.FuncDecl{
		Name: dst.NewIdent("baz"),
		Type: &dst.FuncType{},
		Body: &dst.BlockStmt{},
	}
	file.Apply(astutil.ReplaceNode(oldFn, newFn))

	funcs := file.FindFunctions()
	require.Len(t, funcs, 2)
	require.Equal(t, "baz", funcs[0].Name.Name)
	require.Equal(t, "bar", funcs[1].Name.Name)
}

func TestFile_ReplaceNode_TypeDecl(t *testing.T) {
	file, err := astutil.ParseSource([]byte(`package main

type Foo struct{}
type Bar struct{}
`))
	require.NoError(t, err)

	decls := file.FindGenDecls(token.TYPE)
	require.Len(t, decls, 2)

	newType := &dst.GenDecl{
		Tok: token.TYPE,
		Specs: []dst.Spec{
			&dst.TypeSpec{
				Name: dst.NewIdent("Baz"),
				Type: &dst.StructType{},
			},
		},
	}
	file.Apply(astutil.ReplaceNode(decls[0], newType))

	updated := file.FindGenDecls(token.TYPE)
	require.Len(t, updated, 2)
	require.Equal(t, "Baz", updated[0].Specs[0].(*dst.TypeSpec).Name.Name)
	require.Equal(t, "Bar", updated[1].Specs[0].(*dst.TypeSpec).Name.Name)
}

func TestFile_ReplaceNode_ImportDecl(t *testing.T) {
	file, err := astutil.ParseSource([]byte(`package main

import "fmt"
import "os"
`))
	require.NoError(t, err)

	decls := file.FindGenDecls(token.IMPORT)
	require.Len(t, decls, 2)

	newImport := &dst.GenDecl{
		Tok: token.IMPORT,
		Specs: []dst.Spec{
			&dst.ImportSpec{
				Path: &dst.BasicLit{
					Kind:  token.STRING,
					Value: `"strings"`,
				},
			},
		},
	}
	file.Apply(astutil.ReplaceNode(decls[0], newImport))

	imports := file.FindImports()
	require.Len(t, imports, 2)
	require.Contains(t, imports[0].Path.Value, "strings")
	require.Contains(t, imports[1].Path.Value, "os")
}

func TestFile_ReplaceNode_NotFound(t *testing.T) {
	file, err := astutil.ParseSource([]byte(`package main

func foo() {}
`))
	require.NoError(t, err)

	// 构造一个不在文件中的节点
	nonExistent := &dst.FuncDecl{
		Name: dst.NewIdent("nonexistent"),
		Type: &dst.FuncType{},
		Body: &dst.BlockStmt{},
	}
	newFn := &dst.FuncDecl{
		Name: dst.NewIdent("bar"),
		Type: &dst.FuncType{},
		Body: &dst.BlockStmt{},
	}
	file.Apply(astutil.ReplaceNode(nonExistent, newFn))

	// 文件应保持不变
	funcs := file.FindFunctions()
	require.Len(t, funcs, 1)
	require.Equal(t, "foo", funcs[0].Name.Name)
}

func TestFile_ReplaceNode_FormatOutput(t *testing.T) {
	file, err := astutil.ParseSource([]byte(`package main

func foo() {}
`))
	require.NoError(t, err)

	oldFn := file.FindFunction("foo")
	require.NotNil(t, oldFn)

	newFn := &dst.FuncDecl{
		Name: dst.NewIdent("bar"),
		Type: &dst.FuncType{},
		Body: &dst.BlockStmt{},
	}
	file.Apply(astutil.ReplaceNode(oldFn, newFn))

	out, err := file.Format()
	require.NoError(t, err)
	require.Contains(t, string(out), "func bar()")
	require.NotContains(t, string(out), "func foo()")
}
