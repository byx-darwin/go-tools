// Package templateutil 提供可插拔的模板辅助函数库。
package templateutil

import (
	"bytes"
	"strings"
	"text/template"
	"unicode"
)

// Registry 可插拔的函数注册器。
type Registry struct {
	funcs template.FuncMap
}

// NewRegistry 创建空注册器。
func NewRegistry() *Registry {
	return &Registry{
		funcs: make(template.FuncMap),
	}
}

// Register 注册单个函数。
func (r *Registry) Register(name string, fn any) *Registry {
	r.funcs[name] = fn
	return r
}

// RegisterAll 批量注册。
func (r *Registry) RegisterAll(funcs template.FuncMap) *Registry {
	for k, v := range funcs {
		r.funcs[k] = v
	}
	return r
}

// FuncMap 返回已注册的所有函数。
func (r *Registry) FuncMap() template.FuncMap {
	return r.funcs
}

// Render 使用默认函数集渲染模板。
func Render(tmpl string, data any) (string, error) {
	reg := NewRegistry().Default()
	return RenderWith(tmpl, data, reg)
}

// RenderWith 使用自定义 Registry 渲染。
func RenderWith(tmpl string, data any, reg *Registry) (string, error) {
	t, err := template.New("").Funcs(reg.FuncMap()).Parse(tmpl)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// Default 返回内置默认函数集。
func (r *Registry) Default() *Registry {
	r.Register("ToLower", strings.ToLower)
	r.Register("ToUpper", strings.ToUpper)
	r.Register("LowerFirst", lowerFirst)
	r.Register("UpperFirst", upperFirst)
	r.Register("ToCamel", toCamel)
	r.Register("ToSnake", toSnake)
	r.Register("ToKebab", toKebab)
	r.Register("ExportName", upperFirst)
	r.Register("PrivateName", lowerFirst)
	return r
}

func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
}

func upperFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func toCamel(s string) string {
	parts := strings.Split(s, "_")
	for i := 1; i < len(parts); i++ {
		parts[i] = upperFirst(parts[i])
	}
	return strings.Join(parts, "")
}

func toSnake(s string) string {
	var result []rune
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result = append(result, '_')
		}
		result = append(result, unicode.ToLower(r))
	}
	return string(result)
}

func toKebab(s string) string {
	return strings.ReplaceAll(toSnake(s), "_", "-")
}
