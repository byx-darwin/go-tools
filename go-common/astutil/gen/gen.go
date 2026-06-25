// Package gen 提供 jennifer 风格的流式 API，用于从零生成 Go 代码。
package gen

import (
	"bytes"
	"fmt"
)

// File 表示一个 Go 源文件。
type File struct {
	pkg     string
	imports []string
	decls   []string
}

// NewFile 创建新文件。
func NewFile(pkg string) *File {
	return &File{pkg: pkg}
}

// Import 添加 import。
func (f *File) Import(path string) *File {
	f.imports = append(f.imports, path)
	return f
}

// Render 返回格式化的源码。
func (f *File) Render() (string, error) {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "package %s\n\n", f.pkg)
	if len(f.imports) > 0 {
		buf.WriteString("import (\n")
		for _, imp := range f.imports {
			fmt.Fprintf(&buf, "\t%q\n", imp)
		}
		buf.WriteString(")\n\n")
	}
	for _, decl := range f.decls {
		buf.WriteString(decl)
		buf.WriteString("\n\n")
	}
	return buf.String(), nil
}
