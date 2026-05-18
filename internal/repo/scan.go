package repo

import (
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
)

var supportedExtensions = map[string]bool{
	".go":         true,
	".java":       true,
	".ts":         true,
	".tsx":        true,
	".js":         true,
	".jsx":        true,
	".json":       true,
	".xml":        true,
	".xhtml":      true,
	".html":       true,
	".css":        true,
	".sql":        true,
	".properties": true,
	".jrxml":      true,
	".vm":         true,
	".yaml":       true,
	".yml":        true,
	".md":         true,
	".mod":        true,
	".sum":        true,
}

var supportedExactFiles = map[string]bool{
	"package.json":        true,
	"pom.xml":             true,
	"angular.json":        true,
	"pnpm-workspace.yaml": true,
	"yarn.lock":           true,
	"pnpm-lock.yaml":      true,
	"package-lock.json":   true,
	"go.mod":              true,
	"go.sum":              true,
	"tsconfig.json":       true,
	"vite.config.ts":      true,
	"next.config.js":      true,
	"next.config.mjs":     true,
	"tailwind.config.js":  true,
	"tailwind.config.ts":  true,
	"eslint.config.js":    true,
	"README.md":           true,
	"readme.md":           true,
	"docker-compose.yml":  true,
	"docker-compose.yaml": true,
	"buf.yaml":            true,
	"buf.gen.yaml":        true,
	"turbo.json":          true,
	"workspace.json":      true,
	"nx.json":             true,
	"CLAUDE.md":           true,
	"AGENTS.md":           true,
}

func Scan(root string) ([]string, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}

	matcher := NewIgnoreMatcher(absRoot)
	var files []string
	err = filepath.WalkDir(absRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == absRoot {
			return nil
		}
		if matcher.ShouldIgnore(path, entry) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		if !IsSupportedSource(path) {
			return nil
		}
		rel, err := filepath.Rel(absRoot, path)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

func IsSupportedSource(path string) bool {
	base := filepath.Base(path)
	if supportedExactFiles[base] {
		return true
	}
	ext := strings.ToLower(filepath.Ext(path))
	return supportedExtensions[ext]
}
