package bench

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunSyntheticGoTSFixture(t *testing.T) {
	source := filepath.Join("..", "..", "testdata", "fixtures", "go-ts-monorepo")
	repo := filepath.Join(t.TempDir(), "go-ts-monorepo")
	copyDir(t, source, repo)

	report, err := Run(Options{
		Repo:             repo,
		TasksPath:        filepath.Join(repo, "bench", "tasks.yaml"),
		OutputDir:        filepath.Join(t.TempDir(), "bench-output"),
		Budget:           9000,
		IncludeBaselines: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Tasks) != 1 {
		t.Fatalf("tasks = %d, want 1", len(report.Tasks))
	}
	task := report.Tasks[0]
	if !task.Passed {
		t.Fatalf("synthetic fixture task failed: %+v", task)
	}
	for _, want := range []string{
		"services/billing/api/routes.go",
		"services/billing/domain/fees.go",
		"packages/api-client/src/billing.ts",
		"apps/web/src/pages/billing/BillingPage.tsx",
	} {
		if !containsBenchPath(task.TopFiles, want) {
			t.Fatalf("top files missing %s: %#v", want, task.TopFiles)
		}
	}
	if len(task.Baselines) != 3 {
		t.Fatalf("baselines = %d, want 3: %+v", len(task.Baselines), task.Baselines)
	}
	if task.Baselines[0].Name != "full-repo" || task.Baselines[0].SelectedFiles <= task.SelectedFiles {
		t.Fatalf("unexpected full-repo baseline: %+v", task.Baselines[0])
	}
	if task.Baselines[1].Name != "repomix-style" || task.Baselines[1].SelectedFiles <= task.SelectedFiles {
		t.Fatalf("unexpected repomix-style baseline: %+v", task.Baselines[1])
	}
}

func TestRunSyntheticJavaMavenFixture(t *testing.T) {
	source := filepath.Join("..", "..", "testdata", "fixtures", "java-maven-webapp")
	repo := filepath.Join(t.TempDir(), "java-maven-webapp")
	copyDir(t, source, repo)

	report, err := Run(Options{
		Repo:             repo,
		TasksPath:        filepath.Join(repo, "bench", "tasks.yaml"),
		OutputDir:        filepath.Join(t.TempDir(), "bench-output"),
		Budget:           9000,
		IncludeBaselines: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Tasks) != 2 {
		t.Fatalf("tasks = %d, want 2", len(report.Tasks))
	}
	for _, task := range report.Tasks {
		if !task.Passed {
			t.Fatalf("synthetic Java/Maven fixture task failed: %+v", task)
		}
		if len(task.Baselines) != 3 {
			t.Fatalf("baselines = %d, want 3: %+v", len(task.Baselines), task.Baselines)
		}
	}
}

func copyDir(t *testing.T, src string, dst string) {
	t.Helper()
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dst, 0755); err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		if entry.IsDir() {
			copyDir(t, srcPath, dstPath)
			continue
		}
		data, err := os.ReadFile(srcPath)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dstPath, data, 0644); err != nil {
			t.Fatal(err)
		}
	}
}

func containsBenchPath(paths []string, want string) bool {
	for _, path := range paths {
		if path == want {
			return true
		}
	}
	return false
}
