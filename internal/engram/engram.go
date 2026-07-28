// Package engram provides a lightweight persistent memory store for biggz-ai.
//
// It stores observations as JSON files under ~/.biggz/engram/observations/,
// organized by topic for efficient retrieval. This gives biggz-ai its own
// memory system without depending on external MCP servers.
package engram

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Observation is a single memory entry in the engram store.
type Observation struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Type      string    `json:"type"` // decision, architecture, bugfix, discovery, config, preference
	Content   string    `json:"content"`
	TopicKey  string    `json:"topic_key,omitempty"`
	Project   string    `json:"project,omitempty"`
	Scope     string    `json:"scope,omitempty"` // project, personal
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Store manages persistent observations.
type Store struct {
	mu       sync.RWMutex
	rootDir  string
	obsDir   string
}

// Open creates or opens an engram store at the given root directory.
// Default: ~/.biggz/engram/
func Open(rootDir string) (*Store, error) {
	if rootDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("home dir: %w", err)
		}
		rootDir = filepath.Join(home, ".biggz", "engram")
	}

	obsDir := filepath.Join(rootDir, "observations")
	if err := os.MkdirAll(obsDir, 0755); err != nil {
		return nil, fmt.Errorf("mkdir: %w", err)
	}

	return &Store{
		rootDir: rootDir,
		obsDir:  obsDir,
	}, nil
}

// Save persists an observation. If topic_key is set, updates existing.
func (s *Store) Save(obs *Observation) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if obs.ID == "" {
		obs.ID = fmt.Sprintf("obs-%d", time.Now().UnixNano())
	}

	// Check for existing by topic_key
	if obs.TopicKey != "" {
		existing, err := s.findByTopicKey(obs.TopicKey, obs.Project)
		if err == nil && existing != nil {
			obs.ID = existing.ID
			obs.CreatedAt = existing.CreatedAt
		}
	}

	now := time.Now()
	if obs.CreatedAt.IsZero() {
		obs.CreatedAt = now
	}
	obs.UpdatedAt = now

	return s.writeObservation(obs)
}

// Get retrieves an observation by ID.
func (s *Store) Get(id string) (*Observation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	path := filepath.Join(s.obsDir, id+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", id, err)
	}

	var obs Observation
	if err := json.Unmarshal(data, &obs); err != nil {
		return nil, fmt.Errorf("parse %s: %w", id, err)
	}
	return &obs, nil
}

// Search finds observations matching the query in title, content, and topic_key.
func (s *Store) Search(query string, opts SearchOptions) ([]*Observation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := os.ReadDir(s.obsDir)
	if err != nil {
		return nil, fmt.Errorf("read obs dir: %w", err)
	}

	query = strings.ToLower(query)
	var results []*Observation

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(s.obsDir, entry.Name()))
		if err != nil {
			continue
		}

		var obs Observation
		if err := json.Unmarshal(data, &obs); err != nil {
			continue
		}

		// Filter by project
		if opts.Project != "" && obs.Project != opts.Project {
			continue
		}
		// Filter by type
		if opts.Type != "" && obs.Type != opts.Type {
			continue
		}
		// Filter by scope
		if opts.Scope != "" && obs.Scope != opts.Scope {
			continue
		}

		// Match query
		if query != "" {
			matches := strings.Contains(strings.ToLower(obs.Title), query) ||
				strings.Contains(strings.ToLower(obs.Content), query) ||
				strings.Contains(strings.ToLower(obs.TopicKey), query)
			if !matches {
				continue
			}
		}

		results = append(results, &obs)

		if opts.Limit > 0 && len(results) >= opts.Limit {
			break
		}
	}

	// Sort by UpdatedAt desc (most recent first)
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].UpdatedAt.After(results[i].UpdatedAt) {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	return results, nil
}

// Delete removes an observation by ID.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := filepath.Join(s.obsDir, id+".json")
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("delete %s: %w", id, err)
	}
	return nil
}

// --- internal ---

func (s *Store) writeObservation(obs *Observation) error {
	data, err := json.MarshalIndent(obs, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	path := filepath.Join(s.obsDir, obs.ID+".json")
	return os.WriteFile(path, data, 0644)
}

func (s *Store) findByTopicKey(topicKey, project string) (*Observation, error) {
	entries, err := os.ReadDir(s.obsDir)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.obsDir, entry.Name()))
		if err != nil {
			continue
		}
		var obs Observation
		if err := json.Unmarshal(data, &obs); err != nil {
			continue
		}
		if obs.TopicKey == topicKey {
			if project == "" || obs.Project == project {
				return &obs, nil
			}
		}
	}
	return nil, fmt.Errorf("not found")
}

// SearchOptions filters engram search results.
type SearchOptions struct {
	Project string
	Type    string
	Scope   string
	Limit   int
}

// RootDir returns the engram store root directory.
func (s *Store) RootDir() string { return s.rootDir }
