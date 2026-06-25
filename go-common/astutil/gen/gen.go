// Package gen 提供 jennifer 风格的流式 API，用于从零生成 Go 代码。
package gen

import (
	"bytes"
	"fmt"
	"strings"
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

// Struct 添加结构体。
func (f *File) Struct(name string) *StructDecl {
	sd := &StructDecl{file: f, name: name}
	f.decls = append(f.decls, "") // 占位
	return sd
}

// StructDecl 结构体声明构建器。
type StructDecl struct {
	file   *File
	name   string
	fields []structField
}

type structField struct {
	name string
	typ  string
	tag  string
}

// Field 添加字段。
func (sd *StructDecl) Field(name, typ string) *StructDecl {
	sd.fields = append(sd.fields, structField{name: name, typ: typ})
	sd.file.decls[len(sd.file.decls)-1] = sd.render()
	return sd
}

// Tag 为最后一个字段添加 tag。
func (sd *StructDecl) Tag(tag string) *StructDecl {
	if len(sd.fields) > 0 {
		sd.fields[len(sd.fields)-1].tag = tag
		sd.file.decls[len(sd.file.decls)-1] = sd.render()
	}
	return sd
}

func (sd *StructDecl) render() string {
	var b strings.Builder
	b.Grow(128)
	b.WriteString("type ")
	b.WriteString(sd.name)
	b.WriteString(" struct {\n")
	for _, f := range sd.fields {
		b.WriteByte('\t')
		b.WriteString(f.name)
		b.WriteByte(' ')
		b.WriteString(f.typ)
		if f.tag != "" {
			b.WriteString(" `")
			b.WriteString(f.tag)
			b.WriteByte('`')
		}
		b.WriteByte('\n')
	}
	b.WriteByte('}')
	return b.String()
}
