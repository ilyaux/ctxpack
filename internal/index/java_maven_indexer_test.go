package index

import "testing"

func TestAnalyzeJavaAnnotationsAndRoutes(t *testing.T) {
	file := FileInfo{Path: "services/billing/src/main/java/com/acme/billing/BillingResource.java", Language: "java"}
	analyzeJava(&file, `
package com.acme.billing;

import com.acme.billing.FeeService;

@jakarta.ws.rs.Path("/billing")
@RestController
public class BillingResource {
  @POST
  @Path("/commission/preview")
  public CommissionResponse preview(CommissionRequest request) {
    return null;
  }
}
`)
	if !file.IsRoute {
		t.Fatal("resource should be marked as route")
	}
	if file.Package != "com.acme.billing" {
		t.Fatalf("package = %q", file.Package)
	}
	if len(file.Imports) != 1 || file.Imports[0] != "com.acme.billing.FeeService" {
		t.Fatalf("imports = %#v", file.Imports)
	}
	if !hasSymbol(file.Symbols, "BillingResource", "controller") {
		t.Fatalf("missing controller symbol: %#v", file.Symbols)
	}
	if !hasSymbol(file.Symbols, "preview", "handler") {
		t.Fatalf("missing handler symbol: %#v", file.Symbols)
	}
	if !hasSymbol(file.Symbols, "/commission/preview", "route") {
		t.Fatalf("missing route annotation symbol: %#v", file.Symbols)
	}
}

func TestAnalyzeMavenPOMSymbols(t *testing.T) {
	file := FileInfo{Path: "pom.xml", Language: "xml", IsConfig: true}
	analyzeMarkup(&file, `
<project>
  <groupId>com.acme</groupId>
  <artifactId>platform</artifactId>
  <modules>
    <module>services/billing</module>
  </modules>
  <dependencies>
    <dependency>
      <groupId>com.acme</groupId>
      <artifactId>billing-core</artifactId>
    </dependency>
  </dependencies>
</project>
`)
	if file.Package != "com.acme:platform" {
		t.Fatalf("package = %q", file.Package)
	}
	if !hasSymbol(file.Symbols, "com.acme:platform", "maven-artifact") {
		t.Fatalf("missing artifact symbol: %#v", file.Symbols)
	}
	if !hasSymbol(file.Symbols, "services/billing", "maven-module") {
		t.Fatalf("missing module symbol: %#v", file.Symbols)
	}
	if !hasSymbol(file.Symbols, "com.acme:billing-core", "maven-dependency") {
		t.Fatalf("missing dependency symbol: %#v", file.Symbols)
	}
	if len(file.Imports) == 0 {
		t.Fatal("expected maven imports")
	}
}
