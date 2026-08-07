package contracts

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// The default engine shared by the package-level helpers. It lazily embeds
// and registers every contract schema once per process.
var (
	defaultEngineOnce sync.Once
	defaultEngine     *Compiler
)

func engine() *Compiler {
	defaultEngineOnce.Do(func() {
		defaultEngine = NewCompiler()
	})
	return defaultEngine
}

// Schema compiles (once, cached) the schema with the given $id from the
// default engine.
func Schema(id string) (*jsonschema.Schema, error) {
	return engine().Schema(id)
}

// ValidateJSON validates a decoded JSON document against the schema with the
// given $id. Read-only: it never touches the event store, the ledger, or any
// other state.
func ValidateJSON(id string, document any) error {
	return engine().ValidateJSON(id, document)
}

// ValidateEnvelope validates wire envelope bytes against the schema with the
// given $id. Read-only: it never touches the event store, the ledger, or any
// other state.
func ValidateEnvelope(id string, payload []byte) error {
	var document any
	if err := json.Unmarshal(payload, &document); err != nil {
		return fmt.Errorf("contracts: decode envelope: %w", err)
	}
	return engine().ValidateJSON(id, document)
}
