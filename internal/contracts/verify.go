package contracts

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func VerifyProviderContract(lockPath, root string) error {
	b, err := os.ReadFile(lockPath)
	if err != nil {
		return err
	}
	var lock map[string]string
	if json.Unmarshal(b, &lock) != nil {
		return fmt.Errorf("parse lock")
	}
	act := map[string]string{}
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, e error) error {
		if e != nil || d.IsDir() || strings.HasSuffix(p, "provider-contract.lock.json") {
			return e
		}
		rel, _ := filepath.Rel(root, p)
		k := filepath.ToSlash(filepath.Join("contracts/review-integration", rel))
		data, _ := os.ReadFile(p)
		h := sha256.Sum256(data)
		act[k] = hex.EncodeToString(h[:])
		return nil
	})
	for k, v := range lock {
		if act[k] != v {
			return fmt.Errorf("drift %s", k)
		}
	}
	for k := range act {
		if _, ok := lock[k]; !ok {
			return fmt.Errorf("unlisted %s", k)
		}
	}
	return nil
}
