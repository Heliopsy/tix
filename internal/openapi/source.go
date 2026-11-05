// SPDX-License-Identifier: AGPL-3.0-or-later

package openapi

import (
	"fmt"
	"go/ast"
	"go/doc"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// sourceDirs are the packages whose doc comments describe the wire contract.
var sourceDirs = []string{"internal/core", "internal/wire", "internal/httpapi"}

// source is what the generator reads out of the Go source rather than out of
// reflection: doc comments, and the constants that close a type's values.
type source struct {
	// methods holds the doc comment of every core.Service method by name.
	methods map[string]string
	// types holds the doc comment of every type, keyed "pkg.Type".
	types map[string]string
	// fields holds the doc comment of every struct field, keyed "pkg.Type"
	// then field name.
	fields map[string]map[string]string
	// consts holds the string constants declared with each type, keyed
	// "pkg.Type", in declaration order.
	consts map[string][]string
	// values holds the doc comment of every string constant, keyed by
	// package then the constant's value.
	values map[string]map[string]string
}

// readSource parses the contract packages under the module root.
func readSource(root string) (*source, error) {
	s := &source{
		methods: map[string]string{},
		types:   map[string]string{},
		fields:  map[string]map[string]string{},
		consts:  map[string][]string{},
		values:  map[string]map[string]string{},
	}
	for _, dir := range sourceDirs {
		if err := s.readDir(filepath.Join(root, dir)); err != nil {
			return nil, err
		}
	}
	return s, nil
}

// readDir reads one package's non-test files.
func (s *source) readDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("reading %q: %w", dir, err)
	}
	fset := token.NewFileSet()
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, parser.ParseComments)
		if err != nil {
			return fmt.Errorf("parsing %q: %w", name, err)
		}
		s.readFile(file.Name.Name, file)
	}
	return nil
}

// readFile collects the declarations of one file.
func (s *source) readFile(pkg string, file *ast.File) {
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range gen.Specs {
			switch spec := spec.(type) {
			case *ast.TypeSpec:
				s.readType(pkg, gen, spec)
			case *ast.ValueSpec:
				s.readConst(pkg, gen, spec)
			}
		}
	}
}

// readType records a type's comment and those of its fields or methods.
func (s *source) readType(pkg string, gen *ast.GenDecl, spec *ast.TypeSpec) {
	key := pkg + "." + spec.Name.Name
	comment := spec.Doc
	if comment == nil && len(gen.Specs) == 1 {
		comment = gen.Doc
	}
	s.types[key] = text(comment)
	switch t := spec.Type.(type) {
	case *ast.StructType:
		fields := map[string]string{}
		for _, f := range t.Fields.List {
			doc := text(f.Doc)
			if doc == "" {
				doc = text(f.Comment)
			}
			for _, n := range f.Names {
				fields[n.Name] = doc
			}
		}
		s.fields[key] = fields
	case *ast.InterfaceType:
		if pkg != "core" {
			return
		}
		for _, m := range t.Methods.List {
			for _, n := range m.Names {
				s.methods[n.Name] = text(m.Doc)
			}
		}
	}
}

// readConst records a string constant's comment by value, and a typed one
// against its type.
func (s *source) readConst(pkg string, gen *ast.GenDecl, spec *ast.ValueSpec) {
	if gen.Tok != token.CONST {
		return
	}
	s.readValueDoc(pkg, gen, spec)
	if spec.Type == nil {
		return
	}
	typ, ok := spec.Type.(*ast.Ident)
	if !ok {
		return
	}
	for _, v := range spec.Values {
		lit, ok := v.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			continue
		}
		if val, err := strconv.Unquote(lit.Value); err == nil {
			key := pkg + "." + typ.Name
			s.consts[key] = append(s.consts[key], val)
		}
	}
}

// readValueDoc records the comment of each string constant in a spec under
// its value.
func (s *source) readValueDoc(pkg string, gen *ast.GenDecl, spec *ast.ValueSpec) {
	comment := spec.Doc
	if comment == nil && len(gen.Specs) == 1 {
		comment = gen.Doc
	}
	d := text(comment)
	if d == "" {
		return
	}
	for _, v := range spec.Values {
		lit, ok := v.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			continue
		}
		if val, err := strconv.Unquote(lit.Value); err == nil {
			if s.values[pkg] == nil {
				s.values[pkg] = map[string]string{}
			}
			if _, taken := s.values[pkg][val]; !taken {
				s.values[pkg][val] = d
			}
		}
	}
}

// text renders a comment group as one paragraph of plain text.
func text(g *ast.CommentGroup) string {
	if g == nil {
		return ""
	}
	var p doc.Package
	d := p.Parser().Parse(g.Text())
	return strings.TrimSpace(strings.Join(strings.Fields(string(p.Printer().Text(d))), " "))
}
