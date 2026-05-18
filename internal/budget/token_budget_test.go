package budget

import (
	"fmt"
	"strings"
	"testing"

	"github.com/ilyaux/ctxpack/internal/index"
	"github.com/ilyaux/ctxpack/internal/rank"
)

func TestSliceBlockExtractsRelevantLines(t *testing.T) {
	file := largeJavaFile("AuditLogService", 180)
	scored := rank.ScoredFile{
		File:    file,
		Score:   100,
		Reasons: []string{"symbol matches task term audit", "content mentions task term database"},
	}

	block := SliceBlock(scored)
	if !strings.Contains(block, "AuditLogService") {
		t.Fatalf("slice missing relevant symbol:\n%s", block)
	}
	if !strings.Contains(block, "insertAuditLog") {
		t.Fatalf("slice missing relevant method:\n%s", block)
	}
	if strings.Contains(block, "  20 |") || strings.Contains(block, " 300 |") {
		t.Fatalf("slice included distant noise:\n%s", block)
	}
}

func TestSelectUsesRelevantSlicesForLargeFile(t *testing.T) {
	file := largeJavaFile("AuditLogService", 180)
	scored := []rank.ScoredFile{{
		File:    file,
		Score:   100,
		Reasons: []string{"symbol matches task term audit", "content mentions task term database"},
	}}

	selections := Select(scored, 4000)
	if len(selections) != 1 {
		t.Fatalf("selection count = %d", len(selections))
	}
	if selections[0].Mode != ModeSlices {
		t.Fatalf("mode = %s, want %s; cost=%d", selections[0].Mode, ModeSlices, selections[0].EstimatedTokens)
	}
}

func largeJavaFile(name string, symbolLine int) index.FileInfo {
	lines := make([]string, 0, 420)
	for i := 1; i <= 420; i++ {
		if i == symbolLine {
			lines = append(lines, "public class "+name+" {")
			continue
		}
		if i == symbolLine+3 {
			lines = append(lines, "    public void insertAuditLog(Database db) { db.query(\"insert into audit_log\"); }")
			continue
		}
		lines = append(lines, fmt.Sprintf("// noise line %d with enough filler text to make the full file expensive", i))
	}
	content := strings.Join(lines, "\n")
	return index.FileInfo{
		Path:            "src/main/java/example/" + name + ".java",
		Language:        "java",
		SizeBytes:       int64(len(content)),
		EstimatedTokens: 8000,
		Content:         content,
		Symbols: []index.Symbol{{
			Name:      name,
			Kind:      "class",
			Signature: "class " + name,
			Line:      symbolLine,
		}},
	}
}
