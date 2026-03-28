package barometer

import (
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type compositeAction struct {
	Inputs map[string]struct {
		Default string `yaml:"default"`
	} `yaml:"inputs"`
	Outputs map[string]struct {
		Description string `yaml:"description"`
		Value       string `yaml:"value"`
	} `yaml:"outputs"`
	Runs struct {
		Steps []struct {
			Name string `yaml:"name"`
			Run  string `yaml:"run"`
		} `yaml:"steps"`
	} `yaml:"runs"`
}

func TestBarometerActionContract(t *testing.T) {
	data, err := os.ReadFile(".github/actions/barometer/action.yml")
	if err != nil {
		t.Fatalf("read action: %v", err)
	}

	var action compositeAction
	if err := yaml.Unmarshal(data, &action); err != nil {
		t.Fatalf("unmarshal action: %v", err)
	}

	for _, key := range []string{
		"openapi-spec",
		"openapi-tags",
		"base-url",
		"arazzo-doc",
		"arazzo-workflows",
		"config",
		"output-format",
	} {
		if _, ok := action.Inputs[key]; !ok {
			t.Fatalf("missing input %q", key)
		}
	}
	if got := action.Inputs["output-format"].Default; got != "junit" {
		t.Fatalf("output-format default = %q, want junit", got)
	}
	for _, key := range []string{"result", "report-path", "report-json", "report-junit"} {
		if _, ok := action.Outputs[key]; !ok {
			t.Fatalf("missing output %q", key)
		}
		if action.Outputs[key].Value == "" {
			t.Fatalf("output %q missing value mapping", key)
		}
	}

	runScript := ""
	for _, step := range action.Runs.Steps {
		if step.Name == "Run contract tests" {
			runScript = step.Run
			break
		}
	}
	if runScript == "" {
		t.Fatal("missing Run contract tests step")
	}
	for _, want := range []string{
		`mktemp`,
		`contract" "test"`,
		`openapi-spec/arazzo-doc`,
		`report-path`,
	} {
		if !strings.Contains(runScript, want) {
			t.Fatalf("run step missing %q:\n%s", want, runScript)
		}
	}
}
