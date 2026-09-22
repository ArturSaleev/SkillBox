package codereview

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/aibox/skillbox/internal/domain"
	skillpackage "github.com/aibox/skillbox/internal/skills/package"
	"github.com/aibox/skillbox/internal/skills/securityscan"
)

const maxReviewPayloadBytes = 2 << 20

// RequestFromPackage reads text files from one validated package without
// executing package content. Binary files are omitted from the AI payload.
func RequestFromPackage(root string, skill *domain.Skill) (Request, error) {
	pkg, err := skillpackage.Load(root)
	if err != nil {
		return Request{}, err
	}
	report, err := securityscan.Default().Scan(root)
	if err != nil {
		return Request{}, err
	}
	request := Request{SkillID: skill.ID, PackageHash: pkg.Hash, PackageName: skill.Name, StaticReport: report}
	total := 0
	for _, listed := range pkg.Files {
		data, readErr := os.ReadFile(filepath.Join(root, filepath.FromSlash(listed.Path)))
		if readErr != nil {
			return Request{}, readErr
		}
		if bytes.IndexByte(data, 0) >= 0 {
			continue
		}
		total += len(data)
		if total > maxReviewPayloadBytes {
			return Request{}, fmt.Errorf("text files selected for review exceed %d bytes", maxReviewPayloadBytes)
		}
		request.Files = append(request.Files, File{Path: listed.Path, Content: string(data)})
	}
	return request, nil
}
