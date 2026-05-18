package index

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	javaPackageRe         = regexp.MustCompile(`(?m)^\s*package\s+([A-Za-z_][\w.]*);`)
	javaImportRe          = regexp.MustCompile(`(?m)^\s*import\s+(?:static\s+)?([A-Za-z_][\w.*]*);`)
	javaTypeRe            = regexp.MustCompile(`(?m)^\s*(?:@\w+(?:\([^)]*\))?\s*)*(?:public|protected|private|abstract|final|static|\s)*\b(class|interface|enum|record)\s+([A-Za-z_]\w*)`)
	javaMethodRe          = regexp.MustCompile(`(?m)^\s*(?:@\w+(?:\([^)]*\))?\s*)*(?:public|protected|private|static|final|abstract|synchronized|native|\s)+([A-Za-z_][\w<>\[\], ?.]*)\s+([A-Za-z_]\w*)\s*\(([^;{}]*)\)\s*(?:throws\s+[^{]+)?\{?`)
	javaAnnotationRe      = regexp.MustCompile(`@(?:[A-Za-z_]\w*\.)*(RequestMapping|GetMapping|PostMapping|PutMapping|DeleteMapping|PatchMapping|Path|GET|POST|PUT|DELETE|ApplicationScoped|Stateless|Controller|RestController|Service|Repository|ManagedBean|Named)\b`)
	javaRouteAnnotationRe = regexp.MustCompile(`@(?:[A-Za-z_]\w*\.)*(RequestMapping|GetMapping|PostMapping|PutMapping|DeleteMapping|PatchMapping|Path|GET|POST|PUT|DELETE)\b(?:\s*\(\s*(?:"([^"]+)"|'([^']+)'|value\s*=\s*"([^"]+)"))?`)
)

func analyzeJava(file *FileInfo, content string) {
	lowerPath := strings.ToLower(file.Path)
	file.IsTest = strings.Contains(lowerPath, "/test/") ||
		strings.HasSuffix(lowerPath, "test.java") ||
		strings.HasSuffix(lowerPath, "tests.java") ||
		strings.Contains(lowerPath, "selenium")
	file.IsRoute = routeLikePath(file.Path) || javaAnnotationRe.MatchString(content)

	if match := javaPackageRe.FindStringSubmatch(content); len(match) > 1 {
		file.Package = match[1]
	}
	for _, match := range javaImportRe.FindAllStringSubmatch(content, -1) {
		if len(match) > 1 {
			file.Imports = append(file.Imports, match[1])
		}
	}

	for _, match := range javaTypeRe.FindAllStringSubmatchIndex(content, -1) {
		kind := content[match[2]:match[3]]
		name := content[match[4]:match[5]]
		if annotatedJavaKind(content, match[0], match[1]) != "" {
			kind = annotatedJavaKind(content, match[0], match[1])
		}
		file.Symbols = append(file.Symbols, Symbol{
			Name:      name,
			Kind:      kind,
			Signature: fmt.Sprintf("%s %s", kind, name),
			Line:      lineAt(content, match[0]),
			Exported:  true,
		})
	}

	for _, match := range javaMethodRe.FindAllStringSubmatchIndex(content, -1) {
		returnType := compactTS(content[match[2]:match[3]])
		name := content[match[4]:match[5]]
		params := compactTS(content[match[6]:match[7]])
		if javaKeywordLikeMethod(name) {
			continue
		}
		file.Symbols = append(file.Symbols, Symbol{
			Name:      name,
			Kind:      javaMethodKind(name, file.Path, content, match[0]),
			Signature: fmt.Sprintf("%s %s(%s)", returnType, name, params),
			Line:      lineAt(content, match[0]),
			Exported:  true,
		})
	}

	for _, match := range javaRouteAnnotationRe.FindAllStringSubmatchIndex(content, -1) {
		name := javaRouteName(content, match)
		file.Symbols = append(file.Symbols, Symbol{
			Name:      name,
			Kind:      "route",
			Signature: content[match[0]:match[1]],
			Line:      lineAt(content, match[0]),
			Exported:  true,
		})
	}

	base := strings.ToLower(filepath.Base(file.Path))
	if strings.Contains(base, "controller") || strings.Contains(base, "resource") || strings.Contains(base, "endpoint") {
		file.IsRoute = true
	}
}

func javaKeywordLikeMethod(name string) bool {
	switch name {
	case "if", "for", "while", "switch", "catch", "return", "new":
		return true
	default:
		return false
	}
}

func javaMethodKind(name string, path string, content string, offset int) string {
	lowerPath := strings.ToLower(path)
	if javaRouteAnnotationRe.MatchString(precedingJavaAnnotations(content, offset)) {
		return "handler"
	}
	if strings.Contains(lowerPath, "controller") || strings.Contains(lowerPath, "resource") {
		return "handler"
	}
	if strings.HasPrefix(name, "get") || strings.HasPrefix(name, "set") || strings.HasPrefix(name, "is") {
		return "accessor"
	}
	return "method"
}

func annotatedJavaKind(content string, start int, end int) string {
	annotations := precedingJavaAnnotations(content, start) + content[start:end]
	switch {
	case javaAnnotationNamed(annotations, "RestController") || javaAnnotationNamed(annotations, "Controller"):
		return "controller"
	case javaAnnotationNamed(annotations, "Service") || javaAnnotationNamed(annotations, "Stateless") || javaAnnotationNamed(annotations, "ApplicationScoped"):
		return "service"
	case javaAnnotationNamed(annotations, "Repository"):
		return "repository"
	default:
		return ""
	}
}

func javaAnnotationNamed(text string, name string) bool {
	return regexp.MustCompile(`@(?:[A-Za-z_]\w*\.)*` + regexp.QuoteMeta(name) + `\b`).MatchString(text)
}

func precedingJavaAnnotations(content string, offset int) string {
	start := offset - 320
	if start < 0 {
		start = 0
	}
	return content[start:offset]
}

func javaRouteName(content string, match []int) string {
	for _, pair := range [][2]int{{4, 5}, {6, 7}, {8, 9}} {
		if pair[0] < len(match) && match[pair[0]] >= 0 && match[pair[1]] >= 0 {
			return content[match[pair[0]]:match[pair[1]]]
		}
	}
	text := content[match[0]:match[1]]
	text = strings.TrimPrefix(text, "@")
	if idx := strings.Index(text, "("); idx >= 0 {
		text = text[:idx]
	}
	return strings.TrimSpace(text)
}
