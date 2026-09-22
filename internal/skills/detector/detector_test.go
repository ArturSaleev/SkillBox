package detector_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aibox/skillbox/internal/skills/detector"
)

func TestDefaultDetectorFindsExtensionsNamesAndShebangWithoutExecution(t *testing.T) {
	root := t.TempDir()
	marker := filepath.Join(root, "executed")
	files := map[string]string{
		"scripts/main.py": "open(" + marker + ", 'w').close()\n",
		"tools/runner":    "#!/usr/bin/env bash\ntouch " + marker + "\n",
		"Makefile":        "all:\n\ttouch " + marker + "\n",
		"references/a.md": "documentation\n",
	}
	for name, body := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	findings, err := detector.Default().Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 3 {
		t.Fatalf("findings=%#v", findings)
	}
	if _, err = os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("detected code was executed: %v", err)
	}
}

func TestDetectorAcceptsCustomRules(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "workflow.acme"), []byte("custom"), 0o600); err != nil {
		t.Fatal(err)
	}
	custom := detector.RuleFunc(func(candidate detector.Candidate) (detector.Finding, bool) {
		return detector.Finding{Language: "Acme", Reason: "custom rule"}, candidate.Extension == ".acme"
	})
	findings, err := detector.New(custom).Scan(root)
	if err != nil || len(findings) != 1 || findings[0].Language != "Acme" {
		t.Fatalf("findings=%#v err=%v", findings, err)
	}
}
