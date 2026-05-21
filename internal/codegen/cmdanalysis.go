package codegen

import (
	"fmt"

	"github.com/victor141516/besht/internal/ast"
)

// CmdIdentity records everything about one $() pipeline instance.
type CmdIdentity struct {
	ID             int
	RootCmd        *ast.CmdExpr   // the innermost $() node
	FullChain      ast.Expression // full expression including .pipe/.stdout/.stderr
	VarName        string         // besht variable name bound to this identity
	RunPos         ast.Pos        // position of .run() call; zero if never run
	HasRun         bool
	UsesText       bool
	UsesLines      bool
	UsesExitCode   bool
	UsesReadStderr bool
	UsedByClone    bool
	IsForIter      bool // true when used directly as a for-loop iterator
}

// cmdScope is a lexical scope for variable→identity mapping.
type cmdScope struct {
	vars   map[string]int
	parent *cmdScope
}

func newCmdScope(parent *cmdScope) *cmdScope {
	return &cmdScope{vars: make(map[string]int), parent: parent}
}

func (s *cmdScope) define(name string, id int) {
	s.vars[name] = id
}

func (s *cmdScope) lookup(name string) (int, bool) {
	if id, ok := s.vars[name]; ok {
		return id, true
	}
	if s.parent != nil {
		return s.parent.lookup(name)
	}
	return -1, false
}

// CmdAnalysis is the result of the command analysis pass.
type CmdAnalysis struct {
	Identities []*CmdIdentity
	nodeToID   map[*ast.CmdExpr]int // CmdExpr node pointer → identity ID
	exprToID   map[ast.Expression]int
	Warnings   []CmdWarning
	Errors     []CmdError
	nextID     int
}

type CmdWarning struct {
	Pos     ast.Pos
	Message string
}

type CmdError struct {
	Pos     ast.Pos
	Message string
}

func NewCmdAnalysis() *CmdAnalysis {
	return &CmdAnalysis{
		nodeToID: make(map[*ast.CmdExpr]int),
		exprToID: make(map[ast.Expression]int),
	}
}

func (ca *CmdAnalysis) identity(id int) *CmdIdentity {
	if id < 0 || id >= len(ca.Identities) {
		return nil
	}
	return ca.Identities[id]
}

func (ca *CmdAnalysis) alloc(rootCmd *ast.CmdExpr, chain ast.Expression, varName string) int {
	id := ca.nextID
	ca.nextID++
	ca.Identities = append(ca.Identities, &CmdIdentity{
		ID:        id,
		RootCmd:   rootCmd,
		FullChain: chain,
		VarName:   varName,
	})
	if rootCmd != nil {
		if _, exists := ca.nodeToID[rootCmd]; exists {
			return id
		}
		ca.nodeToID[rootCmd] = id
	}
	return id
}

// resolveIdentity finds the CmdIdentity ID for a receiver expression.
// Returns -1 if unresolvable (function param, non-command expression, etc.)
func (ca *CmdAnalysis) resolveIdentityExpr(expr ast.Expression, scope *cmdScope) int {
	switch e := expr.(type) {
	case *ast.IdentExpr:
		if id, ok := scope.lookup(e.Name); ok {
			return id
		}
		return -1
	case *ast.CmdExpr:
		if id, ok := ca.nodeToID[e]; ok {
			return id
		}
		return -1
	case *ast.MethodCallExpr:
		switch e.Method {
		case "run", "pipe", "stdout", "stderr", "workdir", "env", "clone", "readStdout", "readStdoutLines", "readStderr", "exitCode":
			return ca.resolveIdentityExpr(e.Receiver, scope)
		}
		return -1
	}
	return -1
}

// findRootCmd extracts the innermost CmdExpr from a chain expression.
func findRootCmd(expr ast.Expression) *ast.CmdExpr {
	switch e := expr.(type) {
	case *ast.CmdExpr:
		return e
	case *ast.MethodCallExpr:
		return findRootCmd(e.Receiver)
	}
	return nil
}

// AnalyzeProgram runs the two-phase command analysis over a program's statements.
func AnalyzeProgram(stmts []ast.Statement) *CmdAnalysis {
	ca := NewCmdAnalysis()
	scope := newCmdScope(nil)
	ca.phase1Stmts(stmts, scope)
	ca.phase2Stmts(stmts, scope)
	ca.emitWarnings()
	return ca
}

// ─── Phase 1: identity allocation ────────────────────────────────────────────

func (ca *CmdAnalysis) phase1Stmts(stmts []ast.Statement, scope *cmdScope) {
	for _, s := range stmts {
		ca.phase1Stmt(s, scope)
	}
}

func (ca *CmdAnalysis) phase1Stmt(stmt ast.Statement, scope *cmdScope) {
	switch s := stmt.(type) {
	case *ast.LetDecl:
		ca.phase1Expr(s.Value, scope, s.Name)
		if id := ca.resolveIdentityExpr(s.Value, scope); id >= 0 {
			scope.define(s.Name, id)
		}
	case *ast.DestructureDecl:
		ca.phase1Expr(s.Value, scope, "")
	case *ast.Assignment:
		ca.phase1Expr(s.Value, scope, s.Name)
		if id := ca.resolveIdentityExpr(s.Value, scope); id >= 0 {
			scope.define(s.Name, id)
		}
	case *ast.IndexAssignStmt:
		ca.phase1Expr(s.Index, scope, "")
		ca.phase1Expr(s.Value, scope, "")
	case *ast.PropertyAssignStmt:
		ca.phase1Expr(s.Value, scope, "")
	case *ast.ExprStmt:
		ca.phase1Expr(s.Expr, scope, "")
	case *ast.Block:
		inner := newCmdScope(scope)
		ca.phase1Stmts(s.Statements, inner)
	case *ast.IfStmt:
		ca.phase1Expr(s.Condition, scope, "")
		ca.phase1Stmts(s.Then.Statements, newCmdScope(scope))
		for _, ei := range s.ElseIfs {
			ca.phase1Expr(ei.Condition, scope, "")
			ca.phase1Stmts(ei.Body.Statements, newCmdScope(scope))
		}
		if s.Else != nil {
			ca.phase1Stmts(s.Else.Statements, newCmdScope(scope))
		}
	case *ast.ForStmt:
		// If the iterator is a command expression, mark it as a for-loop iterator
		// (implicitly executed by the loop, no .run() needed)
		ca.phase1ForIterator(s.Iterator, scope)
		inner := newCmdScope(scope)
		ca.phase1Stmts(s.Body.Statements, inner)
	case *ast.CStyleForStmt:
		inner := newCmdScope(scope)
		ca.phase1Stmt(s.Init, inner)
		ca.phase1Expr(s.Condition, inner, "")
		ca.phase1Stmt(s.Update, inner)
		ca.phase1Stmts(s.Body.Statements, inner)
	case *ast.WhileStmt:
		ca.phase1Expr(s.Condition, scope, "")
		ca.phase1Stmts(s.Body.Statements, newCmdScope(scope))
	case *ast.TryStmt:
		ca.phase1Stmts(s.Body.Statements, newCmdScope(scope))
		inner := newCmdScope(scope)
		ca.phase1Stmts(s.Catch.Statements, inner)
	case *ast.SwitchStmt:
		ca.phase1Expr(s.Value, scope, "")
		for _, swCase := range s.Cases {
			if !swCase.IsDefault {
				ca.phase1Expr(swCase.Value, scope, "")
			}
			ca.phase1Stmts(swCase.Body.Statements, newCmdScope(scope))
		}
	case *ast.ReturnStmt:
		if s.Value != nil {
			ca.phase1Expr(s.Value, scope, "")
		}
	case *ast.FnDecl:
		inner := newCmdScope(scope)
		ca.phase1Stmts(s.Body.Statements, inner)
	case *ast.ClassDecl:
		for _, prop := range s.Properties {
			if prop.Value != nil {
				ca.phase1Expr(prop.Value, scope, "")
			}
		}
		for _, prop := range s.StaticProps {
			if prop.Value != nil {
				ca.phase1Expr(prop.Value, scope, "")
			}
		}
		if s.Constructor != nil {
			ca.phase1Stmts(s.Constructor.Body.Statements, newCmdScope(scope))
		}
		for _, method := range s.Methods {
			ca.phase1Stmts(method.Body.Statements, newCmdScope(scope))
		}
	}
}

func (ca *CmdAnalysis) phase1ForIterator(expr ast.Expression, scope *cmdScope) {
	root := findRootCmd(expr)
	if root == nil {
		return
	}
	if _, exists := ca.nodeToID[root]; !exists {
		id := ca.alloc(root, expr, "")
		ca.Identities[id].IsForIter = true
		ca.Identities[id].HasRun = true // for iterators are implicitly run
	}
}

func (ca *CmdAnalysis) phase1Expr(expr ast.Expression, scope *cmdScope, bindName string) {
	switch e := expr.(type) {
	case *ast.CmdExpr:
		if _, exists := ca.nodeToID[e]; !exists {
			ca.alloc(e, e, bindName)
		} else if bindName != "" {
			id := ca.nodeToID[e]
			if ca.Identities[id].VarName == "" {
				ca.Identities[id].VarName = bindName
			}
		}
	case *ast.MethodCallExpr:
		root := findRootCmd(e)
		if root != nil {
			if _, exists := ca.nodeToID[root]; !exists {
				id := ca.alloc(root, e, bindName)
				_ = id
			} else if bindName != "" {
				id := ca.nodeToID[root]
				if ca.Identities[id].VarName == "" {
					ca.Identities[id].VarName = bindName
				}
			}
			// recurse into arg expressions of methods (e.g. .pipe($(...)))
			for _, arg := range e.Args {
				ca.phase1Expr(arg, scope, "")
			}
		} else {
			// non-command method chain (string methods etc)
			ca.phase1Expr(e.Receiver, scope, "")
			for _, arg := range e.Args {
				ca.phase1Expr(arg, scope, "")
			}
		}
	case *ast.BinaryExpr:
		ca.phase1Expr(e.Left, scope, "")
		ca.phase1Expr(e.Right, scope, "")
	case *ast.UnaryExpr:
		ca.phase1Expr(e.Expr, scope, "")
	case *ast.UpdateExpr:
	case *ast.ListLit:
		for _, el := range e.Elements {
			ca.phase1Expr(el, scope, "")
		}
	case *ast.ObjectLit:
		for _, field := range e.Fields {
			ca.phase1Expr(field.Value, scope, "")
		}
	case *ast.NewExpr:
		for _, arg := range e.Args {
			ca.phase1Expr(arg, scope, "")
		}
	case *ast.TemplateLit:
		for _, expr := range e.Exprs {
			ca.phase1Expr(expr, scope, "")
		}
	case *ast.FnCallExpr:
		for _, arg := range e.Args {
			ca.phase1Expr(arg, scope, "")
		}
	case *ast.BuiltinCallExpr:
		for _, arg := range e.Args {
			ca.phase1Expr(arg, scope, "")
		}
	case *ast.PropertyExpr:
		ca.phase1Expr(e.Receiver, scope, "")
	case *ast.SpreadExpr:
		ca.phase1Expr(e.Expr, scope, "")
	case *ast.PropagateExpr:
		ca.phase1Expr(e.Expr, scope, "")
	case *ast.IndexExpr:
		ca.phase1Expr(e.Expr, scope, "")
		ca.phase1Expr(e.Index, scope, "")
	}
}

// extractIdentityFromExpr returns the identity ID if the expression is
// or wraps a command (for alias tracking in let/assignment).
func (ca *CmdAnalysis) extractIdentityFromExpr(expr ast.Expression, scope *cmdScope) int {
	switch e := expr.(type) {
	case *ast.CmdExpr:
		if id, ok := ca.nodeToID[e]; ok {
			return id
		}
	case *ast.MethodCallExpr:
		switch e.Method {
		case "clone":
			// clone() creates a new identity with the same pipeline
			srcID := ca.resolveIdentityExpr(e.Receiver, scope)
			if srcID < 0 {
				return -1
			}
			src := ca.identity(srcID)
			src.UsedByClone = true
			newID := ca.alloc(src.RootCmd, src.FullChain, "")
			ca.Identities[newID].RootCmd = src.RootCmd
			ca.Identities[newID].FullChain = src.FullChain
			return newID
		case "run", "pipe", "stdout", "stderr", "workdir", "env":
			root := findRootCmd(e)
			if root != nil {
				if id, ok := ca.nodeToID[root]; ok {
					return id
				}
			}
		}
	case *ast.IdentExpr:
		if id, ok := scope.lookup(e.Name); ok {
			return id
		}
	}
	return -1
}

// ─── Phase 2: usage scanning ──────────────────────────────────────────────────

func (ca *CmdAnalysis) phase2Stmts(stmts []ast.Statement, scope *cmdScope) {
	for _, s := range stmts {
		ca.phase2Stmt(s, scope)
	}
}

func (ca *CmdAnalysis) phase2Stmt(stmt ast.Statement, scope *cmdScope) {
	switch s := stmt.(type) {
	case *ast.LetDecl:
		// rebuild scope alias from value
		if id := ca.extractIdentityFromExpr(s.Value, scope); id >= 0 {
			scope.define(s.Name, id)
			ca.exprToID[s.Value] = id
		}
		ca.phase2Expr(s.Value, scope)
	case *ast.DestructureDecl:
		ca.phase2Expr(s.Value, scope)
	case *ast.Assignment:
		if id := ca.extractIdentityFromExpr(s.Value, scope); id >= 0 {
			scope.define(s.Name, id)
			ca.exprToID[s.Value] = id
		}
		ca.phase2Expr(s.Value, scope)
	case *ast.IndexAssignStmt:
		ca.phase2Expr(s.Index, scope)
		ca.phase2Expr(s.Value, scope)
	case *ast.PropertyAssignStmt:
		ca.phase2Expr(s.Value, scope)
	case *ast.ExprStmt:
		ca.phase2Expr(s.Expr, scope)
	case *ast.Block:
		inner := newCmdScope(scope)
		ca.phase2Stmts(s.Statements, inner)
	case *ast.IfStmt:
		ca.phase2Expr(s.Condition, scope)
		ca.phase2Stmts(s.Then.Statements, newCmdScope(scope))
		for _, ei := range s.ElseIfs {
			ca.phase2Expr(ei.Condition, scope)
			ca.phase2Stmts(ei.Body.Statements, newCmdScope(scope))
		}
		if s.Else != nil {
			ca.phase2Stmts(s.Else.Statements, newCmdScope(scope))
		}
	case *ast.ForStmt:
		ca.phase2Expr(s.Iterator, scope)
		ca.phase2Stmts(s.Body.Statements, newCmdScope(scope))
	case *ast.CStyleForStmt:
		inner := newCmdScope(scope)
		ca.phase2Stmt(s.Init, inner)
		ca.phase2Expr(s.Condition, inner)
		ca.phase2Stmt(s.Update, inner)
		ca.phase2Stmts(s.Body.Statements, inner)
	case *ast.WhileStmt:
		ca.phase2Expr(s.Condition, scope)
		ca.phase2Stmts(s.Body.Statements, newCmdScope(scope))
	case *ast.TryStmt:
		ca.phase2Stmts(s.Body.Statements, newCmdScope(scope))
		inner := newCmdScope(scope)
		ca.phase2Stmts(s.Catch.Statements, inner)
	case *ast.SwitchStmt:
		ca.phase2Expr(s.Value, scope)
		for _, swCase := range s.Cases {
			if !swCase.IsDefault {
				ca.phase2Expr(swCase.Value, scope)
			}
			ca.phase2Stmts(swCase.Body.Statements, newCmdScope(scope))
		}
	case *ast.ReturnStmt:
		if s.Value != nil {
			ca.phase2Expr(s.Value, scope)
		}
	case *ast.FnDecl:
		inner := newCmdScope(scope)
		ca.phase2Stmts(s.Body.Statements, inner)
	case *ast.ClassDecl:
		for _, prop := range s.Properties {
			if prop.Value != nil {
				ca.phase2Expr(prop.Value, scope)
			}
		}
		for _, prop := range s.StaticProps {
			if prop.Value != nil {
				ca.phase2Expr(prop.Value, scope)
			}
		}
		if s.Constructor != nil {
			ca.phase2Stmts(s.Constructor.Body.Statements, newCmdScope(scope))
		}
		for _, method := range s.Methods {
			ca.phase2Stmts(method.Body.Statements, newCmdScope(scope))
		}
	}
}

func (ca *CmdAnalysis) phase2Expr(expr ast.Expression, scope *cmdScope) {
	me, ok := expr.(*ast.MethodCallExpr)
	if !ok {
		// recurse into sub-expressions
		switch e := expr.(type) {
		case *ast.BinaryExpr:
			ca.phase2Expr(e.Left, scope)
			ca.phase2Expr(e.Right, scope)
		case *ast.UnaryExpr:
			ca.phase2Expr(e.Expr, scope)
		case *ast.UpdateExpr:
		case *ast.PropagateExpr:
			ca.phase2Expr(e.Expr, scope)
		case *ast.IndexExpr:
			ca.phase2Expr(e.Expr, scope)
			ca.phase2Expr(e.Index, scope)
		case *ast.ListLit:
			for _, el := range e.Elements {
				ca.phase2Expr(el, scope)
			}
		case *ast.ObjectLit:
			for _, field := range e.Fields {
				ca.phase2Expr(field.Value, scope)
			}
		case *ast.NewExpr:
			for _, arg := range e.Args {
				ca.phase2Expr(arg, scope)
			}
		case *ast.TemplateLit:
			for _, expr := range e.Exprs {
				ca.phase2Expr(expr, scope)
			}
		case *ast.FnCallExpr:
			for _, arg := range e.Args {
				ca.phase2Expr(arg, scope)
			}
		case *ast.BuiltinCallExpr:
			for _, arg := range e.Args {
				ca.phase2Expr(arg, scope)
			}
		case *ast.PropertyExpr:
			ca.phase2Expr(e.Receiver, scope)
		case *ast.SpreadExpr:
			ca.phase2Expr(e.Expr, scope)
		}
		return
	}

	// Recurse into method args
	for _, arg := range me.Args {
		ca.phase2Expr(arg, scope)
	}
	ca.phase2Expr(me.Receiver, scope)

	id := ca.resolveIdentityExpr(me.Receiver, scope)
	if id < 0 {
		return
	}
	ident := ca.identity(id)
	if ident == nil {
		return
	}

	switch me.Method {
	case "run":
		if ident.HasRun {
			ca.Errors = append(ca.Errors, CmdError{
				Pos: me.Pos,
				Message: fmt.Sprintf(
					"command already run on line %d; use .clone() to run it again",
					ident.RunPos.Line,
				),
			})
		} else {
			ident.HasRun = true
			ident.RunPos = me.Pos
		}
	case "readStdout":
		ident.UsesText = true
	case "readStdoutLines":
		ident.UsesLines = true
	case "readStderr":
		ident.UsesReadStderr = true
	case "exitCode":
		ident.UsesExitCode = true
	}
}

// ─── Post-analysis ────────────────────────────────────────────────────────────

func (ca *CmdAnalysis) emitWarnings() {
	for _, ident := range ca.Identities {
		if !ident.HasRun && !ident.IsForIter && !ident.UsedByClone {
			pos := ast.Pos{}
			if ident.RootCmd != nil {
				pos = ident.RootCmd.Pos
			}
			ca.Warnings = append(ca.Warnings, CmdWarning{
				Pos:     pos,
				Message: "command declared but never run (add .run() to execute it)",
			})
		}
	}
}

// CaptureVarName returns the shell variable name to use for capturing
// this command's stdout. Uses the besht variable name for readability.
func (ident *CmdIdentity) CaptureVarName(resolveVar func(string) string) string {
	if ident.VarName != "" {
		return resolveVar(ident.VarName)
	}
	return fmt.Sprintf("_cmd%d", ident.ID)
}

func (ident *CmdIdentity) StderrCaptureVarName(resolveVar func(string) string) string {
	base := ident.CaptureVarName(resolveVar)
	return base + "_stderr"
}

// ExitCodeVarName returns the shell variable for the exit code.
func (ident *CmdIdentity) ExitCodeVarName(resolveVar func(string) string) string {
	base := ident.CaptureVarName(resolveVar)
	return base + "_exit"
}
