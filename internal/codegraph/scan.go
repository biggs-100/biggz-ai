package codegraph

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/tools/go/packages"
)

// ScanResult holds the discovered files and edges.
type ScanResult struct {
	Files []string `json:"files"`
	Edges []Edge   `json:"edges"`
}

var (
	scanCacheMu sync.Mutex
	scanCache   = make(map[string]*ScanResult)
)

// ScanGo scans Go source under cwd for import and call edges.
// Primary path uses go/packages with ctx (cached, 30s timeout); fallback uses parser+ast.Inspect.
// It is Go-only: only *.go files are considered.
func ScanGo(cwd string, ctx context.Context) (*ScanResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	root, err := resolveCwd(cwd)
	if err != nil {
		return nil, err
	}
	// Check cache
	scanCacheMu.Lock()
	if cached, ok := scanCache[root]; ok {
		scanCacheMu.Unlock()
		// Return clone to avoid mutation
		clone := &ScanResult{
			Files: append([]string(nil), cached.Files...),
			Edges: append([]Edge(nil), cached.Edges...),
		}
		return clone, nil
	}
	scanCacheMu.Unlock()

	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("scan timeout: %w", ctx.Err())
	default:
	}

	// Collect Go files (Go-only filter)
	var goFiles []string
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			// Skip common non-source dirs
			if name == ".git" || name == ".codegraph" || name == ".biggz" || name == "vendor" || name == ".atl" || name == ".engram" || name == ".bigmem" || name == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".go") {
			return nil
		}
		// Also skip testdata? No, include but they are Go files anyway
		rel, err := filepath.Rel(root, path)
		if err != nil {
			rel = path
		}
		rel = filepath.ToSlash(rel)
		goFiles = append(goFiles, rel)
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Try primary: go/packages
	if result, err := scanWithPackages(ctx, root, goFiles); err == nil && result != nil {
		// Cache success
		scanCacheMu.Lock()
		scanCache[root] = result
		scanCacheMu.Unlock()
		return result, nil
	}
	// Fallback: parser + ast.Inspect
	result, err := scanWithParser(root, goFiles)
	if err != nil {
		return nil, err
	}
	// Respect ctx cancellation
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("scan timeout: %w", ctx.Err())
	default:
	}
	scanCacheMu.Lock()
	scanCache[root] = result
	scanCacheMu.Unlock()
	return result, nil
}

func scanWithPackages(ctx context.Context, root string, relFiles []string) (*ScanResult, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedImports | packages.NeedFiles | packages.NeedCompiledGoFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedDeps,
		Dir:  root,
		// Use context from caller
		Context: ctx,
		Tests:   false,
	}
	// Load ./... pattern
	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		return nil, err
	}
	// If packages have errors, consider fallback
	hasPackages := false
	for _, p := range pkgs {
		if len(p.GoFiles) > 0 || len(p.CompiledGoFiles) > 0 {
			hasPackages = true
			break
		}
	}
	if !hasPackages {
		return nil, fmt.Errorf("no packages loaded")
	}
	// Build file -> package map
	fileToPkg := make(map[string]*packages.Package)
	pkgByPath := make(map[string]*packages.Package)
	for _, p := range pkgs {
		pkgByPath[p.PkgPath] = p
		for _, f := range p.GoFiles {
			rel, err := filepath.Rel(root, f)
			if err != nil {
				rel = f
			}
			rel = filepath.ToSlash(rel)
			fileToPkg[rel] = p
		}
		for _, f := range p.CompiledGoFiles {
			rel, err := filepath.Rel(root, f)
			if err != nil {
				rel = f
			}
			rel = filepath.ToSlash(rel)
			fileToPkg[rel] = p
		}
	}

	var edges []Edge
	// Import edges: for each package, for each import, create file-level edges
	for _, p := range pkgs {
		if p == nil {
			continue
		}
		for impPath, impPkg := range p.Imports {
			_ = impPath
			if impPkg == nil {
				continue
			}
			// Find importer files
			var importerFiles []string
			for _, f := range p.GoFiles {
				rel, _ := filepath.Rel(root, f)
				rel = filepath.ToSlash(rel)
				importerFiles = append(importerFiles, rel)
			}
			for _, f := range p.CompiledGoFiles {
				rel, _ := filepath.Rel(root, f)
				rel = filepath.ToSlash(rel)
				// avoid dup
				found := false
				for _, existing := range importerFiles {
					if existing == rel {
						found = true
						break
					}
				}
				if !found {
					importerFiles = append(importerFiles, rel)
				}
			}
			var importedFiles []string
			for _, f := range impPkg.GoFiles {
				rel, _ := filepath.Rel(root, f)
				rel = filepath.ToSlash(rel)
				importedFiles = append(importedFiles, rel)
			}
			for _, f := range impPkg.CompiledGoFiles {
				rel, _ := filepath.Rel(root, f)
				rel = filepath.ToSlash(rel)
				found := false
				for _, existing := range importedFiles {
					if existing == rel {
						found = true
						break
					}
				}
				if !found {
					importedFiles = append(importedFiles, rel)
				}
			}
			// If importedFiles empty (stdlib), skip file edge but still could create package-level edge
			// For test, we want file edges even for local imports. If no files, skip.
			if len(importedFiles) == 0 {
				continue
			}
			for _, from := range importerFiles {
				for _, to := range importedFiles {
					if from == to {
						continue
					}
					edges = append(edges, Edge{From: from, To: to, Reason: ReasonImport})
				}
			}
		}
	}

	// Call edges: inspect syntax with TypesInfo
	// Build file path -> ast file map for quick lookup
	for _, p := range pkgs {
		if p == nil || p.TypesInfo == nil {
			continue
		}
		for _, file := range p.Syntax {
			// Get file path
			if p.Fset == nil {
				continue
			}
			pos := p.Fset.Position(file.Pos())
			filename := pos.Filename
			rel, err := filepath.Rel(root, filename)
			if err != nil {
				rel = filename
			}
			rel = filepath.ToSlash(rel)
			// Walk AST for CallExpr
			ast.Inspect(file, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				var calleeObjPos string
				var calleeName string
				// Try to resolve via TypesInfo
				switch fun := call.Fun.(type) {
				case *ast.SelectorExpr:
					// e.g., pkg.Func or recv.Method
					if ident, ok := fun.X.(*ast.Ident); ok {
						// Use TypesInfo to find object of Sel
						if obj := p.TypesInfo.ObjectOf(fun.Sel); obj != nil && obj.Pos().IsValid() {
							if objPos := p.Fset.Position(obj.Pos()); objPos.IsValid() {
								calleeObjPos = objPos.Filename
								calleeName = fun.Sel.Name
							}
						} else if selObj := p.TypesInfo.ObjectOf(ident); selObj != nil {
							// fallback
							calleeName = fun.Sel.Name
						} else {
							calleeName = fun.Sel.Name
						}
					} else {
						if obj := p.TypesInfo.ObjectOf(fun.Sel); obj != nil && obj.Pos().IsValid() {
							if objPos := p.Fset.Position(obj.Pos()); objPos.IsValid() {
								calleeObjPos = objPos.Filename
								calleeName = fun.Sel.Name
							}
						} else {
							calleeName = fun.Sel.Name
						}
					}
				case *ast.Ident:
					if obj := p.TypesInfo.ObjectOf(fun); obj != nil && obj.Pos().IsValid() {
						if objPos := p.Fset.Position(obj.Pos()); objPos.IsValid() {
							calleeObjPos = objPos.Filename
							calleeName = fun.Name
						}
					} else {
						calleeName = fun.Name
					}
				default:
					return true
				}
				if calleeObjPos == "" || calleeName == "" {
					return true
				}
				// Map callee file to rel
				calleeRel, err := filepath.Rel(root, calleeObjPos)
				if err != nil {
					calleeRel = calleeObjPos
				}
				calleeRel = filepath.ToSlash(calleeRel)
				if calleeRel == rel {
					return true
				}
				// Validate Go-only
				if !strings.HasSuffix(calleeRel, ".go") {
					return true
				}
				edges = append(edges, Edge{From: rel, To: calleeRel, Reason: ReasonCall})
				return true
			})
		}
	}

	// Deduplicate edges
	edges = dedupEdges(edges)

	// Collect all files (relative) that are Go files + any involved in edges
	filesSet := make(map[string]struct{})
	for _, f := range relFiles {
		filesSet[f] = struct{}{}
	}
	for _, e := range edges {
		filesSet[e.From] = struct{}{}
		filesSet[e.To] = struct{}{}
	}
	var files []string
	for f := range filesSet {
		files = append(files, f)
	}
	// Sort for determinism
	// Use simple bubble? Use sort.Strings
	// Import sort
	// We'll sort inline
	for i := 0; i < len(files); i++ {
		for j := i + 1; j < len(files); j++ {
			if files[j] < files[i] {
				files[i], files[j] = files[j], files[i]
			}
		}
	}
	return &ScanResult{Files: files, Edges: edges}, nil
}

func scanWithParser(root string, relFiles []string) (*ScanResult, error) {
	// Build func definitions and package->files
	type fileInfo struct {
		relPath string
		absPath string
		pkgName string
		imports map[string]string // alias or base -> import path
	}
	var infos []fileInfo
	funcDefs := make(map[string][]string)         // func name -> files
	pkgToFiles := make(map[string][]string)       // pkg name -> files
	importPathToFiles := make(map[string][]string) // import path -> files (not known without module)

	fset := token.NewFileSet()
	for _, rel := range relFiles {
		abs := filepath.Join(root, filepath.FromSlash(rel))
		src, err := os.ReadFile(abs)
		if err != nil {
			continue
		}
		f, err := parser.ParseFile(fset, abs, src, parser.ParseComments)
		if err != nil {
			continue
		}
		pkgName := ""
		if f.Name != nil {
			pkgName = f.Name.Name
		}
		pkgToFiles[pkgName] = append(pkgToFiles[pkgName], rel)
		// Collect func defs
		for _, decl := range f.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name != nil {
				funcDefs[fn.Name.Name] = append(funcDefs[fn.Name.Name], rel)
				// Also method? Include recv? Still name is method name
			}
		}
		// Collect imports
		impMap := make(map[string]string)
		for _, imp := range f.Imports {
			path := strings.Trim(imp.Path.Value, `"`)
			alias := ""
			if imp.Name != nil {
				alias = imp.Name.Name
			} else {
				// base of path
				parts := strings.Split(path, "/")
				alias = parts[len(parts)-1]
			}
			impMap[alias] = path
			// Also map import path to files that provide that package? We don't have module resolution,
			// but we can attempt to map alias to pkgToFiles later (needs pkgToFiles built, but we have partial)
			importPathToFiles[path] = append(importPathToFiles[path], rel)
		}
		infos = append(infos, fileInfo{relPath: rel, absPath: abs, pkgName: pkgName, imports: impMap})
	}

	var edges []Edge
	// Import edges: for each file, for each import alias, find files that have matching pkgName
	for _, info := range infos {
		for alias, impPath := range info.imports {
			// Try alias first
			if files, ok := pkgToFiles[alias]; ok {
				for _, target := range files {
					if target == info.relPath {
						continue
					}
					edges = append(edges, Edge{From: info.relPath, To: target, Reason: ReasonImport})
				}
				continue
			}
			// Try last element of import path as package name (common)
			parts := strings.Split(impPath, "/")
			candidatePkg := parts[len(parts)-1]
			if candidatePkg != alias {
				if files, ok := pkgToFiles[candidatePkg]; ok {
					for _, target := range files {
						if target == info.relPath {
							continue
						}
						edges = append(edges, Edge{From: info.relPath, To: target, Reason: ReasonImport})
					}
				}
			}
		}
	}

	// Call edges: inspect CallExpr
	for _, info := range infos {
		abs := info.absPath
		src, err := os.ReadFile(abs)
		if err != nil {
			continue
		}
		f, err := parser.ParseFile(token.NewFileSet(), abs, src, 0)
		if err != nil {
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			var selName string
			var pkgAlias string
			switch fun := call.Fun.(type) {
			case *ast.SelectorExpr:
				if selName != "" {
					_ = selName
				}
				selName = fun.Sel.Name
				if ident, ok := fun.X.(*ast.Ident); ok {
					pkgAlias = ident.Name
				} else {
					// complex, skip
					return true
				}
			case *ast.Ident:
				// Direct call in same package — skip or create edge within package files?
				// We can create edge to other file in same package defining that func
				selName = fun.Name
				pkgAlias = ""
			default:
				return true
			}
			if selName == "" {
				return true
			}
			candidates, ok := funcDefs[selName]
			if !ok {
				return true
			}
			// Filter candidates: if pkgAlias != "", target must have pkgName == alias or import mapping
			for _, target := range candidates {
				if target == info.relPath {
					continue
				}
				// Find target pkg name
				targetPkg := ""
				for pkg, files := range pkgToFiles {
					for _, tf := range files {
						if tf == target {
							targetPkg = pkg
							break
						}
					}
					if targetPkg != "" {
						break
					}
				}
				if pkgAlias != "" {
					// Need to check if pkgAlias corresponds to import
					if impPath, exists := info.imports[pkgAlias]; exists {
						// Heuristic: pkg alias maps to import, and targetPkg should match alias or base of import
						base := pkgAlias
						// Also base of impPath
						parts := strings.Split(impPath, "/")
						impBase := parts[len(parts)-1]
						if targetPkg != base && targetPkg != impBase {
							continue
						}
					} else {
						// pkgAlias may be local variable, not import — skip unless targetPkg equals alias
						if targetPkg != pkgAlias {
							continue
						}
					}
				} else {
					// Direct call: only within same package
					if targetPkg != info.pkgName {
						continue
					}
				}
				edges = append(edges, Edge{From: info.relPath, To: target, Reason: ReasonCall})
				break // only one edge per call site for simplicity
			}
			return true
		})
	}

	edges = dedupEdges(edges)
	// Files set
	filesSet := make(map[string]struct{})
	for _, rel := range relFiles {
		filesSet[rel] = struct{}{}
	}
	for _, e := range edges {
		filesSet[e.From] = struct{}{}
		filesSet[e.To] = struct{}{}
	}
	var files []string
	for f := range filesSet {
		files = append(files, f)
	}
	// Sort
	for i := 0; i < len(files); i++ {
		for j := i + 1; j < len(files); j++ {
			if files[j] < files[i] {
				files[i], files[j] = files[j], files[i]
			}
		}
	}
	return &ScanResult{Files: files, Edges: edges}, nil
}

func dedupEdges(in []Edge) []Edge {
	seen := make(map[string]struct{})
	var out []Edge
	for _, e := range in {
		key := e.From + "\x00" + e.To + "\x00" + string(e.Reason)
		if _, ok := seen[key]; !ok {
			seen[key] = struct{}{}
			out = append(out, e)
		}
	}
	return out
}

// ClearScanCache clears the scan cache (for testing).
func ClearScanCache() {
	scanCacheMu.Lock()
	defer scanCacheMu.Unlock()
	clear(scanCache)
}
