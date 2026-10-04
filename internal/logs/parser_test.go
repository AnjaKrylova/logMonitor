package logs

import (
	"maps"
	"testing"
)

func TestParseLogfmt(t *testing.T) {
	data := []struct {
		name  string
		input string
		want  map[string]string
	}{
		{"simple", `a=1 b=2 c=foo`, map[string]string{
			"a": "1",
			"b": "2",
			"c": "foo",
		}},
		{"value with space", `a=1 b="2 3" c=foo`, map[string]string{
			"a": "1",
			"b": "2 3",
			"c": "foo",
		}},
		{"value with =", `a=1 b="2=3" c=foo`, map[string]string{
			"a": "1",
			"b": "2=3",
			"c": "foo",
		}},
		{"empty input", ``, map[string]string{}},
		{"single pair", `a=1`, map[string]string{
			"a": "1",
		}},
		{"quoted value at end", `a=1 msg="hello world"`, map[string]string{
			"a":   "1",
			"msg": "hello world",
		}},
		{"empty quoted value", `a="" b=2`, map[string]string{
			"a": "",
			"b": "2",
		}},
		{"extra whitespace", `  a=1    b=2  `, map[string]string{
			"a": "1",
			"b": "2",
		}},
		{"duplicate key last wins", `a=1 a=2`, map[string]string{
			"a": "2",
		}},
		{"realistic line", `level=error msg="db timeout" code=E42 duration=1.5s`, map[string]string{
			"level":    "error",
			"msg":      "db timeout",
			"code":     "E42",
			"duration": "1.5s",
		}},
		{"empty value at end", `a=`, map[string]string{
			"a": "",
		}},
		{"empty value in middle", `a= b=2`, map[string]string{
			"a": "",
			"b": "2",
		}},
		{"missing key is skipped", `=1`, map[string]string{}},
		{"missing key then pair", `=1 b=2`, map[string]string{
			"b": "2",
		}},
		{"unclosed quote runs to end", `a="unclosed`, map[string]string{
			"a": "unclosed",
		}},
		{"unclosed quote swallows rest", `a="unclosed b=2`, map[string]string{
			"a": "unclosed b=2",
		}},
	}

	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			result := parseLogfmt(d.input)
			if !maps.Equal(result, d.want) {
				t.Errorf("parseLogfmt(%q) = %q, want %q", d.input, result, d.want)
			}
		})
	}
}
