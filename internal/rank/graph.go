package rank

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/ilyaux/ctxpack/internal/index"
)

type localGraph struct {
	deps      map[string][]string
	importers map[string][]string
	byPath    map[string]index.FileInfo
}

func applyRelatedFileBoosts(scored map[string]ScoredFile, idx *index.RepoIndex, opt Options) {
	graph := buildLocalGraph(idx.Files)
	if len(graph.deps) == 0 {
		return
	}

	seeds := topSeedPaths(scored, 25)
	for _, seed := range seeds {
		seedScore := scored[seed].Score
		if seedScore < 24 && !containsPath(opt.DiffFiles, seed) {
			continue
		}
		for _, dep := range graph.deps[seed] {
			boostRelated(scored, graph.byPath[dep], relatedBoost(seedScore, 8), "imported by relevant file "+seed)
		}
		for _, importer := range graph.importers[seed] {
			boostRelated(scored, graph.byPath[importer], relatedBoost(seedScore, 6), "imports relevant file "+seed)
		}
		if opt.IncludeTests && canHaveNearbyTests(seed) {
			for _, test := range nearbyTests(seed, idx.Files) {
				boostRelated(scored, test, relatedBoost(seedScore, 6), "nearby test for relevant file "+seed)
			}
		}
	}
}

func buildLocalGraph(files []index.FileInfo) localGraph {
	graph := localGraph{
		deps:      map[string][]string{},
		importers: map[string][]string{},
		byPath:    map[string]index.FileInfo{},
	}
	javaTypes := map[string]string{}
	javaPackages := map[string][]string{}
	tsModules := map[string]string{}
	packageRoots := map[string]string{}
	mavenArtifacts := map[string]string{}
	goPackages := map[string][]string{}

	for _, file := range files {
		graph.byPath[file.Path] = file
		switch file.Language {
		case "java":
			if file.Package != "" {
				javaPackages[file.Package] = append(javaPackages[file.Package], file.Path)
			}
			for _, sym := range file.Symbols {
				if file.Package != "" && isJavaType(sym.Kind) {
					javaTypes[file.Package+"."+sym.Name] = file.Path
				}
			}
		case "json":
			for _, sym := range file.Symbols {
				if sym.Kind == "package" {
					packageRoots[sym.Name] = filepath.Dir(file.Path)
				}
			}
		case "xml":
			if filepath.Base(file.Path) == "pom.xml" {
				for _, sym := range file.Symbols {
					if sym.Kind == "maven-artifact" {
						mavenArtifacts[sym.Name] = file.Path
						if artifact := artifactID(sym.Name); artifact != "" {
							mavenArtifacts[artifact] = file.Path
						}
					}
				}
			}
		case "typescript", "typescriptreact", "javascript", "javascriptreact":
			for _, module := range tsModuleNames(file.Path) {
				tsModules[module] = file.Path
			}
		case "go":
			if file.Package != "" {
				dir := filepath.Dir(file.Path)
				goPackages[filepath.ToSlash(dir)] = append(goPackages[filepath.ToSlash(dir)], file.Path)
			}
		}
	}
	addPackageTSModules(tsModules, packageRoots, files)

	for _, file := range files {
		depSet := map[string]bool{}
		for _, imported := range file.Imports {
			for _, dep := range resolveImport(file, imported, javaTypes, javaPackages, tsModules, goPackages, mavenArtifacts) {
				if dep != "" && dep != file.Path {
					depSet[dep] = true
				}
			}
		}
		for dep := range depSet {
			graph.deps[file.Path] = append(graph.deps[file.Path], dep)
			graph.importers[dep] = append(graph.importers[dep], file.Path)
		}
		sort.Strings(graph.deps[file.Path])
	}
	for path := range graph.importers {
		sort.Strings(graph.importers[path])
	}
	return graph
}

func resolveImport(file index.FileInfo, imported string, javaTypes map[string]string, javaPackages map[string][]string, tsModules map[string]string, goPackages map[string][]string, mavenArtifacts map[string]string) []string {
	imported = strings.TrimSpace(imported)
	if imported == "" {
		return nil
	}
	switch file.Language {
	case "java":
		if strings.HasSuffix(imported, ".*") {
			pkg := strings.TrimSuffix(imported, ".*")
			deps := append([]string(nil), javaPackages[pkg]...)
			sort.Strings(deps)
			if len(deps) > 8 {
				deps = deps[:8]
			}
			return deps
		}
		if dep := javaTypes[imported]; dep != "" {
			return []string{dep}
		}
	case "typescript", "typescriptreact", "javascript", "javascriptreact":
		if !strings.HasPrefix(imported, ".") {
			if dep := tsModules[imported]; dep != "" {
				return []string{dep}
			}
			return nil
		}
		base := filepath.ToSlash(filepath.Join(filepath.Dir(file.Path), imported))
		if dep := tsModules[base]; dep != "" {
			return []string{dep}
		}
	case "xml":
		if filepath.Base(file.Path) == "pom.xml" {
			if dep := mavenArtifacts[imported]; dep != "" {
				return []string{dep}
			}
		}
	case "go":
		imported = filepath.ToSlash(imported)
		var deps []string
		for dir, files := range goPackages {
			if strings.HasSuffix(imported, dir) {
				deps = append(deps, files...)
			}
		}
		sort.Strings(deps)
		if len(deps) > 6 {
			deps = deps[:6]
		}
		return deps
	}
	return nil
}

func addPackageTSModules(tsModules map[string]string, packageRoots map[string]string, files []index.FileInfo) {
	for _, file := range files {
		switch file.Language {
		case "typescript", "typescriptreact", "javascript", "javascriptreact":
		default:
			continue
		}
		for packageName, root := range packageRoots {
			root = filepath.ToSlash(root)
			path := filepath.ToSlash(file.Path)
			if path != root && !strings.HasPrefix(path, root+"/") {
				continue
			}
			noExt := strings.TrimSuffix(path, filepath.Ext(path))
			rel := strings.TrimPrefix(strings.TrimPrefix(noExt, root), "/")
			if rel == "index" || rel == "src/index" {
				tsModules[packageName] = file.Path
			}
			if strings.HasPrefix(rel, "src/") {
				tsModules[packageName+"/"+strings.TrimPrefix(rel, "src/")] = file.Path
			}
			if rel != "" {
				tsModules[packageName+"/"+rel] = file.Path
			}
		}
	}
}

func artifactID(coordinate string) string {
	parts := strings.Split(coordinate, ":")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

func tsModuleNames(path string) []string {
	noExt := strings.TrimSuffix(filepath.ToSlash(path), filepath.Ext(path))
	names := []string{noExt}
	base := filepath.Base(noExt)
	if base == "index" {
		names = append(names, filepath.Dir(noExt))
	}
	return names
}

func isJavaType(kind string) bool {
	switch kind {
	case "class", "interface", "enum", "record", "controller", "service", "repository":
		return true
	default:
		return false
	}
}

func topSeedPaths(scored map[string]ScoredFile, limit int) []string {
	items := make([]ScoredFile, 0, len(scored))
	for _, item := range scored {
		items = append(items, item)
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Score == items[j].Score {
			return items[i].File.Path < items[j].File.Path
		}
		return items[i].Score > items[j].Score
	})
	if len(items) > limit {
		items = items[:limit]
	}
	paths := make([]string, 0, len(items))
	for _, item := range items {
		paths = append(paths, item.File.Path)
	}
	return paths
}

func boostRelated(scored map[string]ScoredFile, file index.FileInfo, points float64, reason string) {
	if file.Path == "" || points <= 0 {
		return
	}
	item := scored[file.Path]
	if item.File.Path == "" {
		item.File = file
	}
	item.Score += points
	item.Components = addComponent(item.Components, reason, points)
	if !contains(item.Reasons, reason) && len(item.Reasons) < 5 {
		item.Reasons = append(item.Reasons, reason)
	}
	scored[file.Path] = item
}

func relatedBoost(seedScore float64, cap float64) float64 {
	boost := seedScore * 0.16
	if boost > cap {
		return cap
	}
	if boost < 4 {
		return 4
	}
	return boost
}

func nearbyTests(path string, files []index.FileInfo) []index.FileInfo {
	dir := filepath.Dir(path)
	base := strings.ToLower(strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)))
	var out []index.FileInfo
	for _, file := range files {
		if !file.IsTest {
			continue
		}
		fileDir := filepath.Dir(file.Path)
		fileBase := strings.ToLower(filepath.Base(file.Path))
		if fileDir == dir || strings.Contains(fileBase, base) || strings.Contains(strings.ToLower(file.Content), base) {
			out = append(out, file)
		}
	}
	return out
}

func canHaveNearbyTests(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go", ".java", ".ts", ".tsx", ".js", ".jsx":
		return true
	default:
		return false
	}
}

func containsPath(paths []string, path string) bool {
	for _, item := range paths {
		if filepath.ToSlash(item) == path {
			return true
		}
	}
	return false
}
