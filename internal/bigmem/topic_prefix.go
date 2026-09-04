package bigmem

import (
	"context"
	"fmt"
	"strings"
)

// TopicRow is a key-only row for the sdd/% status sweep.
// Content is intentionally omitted: callers hydrate via GetCtx only for
// rows surviving visibility filters (minimal hydration).
type TopicRow struct {
	ID       string
	TopicKey string
	Project  string
	Scope    string
}

// escapeLikePrefix escapes LIKE wildcards in a topic prefix.
func escapeLikePrefix(prefix string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return replacer.Replace(prefix)
}

// ListByTopicPrefixCtx lists key-only topic rows for a topic prefix.
// SQL-side filters: topic_key LIKE prefix%, deleted_at IS NULL, optional
// project equality (COLLATE NOCASE) and optional personal-scope exclusion
// (COLLATE NOCASE). project=="" disables the project filter (preserves the
// bigmemStoreRootOverride test semantics). Results are ORDER BY topic_key
// with no cap. Served by idx_obs_topic_lookup(topic_key, project, scope).
func (s *Store) ListByTopicPrefixCtx(ctx context.Context, prefix, project string, excludePersonal bool) ([]TopicRow, error) {
	ctx, cancel := WithTimeout(ctx)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("bigmem sdd-status list-topics: %w", err)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `SELECT id, COALESCE(topic_key,''), COALESCE(project,''), COALESCE(scope,'')
		FROM observations WHERE topic_key LIKE ? ESCAPE '\' AND deleted_at IS NULL`
	args := []any{escapeLikePrefix(prefix) + "%"}
	if strings.TrimSpace(project) != "" {
		query += ` AND project COLLATE NOCASE = ?`
		args = append(args, strings.TrimSpace(project))
	}
	if excludePersonal {
		query += ` AND (scope IS NULL OR scope COLLATE NOCASE != 'personal')`
	}
	query += ` ORDER BY topic_key`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("bigmem sdd-status list-topics: %w", ctx.Err())
		}
		return nil, fmt.Errorf("bigmem sdd-status list-topics: %w", err)
	}
	defer rows.Close()

	var out []TopicRow
	for rows.Next() {
		var row TopicRow
		if err := rows.Scan(&row.ID, &row.TopicKey, &row.Project, &row.Scope); err != nil {
			if ctx.Err() != nil {
				return nil, fmt.Errorf("bigmem sdd-status list-topics: %w", ctx.Err())
			}
			return nil, fmt.Errorf("bigmem sdd-status list-topics: %w", err)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("bigmem sdd-status list-topics: %w", ctx.Err())
		}
		return nil, fmt.Errorf("bigmem sdd-status list-topics: %w", err)
	}
	return out, nil
}
