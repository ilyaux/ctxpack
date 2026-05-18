package bench

import (
	"path/filepath"
	"testing"
)

func TestPublicEvalSuite(t *testing.T) {
	suites := []struct {
		name      string
		repo      string
		tasks     string
		wantTasks int
	}{
		{
			name:      "go-ts-monorepo",
			repo:      filepath.Join("..", "..", "testdata", "fixtures", "go-ts-monorepo"),
			tasks:     filepath.Join("..", "..", "evals", "go-ts-monorepo", "tasks.yaml"),
			wantTasks: 2,
		},
		{
			name:      "java-maven-webapp",
			repo:      filepath.Join("..", "..", "testdata", "fixtures", "java-maven-webapp"),
			tasks:     filepath.Join("..", "..", "evals", "java-maven-webapp", "tasks.yaml"),
			wantTasks: 2,
		},
	}

	for _, suite := range suites {
		t.Run(suite.name, func(t *testing.T) {
			report, err := Run(Options{
				Repo:             suite.repo,
				TasksPath:        suite.tasks,
				OutputDir:        filepath.Join(t.TempDir(), "eval-output"),
				Budget:           9000,
				IncludeBaselines: true,
			})
			if err != nil {
				t.Fatal(err)
			}
			if len(report.Tasks) != suite.wantTasks {
				t.Fatalf("tasks = %d, want %d", len(report.Tasks), suite.wantTasks)
			}
			for _, task := range report.Tasks {
				if !task.Passed {
					t.Fatalf("eval task %s failed: %+v", task.Name, task)
				}
				if len(task.Baselines) != 3 {
					t.Fatalf("eval task %s baselines = %d, want 3", task.Name, len(task.Baselines))
				}
			}
		})
	}
}
