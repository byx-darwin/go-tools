// Package astutil 提供通用 Go AST 操作库，基于 dave/dst 封装。
package astutil

import "github.com/dave/dst"

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
