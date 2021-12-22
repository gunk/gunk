package generate

import "testing"

func TestPathToRoot(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{
			path:     "github.com/foo/bar",
			expected: "../../..",
		},
		{
			path:     "gitlab.com/foo/bar",
			expected: "../../..",
		},
		{
			path:     "github.com/foo/bar/baz",
			expected: "../../../..",
		},
	}
	for _, tc := range tests {
		res := pathToRoot(tc.path)
		if res != tc.expected {
			t.Errorf("wrong path to root from %q, got %q expected %q", tc.path, res, tc.expected)
		}
	}
}

func TestPathFromTo(t *testing.T) {
	tests := []struct {
		from     string
		to       string
		expected string
	}{
		{
			from:     "github.com/foo/bar",
			to:       "github.com/foo/baz",
			expected: "./../baz",
		},
		{
			from:     "github.com/foo/bar",
			to:       "github.com/foo/bar/baz",
			expected: "./baz",
		},
		{
			from:     "github.com/foo/bar/baz",
			to:       "github.com/foo/bar",
			expected: "./..",
		},
		{
			from:     "github.com/foo/bar/baz/abraca/dabra",
			to:       "github.com/serious/business",
			expected: "./../../../../../serious/business",
		},
		{
			from:     "gitlab.com/serious/business",
			to:       "gitlab.com/serious/business",
			expected: ".",
		},
		{
			from:     "github.com/foo/bar/baz",
			to:       "gitlab.com/foo/bar/baz",
			expected: "./../../../../gitlab.com/foo/bar/baz",
		},
	}
	for _, tc := range tests {
		res := pathFromTo(tc.from, tc.to)
		if res != tc.expected {
			t.Errorf("wrong path from %q to %q, got %q expected %q", tc.from, tc.to, res, tc.expected)
		}
	}
}
