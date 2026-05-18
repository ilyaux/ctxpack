package index

import "testing"

func TestAnalyzeTSReactAndRouteSymbols(t *testing.T) {
	file := FileInfo{Path: "apps/web/src/app/billing/page.tsx", Language: "typescriptreact"}
	analyzeTS(&file, `
import type { BillingSummary } from "@acme/api-client/billing";

type BillingPageProps = { accountId: string };

export default function BillingPage(props: BillingPageProps) {
  return <section>{props.accountId}</section>;
}

const useBillingPreview = () => ({});

export enum BillingMode {
  Preview = "preview"
}
`)
	if !hasSymbol(file.Symbols, "BillingPage", "component") {
		t.Fatalf("missing BillingPage component: %#v", file.Symbols)
	}
	if !hasSymbol(file.Symbols, "useBillingPreview", "hook") {
		t.Fatalf("missing hook symbol: %#v", file.Symbols)
	}
	if !hasSymbol(file.Symbols, "BillingPageProps", "type") {
		t.Fatalf("missing type symbol: %#v", file.Symbols)
	}
	if !hasSymbol(file.Symbols, "BillingMode", "enum") {
		t.Fatalf("missing enum symbol: %#v", file.Symbols)
	}
	if len(file.Imports) != 1 || file.Imports[0] != "@acme/api-client/billing" {
		t.Fatalf("imports = %#v", file.Imports)
	}

	route := FileInfo{Path: "apps/web/src/app/api/billing/route.ts", Language: "typescript"}
	analyzeTS(&route, `
export async function POST(request: Request) {
  return Response.json({});
}
`)
	if !route.IsRoute {
		t.Fatal("route.ts should be marked as route")
	}
	if !hasSymbol(route.Symbols, "POST", "handler") {
		t.Fatalf("missing POST handler: %#v", route.Symbols)
	}
}

func TestAnalyzeTSCommonJSDynamicImportsAndExpressRoutes(t *testing.T) {
	file := FileInfo{Path: "services/api/routes/billing.ts", Language: "typescript"}
	analyzeTS(&file, `
const billing = require("@acme/api-client/billing");
const lazy = import("../domain/commission");

router.post("/billing/commission/preview", async (req, res) => {
  res.json(await billing.previewCommission(req.body));
});
`)
	if !file.IsRoute {
		t.Fatal("express router file should be marked as route")
	}
	for _, want := range []string{"@acme/api-client/billing", "../domain/commission"} {
		if !hasImport(file.Imports, want) {
			t.Fatalf("missing import %q: %#v", want, file.Imports)
		}
	}
	if !hasSymbol(file.Symbols, "POST /billing/commission/preview", "route") {
		t.Fatalf("missing express route symbol: %#v", file.Symbols)
	}
}

func hasSymbol(symbols []Symbol, name string, kind string) bool {
	for _, sym := range symbols {
		if sym.Name == name && sym.Kind == kind {
			return true
		}
	}
	return false
}

func hasImport(imports []string, want string) bool {
	for _, imported := range imports {
		if imported == want {
			return true
		}
	}
	return false
}
