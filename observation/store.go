package observation

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
)

// Store materializes immutable receipt objects and mutable per-run references.
type Store struct {
	root string
}

// NewStore returns the receipt store rooted under the supplied .yield directory.
func NewStore(yieldDir string) *Store {
	return &Store{root: filepath.Join(yieldDir, "receipts")}
}

// Materialize projects and durably stores the receipt for one exact prefix.
func (s *Store) Materialize(journalPrefix []byte) (RunReceipt, []byte, error) {
	r, err := Project(journalPrefix)
	if err != nil {
		return RunReceipt{}, nil, err
	}
	raw, err := CanonicalBytes(r)
	if err != nil {
		return RunReceipt{}, nil, err
	}
	if err := s.Put(r, raw); err != nil {
		return RunReceipt{}, nil, err
	}
	return r, raw, nil
}

// Put durably stores an already-sealed receipt.
func (s *Store) Put(r RunReceipt, raw []byte) error {
	if r.ReceiptDigest == "" || r.Run.ID == "" {
		return fmt.Errorf("receipt store: incomplete receipt")
	}
	if err := r.Validate(); err != nil {
		return err
	}
	if err := VerifyCanonical(r, raw); err != nil {
		return err
	}
	digest := strings.TrimPrefix(r.ReceiptDigest, "sha256:")
	if len(digest) != 64 {
		return fmt.Errorf("receipt store: invalid receipt digest")
	}
	objects := filepath.Join(s.root, "objects", "sha256", digest[:2])
	references := filepath.Join(s.root, "runs")
	if err := secureMkdirAll(objects); err != nil {
		return err
	}
	if err := secureMkdirAll(references); err != nil {
		return err
	}
	objectPath := filepath.Join(objects, digest+".json")
	if err := writeImmutable(objectPath, raw); err != nil {
		return err
	}
	return writeReplace(filepath.Join(references, r.Run.ID+".ref"), []byte(r.ReceiptDigest+"\n"))
}

// LoadRun reads the latest materialized receipt for a run.
func (s *Store) LoadRun(runID string) (RunReceipt, []byte, error) {
	ref, err := os.ReadFile(filepath.Join(s.root, "runs", runID+".ref"))
	if err != nil {
		return RunReceipt{}, nil, err
	}
	digest := strings.TrimSpace(string(ref))
	return s.LoadDigest(digest)
}

// LoadDigest reads and verifies an immutable receipt object.
func (s *Store) LoadDigest(receiptDigest string) (RunReceipt, []byte, error) {
	if !validDigest(receiptDigest) {
		return RunReceipt{}, nil, fmt.Errorf("receipt store: invalid receipt digest")
	}
	hexDigest := strings.TrimPrefix(receiptDigest, "sha256:")
	raw, err := os.ReadFile(filepath.Join(s.root, "objects", "sha256", hexDigest[:2], hexDigest+".json"))
	if err != nil {
		return RunReceipt{}, nil, err
	}
	r, err := Parse(raw)
	if err != nil {
		return RunReceipt{}, nil, fmt.Errorf("receipt store: decode object: %w", err)
	}
	if r.ReceiptDigest != receiptDigest {
		return RunReceipt{}, nil, fmt.Errorf("receipt store: reference and object digest differ")
	}
	return r, raw, nil
}

// ListRuns returns run IDs with materialized latest-receipt references.
func (s *Store) ListRuns() ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(s.root, "runs"))
	if errors.Is(err, os.ErrNotExist) {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".ref") {
			ids = append(ids, strings.TrimSuffix(entry.Name(), ".ref"))
		}
	}
	sort.Strings(ids)
	return ids, nil
}

func writeImmutable(path string, content []byte) error {
	if existing, err := os.ReadFile(path); err == nil {
		if !bytes.Equal(existing, content) {
			return fmt.Errorf("receipt store: digest collision at %s", path)
		}
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".receipt-object-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(content); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Link(tmpPath, path); err != nil {
		if existing, readErr := os.ReadFile(path); readErr == nil {
			if !bytes.Equal(existing, content) {
				return fmt.Errorf("receipt store: digest collision at %s", path)
			}
			return nil
		}
		return err
	}
	return syncDir(filepath.Dir(path))
}

func writeReplace(path string, content []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".receipt-ref-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(content); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	return syncDir(filepath.Dir(path))
}

func secureMkdirAll(path string) error {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return err
	}
	return os.Chmod(path, 0o700)
}

func syncDir(path string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

func expectEOF(decoder *json.Decoder) error {
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			err = fmt.Errorf("multiple JSON values")
		}
		return fmt.Errorf("receipt store: trailing content: %w", err)
	}
	return nil
}

var experimentIdentifier = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]*$`)
var semanticVersion = regexp.MustCompile(`^\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.-]+)?$`)
var sourceDigestProfile = regexp.MustCompile(`^yield\.skill-source\.v[0-9]+$`)

// DecodeExperiment admits a closed, non-personal experiment context.
func DecodeExperiment(raw []byte) (*ExperimentContext, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var context ExperimentContext
	if err := decoder.Decode(&context); err != nil {
		return nil, fmt.Errorf("experiment does not decode: %w", err)
	}
	if err := expectEOF(decoder); err != nil {
		return nil, err
	}
	if err := context.Validate(); err != nil {
		return nil, err
	}
	return &context, nil
}

func (context ExperimentContext) Validate() error {
	for name, value := range map[string]string{
		"experiment_id": context.ExperimentID,
		"variant_id":    context.VariantID,
	} {
		if !validExperimentIdentifier(value) {
			return fmt.Errorf("experiment %s is not a portable identifier", name)
		}
	}
	for name, value := range map[string]string{
		"cohort_id":           context.CohortID,
		"baseline_variant_id": context.BaselineVariantID,
	} {
		if value != "" && !validExperimentIdentifier(value) {
			return fmt.Errorf("experiment %s is not a portable identifier", name)
		}
	}
	if context.Role != "baseline" && context.Role != "candidate" {
		return fmt.Errorf("experiment role must be baseline or candidate")
	}
	if context.ParentSkillVersion != "" && !semanticVersion.MatchString(context.ParentSkillVersion) {
		return fmt.Errorf("experiment parent_skill_version must be an exact semantic version")
	}
	return nil
}

func validExperimentIdentifier(value string) bool {
	return len(value) > 0 && len(value) <= 128 && experimentIdentifier.MatchString(value)
}
