package repo

import (
	"bufio"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type IgnoreMatcher struct {
	root     string
	patterns []ignorePattern
}

type ignorePattern struct {
	raw      string
	pattern  string
	dirOnly  bool
	anchored bool
}

var builtInIgnoredDirs = map[string]bool{
	".git":          true,
	".hg":           true,
	".svn":          true,
	".ctxpack":      true,
	"node_modules":  true,
	"vendor":        true,
	"dist":          true,
	"build":         true,
	"out":           true,
	"coverage":      true,
	".next":         true,
	".nuxt":         true,
	".turbo":        true,
	".cache":        true,
	"target":        true,
	"bin":           true,
	"obj":           true,
	"tmp":           true,
	".idea":         true,
	".vscode":       true,
	"__pycache__":   true,
	".pytest_cache": true,
}

func NewIgnoreMatcher(root string) *IgnoreMatcher {
	m := &IgnoreMatcher{root: root}
	m.load(filepath.Join(root, ".gitignore"))
	return m
}

func (m *IgnoreMatcher) load(path string) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "!") {
			continue
		}
		line = filepath.ToSlash(line)
		p := ignorePattern{raw: line, pattern: strings.TrimPrefix(line, "/")}
		p.anchored = strings.HasPrefix(line, "/")
		p.dirOnly = strings.HasSuffix(p.pattern, "/")
		p.pattern = strings.TrimSuffix(p.pattern, "/")
		if p.pattern != "" {
			m.patterns = append(m.patterns, p)
		}
	}
}

func (m *IgnoreMatcher) ShouldIgnore(path string, entry fs.DirEntry) bool {
	name := entry.Name()
	if entry.IsDir() && builtInIgnoredDirs[name] {
		return true
	}

	rel, err := filepath.Rel(m.root, path)
	if err != nil {
		return false
	}
	rel = filepath.ToSlash(rel)
	if rel == "." {
		return false
	}

	for _, pattern := range m.patterns {
		if m.match(pattern, rel, name, entry.IsDir()) {
			return true
		}
	}
	return false
}

func (m *IgnoreMatcher) match(pattern ignorePattern, rel string, name string, isDir bool) bool {
	if pattern.dirOnly {
		if !isDir && !strings.HasPrefix(rel, pattern.pattern+"/") {
			return false
		}
		return rel == pattern.pattern || strings.HasPrefix(rel, pattern.pattern+"/") || name == pattern.pattern
	}

	if pattern.anchored || strings.Contains(pattern.pattern, "/") {
		ok, _ := filepath.Match(pattern.pattern, rel)
		return ok || rel == pattern.pattern || strings.HasPrefix(rel, pattern.pattern+"/")
	}

	ok, _ := filepath.Match(pattern.pattern, name)
	if ok {
		return true
	}
	for _, part := range strings.Split(rel, "/") {
		ok, _ := filepath.Match(pattern.pattern, part)
		if ok {
			return true
		}
	}
	return false
}
