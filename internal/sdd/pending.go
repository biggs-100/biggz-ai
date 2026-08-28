package sdd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/biggs-100/biggz-ai/internal/bigmem"
	"gopkg.in/yaml.v3"
)

const PendingSchema = "biggz-ai.pending-question/v1"

type PendingQuestion struct {
	Schema      string           `json:"schema" yaml:"schema"`
	Change      string           `json:"change,omitempty" yaml:"change,omitempty"`
	Envelope    QuestionEnvelope `json:"envelope" yaml:"envelope"`
	SynthesisMD string           `json:"synthesis_md" yaml:"synthesis_md"`
	CreatedAt   string           `json:"created_at,omitempty" yaml:"created_at,omitempty"`
}

func pendingTopicKey(c string) string { return fmt.Sprintf("sdd/%s/pending-question", c) }
func pendingStatePath(root, c string) string {
	if root == "" {
		root = "."
	}
	return filepath.Join(root, "openspec", "changes", c, "state.yaml")
}
func findWorkspaceRoot() string {
	wd, _ := os.Getwd()
	for d := wd; ; {
		if _, err := os.Stat(filepath.Join(d, "openspec")); err == nil {
			return d
		}
		p := filepath.Dir(d)
		if p == d {
			return wd
		}
		d = p
	}
}
func openPendingStore() (*bigmem.Store, error) { return bigmem.Open(bigmemStoreRootOverride) }

func writePendingToState(root, change string, pq PendingQuestion) error {
	path := pendingStatePath(root, change)
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	var m map[string]any
	if data, err := os.ReadFile(path); err == nil && len(data) > 0 {
		_ = yaml.Unmarshal(data, &m)
	}
	if m == nil {
		m = make(map[string]any)
	}
	b, _ := json.Marshal(pq)
	var pm map[string]any
	_ = json.Unmarshal(b, &pm)
	m["pending_question"] = pm
	out, _ := yaml.Marshal(m)
	return os.WriteFile(path, out, 0o644)
}
func readPendingFromState(root, change string) (PendingQuestion, error) {
	data, err := os.ReadFile(pendingStatePath(root, change))
	if err != nil {
		return PendingQuestion{}, fmt.Errorf("read state: %w", err)
	}
	var m map[string]any
	if err := yaml.Unmarshal(data, &m); err != nil {
		return PendingQuestion{}, err
	}
	raw, ok := m["pending_question"]
	if !ok {
		return PendingQuestion{}, fmt.Errorf("pending_question not found")
	}
	b, _ := json.Marshal(raw)
	var pq PendingQuestion
	if err := json.Unmarshal(b, &pq); err != nil {
		return PendingQuestion{}, err
	}
	return pq, nil
}
func savePendingToBigMem(root, change string, pq PendingQuestion) error {
	if pq.Schema == "" {
		pq.Schema = PendingSchema
	}
	if pq.CreatedAt == "" {
		pq.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	if pq.Change == "" {
		pq.Change = change
	}
	b, _ := json.Marshal(pq)
	store, err := openPendingStore()
	if err != nil {
		return err
	}
	defer store.Close()
	proj := strings.ToLower(strings.TrimSpace(inferBigMemProject(root)))
	if proj == "" {
		proj = "bigg"
	}
	return store.Save(&bigmem.Observation{TopicKey: pendingTopicKey(change), Type: "pending-question", Title: change, Content: string(b), Project: proj, Scope: "project"})
}
func loadPendingFromBigMem(root, change string) (PendingQuestion, error) {
	topic := pendingTopicKey(change)
	store, err := openPendingStore()
	if err != nil {
		return PendingQuestion{}, err
	}
	defer store.Close()
	results, err := store.Search(topic, bigmem.SearchOptions{Limit: 1})
	if err != nil {
		return PendingQuestion{}, err
	}
	for _, r := range results {
		if strings.EqualFold(strings.TrimSpace(r.TopicKey), topic) {
			var pq PendingQuestion
			if err := json.Unmarshal([]byte(r.Content), &pq); err != nil {
				return PendingQuestion{}, err
			}
			return pq, nil
		}
	}
	return PendingQuestion{}, fmt.Errorf("not found %s", topic)
}
func SavePendingDualWriteAt(root, change string, pq PendingQuestion) error {
	if strings.TrimSpace(change) == "" {
		return fmt.Errorf("change required")
	}
	if pq.Schema == "" {
		pq.Schema = PendingSchema
	}
	if pq.Change == "" {
		pq.Change = change
	}
	if pq.CreatedAt == "" {
		pq.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	if pq.Schema != PendingSchema {
		return fmt.Errorf("invalid schema %q", pq.Schema)
	}
	do := func() error {
		if err := savePendingToBigMem(root, change, pq); err != nil {
			return err
		}
		return writePendingToState(root, change, pq)
	}
	if err := do(); err != nil {
		return err
	}
	if eq, err := VerifyEqualityAt(root, change); err != nil {
		return err
	} else if !eq {
		if err := do(); err != nil {
			return err
		}
		if eq2, err := VerifyEqualityAt(root, change); err != nil {
			return err
		} else if !eq2 {
			return fmt.Errorf("verify failed for %s", change)
		}
	}
	return nil
}
func SavePendingDualWrite(change string, pq PendingQuestion) error {
	return SavePendingDualWriteAt(findWorkspaceRoot(), change, pq)
}
func VerifyEqualityAt(root, change string) (bool, error) {
	bm, errBM := loadPendingFromBigMem(root, change)
	fs, errFS := readPendingFromState(root, change)
	if errBM != nil && errFS != nil {
		return false, fmt.Errorf("both missing: %v %v", errBM, errFS)
	}
	if errBM != nil || errFS != nil {
		return false, nil
	}
	a, _ := json.Marshal(bm)
	b, _ := json.Marshal(fs)
	return string(a) == string(b), nil
}
func VerifyEquality(change string) (bool, error) { return VerifyEqualityAt(findWorkspaceRoot(), change) }
func LoadOnCompactionAt(root, change string) (PendingQuestion, error) {
	if pq, err := loadPendingFromBigMem(root, change); err == nil {
		return pq, nil
	}
	return readPendingFromState(root, change)
}
func LoadOnCompaction(change string) (PendingQuestion, error) {
	return LoadOnCompactionAt(findWorkspaceRoot(), change)
}
func PendingFallbackMD(pq PendingQuestion) string {
	if len(pq.Envelope.Questions) == 0 && len(pq.Envelope.Options) == 0 {
		return strings.TrimSpace(pq.SynthesisMD)
	}
	return FormatFallback(pq.Envelope)
}
