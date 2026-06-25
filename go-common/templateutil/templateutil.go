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
	r.Register("Singular", Singular)
	r.Register("Plural", Plural)
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

// irregularPlural 不规则复数映射（单数→复数）。
var irregularPlural = map[string]string{
	"child":      "children",
	"person":     "people",
	"foot":       "feet",
	"tooth":      "teeth",
	"mouse":      "mice",
	"goose":      "geese",
	"man":        "men",
	"woman":      "women",
	"ox":         "oxen",
	"datum":      "data",
	"cactus":     "cacti",
	"focus":      "foci",
	"nucleus":    "nuclei",
	"syllabus":   "syllabi",
	"analysis":   "analyses", //nolint:misspell // analyses 是 analysis 的正确复数形式
	"diagnosis":  "diagnoses",
	"oasis":      "oases",
	"thesis":     "theses",
	"crisis":     "crises",
	"phenomenon": "phenomena",
	"criterion":  "criteria",
	"index":      "indices",
	"appendix":   "appendices",
	"matrix":     "matrices",
	"vertex":     "vertices",
	"axis":       "axes",
	"life":       "lives",
	"wife":       "wives",
	"knife":      "knives",
	"leaf":       "leaves",
	"wolf":       "wolves",
	"half":       "halves",
	"self":       "selves",
	"shelf":      "shelves",
	"thief":      "thieves",
	"potato":     "potatoes",
	"tomato":     "tomatoes",
	"hero":       "heroes",
	"echo":       "echoes",
	"volcano":    "volcanoes",
	"quiz":       "quizzes",
}

// irregularSingular 不规则单数映射（复数→单数）。
var irregularSingular map[string]string

func init() {
	irregularSingular = make(map[string]string, len(irregularPlural))
	for singular, plural := range irregularPlural {
		irregularSingular[plural] = singular
	}
}

// Plural 将英文单数转为复数。
func Plural(s string) string {
	if s == "" {
		return s
	}

	lower := strings.ToLower(s)

	// 不规则变化。
	if plural, ok := irregularPlural[lower]; ok {
		return plural
	}

	// 以 s, sh, ch, x, z 结尾加 es。
	if strings.HasSuffix(lower, "s") ||
		strings.HasSuffix(lower, "sh") ||
		strings.HasSuffix(lower, "ch") ||
		strings.HasSuffix(lower, "x") ||
		strings.HasSuffix(lower, "z") {
		return lower + "es"
	}

	// 辅音 + y → ies。
	if strings.HasSuffix(lower, "y") && len(lower) > 1 {
		prev := lower[len(lower)-2]
		if prev != 'a' && prev != 'e' && prev != 'i' && prev != 'o' && prev != 'u' {
			return lower[:len(lower)-1] + "ies"
		}
	}

	// 以 f 结尾 → ves。
	if strings.HasSuffix(lower, "f") {
		return lower[:len(lower)-1] + "ves"
	}

	// 以 fe 结尾 → ves。
	if strings.HasSuffix(lower, "fe") {
		return lower[:len(lower)-2] + "ves"
	}

	// 默认加 s。
	return lower + "s"
}

// Singular 将英文复数转为单数。
func Singular(s string) string {
	if s == "" {
		return s
	}

	lower := strings.ToLower(s)

	// 不规则变化。
	if singular, ok := irregularSingular[lower]; ok {
		return singular
	}

	// 以 ies 结尾 → y（cities→city）。
	if strings.HasSuffix(lower, "ies") && len(lower) > 3 {
		return lower[:len(lower)-3] + "y"
	}

	// 以 ves 结尾 → fe 或 f（knives→knife, wolves→wolf）。
	if strings.HasSuffix(lower, "ves") && len(lower) > 3 {
		base := lower[:len(lower)-3]
		// 优先尝试 fe 形式。
		if _, ok := irregularPlural[base+"fe"]; ok {
			return base + "fe"
		}
		return base + "f"
	}

	// 以 ches, shes, xes, zes 结尾 → 去 es（churches→church, boxes→box, quizzes→quiz）。
	if strings.HasSuffix(lower, "ches") || strings.HasSuffix(lower, "shes") ||
		strings.HasSuffix(lower, "xes") || strings.HasSuffix(lower, "zes") {
		return lower[:len(lower)-2]
	}

	// 以 ses 结尾 → 去 es（buses→bus）。
	if strings.HasSuffix(lower, "ses") {
		return lower[:len(lower)-2]
	}

	// 以 es 结尾 → 去 es。
	if strings.HasSuffix(lower, "es") && len(lower) > 2 {
		return lower[:len(lower)-2]
	}

	// 以 s 结尾 → 去 s。
	if strings.HasSuffix(lower, "s") && len(lower) > 1 {
		return lower[:len(lower)-1]
	}

	return lower
}
