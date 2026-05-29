package codegen

import (
	"fmt"
	"hash/fnv"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/victor141516/besht/internal/ast"
	"github.com/victor141516/besht/internal/checker"
	"github.com/victor141516/besht/internal/parser"
)

type Module struct {
	Path    string
	ModName string
	Prog    *ast.Program
}

type Compiler struct {
	visited     map[string]bool
	modules     []*Module
	root        string
	entryPath   string
	globalSigs  map[string]*checker.FnSig
	globalDecls map[string]bool
	globalVars  map[string]*ast.Type
	opts        Options
}

func NewCompiler(root string, opts Options) *Compiler {
	return &Compiler{
		visited:     make(map[string]bool),
		modules:     nil,
		root:        root,
		globalSigs:  make(map[string]*checker.FnSig),
		globalDecls: make(map[string]bool),
		globalVars:  make(map[string]*ast.Type),
		opts:        opts,
	}
}

func CompileFile(entryPath string, opts Options) (string, error) {
	absEntry, err := filepath.Abs(entryPath)
	if err != nil {
		return "", err
	}
	root := filepath.Dir(absEntry)
	c := NewCompiler(root, opts)
	c.entryPath = absEntry
	if err := c.load(absEntry); err != nil {
		return "", err
	}
	return c.emit()
}

func CompileFileSplit(entryPath, outDir string, opts Options) error {
	absEntry, err := filepath.Abs(entryPath)
	if err != nil {
		return err
	}
	absOut, err := filepath.Abs(outDir)
	if err != nil {
		return err
	}
	root := filepath.Dir(absEntry)
	c := NewCompiler(root, opts)
	c.entryPath = absEntry
	if err := c.load(absEntry); err != nil {
		return err
	}
	return c.emitSplit(absOut)
}

func CheckFile(entryPath string, opts Options) error {
	absEntry, err := filepath.Abs(entryPath)
	if err != nil {
		return err
	}
	root := filepath.Dir(absEntry)
	c := NewCompiler(root, opts)
	c.entryPath = absEntry
	if err := c.load(absEntry); err != nil {
		return err
	}
	for _, mod := range c.modules {
		analysis := AnalyzeProgram(mod.Prog.Statements)
		for _, w := range analysis.Warnings {
			fmt.Fprintf(os.Stderr, "besht: warning: %s: %s\n", w.Pos, w.Message)
		}
		for _, e := range analysis.Errors {
			return fmt.Errorf("%s: %s", e.Pos, e.Message)
		}
	}
	return nil
}

func (c *Compiler) load(absPath string) error {
	if err := c.ensureModuleWithinRoot(absPath); err != nil {
		return err
	}
	if c.visited[absPath] {
		return nil
	}
	c.visited[absPath] = true

	if absPath == c.entryPath {
		stdlibPath := filepath.Join(filepath.Dir(absPath), "stdlib.d.bsh")
		if stdlibPath != absPath {
			if _, err := os.Stat(stdlibPath); err == nil {
				if err := c.load(stdlibPath); err != nil {
					return err
				}
			}
		}
	}

	src, err := os.ReadFile(absPath)
	if err != nil {
		return fmt.Errorf("cannot read %s: %w", absPath, err)
	}

	prog, err := parser.Parse(string(src), absPath)
	if err != nil {
		return err
	}

	for _, imp := range prog.Imports {
		if err := c.validateImport(imp, filepath.Dir(absPath)); err != nil {
			return err
		}
		if isShellImport(imp) {
			continue
		}
		impPath := c.resolveImportPath(imp.Source, filepath.Dir(absPath))
		if err := c.load(impPath); err != nil {
			return err
		}
	}

	modName := pathToModName(absPath, c.root)

	chk := checker.NewWithOptions(checker.Options{Strict: c.opts.Strict})
	for qualName, sig := range c.globalSigs {
		chk.RegisterFn(qualName, sig)
	}
	for _, imp := range prog.Imports {
		if isShellImport(imp) {
			for _, name := range imp.Names {
				chk.RegisterUncheckedFunction(name)
			}
			continue
		}
		impPath := c.resolveImportPath(imp.Source, filepath.Dir(absPath))
		impModName := pathToModName(impPath, c.root)
		prefix := modNameToPrefix(impModName)
		if imp.DefaultName != "" {
			qualName := prefix + "__default"
			if typ, ok := c.globalVars[qualName]; ok {
				chk.RegisterVar(imp.DefaultName, typ)
			}
		}
		for _, name := range imp.Names {
			qualName := prefix + "__" + name
			if sig, ok := c.globalSigs[qualName]; ok {
				chk.RegisterFn(name, sig)
				chk.RegisterFn(qualName, sig)
			} else if sig, ok := c.globalSigs[name]; ok {
				chk.RegisterFn(name, sig)
			} else if typ, ok := c.globalVars[qualName]; ok {
				chk.RegisterVar(name, typ)
			}
		}
	}

	if err := chk.Check(prog); err != nil {
		return err
	}

	isDeclFile := strings.HasSuffix(absPath, ".d.bsh")

	for _, stmt := range prog.Statements {
		switch fn := stmt.(type) {
		case *ast.FnDecl:
			if fn.Exported {
				retType := fn.ReturnType
				if retType == nil {
					retType = &ast.Type{Kind: ast.TypeVoid}
				}
				qualName := modNameToPrefix(modName) + "__" + fn.Name
				sig := &checker.FnSig{
					Params:     fn.Params,
					ReturnType: retType,
				}
				c.globalSigs[qualName] = sig
				c.globalSigs[fn.Name] = sig
			}
		case *ast.DeclareFnStmt:
			retType := fn.ReturnType
			if retType == nil {
				retType = &ast.Type{Kind: ast.TypeVoid}
			}
			sig := &checker.FnSig{Params: fn.Params, ReturnType: retType}
			c.globalSigs[fn.Name] = sig
			c.globalDecls[fn.Name] = true
		case *ast.LetDecl:
			if fn.Exported {
				name := fn.Name
				if fn.DefaultExport {
					name = "default"
				}
				typ := fn.ResolvedTy
				if typ == nil {
					typ = fn.TypeAnnot
				}
				if typ == nil {
					typ = inferExportValueType(fn.Value)
				}
				c.globalVars[modNameToPrefix(modName)+"__"+name] = typ
			}
		}
	}

	if !isDeclFile {
		if err := c.ensureModuleWithinRoot(absPath); err != nil {
			return err
		}
		c.modules = append(c.modules, &Module{
			Path:    absPath,
			ModName: modName,
			Prog:    prog,
		})
	}
	return nil
}

func (c *Compiler) emit() (string, error) {
	var body strings.Builder
	neededHelpers := make(map[string]bool)

	importedFnMap, importedVarMap, importedVarTypeMap := c.buildImportedMaps()
	argsOptions, argsFlags := c.collectArgsSchema()

	for _, mod := range c.modules {
		if len(mod.Prog.Statements) == 0 && len(mod.Prog.Imports) == 0 {
			continue
		}
		body.WriteString(fmt.Sprintf("# --- module: %s ---\n", mod.ModName))
		body.WriteString(c.shellImportSources(mod))

		importMap := importedFnMap[mod.Path]
		importVarMap := importedVarMap[mod.Path]
		importVarTypes := importedVarTypeMap[mod.Path]
		analysis := AnalyzeProgram(mod.Prog.Statements)
		for _, w := range analysis.Warnings {
			fmt.Fprintf(os.Stderr, "besht: warning: %s: %s\n", w.Pos, w.Message)
		}
		for _, e := range analysis.Errors {
			return "", fmt.Errorf("%s: %s", e.Pos, e.Message)
		}
		g := newModuleGenerator(mod.ModName, importMap, importVarMap, importVarTypes)
		g.NoCheck = c.opts.NoCheck
		g.NoSourceMap = c.opts.NoSourceMap
		g.cmdAnalysis = analysis
		g.argsOptions = cloneBoolMap(argsOptions)
		g.argsFlags = cloneBoolMap(argsFlags)
		out, err := g.generateModule(mod.Prog)
		if err != nil {
			return "", fmt.Errorf("in module %s: %w", mod.ModName, err)
		}
		for name := range g.runtimeHelpers {
			neededHelpers[name] = true
		}
		body.WriteString(out)
		body.WriteString("\n")
	}

	var sb strings.Builder
	sb.WriteString("#!/bin/sh\n")
	sb.WriteString("# Generated by besht — do not edit by hand\n")
	sb.WriteString("\n")
	sb.WriteString(runtimeHelpersSource(neededHelpers))
	if neededHelpers["args"] {
		sb.WriteString(beshtRuntimeArgsSnapshot)
	}
	if !c.opts.NoCheck {
		sb.WriteString(beshtCheckBlock)
	}
	sb.WriteString("\n")
	sb.WriteString(body.String())
	return sb.String(), nil
}

func (c *Compiler) emitSplit(outDir string) error {
	compiledOutputs := make(map[string]string)
	for _, mod := range c.modules {
		if err := c.ensureModuleWithinRoot(mod.Path); err != nil {
			return err
		}
		outPath := filepath.Join(outDir, mod.ModName+".sh")
		if owner, ok := compiledOutputs[outPath]; ok && owner != mod.Path {
			return fmt.Errorf("split output collision: %s from both %s and %s", outPath, owner, mod.Path)
		}
		compiledOutputs[outPath] = mod.Path
	}

	shellCopies, err := c.collectSplitShellCopies(outDir, compiledOutputs)
	if err != nil {
		return err
	}

	importedFnMap, importedVarMap, importedVarTypeMap := c.buildImportedMaps()
	argsOptions, argsFlags := c.collectArgsSchema()
	splitNeedsArgs := false
	for _, mod := range c.modules {
		if statementsUseArgs(mod.Prog.Statements) {
			splitNeedsArgs = true
			break
		}
	}

	for _, mod := range c.modules {
		isEntry := mod.Path == c.entryPath

		outPath := filepath.Join(outDir, mod.ModName+".sh")
		if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
			return fmt.Errorf("cannot create output dir for %s: %w", mod.ModName, err)
		}

		importMap := importedFnMap[mod.Path]
		importVarMap := importedVarMap[mod.Path]
		importVarTypes := importedVarTypeMap[mod.Path]
		analysis := AnalyzeProgram(mod.Prog.Statements)
		for _, w := range analysis.Warnings {
			fmt.Fprintf(os.Stderr, "besht: warning: %s: %s\n", w.Pos, w.Message)
		}
		for _, e := range analysis.Errors {
			return fmt.Errorf("%s: %s", e.Pos, e.Message)
		}
		g := newModuleGenerator(mod.ModName, importMap, importVarMap, importVarTypes)
		g.NoCheck = c.opts.NoCheck
		g.NoSourceMap = c.opts.NoSourceMap
		g.cmdAnalysis = analysis
		g.argsOptions = cloneBoolMap(argsOptions)
		g.argsFlags = cloneBoolMap(argsFlags)
		body, err := g.generateModule(mod.Prog)
		if err != nil {
			return fmt.Errorf("in module %s: %w", mod.ModName, err)
		}

		var sb strings.Builder

		if isEntry {
			sb.WriteString("#!/bin/sh\n")
			sb.WriteString("# Generated by besht — do not edit by hand\n")
			sb.WriteString("\n")
			entryHelpers := g.runtimeHelpers
			if splitNeedsArgs {
				entryHelpers = make(map[string]bool, len(g.runtimeHelpers)+1)
				for name, needed := range g.runtimeHelpers {
					entryHelpers[name] = needed
				}
				entryHelpers["args"] = true
			}
			sb.WriteString(runtimeHelpersSource(entryHelpers))
			if splitNeedsArgs {
				sb.WriteString(beshtRuntimeArgsSnapshot)
			}
			if !c.opts.NoCheck {
				sb.WriteString(beshtCheckBlock)
			}
			sb.WriteString("\n")
			sb.WriteString(`_BESHT_ROOT="$(cd "$(dirname "$0")" && pwd)"`)
			sb.WriteString("\n")
		} else {
			sb.WriteString("# Generated by besht — do not edit by hand\n")
			guard := includeGuardVar(mod.ModName)
			sb.WriteString(fmt.Sprintf("[ -n \"$%s\" ] && return 0\n", guard))
			sb.WriteString(fmt.Sprintf("%s=1\n", guard))
			sb.WriteString(runtimeHelpersSource(g.runtimeHelpers))
		}

		for _, imp := range mod.Prog.Imports {
			if isShellImport(imp) {
				source, err := c.splitShellImportSource(imp, mod.Path)
				if err != nil {
					return err
				}
				sb.WriteString(source)
				continue
			}
			impPath := c.resolveImportPath(imp.Source, filepath.Dir(mod.Path))
			if strings.HasSuffix(impPath, ".d.bsh") {
				continue
			}
			impModName := pathToModName(impPath, c.root)
			sb.WriteString(sourceFromBeshtRoot(impModName + ".sh"))
		}

		if len(mod.Prog.Imports) > 0 || isEntry {
			sb.WriteString("\n")
		}

		sb.WriteString(body)

		if err := os.WriteFile(outPath, []byte(sb.String()), 0755); err != nil {
			return fmt.Errorf("cannot write %s: %w", outPath, err)
		}
	}

	for srcPath, outPath := range shellCopies {
		if err := copyFile(srcPath, outPath, outDir); err != nil {
			return err
		}
	}
	return nil
}

func (c *Compiler) collectArgsSchema() (map[string]bool, map[string]bool) {
	g := New()
	for _, mod := range c.modules {
		g.collectArgsSchema(mod.Prog.Statements)
	}
	return g.argsOptions, g.argsFlags
}

func cloneBoolMap(src map[string]bool) map[string]bool {
	dst := make(map[string]bool, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func (c *Compiler) collectSplitShellCopies(outDir string, compiledOutputs map[string]string) (map[string]string, error) {
	shellCopies := make(map[string]string)
	shellOutputs := make(map[string]string)
	for _, mod := range c.modules {
		for _, imp := range mod.Prog.Imports {
			if !isShellImport(imp) {
				continue
			}
			absPath, err := c.resolveShellImportPath(imp, filepath.Dir(mod.Path))
			if err != nil {
				return nil, err
			}
			if c.opts.AllowExternalShellImports && !c.modulePathWithinRoot(absPath) {
				continue
			}
			relPath, err := filepath.Rel(c.root, absPath)
			if err != nil {
				return nil, err
			}
			outPath := filepath.Join(outDir, relPath)
			if owner, ok := compiledOutputs[outPath]; ok {
				return nil, fmt.Errorf("split output collision: raw shell import %s would overwrite compiled module %s at %s", absPath, owner, outPath)
			}
			if owner, ok := shellOutputs[outPath]; ok && owner != absPath {
				return nil, fmt.Errorf("split output collision: raw shell imports %s and %s both write %s", owner, absPath, outPath)
			}
			shellOutputs[outPath] = absPath
			shellCopies[absPath] = outPath
		}
	}
	return shellCopies, nil
}

func (c *Compiler) splitShellImportSource(imp *ast.ImportDecl, modPath string) (string, error) {
	absPath, err := c.resolveShellImportPath(imp, filepath.Dir(modPath))
	if err != nil {
		return "", err
	}
	if c.opts.AllowExternalShellImports && !c.modulePathWithinRoot(absPath) {
		guard := c.shellImportGuardVar(absPath)
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("if [ -z \"$%s\" ]; then\n", guard))
		sb.WriteString(fmt.Sprintf("    %s=1\n", guard))
		sb.WriteString(fmt.Sprintf("    . %s\n", shellQuote(absPath)))
		sb.WriteString("fi\n")
		return sb.String(), nil
	}
	relPath, err := filepath.Rel(c.root, absPath)
	if err != nil {
		return "", err
	}
	relPath = filepath.ToSlash(relPath)
	guard := c.shellImportGuardVar(absPath)
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("if [ -z \"$%s\" ]; then\n", guard))
	sb.WriteString(fmt.Sprintf("    %s=1\n", guard))
	sb.WriteString("    ")
	sb.WriteString(sourceFromBeshtRoot(relPath))
	sb.WriteString("fi\n")
	return sb.String(), nil
}

func copyFile(srcPath, outPath, outRoot string) error {
	info, err := os.Stat(srcPath)
	if err != nil {
		return fmt.Errorf("cannot stat %s: %w", srcPath, err)
	}
	if err := ensureOutputParentSafe(outPath, outRoot); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		return fmt.Errorf("cannot create output dir for %s: %w", outPath, err)
	}
	if err := ensureOutputDirWithinRoot(filepath.Dir(outPath), outRoot); err != nil {
		return err
	}
	if outLstat, err := os.Lstat(outPath); err == nil {
		if outLstat.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing to overwrite symlink %s", outPath)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("cannot lstat %s: %w", outPath, err)
	}
	if outInfo, err := os.Stat(outPath); err == nil {
		if os.SameFile(info, outInfo) {
			return nil
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("cannot stat %s: %w", outPath, err)
	}
	in, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("cannot read %s: %w", srcPath, err)
	}
	defer in.Close()
	out, err := os.OpenFile(outPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return fmt.Errorf("cannot write %s: %w", outPath, err)
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return fmt.Errorf("cannot copy %s to %s: %w", srcPath, outPath, err)
	}
	if err := out.Close(); err != nil {
		return fmt.Errorf("cannot write %s: %w", outPath, err)
	}
	return nil
}

func ensureOutputParentSafe(outPath, outRoot string) error {
	outDir := filepath.Dir(outPath)
	rel, err := filepath.Rel(outRoot, outDir)
	if err != nil {
		return fmt.Errorf("output %s is outside output root %s: %w", outDir, outRoot, err)
	}
	if filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("output %s is outside output root %s", outDir, outRoot)
	}
	current := outRoot
	if rel == "." {
		return ensureOutputDirWithinRoot(current, outRoot)
	}
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("cannot lstat %s: %w", current, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing to write through symlinked output dir %s", current)
		}
		if !info.IsDir() {
			return fmt.Errorf("output parent %s is not a directory", current)
		}
		if err := ensureOutputDirWithinRoot(current, outRoot); err != nil {
			return err
		}
	}
	return nil
}

func ensureOutputDirWithinRoot(outDir, outRoot string) error {
	realOutRoot, err := filepath.EvalSymlinks(outRoot)
	if err != nil {
		realOutRoot = outRoot
	}
	realOutDir, err := filepath.EvalSymlinks(outDir)
	if err != nil {
		return fmt.Errorf("cannot resolve output dir %s: %w", outDir, err)
	}
	return ensurePathWithinRoot(realOutDir, realOutRoot)
}

func (c *Compiler) ensureModuleWithinRoot(absPath string) error {
	rel, err := filepath.Rel(c.root, absPath)
	if err != nil {
		return fmt.Errorf("import %s is outside compiler root %s: %w", absPath, c.root, err)
	}
	if filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("import %s is outside compiler root %s", absPath, c.root)
	}
	return nil
}

func (c *Compiler) modulePathWithinRoot(absPath string) bool {
	return c.ensureModuleWithinRoot(absPath) == nil
}

func includeGuardVar(modName string) string {
	return "_BESHT_LOADED_" + modNameToPrefix(modName)
}

func (c *Compiler) shellImportGuardVar(absPath string) string {
	rel, err := filepath.Rel(c.root, absPath)
	if err != nil {
		rel = absPath
	}
	return "_BESHT_SHELL_LOADED_" + safeShellIdentSuffix(filepath.ToSlash(rel))
}

func sourceFromBeshtRoot(relPath string) string {
	return ". \"$_BESHT_ROOT\"/" + shellQuote(filepath.ToSlash(relPath)) + "\n"
}

func isShellImport(imp *ast.ImportDecl) bool {
	return imp.AssertType == "shell"
}

func (c *Compiler) validateImport(imp *ast.ImportDecl, fromDir string) error {
	if isShellImport(imp) {
		_, err := c.resolveShellImportPath(imp, fromDir)
		return err
	}
	if strings.HasSuffix(imp.Source, ".sh") {
		return fmt.Errorf("%s: .sh imports require assert { type: \"shell\" }", imp.Pos)
	}
	return nil
}

func (c *Compiler) resolveShellImportPath(imp *ast.ImportDecl, fromDir string) (string, error) {
	if imp.DefaultName != "" {
		return "", fmt.Errorf("%s: shell imports do not support default imports", imp.Pos)
	}
	if len(imp.Names) == 0 {
		return "", fmt.Errorf("%s: shell imports require named imports", imp.Pos)
	}
	if !strings.HasSuffix(imp.Source, ".sh") {
		return "", fmt.Errorf("%s: shell import assertion requires an explicit .sh source", imp.Pos)
	}
	absPath, err := filepath.Abs(filepath.Join(fromDir, imp.Source))
	if err != nil {
		return "", err
	}
	if !c.opts.AllowExternalShellImports {
		if err := c.ensureModuleWithinRoot(absPath); err != nil {
			return "", err
		}
	}
	info, err := os.Stat(absPath)
	if err != nil {
		return "", fmt.Errorf("cannot read shell import %s: %w", absPath, err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("shell import %s is a directory", absPath)
	}
	realPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		return "", fmt.Errorf("cannot resolve shell import %s: %w", absPath, err)
	}
	if c.opts.AllowExternalShellImports {
		return absPath, nil
	}
	realRoot, err := filepath.EvalSymlinks(c.root)
	if err != nil {
		realRoot = c.root
	}
	if err := ensurePathWithinRoot(realPath, realRoot); err != nil {
		return "", err
	}
	return absPath, nil
}

func ensurePathWithinRoot(absPath, root string) error {
	rel, err := filepath.Rel(root, absPath)
	if err != nil {
		return fmt.Errorf("import %s is outside compiler root %s: %w", absPath, root, err)
	}
	if filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("import %s is outside compiler root %s", absPath, root)
	}
	return nil
}

func (c *Compiler) shellImportSources(mod *Module) string {
	var sb strings.Builder
	for _, imp := range mod.Prog.Imports {
		if !isShellImport(imp) {
			continue
		}
		absPath, err := c.resolveShellImportPath(imp, filepath.Dir(mod.Path))
		if err != nil {
			continue
		}
		guard := c.shellImportGuardVar(absPath)
		sb.WriteString(fmt.Sprintf("if [ -z \"$%s\" ]; then\n", guard))
		sb.WriteString(fmt.Sprintf("    %s=1\n", guard))
		sb.WriteString(fmt.Sprintf("    . %s\n", shellQuote(absPath)))
		sb.WriteString("fi\n")
	}
	if sb.Len() > 0 {
		sb.WriteString("\n")
	}
	return sb.String()
}

func inferExportValueType(expr ast.Expression) *ast.Type {
	switch e := expr.(type) {
	case *ast.ListLit:
		if t := e.GetType(); t != nil {
			return t
		}
		return &ast.Type{Kind: ast.TypeList, Elem: &ast.Type{Kind: ast.TypeString}}
	case *ast.StringLit, *ast.RawStringLit, *ast.TemplateLit:
		return &ast.Type{Kind: ast.TypeString}
	case *ast.IntLit, *ast.FloatLit:
		return &ast.Type{Kind: ast.TypeNumber}
	case *ast.BoolLit:
		return &ast.Type{Kind: ast.TypeBoolean}
	case *ast.ObjectLit:
		return &ast.Type{Kind: ast.TypeObject}
	}
	if t := expr.GetType(); t != nil {
		return t
	}
	return &ast.Type{Kind: ast.TypeString}
}

func (c *Compiler) buildImportedMaps() (map[string]map[string]string, map[string]map[string]string, map[string]map[string]*ast.Type) {
	fnResult := make(map[string]map[string]string)
	varResult := make(map[string]map[string]string)
	varTypeResult := make(map[string]map[string]*ast.Type)

	exportedFns := make(map[string]map[string]string)
	exportedVars := make(map[string]map[string]string)
	localFns := make(map[string]map[string]bool)
	for _, mod := range c.modules {
		prefix := modNameToPrefix(mod.ModName)
		fns := make(map[string]string)
		vars := make(map[string]string)
		locals := make(map[string]bool)
		for _, stmt := range mod.Prog.Statements {
			switch fn := stmt.(type) {
			case *ast.FnDecl:
				locals[fn.Name] = true
				if fn.Exported {
					fns[fn.Name] = prefix + "__" + fn.Name
				}
			case *ast.ClassDecl:
				locals[fn.Name] = true
				if fn.Exported {
					fns[fn.Name] = prefix + "__" + fn.Name
				}
			case *ast.LetDecl:
				if fn.Exported {
					name := fn.Name
					if fn.DefaultExport {
						name = "default"
					}
					vars[name] = prefix + "__" + name
				}
			}
		}
		exportedFns[mod.Path] = fns
		exportedVars[mod.Path] = vars
		localFns[mod.Path] = locals
	}

	for _, mod := range c.modules {
		importMap := make(map[string]string)
		importVarMap := make(map[string]string)
		importVarTypes := make(map[string]*ast.Type)
		for name := range c.globalDecls {
			if localFns[mod.Path][name] {
				continue
			}
			importMap[name] = name
		}
		for _, imp := range mod.Prog.Imports {
			if isShellImport(imp) {
				for _, name := range imp.Names {
					importMap[name] = name
				}
				continue
			}
			impPath := c.resolveImportPath(imp.Source, filepath.Dir(mod.Path))
			if strings.HasSuffix(impPath, ".d.bsh") {
				for _, name := range imp.Names {
					importMap[name] = name
				}
				continue
			}
			if imp.DefaultName != "" {
				if vars, ok := exportedVars[impPath]; ok {
					if qualName, ok := vars["default"]; ok {
						importVarMap[imp.DefaultName] = qualName
						if typ, ok := c.globalVars[qualName]; ok {
							importVarTypes[imp.DefaultName] = typ
						}
					}
				}
			}
			if fns, ok := exportedFns[impPath]; ok {
				for _, name := range imp.Names {
					if qualName, ok := fns[name]; ok {
						importMap[name] = qualName
					}
				}
			}
			if vars, ok := exportedVars[impPath]; ok {
				for _, name := range imp.Names {
					if qualName, ok := vars[name]; ok {
						importVarMap[name] = qualName
						if typ, ok := c.globalVars[qualName]; ok {
							importVarTypes[name] = typ
						}
					}
				}
			}
		}
		fnResult[mod.Path] = importMap
		varResult[mod.Path] = importVarMap
		varTypeResult[mod.Path] = importVarTypes
	}
	return fnResult, varResult, varTypeResult
}

func rewriteFnCalls(stmts []ast.Statement, importMap map[string]string, qualify func(string) string) {
	for _, s := range stmts {
		rewriteStmt(s, importMap, qualify)
	}
}

func rewriteStmt(stmt ast.Statement, importMap map[string]string, qualify func(string) string) {
	switch s := stmt.(type) {
	case *ast.FnDecl:
		rewriteFnCalls(s.Body.Statements, importMap, qualify)
	case *ast.ClassDecl:
		if qualName, ok := importMap[s.Name]; ok {
			s.Name = qualName
		} else if !strings.Contains(s.Name, "__") {
			s.Name = qualify(s.Name)
		}
		if s.Constructor != nil {
			rewriteFnCalls(s.Constructor.Body.Statements, importMap, qualify)
		}
		for _, prop := range s.StaticProps {
			if prop.Value != nil {
				rewriteExpr(prop.Value, importMap, qualify)
			}
		}
		for _, method := range s.Methods {
			rewriteFnCalls(method.Body.Statements, importMap, qualify)
		}
		for _, accessor := range s.Accessors {
			rewriteFnCalls(accessor.Body.Statements, importMap, qualify)
		}
	case *ast.Block:
		rewriteFnCalls(s.Statements, importMap, qualify)
	case *ast.IfStmt:
		rewriteExpr(s.Condition, importMap, qualify)
		rewriteFnCalls(s.Then.Statements, importMap, qualify)
		for _, ei := range s.ElseIfs {
			rewriteExpr(ei.Condition, importMap, qualify)
			rewriteFnCalls(ei.Body.Statements, importMap, qualify)
		}
		if s.Else != nil {
			rewriteFnCalls(s.Else.Statements, importMap, qualify)
		}
	case *ast.ForStmt:
		rewriteExpr(s.Iterator, importMap, qualify)
		rewriteFnCalls(s.Body.Statements, importMap, qualify)
	case *ast.CStyleForStmt:
		rewriteStmt(s.Init, importMap, qualify)
		rewriteExpr(s.Condition, importMap, qualify)
		rewriteStmt(s.Update, importMap, qualify)
		rewriteFnCalls(s.Body.Statements, importMap, qualify)
	case *ast.WhileStmt:
		rewriteExpr(s.Condition, importMap, qualify)
		rewriteFnCalls(s.Body.Statements, importMap, qualify)
	case *ast.TryStmt:
		rewriteFnCalls(s.Body.Statements, importMap, qualify)
		rewriteFnCalls(s.Catch.Statements, importMap, qualify)
	case *ast.LetDecl:
		rewriteExpr(s.Value, importMap, qualify)
	case *ast.DestructureDecl:
		rewriteExpr(s.Value, importMap, qualify)
	case *ast.Assignment:
		rewriteExpr(s.Value, importMap, qualify)
	case *ast.IndexAssignStmt:
		rewriteExpr(s.Index, importMap, qualify)
		rewriteExpr(s.Value, importMap, qualify)
	case *ast.PropertyAssignStmt:
		rewriteExpr(s.Value, importMap, qualify)
	case *ast.SwitchStmt:
		rewriteExpr(s.Value, importMap, qualify)
		for _, swCase := range s.Cases {
			if !swCase.IsDefault {
				rewriteExpr(swCase.Value, importMap, qualify)
			}
			rewriteFnCalls(swCase.Body.Statements, importMap, qualify)
		}
	case *ast.ReturnStmt:
		if s.Value != nil {
			rewriteExpr(s.Value, importMap, qualify)
		}
	case *ast.ExprStmt:
		rewriteExpr(s.Expr, importMap, qualify)
	case *ast.DeclareStmt:
	case *ast.DeclareFnStmt:
	}
}

func rewriteExpr(expr ast.Expression, importMap map[string]string, qualify func(string) string) {
	switch e := expr.(type) {
	case *ast.FnCallExpr:
		if qualName, ok := importMap[e.Name]; ok {
			e.Name = qualName
		} else if _, ok := importMap[e.Name]; !ok {
			qual := qualify(e.Name)
			if qual != e.Name {
				e.Name = qual
			}
		}
		for _, arg := range e.Args {
			rewriteExpr(arg, importMap, qualify)
		}
	case *ast.NewExpr:
		if e.ClassName != "Set" {
			if qualName, ok := importMap[e.ClassName]; ok {
				e.ClassName = qualName
			} else if !strings.Contains(e.ClassName, "__") {
				e.ClassName = qualify(e.ClassName)
			}
		}
		for _, arg := range e.Args {
			rewriteExpr(arg, importMap, qualify)
		}
	case *ast.BinaryExpr:
		rewriteExpr(e.Left, importMap, qualify)
		rewriteExpr(e.Right, importMap, qualify)
	case *ast.TernaryExpr:
		rewriteExpr(e.Condition, importMap, qualify)
		rewriteExpr(e.Then, importMap, qualify)
		rewriteExpr(e.Else, importMap, qualify)
	case *ast.UnaryExpr:
		rewriteExpr(e.Expr, importMap, qualify)
	case *ast.UpdateExpr:
	case *ast.CmdExpr:
		for _, arg := range e.Args {
			rewriteExpr(arg, importMap, qualify)
		}
	case *ast.PropagateExpr:
		rewriteExpr(e.Expr, importMap, qualify)
	case *ast.IndexExpr:
		rewriteExpr(e.Expr, importMap, qualify)
		rewriteExpr(e.Index, importMap, qualify)
	case *ast.ListLit:
		for _, elem := range e.Elements {
			rewriteExpr(elem, importMap, qualify)
		}
	case *ast.ObjectLit:
		for _, field := range e.Fields {
			rewriteExpr(field.Value, importMap, qualify)
		}
	case *ast.TemplateLit:
		for _, expr := range e.Exprs {
			rewriteExpr(expr, importMap, qualify)
		}
	case *ast.BuiltinCallExpr:
		for _, arg := range e.Args {
			rewriteExpr(arg, importMap, qualify)
		}
	case *ast.MethodCallExpr:
		if ident, ok := e.Receiver.(*ast.IdentExpr); ok {
			qualifyClassIdent(ident, importMap, qualify)
		}
		rewriteExpr(e.Receiver, importMap, qualify)
		for _, arg := range e.Args {
			rewriteExpr(arg, importMap, qualify)
		}
	case *ast.PropertyExpr:
		if ident, ok := e.Receiver.(*ast.IdentExpr); ok {
			qualifyClassIdent(ident, importMap, qualify)
		}
		rewriteExpr(e.Receiver, importMap, qualify)
	case *ast.ArrowExpr:
		rewriteExpr(e.Body, importMap, qualify)
		if e.BlockBody != nil {
			for _, stmt := range e.BlockBody.Statements {
				rewriteStmt(stmt, importMap, qualify)
			}
		}
	case *ast.SpreadExpr:
		rewriteExpr(e.Expr, importMap, qualify)
	case *ast.AsExpr:
		rewriteExpr(e.Expr, importMap, qualify)
	}
}

func qualifyClassIdent(ident *ast.IdentExpr, importMap map[string]string, qualify func(string) string) {
	if qualName, ok := importMap[ident.Name]; ok {
		ident.Name = qualName
		return
	}
	if ident.Name == "" || ident.Name == "Besht" || ident.Name == "Math" || ident.Name == "Number" || ident.Name == "String" || ident.Name == "Set" || ident.Name == "console" || strings.Contains(ident.Name, "__") || ident.Name[0] < 'A' || ident.Name[0] > 'Z' {
		return
	}
	ident.Name = qualify(ident.Name)
}

func pathToModName(absPath, root string) string {
	rel, err := filepath.Rel(root, absPath)
	if err != nil {
		rel = absPath
	}
	rel = strings.TrimSuffix(rel, ".d.bsh")
	rel = strings.TrimSuffix(rel, ".bsh")
	rel = strings.TrimSuffix(rel, ".ts")
	rel = strings.ReplaceAll(rel, string(filepath.Separator), "/")
	return rel
}

func modNameToPrefix(modName string) string {
	s := strings.ReplaceAll(modName, "/", "__")
	s = strings.ReplaceAll(s, "-", "_")
	s = strings.ReplaceAll(s, ".", "_")
	return safeShellIdentSuffix(s)
}

func safeShellIdentSuffix(s string) string {
	var b strings.Builder
	changed := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' {
			b.WriteByte(c)
			continue
		}
		b.WriteByte('_')
		changed = true
	}
	out := b.String()
	if out == "" {
		out = "m"
		changed = true
	}
	if out[0] >= '0' && out[0] <= '9' {
		out = "m_" + out
		changed = true
	}
	if !changed {
		return out
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(s))
	return fmt.Sprintf("%s_%08x", out, h.Sum32())
}

func resolveImportPath(source, fromDir string) string {
	return resolveImportPathWithOptions(source, fromDir, Options{})
}

func (c *Compiler) resolveImportPath(source, fromDir string) string {
	return resolveImportPathWithOptions(source, fromDir, c.opts)
}

func resolveImportPathWithOptions(source, fromDir string, opts Options) string {
	if !strings.HasPrefix(source, ".") {
		source = "./" + source
	}
	base := filepath.Join(fromDir, source)
	if strings.HasSuffix(base, ".bsh") || strings.HasSuffix(base, ".d.bsh") {
		return base
	}
	if strings.HasSuffix(base, ".ts") {
		if opts.ResolveTsImports {
			return base
		}
		return strings.TrimSuffix(base, ".ts") + ".bsh"
	}
	if strings.HasSuffix(base, ".d") {
		return base + ".bsh"
	}
	bshPath := base + ".bsh"
	if opts.ResolveTsImports {
		if _, err := os.Stat(bshPath); os.IsNotExist(err) {
			tsPath := base + ".ts"
			if _, tsErr := os.Stat(tsPath); tsErr == nil {
				return tsPath
			}
		}
	}
	return bshPath
}

type moduleGenerator struct {
	Generator
	modName             string
	importMap           map[string]string
	importVarMap        map[string]string
	importVarTypeMap    map[string]*ast.Type
	exportedVarAliasMap map[string]string
}

func newModuleGenerator(modName string, importMap, importVarMap map[string]string, importVarTypeMap map[string]*ast.Type) *moduleGenerator {
	g := &moduleGenerator{
		modName:             modName,
		importMap:           importMap,
		importVarMap:        importVarMap,
		importVarTypeMap:    importVarTypeMap,
		exportedVarAliasMap: make(map[string]string),
	}
	g.fnReturnMap = make(map[string]*ast.Type)
	g.varTypeMap = make(map[string]*ast.Type)
	g.floatVars = make(map[string]bool)
	g.paramMap = make(map[string]string)
	g.objAliasMap = make(map[string]string)
	g.objFieldsMap = make(map[string][]string)
	g.objPropTypeMap = make(map[string]*ast.Type)
	g.fnParamTypes = make(map[string]*ast.Type)
	g.fnParamNames = make(map[string][]string)
	g.classMap = make(map[string]*ast.ClassDecl)
	g.varClassMap = make(map[string]string)
	g.listLenMap = make(map[string]string)
	g.runtimeHelpers = make(map[string]bool)
	g.argsOptions = make(map[string]bool)
	g.argsFlags = make(map[string]bool)
	g.cmdScope = newCmdScope(nil)
	g.topLevel = true
	return g
}

func (g *moduleGenerator) generateModule(prog *ast.Program) (string, error) {
	for _, stmt := range prog.Statements {
		switch fn := stmt.(type) {
		case *ast.FnDecl:
			retType := fn.ReturnType
			if retType == nil {
				retType = &ast.Type{Kind: ast.TypeVoid}
			}
			qualName := g.qualifyFnName(fn.Name)
			g.fnReturnMap[qualName] = retType
			g.fnReturnMap[fn.Name] = retType
			var pnames []string
			for _, p := range fn.Params {
				pnames = append(pnames, p.Name)
			}
			g.fnParamNames[fn.Name] = pnames
			g.fnParamNames[qualName] = pnames
		case *ast.ClassDecl:
			originalName := fn.Name
			qualName := g.qualifyFnName(fn.Name)
			fn.Name = qualName
			g.registerClass(fn)
			g.classMap[originalName] = fn
		case *ast.LetDecl:
			if fn.Exported {
				name := fn.Name
				if fn.DefaultExport {
					name = "default"
				}
				g.exportedVarAliasMap[name] = g.qualifyFnName(name)
			}
		}
	}

	for importedName, qualName := range g.importMap {
		g.fnReturnMap[importedName] = g.fnReturnMap[qualName]
	}
	for importedName, qualName := range g.importVarMap {
		g.paramMap[importedName] = qualName
		if typ, ok := g.importVarTypeMap[importedName]; ok {
			g.varTypeMap[importedName] = typ
			g.varTypeMap[qualName] = typ
		}
	}
	for name, qualName := range g.exportedVarAliasMap {
		g.paramMap[name] = qualName
	}

	g.collectObjectTypes(prog.Statements)
	g.collectArgsSchema(prog.Statements)

	rewriteFnCalls(prog.Statements, g.importMap, g.qualifyFnName)

	for _, stmt := range prog.Statements {
		if err := g.genModuleStmt(stmt); err != nil {
			return "", err
		}
	}
	return g.sb.String(), nil
}

func (g *moduleGenerator) qualifyFnName(name string) string {
	if g.modName == "" || g.modName == "." {
		return name
	}
	prefix := modNameToPrefix(g.modName)
	return fmt.Sprintf("%s__%s", prefix, name)
}

func (g *moduleGenerator) resolveFnName(name string) string {
	if qualName, ok := g.importMap[name]; ok {
		return qualName
	}
	if _, ok := g.fnReturnMap[name]; ok {
		prefix := modNameToPrefix(g.modName) + "__"
		if strings.HasPrefix(name, prefix) {
			return name
		}
		return g.qualifyFnName(name)
	}
	return name
}

func (g *moduleGenerator) genModuleStmt(stmt ast.Statement) error {
	if _, ok := stmt.(*ast.ClassDecl); !ok {
		g.genSourceComment(stmtPos(stmt))
	}
	if fn, ok := stmt.(*ast.FnDecl); ok {
		return g.genModuleFnDecl(fn)
	}
	if classDecl, ok := stmt.(*ast.ClassDecl); ok {
		return g.genClassDecl(classDecl)
	}
	return g.genModuleTopStmt(stmt)
}

func (g *moduleGenerator) genModuleTopStmt(stmt ast.Statement) error {
	switch s := stmt.(type) {
	case *ast.ExprStmt:
		return g.genModuleExprStmt(s)
	case *ast.LetDecl:
		if s.Exported {
			name := s.Name
			if s.DefaultExport {
				name = "default"
			}
			g.paramMap[s.Name] = g.qualifyFnName(name)
		}
		return g.genLetDecl(s)
	case *ast.Assignment:
		return g.genAssignment(s)
	case *ast.IndexAssignStmt:
		return g.genIndexAssign(s)
	case *ast.PropertyAssignStmt:
		return g.genPropertyAssign(s)
	case *ast.IfStmt:
		return g.genIf(s)
	case *ast.SwitchStmt:
		return g.genSwitch(s)
	case *ast.ForStmt:
		return g.genFor(s)
	case *ast.CStyleForStmt:
		return g.genCStyleFor(s)
	case *ast.WhileStmt:
		return g.genWhile(s)
	case *ast.TryStmt:
		return g.genTry(s)
	case *ast.ReturnStmt:
		return g.genReturn(s)
	case *ast.ImportDecl:
		return nil
	case *ast.DeclareStmt:
		return nil
	case *ast.ClassDecl:
		return g.genClassDecl(s)
	}
	return g.genStmt(stmt)
}

func (g *moduleGenerator) genModuleExprStmt(s *ast.ExprStmt) error {
	if call, ok := s.Expr.(*ast.FnCallExpr); ok {
		qualName := g.resolveFnName(call.Name)
		argStrs, err := g.genArgs(call.Args)
		if err != nil {
			return err
		}
		retType := g.fnReturnMap[call.Name]
		isVoid := retType == nil || retType.Kind == ast.TypeVoid
		if isVoid {
			if len(argStrs) == 0 {
				g.line(qualName)
			} else {
				g.line(fmt.Sprintf("%s %s", qualName, strings.Join(argStrs, " ")))
			}
		} else {
			if len(argStrs) == 0 {
				g.line(qualName)
			} else {
				g.line(fmt.Sprintf("%s %s", qualName, strings.Join(argStrs, " ")))
			}
		}
		return nil
	}
	return g.genExprStmt(s)
}

func (g *moduleGenerator) genModuleFnDecl(s *ast.FnDecl) error {
	qualName := g.qualifyFnName(s.Name)
	g.line(fmt.Sprintf("%s() {", qualName))
	g.push()

	prevFn := g.currentFn
	prevInFn := g.inFunction
	prevParamMap := g.paramMap
	prevFnParamTypes := g.fnParamTypes
	g.currentFn = s.Name
	g.inFunction = true
	g.paramMap = make(map[string]string)
	for importedName, qualName := range g.importVarMap {
		g.paramMap[importedName] = qualName
		if typ, ok := g.importVarTypeMap[importedName]; ok {
			g.varTypeMap[importedName] = typ
			g.varTypeMap[qualName] = typ
		}
	}
	for name, qualName := range g.exportedVarAliasMap {
		g.paramMap[name] = qualName
	}
	g.fnParamTypes = make(map[string]*ast.Type)

	for i, param := range s.Params {
		varName := fmt.Sprintf("_%s_%s", qualName, param.Name)
		g.paramMap[param.Name] = varName
		if param.Type != nil {
			g.fnParamTypes[param.Name] = param.Type
			g.varTypeMap[varName] = param.Type
		}
		g.line(fmt.Sprintf("%s=\"$%d\"", varName, i+1))
	}

	if len(s.Params) > 0 {
		g.line("")
	}

	for _, bodyStmt := range s.Body.Statements {
		if err := g.genModuleFnBodyStmt(bodyStmt); err != nil {
			return err
		}
	}

	g.currentFn = prevFn
	g.inFunction = prevInFn
	g.paramMap = prevParamMap
	g.fnParamTypes = prevFnParamTypes
	g.pop()
	g.line("}")
	g.line("")
	return nil
}

func (g *moduleGenerator) genModuleFnBodyStmt(stmt ast.Statement) error {
	if _, ok := stmt.(*ast.ClassDecl); !ok {
		g.genSourceComment(stmtPos(stmt))
	}
	if s, ok := stmt.(*ast.ExprStmt); ok {
		if call, ok := s.Expr.(*ast.FnCallExpr); ok {
			qualName := g.resolveFnName(call.Name)
			argStrs, err := g.genArgs(call.Args)
			if err != nil {
				return err
			}
			retType := g.fnReturnMap[call.Name]
			isVoid := retType == nil || retType.Kind == ast.TypeVoid
			if isVoid {
				if len(argStrs) == 0 {
					g.line(qualName)
				} else {
					g.line(fmt.Sprintf("%s %s", qualName, strings.Join(argStrs, " ")))
				}
			} else {
				if len(argStrs) == 0 {
					g.line(fmt.Sprintf("$(%s)", qualName))
				} else {
					g.line(fmt.Sprintf("$(%s %s)", qualName, strings.Join(argStrs, " ")))
				}
			}
			return nil
		}
	}
	return g.genStmt(stmt)
}
