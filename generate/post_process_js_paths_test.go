package generate
import "testing"
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
	}
	for _, tc := range tests {
		res := pathFromTo(tc.from, tc.to)
		if res != tc.expected {
			t.Errorf("wrong path from %q to %q, got %q expected %q", tc.from, tc.to, res, tc.expected)
		}
	}
}
