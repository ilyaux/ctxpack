package index

import (
	"regexp"
	"strings"
)

var (
	markupViewIdRe = regexp.MustCompile(`(?i)\b(id|action|outcome|href|url|viewId)\s*=\s*["']([^"']+)["']`)
	xmlTagRe       = regexp.MustCompile(`(?m)<([A-Za-z_][\w:.-]*)\b`)
)

func analyzeMarkup(file *FileInfo, content string) {
	analyzeMavenPOM(file, content)

	lowerPath := strings.ToLower(file.Path)
	file.IsRoute = routeLikePath(file.Path) ||
		strings.Contains(lowerPath, "/webapp/") ||
		strings.Contains(lowerPath, "/views/") ||
		strings.Contains(lowerPath, "/pages/")

	for _, match := range markupViewIdRe.FindAllStringSubmatchIndex(content, -1) {
		name := content[match[4]:match[5]]
		if len(name) > 80 {
			name = name[:80]
		}
		file.Symbols = append(file.Symbols, Symbol{
			Name:      name,
			Kind:      "view-ref",
			Signature: content[match[0]:match[1]],
			Line:      lineAt(content, match[0]),
			Exported:  true,
		})
		if len(file.Symbols) >= 20 {
			return
		}
	}

	seen := map[string]bool{}
	for _, match := range xmlTagRe.FindAllStringSubmatch(content, -1) {
		if len(match) < 2 || seen[match[1]] {
			continue
		}
		seen[match[1]] = true
		file.Symbols = append(file.Symbols, Symbol{
			Name:      match[1],
			Kind:      "tag",
			Signature: "<" + match[1] + ">",
			Exported:  true,
		})
		if len(file.Symbols) >= 12 {
			break
		}
	}
}
