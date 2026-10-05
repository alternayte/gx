package compiler

import (
	"crypto/sha256"
	"fmt"
	"go/ast"
	goparser "go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"golang.org/x/tools/go/packages"
)

// Session caches one module analysis across Generate calls, so a change to
// one .gx body re-checks that file alone and regenerates that file alone
// (NFR-05, REQ-DEV-03). A change to a Go file, to a .gx signature or to the
// file set falls back to a full Generate.
type Session struct {
	mu sync.Mutex

	root string
	l    *loader
	dirs []string
	res  *typesResult

	files     map[string][]byte
	lastDiags []Diagnostic
	goDiags   []Diagnostic
	stamps    map[string]fileStamp
	sigs      map[string]string
	imp       types.Importer
	seq       int
	overlay   map[string][]byte
}

// fileStamp is the content hash of one input file.
type fileStamp struct {
	hash [sha256.Size]byte
}

// NewSession returns an empty compiler session.
func NewSession() *Session { return &Session{} }

// Generate returns the generated files and diagnostics of root, reusing the
// previous analysis when only .gx bodies changed (NFR-05).
func (s *Session) Generate(root string) (map[string][]byte, []Diagnostic) {
	root = absoluteRoot(root)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.root != "" && s.root != root {
		s.reset()
	}
	stamps, err := snapshotInputsWith(root, s.overlay)
	if err != nil {
		files, diags, _, _, _ := generate(root, s.overlay)
		return files, diags
	}
	if s.res == nil || s.stamps == nil || !sameFileSet(s.stamps, stamps) || goInputsChanged(s.stamps, stamps) {
		return s.full(root, stamps)
	}
	changed := changedGXFiles(s.stamps, stamps)
	if len(changed) == 0 {
		s.refreshContent(s.files)
		return s.files, s.lastDiags
	}
	for path := range changed {
		data, ok := inputContent(path, s.overlay)
		if !ok {
			continue // a parse error: handle it incrementally
		}
		sig, ok := fileSignatureData(path, data)
		if !ok {
			continue
		}
		if old, had := s.sigs[path]; !had || old != sig {
			return s.full(root, stamps)
		}
	}
	return s.incremental(root, changed, stamps)
}

// full discards the cache and runs the whole analysis.
func (s *Session) full(root string, stamps map[string]fileStamp) (map[string][]byte, []Diagnostic) {
	files, diags, res, l, dirs := generate(root, s.overlay)
	s.root, s.l, s.dirs, s.res = root, l, dirs, res
	s.files = files
	s.lastDiags = diags
	s.goDiags = nil
	for _, d := range diags {
		if !strings.HasSuffix(d.File, ".gx") {
			s.goDiags = append(s.goDiags, d)
		}
	}
	s.stamps = stamps
	s.imp = buildImporter(res.pkgs)
	s.sigs = map[string]string{}
	for path := range stamps {
		if strings.HasSuffix(path, ".gx") {
			if data, ok := inputContent(path, s.overlay); ok {
				if sig, ok := fileSignatureData(path, data); ok {
					s.sigs[path] = sig
				}
			}
		}
	}
	return files, diags
}

// incremental re-checks the changed files against the cached packages and
// regenerates only those files (NFR-05).
func (s *Session) incremental(root string, changed map[string]bool, stamps map[string]fileStamp) (map[string][]byte, []Diagnostic) {
	s.l.overlay = s.overlay
	// Record what was on disk for this pass even when it fails, so the next
	// change is diffed against the observed state.
	s.stamps = stamps
	refresh := map[string]bool{}
	for path := range changed {
		refresh[filepath.Dir(path)] = true
	}
	for dir := range refresh {
		delete(s.l.pkgs, dir)
	}
	var diags []Diagnostic
	for _, dir := range s.dirs {
		diags = append(diags, s.l.checkDir(dir)...)
	}
	// A reloaded directory re-parses every .gx file in it, so every file
	// there is re-checked against the new ASTs.
	recheck := map[string]bool{}
	for path := range changed {
		recheck[path] = true
	}
	for dir := range refresh {
		p := s.l.load(dir)
		for _, f := range p.Files {
			recheck[f.File] = true
		}
	}
	for path := range recheck {
		s.recheck(path)
	}
	paths := make([]string, 0, len(s.res.typeDiags))
	for path := range s.res.typeDiags {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		diags = append(diags, s.res.typeDiags[path]...)
	}
	diags = append(diags, s.l.checkAttributes(s.res, s.dirs)...)
	diags = append(diags, s.l.checkActionInvocations(s.res, s.dirs)...)
	diags = append(diags, s.l.checkSignals(s.dirs)...)
	diags = append(diags, s.l.checkSecrets(s.res, s.dirs)...)
	diags = append(diags, s.l.checkKeys(s.dirs)...)
	diags = append(diags, s.goDiags...)
	if len(diags) > 0 {
		sortDiags(diags)
		s.lastDiags = diags
		return nil, diags
	}
	if s.files == nil {
		// The last full run failed. Rebuild everything once the tree is
		// clean again.
		return s.full(root, stamps)
	}
	out := make(map[string][]byte, len(s.files))
	for path, src := range s.files {
		out[path] = src
	}
	for path := range changed {
		p := s.l.load(filepath.Dir(path))
		base := strings.TrimSuffix(filepath.Base(path), ".gx")
		genPath := strings.TrimSuffix(path, ".gx") + "_gx.go"
		f := p.Files[base]
		if f == nil {
			delete(out, genPath)
			continue
		}
		src, gdiags := generateFile(s.l, p, base, f, s.res)
		if len(gdiags) > 0 {
			diags = append(diags, gdiags...)
			continue
		}
		out[genPath] = src
	}
	if len(diags) > 0 {
		sortDiags(diags)
		s.lastDiags = diags
		return nil, diags
	}
	out[classesFilePath(root)] = classesBytes(collectClasses(s.dirs, s.l, s.res.pkgs))
	out[galleryFilePath(root)] = renderGallery(root, s.dirs, s.l, s.res.pkgs)
	s.refreshContent(out)
	s.files = out
	s.lastDiags = nil
	s.stamps = stamps
	for path := range changed {
		if data, ok := inputContent(path, s.overlay); ok {
			if sig, ok := fileSignatureData(path, data); ok {
				s.sigs[path] = sig
				continue
			}
		}
		delete(s.sigs, path)
	}
	return out, nil
}

// recheck type-checks one changed .gx file against the cached packages and
// refreshes its contributions to the analysis (NFR-05).
func (s *Session) recheck(path string) {
	dir := filepath.Dir(path)
	p := s.l.load(dir)
	base := strings.TrimSuffix(filepath.Base(path), ".gx")
	f := p.Files[base]
	if f == nil {
		delete(s.res.typeDiags, path)
		return
	}
	synth := fmt.Sprintf("gxr%d", s.seq)
	s.seq++
	pr, src := buildProbe(s.l, p, f, synth)
	refs := map[string]synthRef{synth: {file: f}}
	for _, name := range pr.frags {
		refs[name] = synthRef{file: f, code: CodeFragment}
	}
	for idx, name := range pr.defs {
		fld := f.Props[idx]
		refs[name] = synthRef{file: f, line: fld.At.Line, col: fld.At.Col}
	}
	fset := token.NewFileSet()
	// The probe name is not a ref: only positions that its //line directives
	// move into a ref map to the .gx file, exactly as packages.Load sees it.
	file, err := goparser.ParseFile(fset, synth+".go", src, goparser.ParseComments)
	if err != nil {
		s.res.typeDiags[path] = []Diagnostic{{
			Code: CodeParse,
			File: f.File,
			Line: 1,
			Col:  1,
			Msg:  "cannot parse the generated probe: " + err.Error(),
		}}
		return
	}
	info := &types.Info{
		Types:      map[ast.Expr]types.TypeAndValue{},
		Defs:       map[*ast.Ident]types.Object{},
		Uses:       map[*ast.Ident]types.Object{},
		Implicits:  map[ast.Node]types.Object{},
		Selections: map[*ast.SelectorExpr]*types.Selection{},
		Scopes:     map[ast.Node]*types.Scope{},
		Instances:  map[*ast.Ident]types.Instance{},
	}
	var tds []Diagnostic
	conf := types.Config{Importer: s.imp, Error: func(err error) {
		terr, ok := err.(types.Error)
		if !ok {
			return
		}
		if d, ok := mapTypeError(refs, fset.Position(terr.Pos), terr.Msg); ok {
			tds = append(tds, d)
		}
	}}
	// Seed the package scope with the cached package-level declarations, so
	// a probe that names a same-package Go symbol type-checks (REQ-DEV-08).
	tpkg := types.NewPackage(s.packagePath(dir), f.Package)
	if cached, err := s.imp.Import(tpkg.Path()); err == nil && cached != nil {
		skip := map[string]bool{
			componentName(f):           true,
			componentName(f) + "Props": true,
		}
		for _, name := range cached.Scope().Names() {
			if skip[name] || strings.HasPrefix(name, "_gx") {
				continue
			}
			tpkg.Scope().Insert(cached.Scope().Lookup(name))
		}
	}
	check := types.NewChecker(&conf, fset, tpkg, info)
	_ = check.Files([]*ast.File{file})
	s.dropFile(f)
	s.l.collectTypes(s.res, info, fset, tpkg, file, pr)
	if tpkg != nil {
		s.l.collectPropTypes(s.res, tpkg, pr)
	}
	for _, d := range pr.clientDiags {
		tds = append(tds, d)
	}
	for _, site := range pr.clients {
		s.res.clientSites = append(s.res.clientSites, site)
		tds = append(tds, s.res.checkClientSite(site)...)
	}
	s.res.typeDiags[path] = tds
}

// packagePath returns the import path of a package directory.
func (s *Session) packagePath(dir string) string {
	p := s.l.load(dir)
	if p.Module == nil {
		return "gxcheck/" + filepath.Base(dir)
	}
	if rel, err := filepath.Rel(p.Module.Dir, dir); err == nil && rel != "." {
		return p.Module.Path + "/" + filepath.ToSlash(rel)
	}
	return p.Module.Path
}

// dropFile removes the cached type contributions of one .gx file.
func (s *Session) dropFile(f *File) {
	kept := s.res.clientSites[:0]
	for _, site := range s.res.clientSites {
		if site.file != f {
			kept = append(kept, site)
		}
	}
	s.res.clientSites = kept
	for attr, site := range s.res.clientBy {
		if site.file == f {
			delete(s.res.clientBy, attr)
		}
	}
	delete(s.res.sigTypes, f)
	delete(s.res.quals, f)
	delete(s.res.qualPkgs, f)
	delete(s.res.symbols, f.File)
}

// reset clears the session.
func (s *Session) reset() {
	s.root, s.l, s.dirs, s.res = "", nil, nil, nil
	s.files, s.lastDiags, s.goDiags = nil, nil, nil
	s.stamps, s.sigs, s.imp = nil, nil, nil
	s.seq = 0
}

// buildImporter returns a types.Importer over the loaded packages
// (NFR-05).
func buildImporter(pkgs []*packages.Package) types.Importer {
	byPath := map[string]*types.Package{}
	seen := map[string]bool{}
	var visit func(p *packages.Package)
	visit = func(p *packages.Package) {
		if p == nil || seen[p.PkgPath] {
			return
		}
		seen[p.PkgPath] = true
		if p.Types != nil {
			byPath[p.PkgPath] = p.Types
		}
		for _, imp := range p.Imports {
			visit(imp)
		}
	}
	for _, p := range pkgs {
		visit(p)
	}
	return sessionImporter{byPath: byPath}
}

type sessionImporter struct{ byPath map[string]*types.Package }

func (s sessionImporter) Import(path string) (*types.Package, error) {
	if p, ok := s.byPath[path]; ok {
		return p, nil
	}
	return nil, fmt.Errorf("gx: no cached package %q", path)
}

// refreshContent writes the content files of each collection again. A
// Markdown file is not an input of the analysis, and the generated code
// holds its text, so each pass reads the content directories (REQ-CNT-12,
// NFR-08).
func (s *Session) refreshContent(files map[string][]byte) {
	if files == nil || s.res == nil {
		return
	}
	for path, src := range renderContentBodies(s.res.collections) {
		files[path] = src
	}
}

// sameFileSet reports whether the two snapshots hold the same paths.
func sameFileSet(a, b map[string]fileStamp) bool {
	if len(a) != len(b) {
		return false
	}
	for path := range a {
		if _, ok := b[path]; !ok {
			return false
		}
	}
	return true
}

// goInputsChanged reports whether a non-.gx input changed.
func goInputsChanged(old, now map[string]fileStamp) bool {
	for path, stamp := range old {
		if strings.HasSuffix(path, ".gx") {
			continue
		}
		if other, ok := now[path]; !ok || other != stamp {
			return true
		}
	}
	return false
}

// changedGXFiles returns the .gx files whose content changed.
func changedGXFiles(old, now map[string]fileStamp) map[string]bool {
	out := map[string]bool{}
	for path, stamp := range now {
		if !strings.HasSuffix(path, ".gx") {
			continue
		}
		if other, ok := old[path]; !ok || other != stamp {
			out[path] = true
		}
	}
	return out
}

// fileSignature returns the part of a .gx file that other files depend on:
// the package, the imports, the props, the signals and the fragment
// parameter lists. A body change keeps the signature (NFR-05).
func fileSignature(path string) (string, bool) {
	src, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	return fileSignatureData(path, src)
}

// fileSignatureData is fileSignature over given source bytes.
func fileSignatureData(path string, src []byte) (string, bool) {
	f, diags := ParseFile(path, src)
	if f == nil || len(diags) > 0 {
		return "", false
	}
	var b strings.Builder
	b.WriteString(f.Package)
	b.WriteByte(0)
	for _, im := range f.Imports {
		b.WriteString(im.Raw)
		b.WriteByte(0)
	}
	for _, p := range f.Props {
		fmt.Fprintf(&b, "%s %s = %s", p.Name, p.Type, p.Default)
		b.WriteByte(0)
	}
	for _, s := range f.Signals {
		fmt.Fprintf(&b, "%s %s = %s", s.Name, s.Type, s.Default)
		b.WriteByte(0)
	}
	for _, el := range fragmentElements(f.Body) {
		for i := range el.Attrs {
			if el.Attrs[i].Kind == AttrFragment {
				fmt.Fprintf(&b, "%s(%s)", el.Attrs[i].Name, el.Attrs[i].Value)
				b.WriteByte(0)
			}
		}
	}
	return b.String(), true
}
