// Package astutil 提供通用 Go AST 操作库，基于 dave/dst 封装。
package astutil

import (
	"go/parser"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
)

// File 表示一个可操作的 Go 源文件。
type File struct {
	file *dst.File
	dec  *decorator.Decorator
}

// ParseSource 从源代码解析。
func ParseSource(src []byte) (*File, error) {
	dec := decorator.NewDecorator(nil)
	f, err := dec.ParseFile("", src, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	return &File{file: f, dec: dec}, nil
}

// ParseFile 从文件路径解析。
func ParseFile(path string) (*File, error) {
	dec := decorator.NewDecorator(nil)
	f, err := dec.ParseFile(path, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	return &File{file: f, dec: dec}, nil
}
