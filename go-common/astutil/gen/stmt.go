// go-common/astutil/gen/stmt.go
package gen

import (
	"fmt"
	"strings"
)

// Param 创建参数。
func Param(name, typ string) string {
	return fmt.Sprintf("%s %s", name, typ)
}

// FuncDecl 函数声明构建器。
type FuncDecl struct {
	file    *File
	name    string
	params  []string
	results []string
	body    []string
}

// Func 添加函数。
func (f *File) Func(name string) *FuncDecl {
	fd := &FuncDecl{file: f, name: name}
	f.decls = append(f.decls, "") // 占位
	return fd
}

// Params 设置参数。
func (fd *FuncDecl) Params(params ...string) *FuncDecl {
	fd.params = params
	return fd
}

// Results 设置返回值。
func (fd *FuncDecl) Results(results ...string) *FuncDecl {
	fd.results = results
	return fd
}

// Body 设置函数体。
func (fd *FuncDecl) Body(stmts ...string) *FuncDecl {
	fd.body = stmts
	fd.file.decls[len(fd.file.decls)-1] = fd.render()
	return fd
}

func (fd *FuncDecl) render() string {
	var b strings.Builder
	b.Grow(256)

	b.WriteString("func ")
	b.WriteString(fd.name)
	b.WriteByte('(')

	for i, p := range fd.params {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(p)
	}
	b.WriteByte(')')

	if len(fd.results) > 0 {
		b.WriteString(" (")
		for i, r := range fd.results {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(r)
		}
		b.WriteByte(')')
	}

	if len(fd.body) > 0 {
		b.WriteByte('\n')
		for _, stmt := range fd.body {
			b.WriteByte('\t')
			b.WriteString(stmt)
			b.WriteByte('\n')
		}
		b.WriteByte('}')
	} else {
		b.WriteString(" {}")
	}

	return b.String()
}
