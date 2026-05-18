package pathspec

import (
	pathpkg "path"
	"path/filepath"
	"strings"
)

func MatchAny(patterns []string, path string) bool {
	for _, pattern := range patterns {
		if Match(pattern, path) {
			return true
		}
	}
	return false
}

func Match(pattern string, path string) bool {
	pattern = filepath.ToSlash(strings.TrimSpace(pattern))
	path = filepath.ToSlash(strings.TrimSpace(path))
	if pattern == "" || path == "" {
		return false
	}
	pattern = strings.TrimPrefix(pattern, "./")
	path = strings.TrimPrefix(path, "./")

	if pattern == path {
		return true
	}
	if strings.HasSuffix(pattern, "/") {
		return strings.HasPrefix(path, pattern)
	}
	if strings.HasSuffix(pattern, "/**") {
		prefix := strings.TrimSuffix(pattern, "/**")
		return path == prefix || strings.HasPrefix(path, prefix+"/")
	}
	if strings.Contains(pattern, "**") {
		return matchDoubleStar(pattern, path)
	}
	if ok, _ := pathpkg.Match(pattern, path); ok {
		return true
	}
	if !strings.Contains(pattern, "/") {
		if ok, _ := pathpkg.Match(pattern, filepath.Base(path)); ok {
			return true
		}
	}
	return strings.HasPrefix(path, pattern+"/")
}

func matchDoubleStar(pattern string, path string) bool {
	parts := strings.Split(pattern, "**")
	pos := 0
	for i, part := range parts {
		if part == "" {
			continue
		}
		if i == 0 && !strings.HasPrefix(path, part) {
			return false
		}
		found := strings.Index(path[pos:], part)
		if found < 0 {
			return false
		}
		pos += found + len(part)
	}
	last := parts[len(parts)-1]
	return last == "" || strings.HasSuffix(path, last) || pos <= len(path)
}
