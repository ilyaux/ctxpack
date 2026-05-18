package index

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	mavenGroupIDRe    = regexp.MustCompile(`(?is)<groupId>\s*([^<]+?)\s*</groupId>`)
	mavenArtifactIDRe = regexp.MustCompile(`(?is)<artifactId>\s*([^<]+?)\s*</artifactId>`)
	mavenModuleRe     = regexp.MustCompile(`(?is)<module>\s*([^<]+?)\s*</module>`)
	mavenDependencyRe = regexp.MustCompile(`(?is)<dependency\b[^>]*>(.*?)</dependency>`)
)

func analyzeMavenPOM(file *FileInfo, content string) {
	if filepath.Base(strings.ToLower(file.Path)) != "pom.xml" {
		return
	}
	groupID := firstXMLText(mavenGroupIDRe, content)
	artifactID := firstXMLText(mavenArtifactIDRe, content)
	if artifactID != "" {
		name := artifactID
		if groupID != "" {
			name = groupID + ":" + artifactID
			file.Package = name
		}
		file.Symbols = append(file.Symbols, Symbol{
			Name:      name,
			Kind:      "maven-artifact",
			Signature: fmt.Sprintf("<artifactId>%s</artifactId>", artifactID),
			Line:      lineAt(content, strings.Index(content, artifactID)),
			Exported:  true,
		})
	}

	for _, match := range mavenModuleRe.FindAllStringSubmatchIndex(content, -1) {
		module := strings.TrimSpace(content[match[2]:match[3]])
		file.Imports = append(file.Imports, module)
		file.Symbols = append(file.Symbols, Symbol{
			Name:      module,
			Kind:      "maven-module",
			Signature: fmt.Sprintf("<module>%s</module>", module),
			Line:      lineAt(content, match[0]),
			Exported:  true,
		})
	}

	for _, match := range mavenDependencyRe.FindAllStringSubmatchIndex(content, -1) {
		block := content[match[2]:match[3]]
		depGroup := firstXMLText(mavenGroupIDRe, block)
		depArtifact := firstXMLText(mavenArtifactIDRe, block)
		if depArtifact == "" {
			continue
		}
		name := depArtifact
		if depGroup != "" {
			name = depGroup + ":" + depArtifact
		}
		file.Imports = append(file.Imports, name, depArtifact)
		file.Symbols = append(file.Symbols, Symbol{
			Name:      name,
			Kind:      "maven-dependency",
			Signature: fmt.Sprintf("<dependency>%s</dependency>", name),
			Line:      lineAt(content, match[0]),
			Exported:  true,
		})
	}
}

func firstXMLText(re *regexp.Regexp, content string) string {
	match := re.FindStringSubmatch(content)
	if len(match) < 2 {
		return ""
	}
	return strings.TrimSpace(match[1])
}
