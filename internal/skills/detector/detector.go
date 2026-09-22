// Package detector identifies files that may contain executable code. It only
// reads file names, modes, and a bounded prefix; it never invokes package code.
package detector

import (
	"bufio"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type Finding struct {
	Path     string `json:"path"`
	Language string `json:"language,omitempty"`
	Reason   string `json:"reason"`
}

type Candidate struct {
	Path      string
	Mode      os.FileMode
	FirstLine string
	Extension string
	BaseName  string
}

type Rule interface {
	Detect(Candidate) (Finding, bool)
}

type RuleFunc func(Candidate) (Finding, bool)

func (f RuleFunc) Detect(candidate Candidate) (Finding, bool) { return f(candidate) }

type Detector struct{ rules []Rule }

func New(rules ...Rule) *Detector { return &Detector{rules: append([]Rule(nil), rules...)} }

func Default() *Detector {
	return New(ExtensionRule(defaultExtensions()), NameRule(defaultNames()), ShebangRule())
}

func (d *Detector) Scan(root string) ([]Finding, error) {
	var findings []Finding
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
		if !info.Mode().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		candidate := Candidate{Path: filepath.ToSlash(rel), Mode: info.Mode(), Extension: strings.ToLower(filepath.Ext(entry.Name())), BaseName: strings.ToLower(entry.Name())}
		candidate.FirstLine, err = firstLine(path)
		if err != nil {
			return err
		}
		for _, rule := range d.rules {
			if finding, ok := rule.Detect(candidate); ok {
				finding.Path = candidate.Path
				findings = append(findings, finding)
				break
			}
		}
		return nil
	})
	return findings, err
}

func ExtensionRule(languages map[string]string) Rule {
	return RuleFunc(func(candidate Candidate) (Finding, bool) {
		language, ok := languages[candidate.Extension]
		return Finding{Language: language, Reason: "executable source extension " + candidate.Extension}, ok
	})
}

func NameRule(languages map[string]string) Rule {
	return RuleFunc(func(candidate Candidate) (Finding, bool) {
		language, ok := languages[candidate.BaseName]
		return Finding{Language: language, Reason: "executable build or command file"}, ok
	})
}

func ShebangRule() Rule {
	return RuleFunc(func(candidate Candidate) (Finding, bool) {
		if !strings.HasPrefix(candidate.FirstLine, "#!") {
			return Finding{}, false
		}
		interpreter := strings.TrimSpace(strings.TrimPrefix(candidate.FirstLine, "#!"))
		fields := strings.Fields(interpreter)
		language := "unknown"
		if len(fields) > 0 {
			language = filepath.Base(fields[0])
		}
		return Finding{Language: language, Reason: "interpreter shebang"}, true
	})
}

func firstLine(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	reader := bufio.NewReader(io.LimitReader(file, 4096))
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func defaultExtensions() map[string]string {
	return map[string]string{
		".py": "Python", ".pyw": "Python", ".js": "JavaScript", ".mjs": "JavaScript", ".cjs": "JavaScript",
		".ts": "TypeScript", ".tsx": "TypeScript", ".jsx": "JavaScript", ".sh": "Shell", ".bash": "Bash",
		".zsh": "Zsh", ".fish": "Fish", ".ps1": "PowerShell", ".psm1": "PowerShell", ".bat": "Batch",
		".cmd": "Batch", ".go": "Go", ".rb": "Ruby", ".php": "PHP", ".pl": "Perl", ".lua": "Lua",
		".r": "R", ".swift": "Swift", ".rs": "Rust", ".java": "Java", ".kt": "Kotlin", ".kts": "Kotlin",
		".cs": "C#", ".fsx": "F#", ".vb": "Visual Basic", ".groovy": "Groovy",
	}
}

func defaultNames() map[string]string {
	return map[string]string{"makefile": "Make", "dockerfile": "Docker", "rakefile": "Ruby", "gemfile": "Ruby", "justfile": "Just", "taskfile.yml": "Task"}
}
