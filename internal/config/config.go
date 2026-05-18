package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ilyaux/ctxpack/internal/index"
	"github.com/ilyaux/ctxpack/internal/pathspec"
)

type Config struct {
	Budget       int
	IncludeTests *bool
	Ignore       []string
	Priority     []string
	Languages    map[string]bool
	CI           CIConfig
}

type CIConfig struct {
	Base        string
	Budget      int
	Output      string
	SummaryJSON string
	FailOn      []string
	Forbid      []string
}

func Load(root string) (Config, string, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return Config{}, "", err
	}
	for _, name := range []string{".ctxpack.yml", ".ctxpack.yaml"} {
		path := filepath.Join(absRoot, name)
		cfg, err := ParseFile(path)
		if err == nil {
			return cfg, path, nil
		}
		if !os.IsNotExist(err) {
			return Config{}, "", err
		}
	}
	return Config{Languages: map[string]bool{}}, "", nil
}

func ParseFile(path string) (Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return Config{}, err
	}
	defer file.Close()
	return Parse(file)
}

func Parse(r interface{ Read([]byte) (int, error) }) (Config, error) {
	cfg := Config{Languages: map[string]bool{}}
	scanner := bufio.NewScanner(r)
	section := ""
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := stripComment(scanner.Text())
		indent := leadingSpaces(line)
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasSuffix(trimmed, ":") && !strings.Contains(strings.TrimSuffix(trimmed, ":"), " ") {
			name := strings.TrimSuffix(trimmed, ":")
			if indent > 0 && strings.HasPrefix(section, "ci") {
				section = "ci." + name
			} else {
				section = name
			}
			continue
		}
		if strings.HasPrefix(trimmed, "- ") {
			value := strings.Trim(strings.TrimSpace(strings.TrimPrefix(trimmed, "- ")), `"'`)
			switch section {
			case "ignore":
				cfg.Ignore = append(cfg.Ignore, value)
			case "priority":
				cfg.Priority = append(cfg.Priority, value)
			case "ci.fail_on":
				cfg.CI.FailOn = append(cfg.CI.FailOn, value)
			case "ci.forbid":
				cfg.CI.Forbid = append(cfg.CI.Forbid, value)
			default:
				return cfg, fmt.Errorf("line %d: list item outside supported section", lineNo)
			}
			continue
		}

		key, value, ok := strings.Cut(trimmed, ":")
		if !ok {
			return cfg, fmt.Errorf("line %d: expected key: value", lineNo)
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if section == "languages" && value != "" {
			enabled, err := parseBool(value)
			if err != nil {
				return cfg, fmt.Errorf("line %d: %w", lineNo, err)
			}
			cfg.Languages[strings.ToLower(key)] = enabled
			continue
		}
		if section == "ci" {
			if err := parseCIKey(&cfg.CI, key, value, lineNo); err != nil {
				return cfg, err
			}
			continue
		}
		section = ""
		switch key {
		case "budget":
			budget, err := strconv.Atoi(value)
			if err != nil {
				return cfg, fmt.Errorf("line %d: invalid budget %q", lineNo, value)
			}
			cfg.Budget = budget
		case "include_tests":
			enabled, err := parseBool(value)
			if err != nil {
				return cfg, fmt.Errorf("line %d: %w", lineNo, err)
			}
			cfg.IncludeTests = &enabled
		default:
			return cfg, fmt.Errorf("line %d: unsupported key %q", lineNo, key)
		}
	}
	return cfg, scanner.Err()
}

func parseCIKey(ci *CIConfig, key string, value string, lineNo int) error {
	switch key {
	case "base":
		ci.Base = value
	case "budget":
		budget, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("line %d: invalid ci.budget %q", lineNo, value)
		}
		ci.Budget = budget
	case "output":
		ci.Output = value
	case "summary_json":
		ci.SummaryJSON = value
	case "fail_on":
		ci.FailOn = append(ci.FailOn, parseCSV(value)...)
	case "forbid":
		ci.Forbid = append(ci.Forbid, parseCSV(value)...)
	default:
		return fmt.Errorf("line %d: unsupported ci key %q", lineNo, key)
	}
	return nil
}

func Apply(idx *index.RepoIndex, cfg Config) *index.RepoIndex {
	if len(cfg.Ignore) == 0 && len(cfg.Languages) == 0 {
		return idx
	}
	copyIdx := *idx
	copyIdx.Files = make([]index.FileInfo, 0, len(idx.Files))
	for _, file := range idx.Files {
		if pathspec.MatchAny(cfg.Ignore, file.Path) {
			continue
		}
		if len(cfg.Languages) > 0 && !languageEnabled(cfg.Languages, file.Language) {
			continue
		}
		copyIdx.Files = append(copyIdx.Files, file)
	}
	copyIdx.Stack = index.ComputeStack(copyIdx.Root, copyIdx.Files)
	return &copyIdx
}

func stripComment(line string) string {
	if idx := strings.Index(line, "#"); idx >= 0 {
		return line[:idx]
	}
	return line
}

func leadingSpaces(line string) int {
	count := 0
	for _, r := range line {
		if r != ' ' {
			break
		}
		count++
	}
	return count
}

func parseCSV(value string) []string {
	var out []string
	for _, item := range strings.Split(value, ",") {
		item = strings.Trim(strings.TrimSpace(item), `"'`)
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}

func parseBool(value string) (bool, error) {
	switch strings.ToLower(value) {
	case "true", "yes", "on", "1":
		return true, nil
	case "false", "no", "off", "0":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean %q", value)
	}
}

func languageEnabled(languages map[string]bool, language string) bool {
	lang := strings.ToLower(language)
	if enabled, ok := languages[lang]; ok {
		return enabled
	}
	switch lang {
	case "typescriptreact":
		return languages["typescript"] || languages["react"]
	case "javascriptreact":
		return languages["javascript"] || languages["react"]
	case "xhtml":
		return languages["html"] || languages["xhtml"]
	case "xml":
		return languages["xml"]
	default:
		return false
	}
}
