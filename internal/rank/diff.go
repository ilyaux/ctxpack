package rank

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type ChangedFile struct {
	Path      string   `json:"path"`
	OldPath   string   `json:"old_path,omitempty"`
	Status    string   `json:"status"`
	Additions int      `json:"additions,omitempty"`
	Deletions int      `json:"deletions,omitempty"`
	Binary    bool     `json:"binary,omitempty"`
	Sources   []string `json:"sources,omitempty"`
}

func ChangedFiles(root string, base string) ([]string, error) {
	changes, err := ChangedFilesDetailed(root, base)
	if err != nil {
		return nil, err
	}
	return ChangedPaths(changes), nil
}

func ChangedFilesDetailed(root string, base string) ([]ChangedFile, error) {
	commands := []struct {
		args   []string
		source string
	}{
		{[]string{"diff", "--name-status", base + "...HEAD"}, base + "...HEAD"},
		{[]string{"diff", "--name-status", base}, base},
		{[]string{"diff", "--name-status", "--cached"}, "staged"},
		{[]string{"diff", "--name-status"}, "unstaged"},
	}

	byPath := map[string]ChangedFile{}
	var firstErr error
	for _, command := range commands {
		cmd := exec.Command("git", append([]string{"-C", root}, command.args...)...)
		data, err := cmd.Output()
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		for _, change := range parseNameStatus(string(data), command.source) {
			mergeChangedFile(byPath, change)
		}
	}

	for _, stat := range readDiffStats(root, base, &firstErr) {
		mergeChangedFile(byPath, stat)
	}

	data, err := exec.Command("git", "-C", root, "ls-files", "--others", "--exclude-standard").Output()
	if err != nil {
		if firstErr == nil {
			firstErr = err
		}
	} else {
		for _, line := range strings.Split(string(data), "\n") {
			path := filepath.ToSlash(strings.TrimSpace(line))
			if path == "" {
				continue
			}
			mergeChangedFile(byPath, ChangedFile{
				Path:      path,
				Status:    "untracked",
				Additions: countTextLines(filepath.Join(root, filepath.FromSlash(path))),
				Sources:   []string{"untracked"},
			})
		}
	}

	if len(byPath) == 0 && firstErr != nil {
		return nil, firstErr
	}
	out := make([]ChangedFile, 0, len(byPath))
	for _, change := range byPath {
		out = append(out, change)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Path < out[j].Path
	})
	return out, nil
}

func ChangedPaths(changes []ChangedFile) []string {
	out := make([]string, 0, len(changes))
	for _, change := range changes {
		if change.Path != "" {
			out = append(out, change.Path)
		}
	}
	return out
}

func readDiffStats(root string, base string, firstErr *error) []ChangedFile {
	commands := []struct {
		args   []string
		source string
	}{
		{[]string{"diff", "--numstat", base + "...HEAD"}, base + "...HEAD"},
		{[]string{"diff", "--numstat", base}, base},
		{[]string{"diff", "--numstat", "--cached"}, "staged"},
		{[]string{"diff", "--numstat"}, "unstaged"},
	}
	var out []ChangedFile
	for _, command := range commands {
		cmd := exec.Command("git", append([]string{"-C", root}, command.args...)...)
		data, err := cmd.Output()
		if err != nil {
			if firstErr != nil && *firstErr == nil {
				*firstErr = err
			}
			continue
		}
		out = append(out, parseNumstat(string(data), command.source)...)
	}
	return out
}

func parseNameStatus(data string, source string) []ChangedFile {
	var out []ChangedFile
	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 2 {
			continue
		}
		rawStatus := strings.TrimSpace(fields[0])
		change := ChangedFile{
			Status:  normalizeGitStatus(rawStatus),
			Sources: []string{source},
		}
		if strings.HasPrefix(rawStatus, "R") || strings.HasPrefix(rawStatus, "C") {
			if len(fields) < 3 {
				continue
			}
			change.OldPath = filepath.ToSlash(strings.TrimSpace(fields[1]))
			change.Path = filepath.ToSlash(strings.TrimSpace(fields[2]))
		} else {
			change.Path = filepath.ToSlash(strings.TrimSpace(fields[1]))
		}
		if change.Path != "" {
			out = append(out, change)
		}
	}
	return out
}

func parseNumstat(data string, source string) []ChangedFile {
	var out []ChangedFile
	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 3 {
			continue
		}
		change := ChangedFile{
			Path:    normalizeNumstatPath(strings.TrimSpace(fields[len(fields)-1])),
			Sources: []string{source},
		}
		if fields[0] == "-" || fields[1] == "-" {
			change.Binary = true
		} else {
			change.Additions, _ = strconv.Atoi(strings.TrimSpace(fields[0]))
			change.Deletions, _ = strconv.Atoi(strings.TrimSpace(fields[1]))
		}
		if change.Path != "" {
			out = append(out, change)
		}
	}
	return out
}

func normalizeNumstatPath(path string) string {
	path = filepath.ToSlash(strings.TrimSpace(path))
	if strings.Contains(path, " => ") {
		if end := strings.LastIndex(path, "}"); end >= 0 && end+1 < len(path) {
			return strings.TrimPrefix(strings.TrimSpace(path[end+1:]), "/")
		}
		parts := strings.Split(path, " => ")
		path = parts[len(parts)-1]
		path = strings.Trim(path, "{} ")
	}
	return path
}

func normalizeGitStatus(status string) string {
	switch {
	case status == "A":
		return "added"
	case status == "D":
		return "deleted"
	case status == "M":
		return "modified"
	case status == "T":
		return "type changed"
	case status == "U":
		return "unmerged"
	case strings.HasPrefix(status, "R"):
		return "renamed"
	case strings.HasPrefix(status, "C"):
		return "copied"
	default:
		if status == "" {
			return "changed"
		}
		return strings.ToLower(status)
	}
}

func mergeChangedFile(byPath map[string]ChangedFile, change ChangedFile) {
	if change.Path == "" {
		return
	}
	change.Path = filepath.ToSlash(change.Path)
	if isCtxpackArtifact(change.Path) {
		return
	}
	change.OldPath = filepath.ToSlash(change.OldPath)
	existing, ok := byPath[change.Path]
	if !ok {
		change.Sources = uniqueSorted(change.Sources)
		byPath[change.Path] = change
		return
	}
	if existing.Status == "" || existing.Status == "changed" {
		existing.Status = change.Status
	}
	if existing.OldPath == "" {
		existing.OldPath = change.OldPath
	}
	if change.Additions > existing.Additions {
		existing.Additions = change.Additions
	}
	if change.Deletions > existing.Deletions {
		existing.Deletions = change.Deletions
	}
	existing.Binary = existing.Binary || change.Binary
	existing.Sources = uniqueSorted(append(existing.Sources, change.Sources...))
	byPath[change.Path] = existing
}

func uniqueSorted(values []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func isCtxpackArtifact(path string) bool {
	path = filepath.ToSlash(strings.TrimSpace(path))
	return path == ".ctxpack" || strings.HasPrefix(path, ".ctxpack/")
}

func countTextLines(path string) int {
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 || looksBinary(data) {
		return 0
	}
	count := strings.Count(string(data), "\n")
	if data[len(data)-1] != '\n' {
		count++
	}
	return count
}

func looksBinary(data []byte) bool {
	limit := len(data)
	if limit > 8000 {
		limit = 8000
	}
	for i := 0; i < limit; i++ {
		if data[i] == 0 {
			return true
		}
	}
	return false
}
