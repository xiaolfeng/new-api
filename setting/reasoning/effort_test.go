package reasoning

import "testing"

// TestNormalizeEffort 验证 max → xhigh 归一化与其余值原样透传。
func TestNormalizeEffort(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"max", "xhigh"},
		{"xhigh", "xhigh"},
		{"high", "high"},
		{"medium", "medium"},
		{"low", "low"},
		{"minimal", "minimal"},
		{"none", "none"},
		{"", ""},
	}
	for _, c := range cases {
		if got := NormalizeEffort(c.in); got != c.want {
			t.Errorf("NormalizeEffort(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
