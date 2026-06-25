package astutil

import (
	"go/token"

	"github.com/dave/dst"
)

// Op 表示一个 AST 修改操作。
type Op func(*dst.File)

// Apply 应用多个修改操作。
func (f *File) Apply(ops ...Op) {
	for _, op := range ops {
		op(f.file)
	}
}

// AddImport 添加 import 语句（幂等，重复添加不会生效）。
func AddImport(path string) Op {
	return func(f *dst.File) {
		quoted := `"` + path + `"`
		// 检查是否已存在
		for _, imp := range f.Imports {
			if imp.Path.Value == quoted {
				return
			}
		}
		// 创建新的 import spec
		imp := &dst.ImportSpec{
			Path: &dst.BasicLit{
				Kind:  token.STRING,
				Value: quoted,
			},
		}
		f.Imports = append(f.Imports, imp)

		// 同步更新到 Decls
		for _, decl := range f.Decls {
			if genDecl, ok := decl.(*dst.GenDecl); ok && genDecl.Tok == token.IMPORT {
				genDecl.Specs = append(genDecl.Specs, imp)
				return
			}
		}
		// 没有 import 声明，创建一个新的并追加
		f.Decls = append(f.Decls, &dst.GenDecl{
			Tok:   token.IMPORT,
			Specs: []dst.Spec{imp},
		})
	}
}

// RemoveImport 移除 import 语句。
func RemoveImport(path string) Op {
	return func(f *dst.File) {
		quoted := `"` + path + `"`
		// 从 Imports 中移除
		var newImports []*dst.ImportSpec
		for _, imp := range f.Imports {
			if imp.Path.Value != quoted {
				newImports = append(newImports, imp)
			}
		}
		f.Imports = newImports

		// 从 Decls 中移除对应的 spec
		var newDecls []dst.Decl
		for _, decl := range f.Decls {
			if genDecl, ok := decl.(*dst.GenDecl); ok && genDecl.Tok == token.IMPORT {
				var newSpecs []dst.Spec
				for _, spec := range genDecl.Specs {
					if imp, ok := spec.(*dst.ImportSpec); !ok || imp.Path.Value != quoted {
						newSpecs = append(newSpecs, spec)
					}
				}
				if len(newSpecs) > 0 {
					genDecl.Specs = newSpecs
					newDecls = append(newDecls, genDecl)
				}
				// 空 GenDecl 跳过（即删除）
			} else {
				newDecls = append(newDecls, decl)
			}
		}
		f.Decls = newDecls
	}
}
