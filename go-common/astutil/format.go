package astutil

import (
	"bytes"
	"os"

	"github.com/dave/dst/decorator"
)

// Format 返回格式化后的源码。
func (f *File) Format() ([]byte, error) {
	restorer := decorator.NewRestorer()
	var buf bytes.Buffer
	if err := restorer.Fprint(&buf, f.file); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// WriteTo 写入文件。
func (f *File) WriteTo(path string) error {
	out, err := f.Format()
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o644)
}
