package mcp

import (
	"bytes"
	"strings"
	"testing"

	"github.com/ilyaux/ctxpack/internal/version"
)

func TestServerListsTools(t *testing.T) {
	input := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}` + "\n")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := (Server{DefaultRepo: "."}).Run(input, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q", stderr.String())
	}
	if !strings.Contains(stdout.String(), "repo_context_pack") {
		t.Fatalf("tools/list response missing repo_context_pack: %s", stdout.String())
	}
}

func TestServerInitializeUsesVersionPackage(t *testing.T) {
	input := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}` + "\n")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := (Server{DefaultRepo: "."}).Run(input, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q", stderr.String())
	}
	want := `"version":"` + version.Version + `"`
	if !strings.Contains(stdout.String(), want) {
		t.Fatalf("initialize response missing %s: %s", want, stdout.String())
	}
}
