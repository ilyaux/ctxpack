package pathspec

import "testing"

func TestMatch(t *testing.T) {
	tests := []struct {
		pattern string
		path    string
		want    bool
	}{
		{"target/**", "target/generated/Foo.java", true},
		{"services/billing/**", "services/billing/api/routes.go", true},
		{"*.md", "README.md", true},
		{"*.md", "docs/README.md", true},
		{"apps/*/pom.xml", "apps/web/pom.xml", true},
		{"apps/*/pom.xml", "apps/web/nested/pom.xml", false},
	}
	for _, tt := range tests {
		if got := Match(tt.pattern, tt.path); got != tt.want {
			t.Fatalf("Match(%q, %q) = %v, want %v", tt.pattern, tt.path, got, tt.want)
		}
	}
}
