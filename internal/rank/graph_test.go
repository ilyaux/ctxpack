package rank

import (
	"strings"
	"testing"

	"github.com/ilyaux/ctxpack/internal/index"
)

func TestScoreFilesBoostsJavaImports(t *testing.T) {
	idx := &index.RepoIndex{Files: []index.FileInfo{
		{
			Path:     "app/src/main/java/com/example/BillingController.java",
			Language: "java",
			Package:  "com.example",
			Imports:  []string{"com.example.rules.PriceRules"},
			Symbols: []index.Symbol{
				{Name: "BillingController", Kind: "class", Signature: "class BillingController"},
				{Name: "preview", Kind: "handler", Signature: "Response preview(Request request)"},
			},
			IsRoute: true,
			Content: "@RestController class BillingController { }",
		},
		{
			Path:     "app/src/main/java/com/example/rules/PriceRules.java",
			Language: "java",
			Package:  "com.example.rules",
			Symbols: []index.Symbol{
				{Name: "PriceRules", Kind: "class", Signature: "class PriceRules"},
			},
			Content: "class PriceRules { }",
		},
	}}

	scored := ScoreFiles("fix billing endpoint", idx, Options{})
	related := findScored(scored, "app/src/main/java/com/example/rules/PriceRules.java")
	if related == nil {
		t.Fatal("expected imported Java file to be selected")
	}
	if !hasReason(related.Reasons, "imported by relevant file") {
		t.Fatalf("expected import reason, got %#v", related.Reasons)
	}
}

func TestScoreFilesBoostsTypeScriptRelativeImports(t *testing.T) {
	idx := &index.RepoIndex{Files: []index.FileInfo{
		{
			Path:     "apps/web/src/pages/BillingPage.tsx",
			Language: "typescriptreact",
			Imports:  []string{"../api/priceClient"},
			Symbols: []index.Symbol{
				{Name: "BillingPage", Kind: "component", Signature: "export function BillingPage()"},
			},
			Content: "export function BillingPage() { return null }",
		},
		{
			Path:     "apps/web/src/api/priceClient.ts",
			Language: "typescript",
			Symbols: []index.Symbol{
				{Name: "previewPrice", Kind: "function", Signature: "export function previewPrice()"},
			},
			Content: "export function previewPrice() {}",
		},
	}}

	scored := ScoreFiles("update billing frontend", idx, Options{})
	related := findScored(scored, "apps/web/src/api/priceClient.ts")
	if related == nil {
		t.Fatal("expected imported TypeScript file to be selected")
	}
	if !hasReason(related.Reasons, "imported by relevant file") {
		t.Fatalf("expected import reason, got %#v", related.Reasons)
	}
}

func TestScoreFilesBoostsTypeScriptPackageImports(t *testing.T) {
	idx := &index.RepoIndex{Files: []index.FileInfo{
		{
			Path:     "packages/api-client/package.json",
			Language: "json",
			Symbols: []index.Symbol{
				{Name: "@acme/api-client", Kind: "package"},
			},
			Content: `{"name":"@acme/api-client"}`,
		},
		{
			Path:     "apps/web/src/pages/BillingPage.tsx",
			Language: "typescriptreact",
			Imports:  []string{"@acme/api-client/billing"},
			Symbols: []index.Symbol{
				{Name: "BillingPage", Kind: "component", Signature: "export function BillingPage()"},
			},
			Content: "export function BillingPage() { return null }",
		},
		{
			Path:     "packages/api-client/src/billing.ts",
			Language: "typescript",
			Symbols: []index.Symbol{
				{Name: "previewCommission", Kind: "function", Signature: "export function previewCommission()"},
			},
			Content: "export function previewCommission() {}",
		},
	}}

	scored := ScoreFiles("update billing frontend", idx, Options{})
	related := findScored(scored, "packages/api-client/src/billing.ts")
	if related == nil {
		t.Fatal("expected package-imported TypeScript file to be selected")
	}
	if !hasReason(related.Reasons, "imported by relevant file") {
		t.Fatalf("expected import reason, got %#v", related.Reasons)
	}
}

func TestScoreFilesBoostsJavaWildcardImports(t *testing.T) {
	idx := &index.RepoIndex{Files: []index.FileInfo{
		{
			Path:     "app/src/main/java/com/example/BillingController.java",
			Language: "java",
			Package:  "com.example",
			Imports:  []string{"com.example.rules.*"},
			Symbols: []index.Symbol{
				{Name: "BillingController", Kind: "controller", Signature: "class BillingController"},
			},
			IsRoute: true,
			Content: "@RestController class BillingController { }",
		},
		{
			Path:     "app/src/main/java/com/example/rules/FeeRules.java",
			Language: "java",
			Package:  "com.example.rules",
			Symbols: []index.Symbol{
				{Name: "FeeRules", Kind: "class", Signature: "class FeeRules"},
			},
			Content: "class FeeRules { }",
		},
	}}

	scored := ScoreFiles("fix billing endpoint", idx, Options{})
	related := findScored(scored, "app/src/main/java/com/example/rules/FeeRules.java")
	if related == nil {
		t.Fatal("expected wildcard-imported Java file to be selected")
	}
	if !hasReason(related.Reasons, "imported by relevant file") {
		t.Fatalf("expected import reason, got %#v", related.Reasons)
	}
}

func TestScoreFilesBoostsMavenDependencies(t *testing.T) {
	idx := &index.RepoIndex{Files: []index.FileInfo{
		{
			Path:     "pom.xml",
			Language: "xml",
			IsConfig: true,
			Imports:  []string{"billing-core"},
			Symbols: []index.Symbol{
				{Name: "com.acme:platform", Kind: "maven-artifact"},
				{Name: "com.acme:billing-core", Kind: "maven-dependency"},
			},
			Content: "<artifactId>platform</artifactId><dependency><artifactId>billing-core</artifactId></dependency>",
		},
		{
			Path:     "services/billing/pom.xml",
			Language: "xml",
			IsConfig: true,
			Symbols: []index.Symbol{
				{Name: "com.acme:billing-core", Kind: "maven-artifact"},
			},
			Content: "<artifactId>billing-core</artifactId>",
		},
	}}

	scored := ScoreFiles("review Maven module dependencies", idx, Options{})
	related := findScored(scored, "services/billing/pom.xml")
	if related == nil {
		t.Fatal("expected Maven dependency pom to be selected")
	}
	if !hasReason(related.Reasons, "imported by relevant file") {
		t.Fatalf("expected Maven graph reason, got %#v", related.Reasons)
	}
}

func findScored(files []ScoredFile, path string) *ScoredFile {
	for i := range files {
		if files[i].File.Path == path {
			return &files[i]
		}
	}
	return nil
}

func hasReason(reasons []string, want string) bool {
	for _, reason := range reasons {
		if strings.Contains(reason, want) {
			return true
		}
	}
	return false
}
