package astutil

import (
	"go/token"
	"strings"

	"github.com/dave/dst"
)

// Field 表示函数参数或返回值。
type Field struct {
	Names []string // 参数名列表，可以为空（如匿名返回值）。
	Type  dst.Expr // 类型表达式。
}

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

// InsertAfter 在指定标记后插入代码。
// marker 可以是以下类型：
//   - string：查找包含该字符串的注释标记（如 "//inject:here"），在该注释所在声明后插入
//   - dst.Decl：在该节点后插入（通过指针比较定位）
//
// 如果找不到标记，操作不执行（no-op）。
func InsertAfter(marker any, nodes ...dst.Decl) Op {
	return func(f *dst.File) {
		switch m := marker.(type) {
		case string:
			insertAfterStringMarker(f, m, nodes)
		case dst.Decl:
			insertAfterDeclMarker(f, m, nodes)
		}
	}
}

// insertAfterStringMarker 在包含指定字符串的注释所在声明后插入节点。
func insertAfterStringMarker(f *dst.File, marker string, nodes []dst.Decl) {
	for i, decl := range f.Decls {
		if containsMarker(decl, marker) {
			f.Decls = insertAt(f.Decls, i+1, nodes...)
			return
		}
	}
}

// insertAfterDeclMarker 在指定声明节点后插入新节点。
func insertAfterDeclMarker(f *dst.File, marker dst.Decl, nodes []dst.Decl) {
	for i, decl := range f.Decls {
		if decl == marker {
			f.Decls = insertAt(f.Decls, i+1, nodes...)
			return
		}
	}
}

// containsMarker 检查声明的注释装饰中是否包含指定标记字符串。
func containsMarker(decl dst.Decl, marker string) bool {
	decs := decl.Decorations()
	if decs == nil {
		return false
	}
	for _, c := range decs.Start {
		if strings.Contains(c, marker) {
			return true
		}
	}
	for _, c := range decs.End {
		if strings.Contains(c, marker) {
			return true
		}
	}
	return false
}

// ReplaceNode 替换指定的 AST 节点。
// 通过指针比较（oldNode == decl）在 f.Decls 中定位目标节点，
// 找到后用 newNode 替换。如果找不到 oldNode，不执行替换（no-op）。
func ReplaceNode(oldNode, newNode dst.Decl) Op {
	return func(f *dst.File) {
		for i, decl := range f.Decls {
			if decl == oldNode {
				f.Decls[i] = newNode
				return
			}
		}
	}
}

// EnsureFunction 确保指定函数存在，不存在则创建。
// 函数存在时不做任何操作，不存在时创建新的函数声明并添加到文件末尾。
func EnsureFunction(name string, params, results []Field, body []dst.Stmt) Op {
	return func(f *dst.File) {
		// 查找函数是否已存在
		for _, decl := range f.Decls {
			if fn, ok := decl.(*dst.FuncDecl); ok && fn.Name.Name == name {
				return
			}
		}

		// 构建参数列表
		var paramList []*dst.Field
		for _, p := range params {
			paramList = append(paramList, &dst.Field{
				Names: toIdents(p.Names),
				Type:  p.Type,
			})
		}

		// 构建返回值列表
		var resultList []*dst.Field
		for _, r := range results {
			resultList = append(resultList, &dst.Field{
				Names: toIdents(r.Names),
				Type:  r.Type,
			})
		}

		// 创建函数声明
		decl := &dst.FuncDecl{
			Name: dst.NewIdent(name),
			Type: &dst.FuncType{
				Params:  &dst.FieldList{List: paramList},
				Results: &dst.FieldList{List: resultList},
			},
			Body: &dst.BlockStmt{List: body},
		}

		f.Decls = append(f.Decls, decl)
	}
}

// toIdents 将字符串列表转换为 Ident 列表。
func toIdents(names []string) []*dst.Ident {
	if len(names) == 0 {
		return nil
	}
	idents := make([]*dst.Ident, len(names))
	for i, name := range names {
		idents[i] = dst.NewIdent(name)
	}
	return idents
}

// insertAt 在切片的指定位置插入元素。
func insertAt[T any](slice []T, pos int, elements ...T) []T {
	if pos >= len(slice) {
		return append(slice, elements...)
	}
	result := make([]T, 0, len(slice)+len(elements))
	result = append(result, slice[:pos]...)
	result = append(result, elements...)
	result = append(result, slice[pos:]...)
	return result
}
