package template

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRender_SimpleSubstitution(t *testing.T) {
	out := Render("Hello {{name}}!", map[string]string{"name": "Alice"})
	assert.Equal(t, "Hello Alice!", out)
}

func TestRender_MultiplePlaceholders(t *testing.T) {
	out := Render("{{greeting}} {{name}}, you have {{count}} messages.", map[string]string{
		"greeting": "Hi",
		"name":     "Bob",
		"count":    "3",
	})
	assert.Equal(t, "Hi Bob, you have 3 messages.", out)
}

func TestRender_MissingVariable_LeftAsIs(t *testing.T) {
	out := Render("Hello {{name}}, code {{otp}}", map[string]string{"name": "Alice"})
	assert.Equal(t, "Hello Alice, code {{otp}}", out)
}

func TestRender_EmptyMap_ReturnsTextUnchanged(t *testing.T) {
	out := Render("Hello {{name}}!", map[string]string{})
	assert.Equal(t, "Hello {{name}}!", out)
}

func TestRender_NilMap_ReturnsTextUnchanged(t *testing.T) {
	out := Render("Hello {{name}}!", nil)
	assert.Equal(t, "Hello {{name}}!", out)
}

func TestRender_MalformedPlaceholder_SingleBrace(t *testing.T) {
	out := Render("Hello {name}!", map[string]string{"name": "Alice"})
	assert.Equal(t, "Hello {name}!", out)
}

func TestRender_MalformedPlaceholder_UnclosedBraces(t *testing.T) {
	out := Render("Hello {{name", map[string]string{"name": "Alice"})
	assert.Equal(t, "Hello {{name", out)
}

func TestRender_MalformedPlaceholder_MissingClosingBraces(t *testing.T) {
	out := Render("Hello {{name}", map[string]string{"name": "Alice"})
	assert.Equal(t, "Hello {{name}", out)
}

func TestRender_NoPlaceholders(t *testing.T) {
	out := Render("Just plain text.", map[string]string{"name": "Alice"})
	assert.Equal(t, "Just plain text.", out)
}

func TestRender_NestedLookingBraces(t *testing.T) {
	out := Render("Value: {{ {{name}} }}", map[string]string{"name": "Alice"})
	// The regex matches the innermost {{name}} first via ReplaceAllStringFunc scanning
	// left to right; outer braces remain literal since "{{ {{name}}" isn't itself a
	// valid placeholder match for the outer pair.
	assert.Equal(t, "Value: {{ Alice }}", out)
}

func TestRender_PlaceholderWithSurroundingWhitespace(t *testing.T) {
	out := Render("Hello {{ name }}!", map[string]string{"name": "Alice"})
	assert.Equal(t, "Hello Alice!", out)
}

func TestRender_EmptyText(t *testing.T) {
	out := Render("", map[string]string{"name": "Alice"})
	assert.Equal(t, "", out)
}

func TestRender_EmptyTextEmptyMap(t *testing.T) {
	out := Render("", map[string]string{})
	assert.Equal(t, "", out)
}

func TestRender_RepeatedPlaceholder(t *testing.T) {
	out := Render("{{name}} and {{name}} again", map[string]string{"name": "Alice"})
	assert.Equal(t, "Alice and Alice again", out)
}

func TestRender_VariableWithUnderscoreAndDigits(t *testing.T) {
	out := Render("{{user_id_1}}", map[string]string{"user_id_1": "42"})
	assert.Equal(t, "42", out)
}

func TestRender_VariableValueEmptyString(t *testing.T) {
	out := Render("Hello {{name}}!", map[string]string{"name": ""})
	assert.Equal(t, "Hello !", out)
}

func TestRender_ExtraVarsInMapNotUsed(t *testing.T) {
	out := Render("Hello {{name}}!", map[string]string{"name": "Alice", "unused": "value"})
	assert.Equal(t, "Hello Alice!", out)
}
