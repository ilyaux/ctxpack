package bench

import "testing"

func TestParseYAMLTasks(t *testing.T) {
	tasks, err := parseYAMLTasks(`
tasks:
  - name: jsf
    task: "fix JSF page"
    budget: 8000
    expect:
      - WWW_Alarms24_P3/**
    avoid:
      - SeleniumTests/**
  - name: maven
    task: review Maven dependencies
`)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 2 {
		t.Fatalf("task count = %d", len(tasks))
	}
	if tasks[0].Name != "jsf" || tasks[0].Budget != 8000 || len(tasks[0].Expect) != 1 || len(tasks[0].Avoid) != 1 {
		t.Fatalf("first task = %#v", tasks[0])
	}
	if tasks[1].Task != "review Maven dependencies" {
		t.Fatalf("second task = %#v", tasks[1])
	}
}
