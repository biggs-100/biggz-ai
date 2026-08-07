package contracts

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// Compiler compiles contract schemas from the embedded FS and caches the
// compiled results per $id.
type Compiler struct {
	c     *jsonschema.Compiler
	mu    sync.Mutex
	cache map[string]*jsonschema.Schema
	added bool
}

// NewCompiler returns a Compiler whose loader resolves ONLY from the
// embedded FS: the jsonschema/v6 loader is consulted when a referenced
// resource was not pre-registered via AddEmbedded, and it never touches the
// network.
func NewCompiler() *Compiler {
	c := &Compiler{
		c:     jsonschema.NewCompiler(),
		cache: make(map[string]*jsonschema.Schema),
	}
	c.c.UseLoader(embeddedLoader{})
	return c
}

// embeddedLoader resolves a schema URL to its decoded document from the
// embedded FS by its declared $id.
type embeddedLoader struct{}

func (embeddedLoader) Load(url string) (any, error) {
	document, ok, err := schemaDocument(url)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("contracts: schema %q is not part of the embedded tree (network loading is disabled)", url)
	}
	return document, nil
}

// AddEmbedded registers EVERY schema under the embedded tree (both
// review-integration and sdd-integration) with its declared $id — including
// transitively referenced ones — so any later Compile can resolve any $ref.
// Idempotent: a second call is a no-op.
func (c *Compiler) AddEmbedded() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.added {
		return nil
	}
	err := fs.WalkDir(FS, ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".schema.json") {
			return nil
		}
		decoded, err := decodeSchemaFile(path)
		if err != nil {
			return err
		}
		if err := c.c.AddResource(decoded.ID, decoded.Document); err != nil {
			return fmt.Errorf("contracts: register %s: %w", path, err)
		}
		return nil
	})
	if err != nil {
		return err
	}
	c.added = true
	return nil
}

// Schema compiles (once, cached) the schema with the given $id. All embedded
// schemas are registered first, so transitive $refs resolve without any
// loader round trip.
func (c *Compiler) Schema(id string) (*jsonschema.Schema, error) {
	if err := c.AddEmbedded(); err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if cached, ok := c.cache[id]; ok {
		return cached, nil
	}
	schema, err := c.c.Compile(id)
	if err != nil {
		return nil, fmt.Errorf("contracts: compile %s: %w", id, err)
	}
	c.cache[id] = schema
	return schema, nil
}

// ValidateJSON validates a decoded JSON document against the schema with the
// given $id. Read-only.
func (c *Compiler) ValidateJSON(id string, document any) error {
	schema, err := c.Schema(id)
	if err != nil {
		return err
	}
	return schema.Validate(document)
}

// ValidateEnvelope validates wire envelope bytes against the schema with the
// given $id. Read-only.
func (c *Compiler) ValidateEnvelope(id string, payload []byte) error {
	var document any
	if err := json.Unmarshal(payload, &document); err != nil {
		return fmt.Errorf("contracts: decode envelope: %w", err)
	}
	return c.ValidateJSON(id, document)
}

// decodedSchema is the decoded document of one schema file with its $id.
type decodedSchema struct {
	ID       string
	Document any
}

// decodeSchemaFile reads and decodes one embedded schema file and extracts
// its declared $id.
func decodeSchemaFile(path string) (decodedSchema, error) {
	data, err := FS.ReadFile(path)
	if err != nil {
		return decodedSchema{}, fmt.Errorf("contracts: read %s: %w", path, err)
	}
	var document any
	if err := json.Unmarshal(data, &document); err != nil {
		return decodedSchema{}, fmt.Errorf("contracts: decode %s: %w", path, err)
	}
	var identity struct {
		ID string `json:"$id"`
	}
	if err := json.Unmarshal(data, &identity); err != nil {
		return decodedSchema{}, fmt.Errorf("contracts: identity of %s: %w", path, err)
	}
	if identity.ID == "" {
		return decodedSchema{}, fmt.Errorf("contracts: %s declares no $id", path)
	}
	return decodedSchema{ID: identity.ID, Document: document}, nil
}

var (
	idIndexOnce sync.Once
	idIndex     map[string]string
	idIndexErr  error
)

// indexSchemaIDs maps every embedded schema $id to its file path.
func indexSchemaIDs() (map[string]string, error) {
	idIndexOnce.Do(func() {
		idIndex = make(map[string]string)
		err := fs.WalkDir(FS, ".", func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".schema.json") {
				return nil
			}
			decoded, err := decodeSchemaFile(path)
			if err != nil {
				return err
			}
			idIndex[decoded.ID] = path
			return nil
		})
		idIndexErr = err
	})
	return idIndex, idIndexErr
}

// schemaDocument resolves a schema URL to its decoded document from the
// embedded FS. ok is false when the URL is not part of the embedded tree.
func schemaDocument(url string) (document any, ok bool, err error) {
	index, err := indexSchemaIDs()
	if err != nil {
		return nil, false, err
	}
	path, ok := index[url]
	if !ok {
		return nil, false, nil
	}
	decoded, err := decodeSchemaFile(path)
	if err != nil {
		return nil, false, err
	}
	return decoded.Document, true, nil
}
