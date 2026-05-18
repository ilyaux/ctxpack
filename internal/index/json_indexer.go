package index

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

func analyzeJSON(file *FileInfo, content string) {
	if filepath.Base(strings.ToLower(file.Path)) != "package.json" {
		return
	}
	var doc struct {
		Name            string            `json:"name"`
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if err := json.Unmarshal([]byte(content), &doc); err != nil {
		return
	}
	if doc.Name != "" {
		file.Package = doc.Name
		file.Symbols = append(file.Symbols, Symbol{
			Name:      doc.Name,
			Kind:      "package",
			Signature: fmt.Sprintf(`"name": "%s"`, doc.Name),
			Exported:  true,
		})
	}
	for name := range doc.Dependencies {
		file.Imports = append(file.Imports, name)
	}
	for name := range doc.DevDependencies {
		file.Imports = append(file.Imports, name)
	}
}
