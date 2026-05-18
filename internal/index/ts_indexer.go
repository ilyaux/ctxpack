package index

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

var (
	tsImportFromRe    = regexp.MustCompile(`(?m)^\s*import\s+[^'"]*?\s+from\s+['"]([^'"]+)['"]`)
	tsImportSideRe    = regexp.MustCompile(`(?m)^\s*import\s+['"]([^'"]+)['"]`)
	tsExportFromRe    = regexp.MustCompile(`(?m)^\s*export\s+[^'"]*?\s+from\s+['"]([^'"]+)['"]`)
	tsRequireRe       = regexp.MustCompile(`\brequire\s*\(\s*['"]([^'"]+)['"]\s*\)`)
	tsDynamicImportRe = regexp.MustCompile(`\bimport\s*\(\s*['"]([^'"]+)['"]\s*\)`)
	tsFunctionRe      = regexp.MustCompile(`(?m)^\s*(?:export\s+)?(?:async\s+)?function\s+([A-Za-z_$][\w$]*)\s*\(([^)]*)\)`)
	tsDefaultFnRe     = regexp.MustCompile(`(?m)^\s*export\s+default\s+(?:async\s+)?function(?:\s+([A-Za-z_$][\w$]*))?\s*\(([^)]*)\)`)
	tsConstFnRe       = regexp.MustCompile(`(?m)^\s*(?:export\s+)?const\s+([A-Za-z_$][\w$]*)(?:\s*:\s*[^=]+)?\s*=\s*(?:React\.memo\s*\(|memo\s*\(|React\.forwardRef\s*\(|forwardRef\s*\()?\s*(?:async\s*)?(?:\([^)]*\)|[A-Za-z_$][\w$]*)\s*=>`)
	tsTypeRe          = regexp.MustCompile(`(?m)^\s*(?:export\s+)?(type|interface)\s+([A-Za-z_$][\w$]*)`)
	tsEnumRe          = regexp.MustCompile(`(?m)^\s*(?:export\s+)?enum\s+([A-Za-z_$][\w$]*)`)
	tsClassRe         = regexp.MustCompile(`(?m)^\s*(?:export\s+)?class\s+([A-Za-z_$][\w$]*)`)
	tsDefaultClassRe  = regexp.MustCompile(`(?m)^\s*export\s+default\s+class(?:\s+([A-Za-z_$][\w$]*))?`)
	tsRouteCallRe     = regexp.MustCompile(`\b(router|app)\.(get|post|put|patch|delete|use)\s*\(`)
	tsRouteDefRe      = regexp.MustCompile(`\b(router|app)\.(get|post|put|patch|delete|use)\s*\(\s*['"]([^'"]+)['"]`)
)

func analyzeTS(file *FileInfo, content string) {
	lowerPath := strings.ToLower(file.Path)
	file.IsTest = strings.Contains(lowerPath, ".test.") ||
		strings.Contains(lowerPath, ".spec.") ||
		strings.Contains(lowerPath, "__tests__/") ||
		strings.Contains(lowerPath, "/test/")
	file.IsRoute = routeLikePath(file.Path) ||
		strings.Contains(lowerPath, "/pages/api/") ||
		strings.Contains(lowerPath, "/app/api/") ||
		filepath.Base(lowerPath) == "route.ts" ||
		filepath.Base(lowerPath) == "route.tsx" ||
		tsRouteCallRe.MatchString(content)

	addMatches := func(re *regexp.Regexp) {
		for _, match := range re.FindAllStringSubmatch(content, -1) {
			if len(match) > 1 {
				file.Imports = append(file.Imports, match[1])
			}
		}
	}
	addMatches(tsImportFromRe)
	addMatches(tsImportSideRe)
	addMatches(tsExportFromRe)
	addMatches(tsRequireRe)
	addMatches(tsDynamicImportRe)

	for _, match := range tsFunctionRe.FindAllStringSubmatchIndex(content, -1) {
		name := content[match[2]:match[3]]
		params := content[match[4]:match[5]]
		addTSSymbol(file, Symbol{
			Name:      name,
			Kind:      tsFunctionKind(name, file.Path),
			Signature: fmt.Sprintf("export function %s(%s)", name, compactTS(params)),
			Line:      lineAt(content, match[0]),
			Exported:  strings.Contains(content[match[0]:match[1]], "export"),
		})
	}

	for _, match := range tsDefaultFnRe.FindAllStringSubmatchIndex(content, -1) {
		name := defaultTSName(file.Path)
		if match[2] >= 0 && match[3] >= 0 {
			name = content[match[2]:match[3]]
		}
		params := content[match[4]:match[5]]
		addTSSymbol(file, Symbol{
			Name:      name,
			Kind:      tsFunctionKind(name, file.Path),
			Signature: fmt.Sprintf("export default function %s(%s)", name, compactTS(params)),
			Line:      lineAt(content, match[0]),
			Exported:  true,
		})
	}

	for _, match := range tsConstFnRe.FindAllStringSubmatchIndex(content, -1) {
		name := content[match[2]:match[3]]
		addTSSymbol(file, Symbol{
			Name:      name,
			Kind:      tsFunctionKind(name, file.Path),
			Signature: fmt.Sprintf("export const %s = (...args) => ...", name),
			Line:      lineAt(content, match[0]),
			Exported:  strings.Contains(content[match[0]:match[1]], "export"),
		})
	}

	for _, match := range tsTypeRe.FindAllStringSubmatchIndex(content, -1) {
		kind := content[match[2]:match[3]]
		name := content[match[4]:match[5]]
		addTSSymbol(file, Symbol{
			Name:      name,
			Kind:      kind,
			Signature: fmt.Sprintf("export %s %s", kind, name),
			Line:      lineAt(content, match[0]),
			Exported:  strings.Contains(content[match[0]:match[1]], "export"),
		})
	}

	for _, match := range tsEnumRe.FindAllStringSubmatchIndex(content, -1) {
		name := content[match[2]:match[3]]
		addTSSymbol(file, Symbol{
			Name:      name,
			Kind:      "enum",
			Signature: fmt.Sprintf("export enum %s", name),
			Line:      lineAt(content, match[0]),
			Exported:  strings.Contains(content[match[0]:match[1]], "export"),
		})
	}

	for _, match := range tsClassRe.FindAllStringSubmatchIndex(content, -1) {
		name := content[match[2]:match[3]]
		addTSSymbol(file, Symbol{
			Name:      name,
			Kind:      "class",
			Signature: fmt.Sprintf("export class %s", name),
			Line:      lineAt(content, match[0]),
			Exported:  strings.Contains(content[match[0]:match[1]], "export"),
		})
	}

	for _, match := range tsDefaultClassRe.FindAllStringSubmatchIndex(content, -1) {
		name := defaultTSName(file.Path)
		if match[2] >= 0 && match[3] >= 0 {
			name = content[match[2]:match[3]]
		}
		addTSSymbol(file, Symbol{
			Name:      name,
			Kind:      "class",
			Signature: fmt.Sprintf("export default class %s", name),
			Line:      lineAt(content, match[0]),
			Exported:  true,
		})
	}

	for _, match := range tsRouteDefRe.FindAllStringSubmatchIndex(content, -1) {
		method := strings.ToUpper(content[match[4]:match[5]])
		path := content[match[6]:match[7]]
		addTSSymbol(file, Symbol{
			Name:      method + " " + path,
			Kind:      "route",
			Signature: content[match[0]:match[1]],
			Line:      lineAt(content, match[0]),
			Exported:  true,
		})
	}
}

func tsFunctionKind(name string, path string) string {
	switch name {
	case "GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS", "HEAD":
		if routeLikePath(path) || strings.Contains(strings.ToLower(filepath.Base(path)), "route.") {
			return "handler"
		}
	}
	if strings.HasPrefix(name, "use") && len(name) > 3 && unicode.IsUpper(rune(name[3])) {
		return "hook"
	}
	if len(name) > 0 && unicode.IsUpper(rune(name[0])) {
		return "component"
	}
	return "function"
}

func addTSSymbol(file *FileInfo, sym Symbol) {
	for _, existing := range file.Symbols {
		if existing.Name == sym.Name && existing.Kind == sym.Kind && existing.Line == sym.Line {
			return
		}
	}
	file.Symbols = append(file.Symbols, sym)
}

func defaultTSName(path string) string {
	noExt := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	if noExt == "page" || noExt == "layout" || noExt == "route" {
		parent := filepath.Base(filepath.Dir(path))
		return pascalCase(parent) + pascalCase(noExt)
	}
	return pascalCase(noExt)
}

func pascalCase(text string) string {
	parts := regexp.MustCompile(`[^A-Za-z0-9]+`).Split(text, -1)
	var b strings.Builder
	for _, part := range parts {
		if part == "" {
			continue
		}
		runes := []rune(part)
		b.WriteRune(unicode.ToUpper(runes[0]))
		if len(runes) > 1 {
			b.WriteString(string(runes[1:]))
		}
	}
	if b.Len() == 0 {
		return "Default"
	}
	return b.String()
}

func compactTS(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

func lineAt(content string, offset int) int {
	if offset <= 0 {
		return 1
	}
	line := 1
	for i, r := range content {
		if i >= offset {
			break
		}
		if r == '\n' {
			line++
		}
	}
	return line
}
