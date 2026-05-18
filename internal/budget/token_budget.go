package budget

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/ilyaux/ctxpack/internal/index"
	"github.com/ilyaux/ctxpack/internal/rank"
	"github.com/ilyaux/ctxpack/internal/tokens"
)

type Mode string

const (
	ModeFull       Mode = "full file"
	ModeSlices     Mode = "relevant slices"
	ModeSignatures Mode = "signatures only"
	ModeSummary    Mode = "summary only"
	ModeOmitted    Mode = "omitted"
)

type Selection struct {
	Scored          rank.ScoredFile
	Mode            Mode
	EstimatedTokens int
}

func Select(scored []rank.ScoredFile, tokenBudget int) []Selection {
	if tokenBudget <= 0 {
		tokenBudget = 12000
	}
	reserve := 1200
	if tokenBudget < 6000 {
		reserve = 700
	}
	if tokenBudget > 20000 {
		reserve = 2000
	}
	remaining := tokenBudget - reserve
	if remaining < tokenBudget/2 {
		remaining = tokenBudget / 2
	}

	maxCandidates := 30
	if len(scored) < maxCandidates {
		maxCandidates = len(scored)
	}

	selections := make([]Selection, 0, maxCandidates)
	omitted := 0
	for i := 0; i < maxCandidates; i++ {
		item := scored[i]
		fullCost := fullCost(item.File)
		slicesCost := sliceCost(item)
		sigCost := signatureCost(item.File)
		summaryCost := summaryCost(item.File)

		mode := ModeOmitted
		cost := 0
		switch {
		case fullCost <= remaining && shouldIncludeFull(i, fullCost, remaining):
			mode = ModeFull
			cost = fullCost
			remaining -= cost
		case slicesCost > 0 && slicesCost < fullCost && slicesCost <= remaining:
			mode = ModeSlices
			cost = slicesCost
			remaining -= cost
		case sigCost <= remaining:
			mode = ModeSignatures
			cost = sigCost
			remaining -= cost
		case summaryCost <= remaining:
			mode = ModeSummary
			cost = summaryCost
			remaining -= cost
		default:
			omitted++
			if omitted > 10 {
				continue
			}
		}
		selections = append(selections, Selection{
			Scored:          item,
			Mode:            mode,
			EstimatedTokens: cost,
		})
	}
	return selections
}

func DowngradeLeastImportant(selections []Selection) bool {
	for _, mode := range []Mode{ModeFull, ModeSlices, ModeSignatures, ModeSummary} {
		for i := len(selections) - 1; i >= 0; i-- {
			if selections[i].Mode != mode {
				continue
			}
			switch mode {
			case ModeFull:
				if sliceCost(selections[i].Scored) > 0 {
					selections[i].Mode = ModeSlices
					selections[i].EstimatedTokens = sliceCost(selections[i].Scored)
				} else {
					selections[i].Mode = ModeSignatures
					selections[i].EstimatedTokens = signatureCost(selections[i].Scored.File)
				}
			case ModeSlices:
				selections[i].Mode = ModeSignatures
				selections[i].EstimatedTokens = signatureCost(selections[i].Scored.File)
			case ModeSignatures:
				selections[i].Mode = ModeSummary
				selections[i].EstimatedTokens = summaryCost(selections[i].Scored.File)
			case ModeSummary:
				selections[i].Mode = ModeOmitted
				selections[i].EstimatedTokens = 0
			}
			return true
		}
	}
	return false
}

var taskTermReasonRe = regexp.MustCompile(`task term ([a-z0-9]+)`)

type lineRange struct {
	start int
	end   int
}

func SliceBlock(scored rank.ScoredFile) string {
	file := scored.File
	if strings.TrimSpace(file.Content) == "" {
		return ""
	}
	terms := termsFromReasons(scored.Reasons)
	if len(terms) == 0 {
		return ""
	}

	lines := strings.Split(file.Content, "\n")
	var ranges []lineRange
	for _, sym := range file.Symbols {
		if sym.Line <= 0 || !symbolMatchesTerms(sym, terms) {
			continue
		}
		ranges = append(ranges, lineRange{
			start: clamp(sym.Line-3, 1, len(lines)),
			end:   clamp(sym.Line+16, 1, len(lines)),
		})
	}

	lowerTerms := make([]string, 0, len(terms))
	for _, term := range terms {
		lowerTerms = append(lowerTerms, strings.ToLower(term))
	}
	for i, line := range lines {
		if len(ranges) >= 8 {
			break
		}
		lower := strings.ToLower(line)
		for _, term := range lowerTerms {
			if strings.Contains(lower, term) {
				lineNo := i + 1
				ranges = append(ranges, lineRange{
					start: clamp(lineNo-3, 1, len(lines)),
					end:   clamp(lineNo+6, 1, len(lines)),
				})
				break
			}
		}
	}
	if len(ranges) == 0 {
		return ""
	}
	ranges = mergeRanges(ranges, 90)
	if len(ranges) == 0 {
		return ""
	}

	var b strings.Builder
	for i, r := range ranges {
		if i > 0 {
			b.WriteString("\n\n")
		}
		fmt.Fprintf(&b, "%s %s:%d-%d\n", snippetCommentPrefix(file), file.Path, r.start, r.end)
		for lineNo := r.start; lineNo <= r.end; lineNo++ {
			if lineNo < 1 || lineNo > len(lines) {
				continue
			}
			fmt.Fprintf(&b, "%4d | %s\n", lineNo, lines[lineNo-1])
		}
	}
	return strings.TrimSpace(b.String())
}

func SignatureBlock(file index.FileInfo) string {
	if len(file.Symbols) == 0 {
		return Summary(file)
	}
	var b strings.Builder
	for _, sym := range file.Symbols {
		if sym.Signature == "" {
			continue
		}
		if sym.Line > 0 {
			fmt.Fprintf(&b, "// %s:%d\n", file.Path, sym.Line)
		}
		b.WriteString(sym.Signature)
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}

func Summary(file index.FileInfo) string {
	parts := []string{
		fmt.Sprintf("%s (%s, ~%d tokens)", file.Path, file.Language, file.EstimatedTokens),
	}
	if file.Package != "" {
		parts = append(parts, "package "+file.Package)
	}
	if file.IsRoute {
		parts = append(parts, "route/API boundary")
	}
	if file.IsTest {
		parts = append(parts, "test file")
	}
	if len(file.Imports) > 0 {
		maxImports := len(file.Imports)
		if maxImports > 8 {
			maxImports = 8
		}
		parts = append(parts, "imports: "+strings.Join(file.Imports[:maxImports], ", "))
	}
	if len(file.Symbols) > 0 {
		names := make([]string, 0, len(file.Symbols))
		for i, sym := range file.Symbols {
			if i >= 12 {
				names = append(names, "...")
				break
			}
			names = append(names, sym.Name)
		}
		parts = append(parts, "symbols: "+strings.Join(names, ", "))
	}
	return strings.Join(parts, "; ")
}

func fullCost(file index.FileInfo) int {
	return tokens.Estimate(file.Content) + 90
}

func sliceCost(scored rank.ScoredFile) int {
	block := SliceBlock(scored)
	if block == "" {
		return 0
	}
	return tokens.Estimate(block) + 80
}

func signatureCost(file index.FileInfo) int {
	return tokens.Estimate(SignatureBlock(file)) + 70
}

func summaryCost(file index.FileInfo) int {
	return tokens.Estimate(Summary(file)) + 45
}

func SummarySelectionCost(file index.FileInfo) int {
	return summaryCost(file)
}

func shouldIncludeFull(index int, cost int, remaining int) bool {
	if cost > 4500 && remaining < 16000 {
		return false
	}
	if index < 4 {
		return true
	}
	if cost > 4500 {
		return false
	}
	return cost <= remaining/2 || remaining > 10000
}

func termsFromReasons(reasons []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, reason := range reasons {
		for _, match := range taskTermReasonRe.FindAllStringSubmatch(strings.ToLower(reason), -1) {
			if len(match) < 2 {
				continue
			}
			term := match[1]
			if !seen[term] {
				seen[term] = true
				out = append(out, term)
			}
		}
	}
	if len(out) > 8 {
		out = out[:8]
	}
	return out
}

func symbolMatchesTerms(sym index.Symbol, terms []string) bool {
	name := strings.ToLower(sym.Name)
	signature := strings.ToLower(sym.Signature)
	for _, term := range terms {
		term = strings.ToLower(term)
		if strings.Contains(name, term) || strings.Contains(signature, term) {
			return true
		}
	}
	return false
}

func mergeRanges(ranges []lineRange, maxLines int) []lineRange {
	sort.Slice(ranges, func(i, j int) bool {
		if ranges[i].start == ranges[j].start {
			return ranges[i].end < ranges[j].end
		}
		return ranges[i].start < ranges[j].start
	})
	var merged []lineRange
	for _, current := range ranges {
		if len(merged) == 0 || current.start > merged[len(merged)-1].end+2 {
			merged = append(merged, current)
			continue
		}
		if current.end > merged[len(merged)-1].end {
			merged[len(merged)-1].end = current.end
		}
	}
	var capped []lineRange
	used := 0
	for _, r := range merged {
		width := r.end - r.start + 1
		if width <= 0 {
			continue
		}
		if used+width > maxLines {
			if used >= maxLines {
				break
			}
			r.end = r.start + (maxLines - used) - 1
			width = r.end - r.start + 1
		}
		capped = append(capped, r)
		used += width
	}
	return capped
}

func clamp(value int, min int, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func snippetCommentPrefix(file index.FileInfo) string {
	switch file.Language {
	case "sql":
		return "--"
	case "html", "xhtml", "xml", "markdown":
		return "<!--"
	default:
		return "//"
	}
}
