// Package securityscan performs conservative static pattern analysis of Skill
// package files. It never executes content and never declares a package safe.
package securityscan

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type Finding struct {
	Path       string `json:"path"`
	Line       int    `json:"line,omitempty"`
	Capability string `json:"capability"`
	Severity   string `json:"severity"`
	Rule       string `json:"rule"`
	Evidence   string `json:"evidence,omitempty"`
}

type Report struct {
	Findings       []Finding `json:"findings"`
	Capabilities   []string  `json:"capabilities"`
	FilesScanned   int       `json:"files_scanned"`
	Recommendation string    `json:"recommendation"`
}

type Context struct {
	Path string
	Line int
	Text string
}

type Rule interface{ Scan(Context) []Finding }
type RuleFunc func(Context) []Finding

func (f RuleFunc) Scan(ctx Context) []Finding { return f(ctx) }

type Scanner struct {
	rules       []Rule
	maxFileSize int64
}

func New(rules ...Rule) *Scanner {
	return &Scanner{rules: append([]Rule(nil), rules...), maxFileSize: 2 << 20}
}
func Default() *Scanner { return New(defaultRules()...) }

func (s *Scanner) Scan(root string) (Report, error) {
	report := Report{Recommendation: "Manual review required. Static analysis is incomplete and cannot establish the absence of risk."}
	capabilities := map[string]struct{}{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() || info.Size() > s.maxFileSize {
			return nil
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		prefix := make([]byte, 512)
		n, readErr := file.Read(prefix)
		if readErr != nil && readErr != io.EOF {
			return readErr
		}
		if bytes.IndexByte(prefix[:n], 0) >= 0 {
			return nil
		}
		if _, err = file.Seek(0, io.SeekStart); err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		report.FilesScanned++
		scanner := bufio.NewScanner(io.LimitReader(file, s.maxFileSize))
		scanner.Buffer(make([]byte, 64<<10), 1<<20)
		line := 0
		for scanner.Scan() {
			line++
			ctx := Context{Path: filepath.ToSlash(rel), Line: line, Text: scanner.Text()}
			for _, rule := range s.rules {
				for _, finding := range rule.Scan(ctx) {
					finding.Path, finding.Line = ctx.Path, ctx.Line
					report.Findings = append(report.Findings, finding)
					capabilities[finding.Capability] = struct{}{}
				}
			}
		}
		return scanner.Err()
	})
	for capability := range capabilities {
		report.Capabilities = append(report.Capabilities, capability)
	}
	sort.Strings(report.Capabilities)
	return report, err
}

func PatternRule(name, capability, severity string, patterns ...string) Rule {
	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, pattern := range patterns {
		compiled = append(compiled, regexp.MustCompile(`(?i)`+pattern))
	}
	return RuleFunc(func(ctx Context) []Finding {
		for _, pattern := range compiled {
			if match := pattern.FindString(ctx.Text); match != "" {
				evidence := strings.TrimSpace(ctx.Text)
				if len(evidence) > 180 {
					evidence = evidence[:180] + "…"
				}
				return []Finding{{Capability: capability, Severity: severity, Rule: name, Evidence: evidence}}
			}
		}
		return nil
	})
}

func defaultRules() []Rule {
	return []Rule{
		PatternRule("filesystem-access", "filesystem", "medium", `\bos\.(open|create|readfile|writefile|remove|rename)\b`, `\b(open|readfile|writefile|unlink|rmdir|mkdir)\s*\(`, `\b(fs|pathlib)\.`),
		PatternRule("network-access", "network", "high", `\b(net/http|requests\.|urllib\.|axios\.|fetch\s*\(|https?\.request|net\.dial)`),
		PatternRule("subprocess-shell", "subprocess/shell", "high", `\b(exec\.command|subprocess\.|child_process|os\.system|runtime\.exec|shell_exec|processbuilder|powershell)\b`),
		PatternRule("dynamic-evaluation", "eval/exec", "critical", `\b(eval|exec)\s*\(`, `\bnew\s+function\s*\(`, `\bvm\.run`),
		PatternRule("environment-access", "environment", "medium", `\bos\.getenv\b`, `\bprocess\.env\b`, `\benv\[`, `\bgetenv\s*\(`),
		PatternRule("credential-material", "credentials", "high", `\b(api[_-]?(key|token)|access[_-]?token|client[_-]?secret|password|private[_-]?key)\b`, `-----begin [a-z ]*private key-----`),
		PatternRule("dynamic-loading", "dynamic loading", "high", `\b(importlib|__import__|dlopen|plugin\.open|assembly\.load|loadlibrary|require\s*\([^"'])`),
		PatternRule("obfuscation", "obfuscation", "high", `\b(base64\.b64decode|atob|fromcharcode|unescape)\b`, `\\x[0-9a-f]{2}.*\\x[0-9a-f]{2}`),
		PatternRule("remote-download", "downloads", "high", `\b(curl|wget|invoke-webrequest|downloadfile|urlretrieve)\b`, `https?://`),
		PatternRule("external-binary", "external binaries", "high", `\b(exec\.command|subprocess\.(run|popen|call)|child_process\.(exec|spawn)|processbuilder|runtime\.exec)\b`),
	}
}

func Summary(report Report) string {
	return fmt.Sprintf("Static scan found %d finding(s) across %d file(s). %s", len(report.Findings), report.FilesScanned, report.Recommendation)
}
