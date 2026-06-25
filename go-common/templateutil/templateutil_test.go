// go-common/templateutil/templateutil_test.go
package templateutil_test

import (
	"testing"

	"github.com/byx-darwin/go-tools/go-common/templateutil"
	"github.com/stretchr/testify/require"
)

func TestNewRegistry(t *testing.T) {
	reg := templateutil.NewRegistry()
	require.NotNil(t, reg)
}

func TestRender_SimpleTemplate(t *testing.T) {
	out, err := templateutil.Render("Hello {{ .Name }}", map[string]any{"Name": "World"})
	require.NoError(t, err)
	require.Equal(t, "Hello World", out)
}

func TestDefault_ToLower(t *testing.T) {
	out, err := templateutil.Render("{{ .Name | ToLower }}", map[string]any{"Name": "HELLO"})
	require.NoError(t, err)
	require.Equal(t, "hello", out)
}

func TestDefault_ToUpper(t *testing.T) {
	out, err := templateutil.Render("{{ .Name | ToUpper }}", map[string]any{"Name": "hello"})
	require.NoError(t, err)
	require.Equal(t, "HELLO", out)
}

func TestDefault_LowerFirst(t *testing.T) {
	out, err := templateutil.Render("{{ .Name | LowerFirst }}", map[string]any{"Name": "Hello"})
	require.NoError(t, err)
	require.Equal(t, "hello", out)
}

func TestDefault_UpperFirst(t *testing.T) {
	out, err := templateutil.Render("{{ .Name | UpperFirst }}", map[string]any{"Name": "hello"})
	require.NoError(t, err)
	require.Equal(t, "Hello", out)
}

func TestDefault_ToCamel(t *testing.T) {
	out, err := templateutil.Render("{{ .Name | ToCamel }}", map[string]any{"Name": "hello_world"})
	require.NoError(t, err)
	require.Equal(t, "helloWorld", out)
}

func TestDefault_ToSnake(t *testing.T) {
	out, err := templateutil.Render("{{ .Name | ToSnake }}", map[string]any{"Name": "HelloWorld"})
	require.NoError(t, err)
	require.Equal(t, "hello_world", out)
}

func TestDefault_ExportName(t *testing.T) {
	out, err := templateutil.Render("{{ .Name | ExportName }}", map[string]any{"Name": "hello"})
	require.NoError(t, err)
	require.Equal(t, "Hello", out)
}

func TestDefault_PrivateName(t *testing.T) {
	out, err := templateutil.Render("{{ .Name | PrivateName }}", map[string]any{"Name": "Hello"})
	require.NoError(t, err)
	require.Equal(t, "hello", out)
}

func TestRegistry_CustomFunction(t *testing.T) {
	reg := templateutil.NewRegistry().
		Register("double", func(s string) string { return s + s })

	out, err := templateutil.RenderWith("{{ .Name | double }}", map[string]any{"Name": "hi"}, reg)
	require.NoError(t, err)
	require.Equal(t, "hihi", out)
}

func TestRegistry_DefaultAndCustom(t *testing.T) {
	reg := templateutil.NewRegistry().
		Default().
		Register("exclaim", func(s string) string { return s + "!" })

	out, err := templateutil.RenderWith("{{ .Name | ToLower | exclaim }}", map[string]any{"Name": "HELLO"}, reg)
	require.NoError(t, err)
	require.Equal(t, "hello!", out)
}

func TestPlural_Regular(t *testing.T) {
	tests := []struct{ input, want string }{
		{"user", "users"},
		{"cat", "cats"},
		{"dog", "dogs"},
		{"book", "books"},
		{"", ""},
	}
	for _, tt := range tests {
		require.Equal(t, tt.want, templateutil.Plural(tt.input), "Plural(%q)", tt.input)
	}
}

func TestPlural_ESuffix(t *testing.T) {
	tests := []struct{ input, want string }{
		{"box", "boxes"},
		{"bus", "buses"},
		{"church", "churches"},
		{"dish", "dishes"},
		{"quiz", "quizzes"},
		{"buzz", "buzzes"},
	}
	for _, tt := range tests {
		require.Equal(t, tt.want, templateutil.Plural(tt.input), "Plural(%q)", tt.input)
	}
}

func TestPlural_ConsonantY(t *testing.T) {
	tests := []struct{ input, want string }{
		{"city", "cities"},
		{"baby", "babies"},
		{"party", "parties"},
		{"key", "keys"},
		{"boy", "boys"},
	}
	for _, tt := range tests {
		require.Equal(t, tt.want, templateutil.Plural(tt.input), "Plural(%q)", tt.input)
	}
}

func TestPlural_FFe(t *testing.T) {
	tests := []struct{ input, want string }{
		{"leaf", "leaves"},
		{"wolf", "wolves"},
		{"life", "lives"},
		{"wife", "wives"},
		{"knife", "knives"},
		{"half", "halves"},
	}
	for _, tt := range tests {
		require.Equal(t, tt.want, templateutil.Plural(tt.input), "Plural(%q)", tt.input)
	}
}

func TestPlural_Irregular(t *testing.T) {
	tests := []struct{ input, want string }{
		{"child", "children"},
		{"person", "people"},
		{"foot", "feet"},
		{"tooth", "teeth"},
		{"mouse", "mice"},
		{"goose", "geese"},
		{"man", "men"},
		{"woman", "women"},
		{"ox", "oxen"},
		{"datum", "data"},
		{"cactus", "cacti"},
		{"analysis", "analyses"}, //nolint:misspell // analyses 是 analysis 的正确复数形式
		{"thesis", "theses"},
		{"crisis", "crises"},
		{"phenomenon", "phenomena"},
		{"criterion", "criteria"},
		{"index", "indices"},
		{"appendix", "appendices"},
		{"matrix", "matrices"},
		{"potato", "potatoes"},
		{"tomato", "tomatoes"},
		{"hero", "heroes"},
		{"echo", "echoes"},
	}
	for _, tt := range tests {
		require.Equal(t, tt.want, templateutil.Plural(tt.input), "Plural(%q)", tt.input)
	}
}

func TestSingular_Regular(t *testing.T) {
	tests := []struct{ input, want string }{
		{"users", "user"},
		{"cats", "cat"},
		{"dogs", "dog"},
		{"books", "book"},
		{"", ""},
	}
	for _, tt := range tests {
		require.Equal(t, tt.want, templateutil.Singular(tt.input), "Singular(%q)", tt.input)
	}
}

func TestSingular_ESuffix(t *testing.T) {
	tests := []struct{ input, want string }{
		{"boxes", "box"},
		{"buses", "bus"},
		{"churches", "church"},
		{"dishes", "dish"},
		{"quizzes", "quiz"},
		{"buzzes", "buzz"},
	}
	for _, tt := range tests {
		require.Equal(t, tt.want, templateutil.Singular(tt.input), "Singular(%q)", tt.input)
	}
}

func TestSingular_Ies(t *testing.T) {
	tests := []struct{ input, want string }{
		{"cities", "city"},
		{"babies", "baby"},
		{"parties", "party"},
	}
	for _, tt := range tests {
		require.Equal(t, tt.want, templateutil.Singular(tt.input), "Singular(%q)", tt.input)
	}
}

func TestSingular_Ves(t *testing.T) {
	tests := []struct{ input, want string }{
		{"leaves", "leaf"},
		{"wolves", "wolf"},
		{"lives", "life"},
		{"wives", "wife"},
		{"knives", "knife"},
		{"halves", "half"},
	}
	for _, tt := range tests {
		require.Equal(t, tt.want, templateutil.Singular(tt.input), "Singular(%q)", tt.input)
	}
}

func TestSingular_Irregular(t *testing.T) {
	tests := []struct{ input, want string }{
		{"children", "child"},
		{"people", "person"},
		{"feet", "foot"},
		{"teeth", "tooth"},
		{"mice", "mouse"},
		{"geese", "goose"},
		{"men", "man"},
		{"women", "woman"},
		{"oxen", "ox"},
		{"data", "datum"},
		{"cacti", "cactus"},
		{"analyses", "analysis"}, //nolint:misspell // analyses 是 analysis 的正确复数形式
		{"theses", "thesis"},
		{"crises", "crisis"},
		{"phenomena", "phenomenon"},
		{"criteria", "criterion"},
		{"indices", "index"},
		{"appendices", "appendix"},
		{"matrices", "matrix"},
		{"potatoes", "potato"},
		{"tomatoes", "tomato"},
		{"heroes", "hero"},
		{"echoes", "echo"},
	}
	for _, tt := range tests {
		require.Equal(t, tt.want, templateutil.Singular(tt.input), "Singular(%q)", tt.input)
	}
}

func TestPlural_Template(t *testing.T) {
	out, err := templateutil.Render("{{ .Name | Plural }}", map[string]any{"Name": "user"})
	require.NoError(t, err)
	require.Equal(t, "users", out)
}

func TestSingular_Template(t *testing.T) {
	out, err := templateutil.Render("{{ .Name | Singular }}", map[string]any{"Name": "children"})
	require.NoError(t, err)
	require.Equal(t, "child", out)
}
