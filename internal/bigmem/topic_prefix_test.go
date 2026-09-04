package bigmem

import (
	"context"
	"testing"
)

func saveTopicRow(t *testing.T, store *Store, topicKey, project, scope string) string {
	t.Helper()
	obs := &Observation{
		Title:    topicKey,
		TopicKey: topicKey,
		Type:     "sdd",
		Content:  "content for " + topicKey,
		Project:  project,
		Scope:    scope,
	}
	if err := store.Save(obs); err != nil {
		t.Fatalf("save %s: %v", topicKey, err)
	}
	return obs.ID
}

func topicKeys(rows []TopicRow) map[string]bool {
	m := map[string]bool{}
	for _, r := range rows {
		m[r.TopicKey] = true
	}
	return m
}

func TestListByTopicPrefix_Predicates(t *testing.T) {
	ctx := context.Background()
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer store.Close()

	saveTopicRow(t, store, "sdd/alpha/proposal", "proj-a", "project")
	saveTopicRow(t, store, "sdd/alpha/tasks", "proj-a", "project")
	saveTopicRow(t, store, "other/alpha/proposal", "proj-a", "project")

	rows, err := store.ListByTopicPrefixCtx(ctx, "sdd/", "", false)
	if err != nil {
		t.Fatalf("ListByTopicPrefixCtx: %v", err)
	}
	got := topicKeys(rows)
	if len(rows) != 2 || !got["sdd/alpha/proposal"] || !got["sdd/alpha/tasks"] {
		t.Errorf("expected only sdd/ rows, got %v", got)
	}
	for _, r := range rows {
		if r.ID == "" || r.TopicKey == "" {
			t.Errorf("key-only row missing id/topic: %+v", r)
		}
	}
}

func TestListByTopicPrefix_DeletedExcluded(t *testing.T) {
	ctx := context.Background()
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer store.Close()

	id := saveTopicRow(t, store, "sdd/gone/proposal", "proj-a", "project")
	keep := saveTopicRow(t, store, "sdd/kept/proposal", "proj-a", "project")
	_ = keep
	if err := store.Delete(id); err != nil {
		t.Fatalf("delete: %v", err)
	}

	rows, err := store.ListByTopicPrefixCtx(ctx, "sdd/", "", false)
	if err != nil {
		t.Fatalf("ListByTopicPrefixCtx: %v", err)
	}
	got := topicKeys(rows)
	if got["sdd/gone/proposal"] {
		t.Errorf("deleted row must be excluded, got %v", got)
	}
	if !got["sdd/kept/proposal"] {
		t.Errorf("kept row missing, got %v", got)
	}
}

func TestListByTopicPrefix_PersonalExcluded(t *testing.T) {
	ctx := context.Background()
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer store.Close()

	saveTopicRow(t, store, "sdd/pub/proposal", "proj-a", "project")
	saveTopicRow(t, store, "sdd/priv/proposal", "proj-a", "personal")
	saveTopicRow(t, store, "sdd/priv2/proposal", "proj-a", "Personal")

	rows, err := store.ListByTopicPrefixCtx(ctx, "sdd/", "", true)
	if err != nil {
		t.Fatalf("ListByTopicPrefixCtx: %v", err)
	}
	got := topicKeys(rows)
	if !got["sdd/pub/proposal"] {
		t.Errorf("project row missing, got %v", got)
	}
	if got["sdd/priv/proposal"] || got["sdd/priv2/proposal"] {
		t.Errorf("personal rows must be excluded, got %v", got)
	}

	// excludePersonal=false keeps personal rows
	rows, err = store.ListByTopicPrefixCtx(ctx, "sdd/", "", false)
	if err != nil {
		t.Fatalf("ListByTopicPrefixCtx: %v", err)
	}
	got = topicKeys(rows)
	if !got["sdd/priv/proposal"] {
		t.Errorf("personal row should be visible when exclusion off, got %v", got)
	}
}

func TestListByTopicPrefix_ProjectMatchCaseInsensitive(t *testing.T) {
	ctx := context.Background()
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer store.Close()

	saveTopicRow(t, store, "sdd/case/proposal", "My-Case-Proj", "project")
	saveTopicRow(t, store, "sdd/other/proposal", "other-proj", "project")

	rows, err := store.ListByTopicPrefixCtx(ctx, "sdd/", "MY-CASE-PROJ", true)
	if err != nil {
		t.Fatalf("ListByTopicPrefixCtx: %v", err)
	}
	got := topicKeys(rows)
	if !got["sdd/case/proposal"] {
		t.Errorf("case-insensitive project match failed, got %v", got)
	}
	if got["sdd/other/proposal"] {
		t.Errorf("non-matching project must be excluded, got %v", got)
	}
}

func TestListByTopicPrefix_EmptyProjectBypass(t *testing.T) {
	ctx := context.Background()
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer store.Close()

	saveTopicRow(t, store, "sdd/a/proposal", "proj-a", "project")
	saveTopicRow(t, store, "sdd/b/proposal", "proj-b", "project")

	rows, err := store.ListByTopicPrefixCtx(ctx, "sdd/", "", true)
	if err != nil {
		t.Fatalf("ListByTopicPrefixCtx: %v", err)
	}
	if len(rows) != 2 {
		t.Errorf("project=\"\" must disable filter, got %d rows", len(rows))
	}
}

func TestListByTopicPrefix_CancelledCtx(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer store.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := store.ListByTopicPrefixCtx(ctx, "sdd/", "", true); err == nil {
		t.Error("expected error for cancelled ctx, got nil")
	}
}
