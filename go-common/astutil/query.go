// Package astutil 提供通用 Go AST 操作库，基于 dave/dst 封装。
package astutil

import (
	"reflect"

	"github.com/dave/dst"
)

// FindDecls 返回所有匹配指定类型的顶层声明。
// declType 应为声明类型的零值，如 *dst.GenDecl 或 *dst.FuncDecl。
func (f *File) FindDecls(declType any) []dst.Decl {
	target := reflect.TypeOf(declType)
	var result []dst.Decl
	for _, decl := range f.file.Decls {
		if reflect.TypeOf(decl) == target {
			result = append(result, decl)
		}
	}
	return result
}

// FindGenDecls 返回所有匹配指定 token 类型的 GenDecl。
// 例如传入 token.IMPORT、token.TYPE、token.CONST、token.VAR。
func (f *File) FindGenDecls(tok interface{ String() string }) []*dst.GenDecl {
	var result []*dst.GenDecl
	for _, decl := range f.file.Decls {
		if gd, ok := decl.(*dst.GenDecl); ok && gd.Tok.String() == tok.String() {
			result = append(result, gd)
		}
	}
	return result
}

// FindFunctions 返回所有函数声明。
func (f *File) FindFunctions() []*dst.FuncDecl {
	var funcs []*dst.FuncDecl
	for _, decl := range f.file.Decls {
		if fn, ok := decl.(*dst.FuncDecl); ok {
			funcs = append(funcs, fn)
		}
	}
	return funcs
}

// FindFunction 按名称查找函数。
func (f *File) FindFunction(name string) *dst.FuncDecl {
	for _, fn := range f.FindFunctions() {
		if fn.Name.Name == name {
			return fn
		}
	}
	return nil
}

// FindImports 返回所有 import。
func (f *File) FindImports() []*dst.ImportSpec {
	var imports []*dst.ImportSpec
	for _, decl := range f.file.Decls {
		if genDecl, ok := decl.(*dst.GenDecl); ok {
			for _, spec := range genDecl.Specs {
				if imp, ok := spec.(*dst.ImportSpec); ok {
					imports = append(imports, imp)
				}
			}
		}
	}
	return imports
}
