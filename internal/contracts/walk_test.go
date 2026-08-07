package contracts

import (
	"encoding/json"
	"io/fs"
	"strings"
	"testing"
)

// walkContractFiles lists embedded files under the given subdir pattern.
func walkContractFiles(t *testing.T, suffix string) []string {
	t.Helper()
	var files []string
	err := fs.WalkDir(FS, ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), suffix) {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk embedded FS: %v", err)
	}
	return files
}

// fixtureSchemaPath maps a fixture file to its same-name schema file (same
// family/version, schemas/ instead of fixtures/).
func fixtureSchemaPath(fixturePath string) string {
	schemaPath := strings.Replace(fixturePath, "/fixtures/", "/schemas/", 1)
	return strings.TrimSuffix(schemaPath, ".fixture.json") + ".schema.json"
}

// TestContractsEverySchemaCompilesWithDeclaredID mirrors gentle-ai's
// validateReviewCapabilitiesSchema discipline (NewCompiler → AddResource(uri=
// $id, doc) → Compile(id) → Validate): every schemas/*.schema.json parses and
// compiles with its declared $id after AddEmbedded. gentle never had this
// directory-wide walk; this is biggz's structural improvement.
func TestContractsEverySchemaCompilesWithDeclaredID(t *testing.T) {
	compiler := NewCompiler()
	if err := compiler.AddEmbedded(); err != nil {
		t.Fatalf("AddEmbedded: %v", err)
	}
	schemas := walkContractFiles(t, ".schema.json")
	if len(schemas) == 0 {
		t.Fatal("no schemas embedded")
	}
	for _, schemaPath := range schemas {
		decoded, err := decodeSchemaFile(schemaPath)
		if err != nil {
			t.Fatalf("%s: %v", schemaPath, err)
		}
		schema, err := compiler.Schema(decoded.ID)
		if err != nil {
			t.Fatalf("%s: compile with declared $id %q failed: %v", schemaPath, decoded.ID, err)
		}
		if schema == nil {
			t.Fatalf("%s: compile returned nil schema for %q", schemaPath, decoded.ID)
		}
	}
}

// TestContractsEveryFixtureValidatesAgainstSameNameSchema checks the 1:1
// positive-fixture rule: every fixtures/*.fixture.json has a same-name schema
// and VALIDATES against it.
func TestContractsEveryFixtureValidatesAgainstSameNameSchema(t *testing.T) {
	compiler := NewCompiler()
	if err := compiler.AddEmbedded(); err != nil {
		t.Fatalf("AddEmbedded: %v", err)
	}
	fixtures := walkContractFiles(t, ".fixture.json")
	if len(fixtures) == 0 {
		t.Fatal("no fixtures embedded")
	}
	for _, fixturePath := range fixtures {
		schemaPath := fixtureSchemaPath(fixturePath)
		decoded, err := decodeSchemaFile(schemaPath)
		if err != nil {
			t.Fatalf("%s: same-name schema missing or invalid: %v", fixturePath, err)
		}
		payload, err := FS.ReadFile(fixturePath)
		if err != nil {
			t.Fatalf("%s: read fixture: %v", fixturePath, err)
		}
		var document any
		if err := json.Unmarshal(payload, &document); err != nil {
			t.Fatalf("%s: fixture is not valid JSON: %v", fixturePath, err)
		}
		if err := compiler.ValidateJSON(decoded.ID, document); err != nil {
			t.Fatalf("%s: rejected by %s: %v", fixturePath, schemaPath, err)
		}
	}
}

// TestContractsNegativeConformance mutates one field per case and asserts the
// schema rejects it: wrong schema const, missing required, extra key, bad
// sha256 identity, wrong enum, and the contract envelope carrying collect and
// execute together. Negative cases are programmatic mutations of the
// positive fixtures — there are deliberately no negative fixture files.
func TestContractsNegativeConformance(t *testing.T) {
	compiler := NewCompiler()
	if err := compiler.AddEmbedded(); err != nil {
		t.Fatalf("AddEmbedded: %v", err)
	}

	loadFixture := func(t *testing.T, name string) map[string]any {
		t.Helper()
		payload, err := FS.ReadFile("review-integration/v1/fixtures/" + name)
		if err != nil {
			t.Fatalf("read fixture %s: %v", name, err)
		}
		var document map[string]any
		if err := json.Unmarshal(payload, &document); err != nil {
			t.Fatalf("decode fixture %s: %v", name, err)
		}
		return document
	}
	envelope := func(t *testing.T, doc map[string]any) []byte {
		t.Helper()
		payload, err := json.Marshal(doc)
		if err != nil {
			t.Fatalf("marshal mutated fixture: %v", err)
		}
		return payload
	}
	nextTransition := func(t *testing.T, doc map[string]any) map[string]any {
		t.Helper()
		raw, ok := doc["next_transition"].(map[string]any)
		if !ok {
			t.Fatal("fixture carries no next_transition object")
		}
		return raw
	}

	tests := []struct {
		name    string
		fixture string
		schema  string
		mutate  func(t *testing.T, doc map[string]any)
	}{
		{
			name: "wrong schema const", fixture: "contract.fixture.json",
			schema: "https://biggz-ai.dev/contracts/review-integration/v1/schemas/contract.schema.json",
			mutate: func(t *testing.T, doc map[string]any) { doc["schema"] = "biggz-ai.review-integration/v2" },
		},
		{
			name: "missing required field", fixture: "receipt.fixture.json",
			schema: "https://biggz-ai.dev/contracts/review-integration/v1/schemas/receipt.schema.json",
			mutate: func(t *testing.T, doc map[string]any) { delete(doc, "terminal_state") },
		},
		{
			name: "extra key rejected", fixture: "start.fixture.json",
			schema: "https://biggz-ai.dev/contracts/review-integration/v1/schemas/start.schema.json",
			mutate: func(t *testing.T, doc map[string]any) { doc["surprise"] = true },
		},
		{
			name: "bad sha256 identity", fixture: "result-artifact.fixture.json",
			schema: "https://biggz-ai.dev/contracts/review-integration/v1/schemas/result-artifact.schema.json",
			mutate: func(t *testing.T, doc map[string]any) { doc["subject_hash"] = "sha256:not-a-hash" },
		},
		{
			name: "wrong enum value", fixture: "consent.fixture.json",
			schema: "https://biggz-ai.dev/contracts/review-integration/v1/schemas/consent.schema.json",
			mutate: func(t *testing.T, doc map[string]any) {
				candidate, ok := doc["candidate"].(map[string]any)
				if !ok {
					t.Fatal("fixture carries no candidate object")
				}
				candidate["risk"] = "extreme"
			},
		},
		{
			name: "collect and execute both present", fixture: "contract.fixture.json",
			schema: "https://biggz-ai.dev/contracts/review-integration/v1/schemas/contract.schema.json",
			mutate: func(t *testing.T, doc map[string]any) {
				nt := nextTransition(t, doc)
				nt["operation"] = "finalize"
				nt["arguments"] = []any{"lineage-contract-001"}
			},
		},
		{
			name: "collect envelope with reason_code", fixture: "contract.fixture.json",
			schema: "https://biggz-ai.dev/contracts/review-integration/v1/schemas/contract.schema.json",
			mutate: func(t *testing.T, doc map[string]any) {
				nextTransition(t, doc)["reason_code"] = "ready_for_gates"
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := loadFixture(t, tt.fixture)
			tt.mutate(t, doc)
			if err := compiler.ValidateEnvelope(tt.schema, envelope(t, doc)); err == nil {
				t.Fatalf("expected %s to reject the mutated fixture", tt.schema)
			}
		})
	}
}
