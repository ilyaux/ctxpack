package index

type RepoIndex struct {
	Root        string     `json:"root"`
	GeneratedAt string     `json:"generatedAt"`
	Stack       StackInfo  `json:"stack"`
	Files       []FileInfo `json:"files"`
}

type StackInfo struct {
	Languages       []string `json:"languages"`
	PackageManagers []string `json:"packageManagers"`
	Workspaces      []string `json:"workspaces"`
	GoModules       []string `json:"goModules"`
}

type FileInfo struct {
	Path            string   `json:"path"`
	AbsPath         string   `json:"-"`
	Language        string   `json:"language"`
	SizeBytes       int64    `json:"sizeBytes"`
	ModTimeUnixNano int64    `json:"modTimeUnixNano,omitempty"`
	EstimatedTokens int      `json:"estimatedTokens"`
	Package         string   `json:"package,omitempty"`
	Imports         []string `json:"imports,omitempty"`
	Symbols         []Symbol `json:"symbols,omitempty"`
	IsTest          bool     `json:"isTest,omitempty"`
	IsRoute         bool     `json:"isRoute,omitempty"`
	IsConfig        bool     `json:"isConfig,omitempty"`
	Content         string   `json:"-"`
}

type Symbol struct {
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	Signature string `json:"signature"`
	Line      int    `json:"line,omitempty"`
	Exported  bool   `json:"exported,omitempty"`
}

func (f FileInfo) FenceLanguage() string {
	switch f.Language {
	case "go":
		return "go"
	case "java":
		return "java"
	case "typescript", "typescriptreact":
		return "tsx"
	case "javascript", "javascriptreact":
		return "jsx"
	case "json":
		return "json"
	case "xml", "xhtml", "html":
		return "html"
	case "css":
		return "css"
	case "sql":
		return "sql"
	case "properties":
		return "properties"
	case "yaml":
		return "yaml"
	case "markdown":
		return "markdown"
	default:
		return ""
	}
}
