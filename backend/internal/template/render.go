// Package template renders {{variable}} placeholders in template bodies/subjects
// against a caller-supplied variable map at send time.
package template

import "regexp"

var placeholder = regexp.MustCompile(`{{\s*([a-zA-Z0-9_]+)\s*}}`)

// Render replaces every {{variable}} placeholder in text with vars[variable].
// A placeholder with no matching key is left as-is, so a missing variable is
// visible in the rendered output instead of silently disappearing.
func Render(text string, vars map[string]string) string {
	if len(vars) == 0 {
		return text
	}
	return placeholder.ReplaceAllStringFunc(text, func(match string) string {
		key := placeholder.FindStringSubmatch(match)[1]
		if val, ok := vars[key]; ok {
			return val
		}
		return match
	})
}
