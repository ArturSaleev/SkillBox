// Package store makes filesystem Skill packages authoritative for Skill
// contents while retaining SQL for searchable index metadata and runtime data.
package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/aibox/skillbox/internal/domain"
	"github.com/aibox/skillbox/internal/ports"
	skillpackage "github.com/aibox/skillbox/internal/skills/package"
	"github.com/google/uuid"
)

type Store struct {
	ports.Storage
	root string
}

type indexSynchronizer interface {
	CreateSkillIndex(context.Context, *domain.Skill) error
	SyncSkillIndex(context.Context, *domain.Skill) error
	RecordSkillVersion(context.Context, *domain.Skill, string, *string) error
}

type legacyMigrationApplier interface {
	ApplyLegacySkillMigration(context.Context, *domain.Skill, map[int]string) error
}

type ReindexResult struct {
	Discovered int
	Created    int
	Updated    int
	Unchanged  int
}

type LegacyMigrationResult struct {
	Discovered int
	Migrated   int
	Unchanged  int
}

func New(index ports.Storage, root string) *Store { return &Store{Storage: index, root: root} }

// MigrateLegacySkills materializes every DB-only Skill and all of its existing
// version snapshots as filesystem packages. SQL lifecycle data, proposals,
// executions, and execution events remain in place. A Skill is switched to the
// package only after every package and immutable snapshot has been validated.
func (s *Store) MigrateLegacySkills(ctx context.Context) (LegacyMigrationResult, error) {
	applier, ok := s.Storage.(legacyMigrationApplier)
	if !ok {
		return LegacyMigrationResult{}, errors.New("storage does not support legacy Skill migration")
	}
	if err := skillpackage.EnsureRoot(s.root); err != nil {
		return LegacyMigrationResult{}, err
	}
	items, err := s.Storage.ListSkills(ctx)
	if err != nil {
		return LegacyMigrationResult{}, err
	}
	result := LegacyMigrationResult{Discovered: len(items)}
	for i := range items {
		if items[i].PackagePath != "" {
			result.Unchanged++
			continue
		}
		if err = s.migrateLegacySkill(ctx, applier, &items[i]); err != nil {
			return result, fmt.Errorf("migrate legacy Skill %s: %w", items[i].ID, err)
		}
		result.Migrated++
	}
	return result, nil
}

func (s *Store) migrateLegacySkill(ctx context.Context, applier legacyMigrationApplier, current *domain.Skill) error {
	if err := skillpackage.ValidateRelativePath(current.ID); err != nil {
		return fmt.Errorf("Skill ID cannot be used as package path: %w", err)
	}
	versions, err := s.Storage.ListVersions(ctx, current.ID)
	if err != nil {
		return err
	}
	if len(versions) == 0 {
		return errors.New("legacy Skill has no version history")
	}

	stagingRoot, err := os.MkdirTemp(s.root, ".legacy-migration-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stagingRoot)
	versionPackages := make(map[int]string, len(versions))
	for _, listed := range versions {
		versioned, getErr := s.Storage.GetVersion(ctx, current.ID, listed.Version)
		if getErr != nil {
			return getErr
		}
		var snapshot domain.Skill
		if getErr = json.Unmarshal([]byte(versioned.Snapshot), &snapshot); getErr != nil {
			return fmt.Errorf("decode version %d: %w", listed.Version, getErr)
		}
		snapshot.ID = current.ID
		snapshot.CurrentVersion = listed.Version
		versionRoot := filepath.Join(stagingRoot, "versions", strconv.Itoa(listed.Version))
		if getErr = writePackageAt(versionRoot, &snapshot); getErr != nil {
			return fmt.Errorf("materialize version %d: %w", listed.Version, getErr)
		}
		versionPackages[listed.Version] = versionRoot
	}
	if _, ok := versionPackages[current.CurrentVersion]; !ok {
		return fmt.Errorf("current version %d is absent from version history", current.CurrentVersion)
	}

	currentRoot := filepath.Join(s.root, filepath.FromSlash(current.ID))
	currentStage := filepath.Join(stagingRoot, "current")
	if err = writePackageAt(currentStage, current); err != nil {
		return err
	}
	currentPackage, err := skillpackage.Load(currentStage)
	if err != nil {
		return err
	}
	createdCurrent := false
	if existing, loadErr := skillpackage.Load(currentRoot); loadErr == nil {
		if existing.Hash != currentPackage.Hash {
			return fmt.Errorf("package path %s already exists with different content", current.ID)
		}
	} else if !os.IsNotExist(loadErr) {
		return loadErr
	} else if err = os.Rename(currentStage, currentRoot); err != nil {
		return fmt.Errorf("commit current package: %w", err)
	} else {
		createdCurrent = true
	}

	createdSnapshots := make([]string, 0, len(versionPackages))
	versionHashes := make(map[int]string, len(versionPackages))
	rollbackFiles := func() {
		for _, path := range createdSnapshots {
			_ = os.RemoveAll(path)
		}
		if createdCurrent {
			_ = os.RemoveAll(currentRoot)
		}
	}
	for version, source := range versionPackages {
		destination := s.historyPath(current.ID, version)
		_, statErr := os.Stat(destination)
		existed := statErr == nil
		if statErr != nil && !os.IsNotExist(statErr) {
			rollbackFiles()
			return statErr
		}
		pkg, snapshotErr := skillpackage.Snapshot(source, destination)
		if snapshotErr != nil {
			rollbackFiles()
			return fmt.Errorf("snapshot version %d: %w", version, snapshotErr)
		}
		if !existed {
			createdSnapshots = append(createdSnapshots, destination)
		}
		versionHashes[version] = pkg.Hash
	}

	indexedAt := time.Now().UTC()
	current.PackagePath = current.ID
	current.PackageHash = currentPackage.Hash
	current.PackageIndexedAt = &indexedAt
	if err = applier.ApplyLegacySkillMigration(ctx, current, versionHashes); err != nil {
		rollbackFiles()
		return err
	}
	return nil
}

func writePackageAt(root string, sk *domain.Skill) error {
	raw, err := json.Marshal(fromSkill(sk))
	if err != nil {
		return err
	}
	var metadata map[string]any
	if err = json.Unmarshal(raw, &metadata); err != nil {
		return err
	}
	if err = skillpackage.WriteSkillMD(root, metadata, sk.Instructions); err != nil {
		return err
	}
	_, err = skillpackage.Load(root)
	return err
}

type document struct {
	Name            string                      `json:"name"`
	Description     string                      `json:"description"`
	Slug            string                      `json:"slug"`
	Purpose         string                      `json:"purpose,omitempty"`
	WhenToUse       string                      `json:"when_to_use,omitempty"`
	WhenNotToUse    string                      `json:"when_not_to_use,omitempty"`
	SuccessCriteria []string                    `json:"success_criteria,omitempty"`
	Priority        int                         `json:"priority,omitempty"`
	Domains         []string                    `json:"domains,omitempty"`
	Intents         []string                    `json:"intents,omitempty"`
	ObjectTypes     []string                    `json:"object_types,omitempty"`
	Tags            []string                    `json:"tags,omitempty"`
	Keywords        []string                    `json:"keywords,omitempty"`
	Capabilities    []string                    `json:"capabilities,omitempty"`
	Compatibility   []string                    `json:"compatibility,omitempty"`
	Steps           []domain.Step               `json:"steps,omitempty"`
	Tools           []domain.ToolRequirement    `json:"tools,omitempty"`
	Contexts        []domain.ContextRequirement `json:"context_requirements,omitempty"`
	Dependencies    []domain.Dependency         `json:"dependencies,omitempty"`
	Examples        []domain.Example            `json:"examples,omitempty"`
}

func fromSkill(sk *domain.Skill) document {
	return document{Name: sk.Name, Description: sk.Description, Slug: sk.Slug, Purpose: sk.Purpose,
		WhenToUse: sk.WhenToUse, WhenNotToUse: sk.WhenNotToUse, SuccessCriteria: sk.SuccessCriteria,
		Priority: sk.Priority, Domains: sk.Domains, Intents: sk.Intents, ObjectTypes: sk.ObjectTypes,
		Tags: sk.Tags, Keywords: sk.Keywords, Capabilities: sk.Capabilities, Compatibility: sk.Compatibility,
		Steps: sk.Steps, Tools: sk.Tools, Contexts: sk.Contexts, Dependencies: sk.Dependencies, Examples: sk.Examples}
}

func applyDocument(sk *domain.Skill, doc document, body string) {
	sk.Name, sk.Description, sk.Slug = doc.Name, doc.Description, doc.Slug
	sk.Purpose, sk.WhenToUse, sk.WhenNotToUse = doc.Purpose, doc.WhenToUse, doc.WhenNotToUse
	sk.Instructions, sk.SuccessCriteria, sk.Priority = body, doc.SuccessCriteria, doc.Priority
	sk.Domains, sk.Intents, sk.ObjectTypes = doc.Domains, doc.Intents, doc.ObjectTypes
	sk.Tags, sk.Keywords, sk.Capabilities, sk.Compatibility = doc.Tags, doc.Keywords, doc.Capabilities, doc.Compatibility
	sk.Steps, sk.Tools, sk.Contexts = doc.Steps, doc.Tools, doc.Contexts
	sk.Dependencies, sk.Examples = doc.Dependencies, doc.Examples
}

func documentFromPackage(pkg *skillpackage.Package) (document, error) {
	raw, err := json.Marshal(pkg.Metadata.Fields)
	if err != nil {
		return document{}, err
	}
	var doc document
	if err = json.Unmarshal(raw, &doc); err != nil {
		return document{}, err
	}
	return doc, nil
}

// Reindex validates every direct filesystem package before changing the SQL
// index, then creates or refreshes index rows without deleting runtime/history
// records for packages that are temporarily absent.
func (s *Store) Reindex(ctx context.Context) (ReindexResult, error) {
	syncer, ok := s.Storage.(indexSynchronizer)
	if !ok {
		return ReindexResult{}, errors.New("storage does not support Skill index synchronization")
	}
	packages, err := skillpackage.Discover(s.root)
	if err != nil {
		return ReindexResult{}, err
	}
	existing, err := s.Storage.ListSkills(ctx)
	if err != nil {
		return ReindexResult{}, err
	}
	byPath := make(map[string]*domain.Skill, len(existing))
	byID := make(map[string]*domain.Skill, len(existing))
	for i := range existing {
		byID[existing[i].ID] = &existing[i]
		if existing[i].PackagePath != "" {
			byPath[existing[i].PackagePath] = &existing[i]
		}
	}

	type plan struct {
		skill   domain.Skill
		created bool
		changed bool
	}
	plans := make([]plan, 0, len(packages))
	stamp := time.Now().UTC()
	for _, pkg := range packages {
		path, relErr := filepath.Rel(s.root, pkg.Root)
		if relErr != nil {
			return ReindexResult{}, relErr
		}
		path = filepath.ToSlash(path)
		if err = skillpackage.ValidateRelativePath(path); err != nil {
			return ReindexResult{}, err
		}
		doc, decodeErr := documentFromPackage(pkg)
		if decodeErr != nil {
			return ReindexResult{}, fmt.Errorf("decode %s: %w", path, decodeErr)
		}
		current := byPath[path]
		if current == nil {
			current = byID[path]
		}
		created := current == nil
		var sk domain.Skill
		if created {
			sk = domain.Skill{ID: packageID(path), Slug: path, Scope: domain.ScopeGlobal, Status: domain.StatusDraft, CurrentVersion: 1, CreatedAt: stamp, UpdatedAt: stamp}
		} else {
			sk = *current
		}
		previousSlug := sk.Slug
		applyDocument(&sk, doc, pkg.Body)
		if sk.Slug == "" {
			sk.Slug = previousSlug
		}
		if sk.Slug == "" {
			sk.Slug = path
		}
		sk.PackagePath, sk.PackageHash, sk.PackageIndexedAt = path, pkg.Hash, &stamp
		if err = sk.Validate(); err != nil {
			return ReindexResult{}, fmt.Errorf("validate %s: %w", path, err)
		}
		changed := created || current.PackageHash != pkg.Hash || current.PackagePath != path
		plans = append(plans, plan{skill: sk, created: created, changed: changed})
	}

	// Create all base rows before writing dependency relations so package order
	// cannot break a clean database rebuild.
	for i := range plans {
		if !plans[i].created {
			continue
		}
		base := plans[i].skill
		base.Dependencies = nil
		snapshotPath, snapshotCreated, snapshotErr := s.snapshotPackage(base.ID, base.PackagePath, base.CurrentVersion)
		if snapshotErr != nil {
			return ReindexResult{}, fmt.Errorf("snapshot %s: %w", base.PackagePath, snapshotErr)
		}
		if err = syncer.CreateSkillIndex(ctx, &base); err != nil {
			if snapshotCreated {
				_ = os.RemoveAll(snapshotPath)
			}
			return ReindexResult{}, fmt.Errorf("create index for %s: %w", plans[i].skill.PackagePath, err)
		}
	}

	result := ReindexResult{Discovered: len(plans)}
	for i := range plans {
		if plans[i].created {
			result.Created++
			if len(plans[i].skill.Dependencies) > 0 {
				if err = syncer.SyncSkillIndex(ctx, &plans[i].skill); err != nil {
					return ReindexResult{}, fmt.Errorf("sync dependencies for %s: %w", plans[i].skill.PackagePath, err)
				}
			}
			if err = syncer.RecordSkillVersion(ctx, &plans[i].skill, "filesystem package discovered", nil); err != nil {
				return ReindexResult{}, fmt.Errorf("record initial package version for %s: %w", plans[i].skill.PackagePath, err)
			}
			continue
		}
		if !plans[i].changed {
			result.Unchanged++
			continue
		}
		nextVersion := plans[i].skill.CurrentVersion + 1
		snapshotPath, snapshotCreated, snapshotErr := s.snapshotPackage(plans[i].skill.ID, plans[i].skill.PackagePath, nextVersion)
		if snapshotErr != nil {
			return ReindexResult{}, fmt.Errorf("snapshot changed package %s: %w", plans[i].skill.PackagePath, snapshotErr)
		}
		if err = s.Storage.UpdateSkill(ctx, &plans[i].skill, "filesystem package changed", nil); err != nil {
			if snapshotCreated {
				_ = os.RemoveAll(snapshotPath)
			}
			return ReindexResult{}, fmt.Errorf("version changed package %s: %w", plans[i].skill.PackagePath, err)
		}
		result.Updated++
	}
	return result, nil
}

func packageID(path string) string {
	if _, err := uuid.Parse(path); err == nil {
		return path
	}
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte("skillbox-package:"+path)).String()
}

func (s *Store) CreateSkill(ctx context.Context, sk *domain.Skill, summary string, actor *string) error {
	if sk.ID == "" {
		sk.ID = uuid.NewString()
	}
	if sk.CurrentVersion == 0 {
		sk.CurrentVersion = 1
	}
	if sk.CreatedAt.IsZero() {
		sk.CreatedAt = time.Now().UTC()
	}
	sk.UpdatedAt = sk.CreatedAt
	if err := sk.Validate(); err != nil {
		return err
	}
	path := sk.ID
	if err := s.write(sk, path); err != nil {
		return err
	}
	snapshotPath, snapshotCreated, err := s.snapshotPackage(sk.ID, path, sk.CurrentVersion)
	if err != nil {
		_ = os.RemoveAll(filepath.Join(s.root, path))
		return err
	}
	if err := s.Storage.CreateSkill(ctx, sk, summary, actor); err != nil {
		if snapshotCreated {
			_ = os.RemoveAll(snapshotPath)
		}
		_ = os.RemoveAll(filepath.Join(s.root, path))
		return err
	}
	return nil
}

func (s *Store) UpdateSkill(ctx context.Context, sk *domain.Skill, summary string, actor *string) error {
	existing, err := s.Storage.GetSkill(ctx, sk.ID)
	if err != nil {
		return err
	}
	path := existing.PackagePath
	if path == "" {
		// Legacy DB-only Skills remain supported. Their explicit migration is a
		// later stage, so updating one here must not migrate it implicitly.
		return s.Storage.UpdateSkill(ctx, sk, summary, actor)
	}
	previous, err := os.ReadFile(filepath.Join(s.root, path, skillpackage.SkillFileName))
	if err != nil {
		return fmt.Errorf("read current Skill package: %w", err)
	}
	sk.PackagePath = path
	if err = s.write(sk, path); err != nil {
		return err
	}
	snapshotPath, snapshotCreated, err := s.snapshotPackage(sk.ID, path, existing.CurrentVersion+1)
	if err != nil {
		_ = os.WriteFile(filepath.Join(s.root, path, skillpackage.SkillFileName), previous, 0o600)
		return err
	}
	if err = s.Storage.UpdateSkill(ctx, sk, summary, actor); err != nil {
		if snapshotCreated {
			_ = os.RemoveAll(snapshotPath)
		}
		_ = os.WriteFile(filepath.Join(s.root, path, skillpackage.SkillFileName), previous, 0o600)
		return err
	}
	return nil
}

func (s *Store) GetSkill(ctx context.Context, id string) (*domain.Skill, error) {
	sk, err := s.Storage.GetSkill(ctx, id)
	if err != nil || sk.PackagePath == "" {
		return sk, err
	}
	return s.hydrate(sk)
}

func (s *Store) ListSkills(ctx context.Context) ([]domain.Skill, error) {
	items, err := s.Storage.ListSkills(ctx)
	if err != nil {
		return nil, err
	}
	for i := range items {
		if items[i].PackagePath == "" {
			continue
		}
		hydrated, loadErr := s.hydrate(&items[i])
		if loadErr != nil {
			return nil, loadErr
		}
		items[i] = *hydrated
	}
	return items, nil
}

func (s *Store) RollbackSkill(ctx context.Context, id string, version int, actor *string) (*domain.Skill, error) {
	versioned, err := s.Storage.GetVersion(ctx, id, version)
	if err != nil {
		return nil, err
	}
	var sk domain.Skill
	if err = json.Unmarshal([]byte(versioned.Snapshot), &sk); err != nil {
		return nil, fmt.Errorf("decode snapshot: %w", err)
	}
	sk.ID = id
	current, err := s.Storage.GetSkill(ctx, id)
	if err != nil {
		return nil, err
	}
	if current.PackagePath == "" || versioned.PackageHash == "" {
		if err = s.UpdateSkill(ctx, &sk, fmt.Sprintf("rollback to version %d", version), actor); err != nil {
			return nil, err
		}
		return &sk, nil
	}
	currentRoot := filepath.Join(s.root, filepath.FromSlash(current.PackagePath))
	backupPath := filepath.Join(s.root, ".history", historyKey(id), ".rollback-"+uuid.NewString())
	if _, err = skillpackage.Snapshot(currentRoot, backupPath); err != nil {
		return nil, fmt.Errorf("backup package before rollback: %w", err)
	}
	defer os.RemoveAll(backupPath)
	restored, err := skillpackage.Restore(s.historyPath(id, version), currentRoot)
	if err != nil {
		return nil, err
	}
	if restored.Hash != versioned.PackageHash {
		_, _ = skillpackage.Restore(backupPath, currentRoot)
		return nil, fmt.Errorf("package snapshot hash mismatch: got %s want %s", restored.Hash, versioned.PackageHash)
	}
	sk.PackagePath, sk.PackageHash = current.PackagePath, restored.Hash
	indexedAt := time.Now().UTC()
	sk.PackageIndexedAt = &indexedAt
	hydrated, err := s.hydrate(&sk)
	if err != nil {
		_, _ = skillpackage.Restore(backupPath, currentRoot)
		return nil, err
	}
	sk = *hydrated
	newSnapshotPath, newSnapshotCreated, err := s.snapshotPackage(id, current.PackagePath, current.CurrentVersion+1)
	if err != nil {
		_, _ = skillpackage.Restore(backupPath, currentRoot)
		return nil, err
	}
	if err = s.Storage.UpdateSkill(ctx, &sk, fmt.Sprintf("rollback to version %d", version), actor); err != nil {
		if newSnapshotCreated {
			_ = os.RemoveAll(newSnapshotPath)
		}
		_, _ = skillpackage.Restore(backupPath, currentRoot)
		return nil, err
	}
	return &sk, nil
}

func historyKey(skillID string) string {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte("skillbox-history:"+skillID)).String()
}

func (s *Store) historyPath(skillID string, version int) string {
	return filepath.Join(s.root, ".history", historyKey(skillID), strconv.Itoa(version))
}

func (s *Store) snapshotPackage(skillID, packagePath string, version int) (string, bool, error) {
	if err := skillpackage.ValidateRelativePath(packagePath); err != nil {
		return "", false, err
	}
	destination := s.historyPath(skillID, version)
	_, statErr := os.Stat(destination)
	existed := statErr == nil
	if statErr != nil && !os.IsNotExist(statErr) {
		return "", false, statErr
	}
	_, err := skillpackage.Snapshot(filepath.Join(s.root, filepath.FromSlash(packagePath)), destination)
	return destination, !existed && err == nil, err
}

func (s *Store) write(sk *domain.Skill, path string) error {
	if err := skillpackage.ValidateRelativePath(path); err != nil {
		return err
	}
	root := filepath.Join(s.root, filepath.FromSlash(path))
	raw, err := json.Marshal(fromSkill(sk))
	if err != nil {
		return err
	}
	var metadata map[string]any
	if err = json.Unmarshal(raw, &metadata); err != nil {
		return err
	}
	if err = skillpackage.WriteSkillMD(root, metadata, sk.Instructions); err != nil {
		return err
	}
	pkg, err := skillpackage.Load(root)
	if err != nil {
		return err
	}
	indexedAt := time.Now().UTC()
	sk.PackagePath, sk.PackageHash, sk.PackageIndexedAt = path, pkg.Hash, &indexedAt
	return nil
}

func (s *Store) hydrate(sk *domain.Skill) (*domain.Skill, error) {
	if err := skillpackage.ValidateRelativePath(sk.PackagePath); err != nil {
		return nil, err
	}
	pkg, err := skillpackage.Load(filepath.Join(s.root, filepath.FromSlash(sk.PackagePath)))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("Skill package %q is missing: %w", sk.PackagePath, ports.ErrNotFound)
		}
		return nil, err
	}
	doc, err := documentFromPackage(pkg)
	if err != nil {
		return nil, err
	}
	applyDocument(sk, doc, pkg.Body)
	sk.PackageHash = pkg.Hash
	return sk, nil
}
