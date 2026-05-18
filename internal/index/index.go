package index

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ilyaux/ctxpack/internal/repo"
	"github.com/ilyaux/ctxpack/internal/tokens"
)

const maxIndexedFileBytes = 768 * 1024

type CacheStats struct {
	Path         string
	Loaded       bool
	ReusedFiles  int
	ParsedFiles  int
	RemovedFiles int
}

func Build(root string) (*RepoIndex, error) {
	idx, _, err := build(root, nil)
	return idx, err
}

func BuildCached(root string) (*RepoIndex, CacheStats, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, CacheStats{}, err
	}
	cachePath := DefaultCachePath(absRoot)
	cached, loaded := loadCache(cachePath)
	idx, stats, err := build(absRoot, cached)
	stats.Path = cachePath
	stats.Loaded = loaded
	return idx, stats, err
}

func DefaultCachePath(root string) string {
	return filepath.Join(root, ".ctxpack", "index.sqlite")
}

func build(root string, cached map[string]FileInfo) (*RepoIndex, CacheStats, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, CacheStats{}, err
	}

	paths, err := repo.Scan(absRoot)
	if err != nil {
		return nil, CacheStats{}, err
	}

	idx := &RepoIndex{
		Root:        absRoot,
		GeneratedAt: time.Now().Format(time.RFC3339),
	}
	stats := CacheStats{}
	seen := map[string]bool{}

	for _, rel := range paths {
		seen[rel] = true
		absPath := filepath.Join(absRoot, filepath.FromSlash(rel))
		info, err := os.Stat(absPath)
		if err != nil || info.IsDir() {
			continue
		}
		contentBytes, err := os.ReadFile(absPath)
		if err != nil {
			continue
		}
		if len(contentBytes) > maxIndexedFileBytes {
			contentBytes = contentBytes[:maxIndexedFileBytes]
		}

		file := FileInfo{
			Path:            rel,
			AbsPath:         absPath,
			Language:        detectLanguage(rel),
			SizeBytes:       info.Size(),
			ModTimeUnixNano: info.ModTime().UnixNano(),
			EstimatedTokens: tokens.Estimate(string(contentBytes)),
			IsConfig:        isConfig(rel),
			Content:         string(contentBytes),
		}
		if cachedFile, ok := cached[rel]; ok && cacheEntryReusable(cachedFile, file) {
			file.Package = cachedFile.Package
			file.Imports = cachedFile.Imports
			file.Symbols = cachedFile.Symbols
			file.IsTest = cachedFile.IsTest
			file.IsRoute = cachedFile.IsRoute
			file.IsConfig = cachedFile.IsConfig
			stats.ReusedFiles++
		} else {
			analyzeFile(&file, contentBytes)
			stats.ParsedFiles++
		}
		idx.Files = append(idx.Files, file)
	}
	for path := range cached {
		if !seen[path] {
			stats.RemovedFiles++
		}
	}

	idx.Stack = ComputeStack(absRoot, idx.Files)
	return idx, stats, nil
}

func Save(idx *RepoIndex, path string) error {
	if isSQLiteCachePath(path) {
		return saveSQLiteCache(idx, path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func loadCache(path string) (map[string]FileInfo, bool) {
	if isSQLiteCachePath(path) {
		files, err := loadSQLiteCache(path)
		if err != nil {
			return nil, false
		}
		return files, true
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	var idx RepoIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, false
	}
	out := make(map[string]FileInfo, len(idx.Files))
	for _, file := range idx.Files {
		if file.Path != "" {
			out[file.Path] = file
		}
	}
	return out, true
}

func isSQLiteCachePath(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".sqlite" || ext == ".sqlite3" || ext == ".db"
}

func cacheEntryReusable(cached FileInfo, current FileInfo) bool {
	return cached.SizeBytes == current.SizeBytes &&
		cached.ModTimeUnixNano == current.ModTimeUnixNano &&
		cached.Language == current.Language &&
		cached.EstimatedTokens == current.EstimatedTokens
}

func analyzeFile(file *FileInfo, content []byte) {
	switch file.Language {
	case "go":
		analyzeGo(file, content)
	case "java":
		analyzeJava(file, string(content))
	case "typescript", "typescriptreact", "javascript", "javascriptreact":
		analyzeTS(file, string(content))
	case "xhtml", "html", "xml":
		analyzeMarkup(file, string(content))
	case "json":
		analyzeJSON(file, string(content))
	default:
		file.IsRoute = routeLikePath(file.Path)
	}
}

func detectLanguage(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go", ".mod", ".sum":
		return "go"
	case ".java":
		return "java"
	case ".ts":
		return "typescript"
	case ".tsx":
		return "typescriptreact"
	case ".js":
		return "javascript"
	case ".jsx":
		return "javascriptreact"
	case ".json":
		return "json"
	case ".xml", ".jrxml":
		return "xml"
	case ".xhtml":
		return "xhtml"
	case ".html":
		return "html"
	case ".css":
		return "css"
	case ".sql":
		return "sql"
	case ".properties":
		return "properties"
	case ".vm":
		return "velocity"
	case ".yaml", ".yml":
		return "yaml"
	case ".md":
		return "markdown"
	default:
		return "text"
	}
}

func isConfig(path string) bool {
	base := strings.ToLower(filepath.Base(path))
	return strings.Contains(base, "config") ||
		base == "package.json" ||
		base == "pom.xml" ||
		base == "angular.json" ||
		base == "go.mod" ||
		base == "pnpm-workspace.yaml" ||
		base == "tsconfig.json" ||
		strings.HasSuffix(base, ".properties") ||
		strings.HasSuffix(base, ".yaml") ||
		strings.HasSuffix(base, ".yml")
}

func routeLikePath(path string) bool {
	lower := strings.ToLower(path)
	parts := strings.Split(lower, "/")
	for _, part := range parts {
		if part == "routes" || part == "router" || part == "handlers" || part == "controllers" || part == "controller" || part == "api" || part == "rest" || part == "endpoint" {
			return true
		}
	}
	base := filepath.Base(lower)
	return strings.Contains(base, "route") ||
		strings.Contains(base, "handler") ||
		strings.Contains(base, "controller") ||
		strings.Contains(base, "resource") ||
		strings.Contains(base, "endpoint") ||
		strings.Contains(base, "webhook")
}

func ComputeStack(root string, files []FileInfo) StackInfo {
	languages := map[string]bool{}
	pms := map[string]bool{}
	workspaces := map[string]bool{}
	goModules := map[string]bool{}

	for _, file := range files {
		switch file.Language {
		case "go":
			languages["Go"] = true
		case "java":
			languages["Java"] = true
		case "typescript", "typescriptreact":
			languages["TypeScript"] = true
		case "javascript", "javascriptreact":
			languages["JavaScript"] = true
		case "xhtml", "html":
			languages["HTML/XHTML"] = true
		case "sql":
			languages["SQL"] = true
		}
		base := filepath.Base(file.Path)
		switch base {
		case "pom.xml":
			pms["Maven"] = true
		case "pnpm-workspace.yaml":
			pms["pnpm"] = true
			workspaces["pnpm workspace"] = true
		case "yarn.lock":
			pms["yarn"] = true
		case "package-lock.json":
			pms["npm"] = true
		case "package.json":
			pms["npm"] = true
		}
		if strings.HasSuffix(file.Path, ".tsx") {
			languages["React"] = true
		}
		if base == "go.mod" {
			module := readGoModule(filepath.Join(root, filepath.FromSlash(file.Path)))
			if module != "" {
				goModules[module] = true
			}
		}
	}

	return StackInfo{
		Languages:       sortedKeys(languages),
		PackageManagers: sortedKeys(pms),
		Workspaces:      sortedKeys(workspaces),
		GoModules:       sortedKeys(goModules),
	}
}

func readGoModule(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module "))
		}
	}
	return ""
}

func sortedKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
