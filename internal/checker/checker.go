package checker

import (
	"fmt"
	"strings"

	"github.com/victor141516/besht/internal/ast"
)

type CheckError struct {
	Pos     ast.Pos
	Message string
}

func (e *CheckError) Error() string {
	return fmt.Sprintf("%s: %s", e.Pos, e.Message)
}

type FnSig struct {
	Params     []*ast.Param
	ReturnType *ast.Type
	Module     string
	VarArgs    bool
}

type Scope struct {
	vars   map[string]*ast.Type
	parent *Scope
}

func newScope(parent *Scope) *Scope {
	return &Scope{vars: make(map[string]*ast.Type), parent: parent}
}

func (s *Scope) define(name string, t *ast.Type) {
	s.vars[name] = t
}

func (s *Scope) lookup(name string) (*ast.Type, bool) {
	if t, ok := s.vars[name]; ok {
		return t, true
	}
	if s.parent != nil {
		return s.parent.lookup(name)
	}
	return nil, false
}

type Checker struct {
	strict     bool
	fns        map[string]*FnSig
	scope      *Scope
	currentFn  *FnSig
	inFunction bool
	inLoop     bool
	consts     map[string]bool
	classProps map[string]*ast.Type
}

type Options struct {
	Strict bool
}

func New() *Checker {
	return NewWithOptions(Options{})
}

func NewStrict() *Checker {
	return NewWithOptions(Options{Strict: true})
}

func NewWithOptions(opts Options) *Checker {
	return &Checker{
		strict:     opts.Strict,
		fns:        make(map[string]*FnSig),
		scope:      newScope(nil),
		consts:     make(map[string]bool),
		classProps: make(map[string]*ast.Type),
	}
}

func (c *Checker) RegisterFn(name string, sig *FnSig) {
	c.fns[name] = sig
}

func (c *Checker) RegisterUncheckedFunction(name string) {
	c.RegisterFn(name, &FnSig{ReturnType: &ast.Type{Kind: ast.TypeString}, VarArgs: true})
}

func (c *Checker) RegisterVar(name string, typ *ast.Type) {
	if typ == nil {
		typ = &ast.Type{Kind: ast.TypeString}
	}
	c.scope.define(name, typ)
}

func (c *Checker) Check(prog *ast.Program) error {
	collectFnSigs(prog.Statements, c.fns)
	if !c.strict {
		return nil
	}
	for _, stmt := range prog.Statements {
		if err := c.checkStmt(stmt); err != nil {
			return err
		}
	}
	return nil
}

func collectFnSigs(stmts []ast.Statement, fns map[string]*FnSig) {
	for _, stmt := range stmts {
		switch fn := stmt.(type) {
		case *ast.FnDecl:
			retType := fn.ReturnType
			if retType == nil {
				retType = &ast.Type{Kind: ast.TypeVoid}
			}
			fns[fn.Name] = &FnSig{Params: fn.Params, ReturnType: retType}
		case *ast.DeclareFnStmt:
			retType := fn.ReturnType
			if retType == nil {
				retType = &ast.Type{Kind: ast.TypeVoid}
			}
			fns[fn.Name] = &FnSig{Params: fn.Params, ReturnType: retType}
		case *ast.ClassDecl:
			for _, method := range fn.Methods {
				retType := method.ReturnType
				if retType == nil {
					retType = &ast.Type{Kind: ast.TypeVoid}
				}
				fns[fn.Name+"__"+method.Name] = &FnSig{Params: method.Params, ReturnType: retType}
			}
		}
	}
}

func (c *Checker) checkStmt(stmt ast.Statement) error {
	switch s := stmt.(type) {
	case *ast.ImportDecl:
		return nil
	case *ast.LetDecl:
		return c.checkLetDecl(s)
	case *ast.DestructureDecl:
		return c.checkDestructureDecl(s)
	case *ast.Assignment:
		return c.checkAssignment(s)
	case *ast.IndexAssignStmt:
		return c.checkIndexAssign(s)
	case *ast.PropertyAssignStmt:
		if s.Object != "this" {
			if _, ok := c.scope.lookup(s.Object); !ok {
				return &CheckError{Pos: s.Pos, Message: fmt.Sprintf("variable %q not declared", s.Object)}
			}
		} else if _, ok := c.scope.lookup("this"); !ok {
			return &CheckError{Pos: s.Pos, Message: fmt.Sprintf("variable %q not declared", s.Object)}
		}
		_, err := c.checkExpr(s.Value)
		return err
	case *ast.FnDecl:
		return c.checkFnDecl(s)
	case *ast.ClassDecl:
		c.scope.define(s.Name, &ast.Type{Kind: ast.TypeObject})
		return c.checkClassDecl(s)
	case *ast.DeclareFnStmt:
		return nil
	case *ast.IfStmt:
		return c.checkIf(s)
	case *ast.ForStmt:
		return c.checkFor(s)
	case *ast.WhileStmt:
		return c.checkWhile(s)
	case *ast.TryStmt:
		return c.checkTry(s)
	case *ast.SwitchStmt:
		return c.checkSwitch(s)
	case *ast.ReturnStmt:
		return c.checkReturn(s)
	case *ast.ExitStmt:
		return nil
	case *ast.ExprStmt:
		_, err := c.checkExpr(s.Expr)
		return err
	case *ast.Block:
		return c.checkBlock(s)
	case *ast.CStyleForStmt:
		return c.checkCStyleFor(s)
	case *ast.BreakStmt:
		if !c.inLoop {
			return &CheckError{Pos: s.Pos, Message: "'break' outside of loop"}
		}
		return nil
	case *ast.ContinueStmt:
		if !c.inLoop {
			return &CheckError{Pos: s.Pos, Message: "'continue' outside of loop"}
		}
		return nil
	}
	return nil
}

func (c *Checker) checkDestructureDecl(s *ast.DestructureDecl) error {
	valType, err := c.checkExpr(s.Value)
	if err != nil {
		return err
	}
	elemType := &ast.Type{Kind: ast.TypeString}
	if valType != nil && valType.Kind == ast.TypeList && valType.Elem != nil {
		elemType = valType.Elem
	}
	for _, name := range s.Names {
		if _, exists := c.scope.vars[name]; exists {
			return &CheckError{Pos: s.Pos, Message: fmt.Sprintf("variable %q already declared in this scope", name)}
		}
		c.scope.define(name, elemType)
		if s.IsConst {
			c.consts[name] = true
		}
	}
	return nil
}

func (c *Checker) checkIndexAssign(s *ast.IndexAssignStmt) error {
	existing, ok := c.scope.lookup(s.Name)
	if !ok {
		return &CheckError{Pos: s.Pos, Message: fmt.Sprintf("variable %q not declared", s.Name)}
	}
	if existing.Kind == ast.TypeObject {
		if _, err := c.checkExpr(s.Index); err != nil {
			return err
		}
		if _, err := c.checkExpr(s.Value); err != nil {
			return err
		}
		return nil
	}
	if existing.Kind != ast.TypeList {
		return &CheckError{Pos: s.Pos, Message: fmt.Sprintf("index assignment requires list<T> or object, got %s", existing)}
	}
	idxType, err := c.checkExpr(s.Index)
	if err != nil {
		return err
	}
	if idxType.Kind != ast.TypeNumber {
		return &CheckError{Pos: s.Pos, Message: "list index must be int"}
	}
	valType, err := c.checkExpr(s.Value)
	if err != nil {
		return err
	}
	if !c.typesCompatible(existing.Elem, valType) {
		return &CheckError{Pos: s.Pos, Message: fmt.Sprintf("cannot assign %s to list element of type %s", valType, existing.Elem)}
	}
	return nil
}

func (c *Checker) checkLetDecl(s *ast.LetDecl) error {
	if _, exists := c.scope.vars[s.Name]; exists {
		return &CheckError{Pos: s.Pos, Message: fmt.Sprintf("variable %q already declared in this scope", s.Name)}
	}

	valType, err := c.checkExpr(s.Value)
	if err != nil {
		return err
	}

	declared := s.TypeAnnot
	if !c.typesCompatible(declared, valType) {
		return &CheckError{Pos: s.Pos, Message: fmt.Sprintf("type mismatch: declared %s, got %s", declared, valType)}
	}
	if declared == nil {
		declared = valType
	}

	s.ResolvedTy = declared
	c.scope.define(s.Name, declared)
	if s.IsConst {
		c.consts[s.Name] = true
	}
	return nil
}

func (c *Checker) checkAssignment(s *ast.Assignment) error {
	existing, ok := c.scope.lookup(s.Name)
	if !ok {
		return &CheckError{Pos: s.Pos, Message: fmt.Sprintf("variable %q not declared (use 'let' to declare)", s.Name)}
	}
	if c.consts[s.Name] {
		return &CheckError{Pos: s.Pos, Message: fmt.Sprintf("cannot assign to const %q", s.Name)}
	}

	valType, err := c.checkExpr(s.Value)
	if err != nil {
		return err
	}

	if !c.typesCompatible(existing, valType) {
		return &CheckError{Pos: s.Pos, Message: fmt.Sprintf("cannot assign %s to variable of type %s", valType, existing)}
	}
	return nil
}

func (c *Checker) checkFnDecl(s *ast.FnDecl) error {
	prevFn := c.currentFn
	prevInFn := c.inFunction

	retType := s.ReturnType
	if retType == nil {
		retType = &ast.Type{Kind: ast.TypeVoid}
	}
	sig := &FnSig{Params: s.Params, ReturnType: retType}
	c.currentFn = sig
	c.inFunction = true

	outer := c.scope
	c.scope = newScope(outer)
	for _, param := range s.Params {
		c.scope.define(param.Name, param.Type)
	}

	err := c.checkBlock(s.Body)

	c.scope = outer
	c.currentFn = prevFn
	c.inFunction = prevInFn

	return err
}

func (c *Checker) checkClassDecl(s *ast.ClassDecl) error {
	for _, prop := range s.StaticProps {
		pt := prop.Type
		if pt == nil && prop.Value != nil {
			var err error
			pt, err = c.checkExpr(prop.Value)
			if err != nil {
				return err
			}
		}
		if pt == nil {
			pt = &ast.Type{Kind: ast.TypeString}
		}
		c.classProps[s.Name+"."+prop.Name] = pt
	}
	for _, prop := range s.Properties {
		if prop.Value != nil {
			if _, err := c.checkExpr(prop.Value); err != nil {
				return err
			}
		}
	}
	if s.Constructor != nil {
		if err := c.checkClassMethod(s.Constructor, true); err != nil {
			return err
		}
	}
	for i := range s.Methods {
		if err := c.checkClassMethod(&s.Methods[i], false); err != nil {
			return err
		}
	}
	return nil
}

func (c *Checker) checkClassMethod(method *ast.ClassMethod, isConstructor bool) error {
	prevFn := c.currentFn
	prevInFn := c.inFunction
	retType := method.ReturnType
	if retType == nil || isConstructor {
		retType = &ast.Type{Kind: ast.TypeVoid}
	}
	c.currentFn = &FnSig{Params: method.Params, ReturnType: retType}
	c.inFunction = true
	outer := c.scope
	c.scope = newScope(outer)
	if !method.IsStatic || isConstructor {
		c.scope.define("this", &ast.Type{Kind: ast.TypeObject})
	}
	for _, param := range method.Params {
		c.scope.define(param.Name, param.Type)
	}
	err := c.checkBlock(method.Body)
	c.scope = outer
	c.currentFn = prevFn
	c.inFunction = prevInFn
	return err
}

func (c *Checker) checkBlock(block *ast.Block) error {
	outer := c.scope
	c.scope = newScope(outer)
	for _, stmt := range block.Statements {
		if err := c.checkStmt(stmt); err != nil {
			c.scope = outer
			return err
		}
	}
	c.scope = outer
	return nil
}

func (c *Checker) checkIf(s *ast.IfStmt) error {
	condType, err := c.checkExpr(s.Condition)
	if err != nil {
		return err
	}
	if condType.Kind != ast.TypeBoolean && condType.Kind != ast.TypeStatus && condType.Kind != ast.TypeCommand {
		return &CheckError{Pos: s.Pos, Message: fmt.Sprintf("if condition must be bool or status, got %s", condType)}
	}
	if err := c.checkBlock(s.Then); err != nil {
		return err
	}
	for _, ei := range s.ElseIfs {
		eiType, err := c.checkExpr(ei.Condition)
		if err != nil {
			return err
		}
		if eiType.Kind != ast.TypeBoolean && eiType.Kind != ast.TypeStatus && eiType.Kind != ast.TypeCommand {
			return &CheckError{Pos: ei.Pos, Message: "else-if condition must be bool or status"}
		}
		if err := c.checkBlock(ei.Body); err != nil {
			return err
		}
	}
	if s.Else != nil {
		if err := c.checkBlock(s.Else); err != nil {
			return err
		}
	}
	return nil
}

func (c *Checker) checkCStyleFor(s *ast.CStyleForStmt) error {
	outer := c.scope
	c.scope = newScope(outer)

	if assign, ok := s.Init.(*ast.Assignment); ok {
		valType, err := c.checkExpr(assign.Value)
		if err != nil {
			c.scope = outer
			return err
		}
		c.scope.define(assign.Name, valType)
	} else if err := c.checkStmt(s.Init); err != nil {
		c.scope = outer
		return err
	}
	condType, err := c.checkExpr(s.Condition)
	if err != nil {
		c.scope = outer
		return err
	}
	if condType.Kind != ast.TypeBoolean && condType.Kind != ast.TypeNumber && condType.Kind != ast.TypeCommand {
		c.scope = outer
		return &CheckError{Pos: s.Pos, Message: fmt.Sprintf("for condition must be boolean, got %s", condType)}
	}
	if err := c.checkStmt(s.Update); err != nil {
		c.scope = outer
		return err
	}

	prevInLoop := c.inLoop
	c.inLoop = true
	err = c.checkBlock(s.Body)
	c.inLoop = prevInLoop
	c.scope = outer
	return err
}

func (c *Checker) checkFor(s *ast.ForStmt) error {
	iterType, err := c.checkExpr(s.Iterator)
	if err != nil {
		return err
	}

	var elemType *ast.Type
	switch iterType.Kind {
	case ast.TypeList:
		elemType = iterType.Elem
	case ast.TypeString:
		elemType = &ast.Type{Kind: ast.TypeString}
	default:
		return &CheckError{Pos: s.Pos, Message: fmt.Sprintf("cannot iterate over %s; expected list<T> or string (from shell)", iterType)}
	}

	prevInLoop := c.inLoop
	c.inLoop = true
	outer := c.scope
	c.scope = newScope(outer)
	c.scope.define(s.VarName, elemType)
	err = c.checkBlock(s.Body)
	c.scope = outer
	c.inLoop = prevInLoop
	return err
}

func (c *Checker) checkWhile(s *ast.WhileStmt) error {
	condType, err := c.checkExpr(s.Condition)
	if err != nil {
		return err
	}
	if condType.Kind != ast.TypeBoolean && condType.Kind != ast.TypeStatus && condType.Kind != ast.TypeCommand {
		return &CheckError{Pos: s.Pos, Message: "while condition must be bool or status"}
	}
	prevInLoop := c.inLoop
	c.inLoop = true
	err = c.checkBlock(s.Body)
	c.inLoop = prevInLoop
	return err
}

func (c *Checker) checkTry(s *ast.TryStmt) error {
	if err := c.checkBlock(s.Body); err != nil {
		return err
	}
	outer := c.scope
	c.scope = newScope(outer)
	c.scope.define(s.CatchVar, &ast.Type{Kind: ast.TypeStatus})
	err := c.checkBlock(s.Catch)
	c.scope = outer
	return err
}

func (c *Checker) checkSwitch(s *ast.SwitchStmt) error {
	if _, err := c.checkExpr(s.Value); err != nil {
		return err
	}
	prevInLoop := c.inLoop
	c.inLoop = true
	for _, swCase := range s.Cases {
		if !swCase.IsDefault {
			if _, err := c.checkExpr(swCase.Value); err != nil {
				c.inLoop = prevInLoop
				return err
			}
		}
		if err := c.checkBlock(swCase.Body); err != nil {
			c.inLoop = prevInLoop
			return err
		}
	}
	c.inLoop = prevInLoop
	return nil
}

func (c *Checker) checkReturn(s *ast.ReturnStmt) error {
	if !c.inFunction {
		return &CheckError{Pos: s.Pos, Message: "'return' outside of function"}
	}
	if s.Value == nil {
		if c.currentFn.ReturnType.Kind != ast.TypeVoid {
			return &CheckError{Pos: s.Pos, Message: fmt.Sprintf("function returns %s, but bare return found", c.currentFn.ReturnType)}
		}
		return nil
	}
	valType, err := c.checkExpr(s.Value)
	if err != nil {
		return err
	}
	if !c.typesCompatible(c.currentFn.ReturnType, valType) {
		return &CheckError{Pos: s.Pos, Message: fmt.Sprintf("return type mismatch: function returns %s, got %s", c.currentFn.ReturnType, valType)}
	}
	return nil
}

func (c *Checker) checkExpr(expr ast.Expression) (*ast.Type, error) {
	var t *ast.Type
	var err error

	switch e := expr.(type) {
	case *ast.IntLit:
		t = &ast.Type{Kind: ast.TypeNumber}
	case *ast.FloatLit:
		t = &ast.Type{Kind: ast.TypeNumber}
	case *ast.StringLit:
		t = &ast.Type{Kind: ast.TypeString}
	case *ast.RawStringLit:
		t = &ast.Type{Kind: ast.TypeString}
	case *ast.TemplateLit:
		t = &ast.Type{Kind: ast.TypeString}
		err = c.checkTemplateInterpolation(e)
	case *ast.BoolLit:
		t = &ast.Type{Kind: ast.TypeBoolean}
	case *ast.UndefinedLit:
		t = &ast.Type{Kind: ast.TypeString}
	case *ast.NullLit:
		t = &ast.Type{Kind: ast.TypeString}
	case *ast.ListLit:
		t, err = c.checkListLit(e)
	case *ast.ObjectLit:
		for _, field := range e.Fields {
			if _, err := c.checkExpr(field.Value); err != nil {
				return nil, err
			}
		}
		t = &ast.Type{Kind: ast.TypeObject}
	case *ast.IdentExpr:
		t, err = c.checkIdent(e)
	case *ast.ArrowExpr:
		t, err = c.checkStandaloneArrow(e)
	case *ast.NewExpr:
		for _, arg := range e.Args {
			if _, err := c.checkExpr(arg); err != nil {
				return nil, err
			}
		}
		if e.ClassName == "Set" {
			if len(e.Args) != 0 {
				return nil, &CheckError{Pos: e.Pos, Message: "Set constructor takes no runtime arguments"}
			}
			if len(e.TypeArgs) > 1 {
				return nil, &CheckError{Pos: e.Pos, Message: "Set constructor takes at most 1 type argument"}
			}
			elem := &ast.Type{Kind: ast.TypeString}
			if len(e.TypeArgs) > 0 {
				elem = e.TypeArgs[0]
			}
			t = &ast.Type{Kind: ast.TypeSet, Elem: elem}
		} else {
			t = &ast.Type{Kind: ast.TypeObject}
		}
	case *ast.ThisExpr:
		t = &ast.Type{Kind: ast.TypeObject}
	case *ast.BinaryExpr:
		t, err = c.checkBinary(e)
	case *ast.TernaryExpr:
		t, err = c.checkTernary(e)
	case *ast.UnaryExpr:
		t, err = c.checkUnary(e)
	case *ast.UpdateExpr:
		t, err = c.checkUpdate(e)
	case *ast.PipeExpr:
		t = &ast.Type{Kind: ast.TypeCommand}
	case *ast.CmdExpr:
		t, err = c.checkCmdExpr(e)
	case *ast.FnCallExpr:
		t, err = c.checkFnCall(e)
	case *ast.BuiltinCallExpr:
		t, err = c.checkBuiltinCall(e)
	case *ast.RangeExpr:
		t, err = c.checkRange(e)
	case *ast.PropagateExpr:
		t, err = c.checkPropagate(e)
	case *ast.IndexExpr:
		t, err = c.checkIndexExpr(e)
	case *ast.MethodCallExpr:
		t, err = c.checkMethodCall(e)
	case *ast.PropertyExpr:
		t, err = c.checkProperty(e)
	case *ast.SpreadExpr:
		t, err = c.checkExpr(e.Expr)
	case *ast.AsExpr:
		if _, err := c.checkExpr(e.Expr); err != nil {
			return nil, err
		}
		t = e.Type
	default:
		return nil, fmt.Errorf("unknown expression type %T", expr)
	}

	if err != nil {
		return nil, err
	}
	if setter, ok := expr.(interface{ SetType(*ast.Type) }); ok {
		setter.SetType(t)
	}
	return t, nil
}

func (c *Checker) checkTemplateInterpolation(e *ast.TemplateLit) error {
	for _, expr := range e.Exprs {
		if _, err := c.checkExpr(expr); err != nil {
			return err
		}
	}
	if len(e.Exprs) > 0 {
		return nil
	}
	refs := extractVarRefs(e.Value)
	for _, ref := range refs {
		if ref == "" || strings.HasPrefix(ref, "(") {
			continue
		}
		name := ref
		if idx := strings.IndexAny(ref, ":-+=?#%"); idx >= 0 {
			name = ref[:idx]
		}
		name = strings.TrimSpace(name)
		if isShellSpecialVar(name) {
			continue
		}
		if _, ok := c.scope.lookup(name); !ok {
			return &CheckError{Pos: e.Pos, Message: fmt.Sprintf("undefined variable %q in template literal", name)}
		}
	}
	return nil
}

func (c *Checker) checkStringInterpolation(e *ast.StringLit) error {
	refs := extractVarRefs(e.Value)
	for _, ref := range refs {
		if ref == "" || strings.HasPrefix(ref, "(") {
			continue
		}
		name := ref
		if idx := strings.IndexAny(ref, ":-+=?#%"); idx >= 0 {
			name = ref[:idx]
		}
		name = strings.TrimSpace(name)
		if isShellSpecialVar(name) {
			continue
		}
		if _, ok := c.scope.lookup(name); !ok {
			return &CheckError{Pos: e.Pos, Message: fmt.Sprintf("undefined variable %q in string interpolation", name)}
		}
	}
	return nil
}

func extractVarRefs(s string) []string {
	var refs []string
	i := 0
	for i < len(s) {
		if s[i] == '$' && i+1 < len(s) && s[i+1] == '{' {
			j := i + 2
			for j < len(s) && s[j] != '}' {
				j++
			}
			if j < len(s) {
				refs = append(refs, s[i+2:j])
			}
			i = j + 1
		} else {
			i++
		}
	}
	return refs
}

func (c *Checker) checkListLit(e *ast.ListLit) (*ast.Type, error) {
	if len(e.Elements) == 0 {
		return &ast.Type{Kind: ast.TypeList, Elem: &ast.Type{Kind: ast.TypeString}}, nil
	}
	firstType, err := c.checkListElementType(e.Elements[0])
	if err != nil {
		return nil, err
	}
	for _, elem := range e.Elements[1:] {
		elemType, err := c.checkListElementType(elem)
		if err != nil {
			return nil, err
		}
		if !c.typesCompatible(firstType, elemType) {
			return nil, &CheckError{Pos: e.Pos, Message: fmt.Sprintf("list elements must have consistent type, got %s and %s", firstType, elemType)}
		}
	}
	return &ast.Type{Kind: ast.TypeList, Elem: firstType}, nil
}

func (c *Checker) checkListElementType(expr ast.Expression) (*ast.Type, error) {
	t, err := c.checkExpr(expr)
	if err != nil {
		return nil, err
	}
	if _, ok := expr.(*ast.SpreadExpr); ok && t.Kind == ast.TypeList {
		return t.Elem, nil
	}
	return t, nil
}

func (c *Checker) checkIdent(e *ast.IdentExpr) (*ast.Type, error) {
	t, ok := c.scope.lookup(e.Name)
	if !ok {
		return nil, &CheckError{Pos: e.Pos, Message: fmt.Sprintf("undefined variable %q", e.Name)}
	}
	return t, nil
}

func (c *Checker) checkBinary(e *ast.BinaryExpr) (*ast.Type, error) {
	leftType, err := c.checkExpr(e.Left)
	if err != nil {
		return nil, err
	}
	rightType, err := c.checkExpr(e.Right)
	if err != nil {
		return nil, err
	}

	switch e.Op {
	case "??":
		if _, ok := e.Left.(*ast.UndefinedLit); ok {
			return rightType, nil
		}
		if _, ok := e.Left.(*ast.NullLit); ok {
			return rightType, nil
		}
		if leftType.Kind == rightType.Kind || c.typesCompatible(leftType, rightType) {
			return leftType, nil
		}
		return rightType, nil
	case "&&", "||":
		if leftType.Kind == rightType.Kind {
			return leftType, nil
		}
		if c.typesCompatible(leftType, rightType) {
			return leftType, nil
		}
		return &ast.Type{Kind: ast.TypeString}, nil
	case "+":
		if leftType.Kind == ast.TypeString || rightType.Kind == ast.TypeString {
			return &ast.Type{Kind: ast.TypeString}, nil
		}
		if leftType.Kind != ast.TypeNumber || rightType.Kind != ast.TypeNumber {
			return nil, &CheckError{Pos: e.Pos, Message: fmt.Sprintf("operator + requires int or string operands, got %s and %s", leftType, rightType)}
		}
		return &ast.Type{Kind: ast.TypeNumber}, nil
	case "-", "*", "/", "%":
		if leftType.Kind != ast.TypeNumber || rightType.Kind != ast.TypeNumber {
			return nil, &CheckError{Pos: e.Pos, Message: fmt.Sprintf("arithmetic operator %q requires int operands, got %s and %s", e.Op, leftType, rightType)}
		}
		return &ast.Type{Kind: ast.TypeNumber}, nil
	case "==", "!=":
		if !c.typesCompatible(leftType, rightType) {
			return nil, &CheckError{Pos: e.Pos, Message: fmt.Sprintf("cannot compare %s and %s", leftType, rightType)}
		}
		return &ast.Type{Kind: ast.TypeBoolean}, nil
	case ">", "<", ">=", "<=":
		if leftType.Kind != ast.TypeNumber || rightType.Kind != ast.TypeNumber {
			return nil, &CheckError{Pos: e.Pos, Message: fmt.Sprintf("comparison %q requires int operands; use == for string comparison", e.Op)}
		}
		return &ast.Type{Kind: ast.TypeBoolean}, nil
	}
	return nil, &CheckError{Pos: e.Pos, Message: fmt.Sprintf("unknown operator %q", e.Op)}
}

func (c *Checker) checkUpdate(e *ast.UpdateExpr) (*ast.Type, error) {
	existing, ok := c.scope.lookup(e.Name)
	if !ok {
		return nil, &CheckError{Pos: e.Pos, Message: fmt.Sprintf("undefined variable %q", e.Name)}
	}
	if c.consts[e.Name] {
		return nil, &CheckError{Pos: e.Pos, Message: fmt.Sprintf("cannot assign to const %q", e.Name)}
	}
	if existing.Kind != ast.TypeNumber {
		return nil, &CheckError{Pos: e.Pos, Message: fmt.Sprintf("prefix update requires number, got %s", existing)}
	}
	return &ast.Type{Kind: ast.TypeNumber}, nil
}

func (c *Checker) checkUnary(e *ast.UnaryExpr) (*ast.Type, error) {
	inner, err := c.checkExpr(e.Expr)
	if err != nil {
		return nil, err
	}
	switch e.Op {
	case "!":
		if inner.Kind != ast.TypeBoolean && inner.Kind != ast.TypeStatus && inner.Kind != ast.TypeCommand {
			return nil, &CheckError{Pos: e.Pos, Message: fmt.Sprintf("'!' requires bool or status operand, got %s", inner)}
		}
		return &ast.Type{Kind: ast.TypeBoolean}, nil
	case "-":
		if inner.Kind != ast.TypeNumber {
			return nil, &CheckError{Pos: e.Pos, Message: fmt.Sprintf("unary '-' requires number operand, got %s", inner)}
		}
		return &ast.Type{Kind: ast.TypeNumber}, nil
	}
	return nil, &CheckError{Pos: e.Pos, Message: fmt.Sprintf("unknown unary operator %q", e.Op)}
}

func (c *Checker) checkCmdExpr(e *ast.CmdExpr) (*ast.Type, error) {
	if len(e.Args) == 0 {
		return nil, &CheckError{Pos: e.Pos, Message: "$() requires at least a command name"}
	}
	for i, arg := range e.Args {
		if spread, ok := arg.(*ast.SpreadExpr); ok {
			if i == 0 && len(e.Args) != 1 {
				return nil, &CheckError{Pos: spread.Pos, Message: "command-name spread must be the only $() argument"}
			}
			argType, err := c.checkExpr(spread.Expr)
			if err != nil {
				return nil, err
			}
			if argType.Kind != ast.TypeList {
				return nil, &CheckError{Pos: spread.Pos, Message: "spread arguments require list<T>"}
			}
			continue
		}
		if _, err := c.checkExpr(arg); err != nil {
			return nil, err
		}
	}
	return &ast.Type{Kind: ast.TypeCommand}, nil
}

func isShellSpecialVar(name string) bool {
	switch name {
	case "0", "1", "2", "3", "4", "5", "6", "7", "8", "9",
		"@", "*", "#", "?", "$", "!", "-", "_",
		"HOME", "PATH", "PWD", "OLDPWD", "IFS", "PS1", "PS2",
		"SHELL", "TERM", "USER", "LOGNAME", "LANG", "LC_ALL",
		"TMPDIR", "TMP", "TEMP", "HOSTNAME":
		return true
	}
	if len(name) > 0 && name[0] >= '0' && name[0] <= '9' {
		return true
	}
	return false
}

func (c *Checker) checkFnCall(e *ast.FnCallExpr) (*ast.Type, error) {
	sig, ok := c.fns[e.Name]
	if !ok {
		return nil, &CheckError{Pos: e.Pos, Message: fmt.Sprintf("undefined function %q", e.Name)}
	}
	if sig.VarArgs {
		for _, arg := range e.Args {
			if _, err := c.checkExpr(arg); err != nil {
				return nil, err
			}
		}
		return sig.ReturnType, nil
	}
	if len(e.Args) != len(sig.Params) {
		return nil, &CheckError{Pos: e.Pos, Message: fmt.Sprintf("function %q expects %d arguments, got %d", e.Name, len(sig.Params), len(e.Args))}
	}
	for i, arg := range e.Args {
		argType, err := c.checkExpr(arg)
		if err != nil {
			return nil, err
		}
		expected := sig.Params[i].Type
		if !c.typesCompatible(expected, argType) {
			return nil, &CheckError{Pos: e.Pos, Message: fmt.Sprintf("argument %d of %q: expected %s, got %s", i+1, e.Name, expected, argType)}
		}
	}
	return sig.ReturnType, nil
}

func (c *Checker) checkBuiltinCall(e *ast.BuiltinCallExpr) (*ast.Type, error) {
	switch e.Name {
	case "file_exists", "is_dir", "is_readable", "is_writable", "is_executable":
		if len(e.Args) != 1 {
			return nil, &CheckError{Pos: e.Pos, Message: fmt.Sprintf("%s() takes 1 argument", e.Name)}
		}
		if _, err := c.checkExpr(e.Args[0]); err != nil {
			return nil, err
		}
		return &ast.Type{Kind: ast.TypeBoolean}, nil

	case "is_empty", "is_set":
		if len(e.Args) != 1 {
			return nil, &CheckError{Pos: e.Pos, Message: fmt.Sprintf("%s() takes 1 argument", e.Name)}
		}
		if _, err := c.checkExpr(e.Args[0]); err != nil {
			return nil, err
		}
		return &ast.Type{Kind: ast.TypeBoolean}, nil

	case "len":
		if len(e.Args) != 1 {
			return nil, &CheckError{Pos: e.Pos, Message: "len() takes 1 argument"}
		}
		argType, err := c.checkExpr(e.Args[0])
		if err != nil {
			return nil, err
		}
		if argType.Kind != ast.TypeList {
			return nil, &CheckError{Pos: e.Pos, Message: fmt.Sprintf("len() requires list<T>, got %s", argType)}
		}
		return &ast.Type{Kind: ast.TypeNumber}, nil

	case "head":
		if len(e.Args) != 1 {
			return nil, &CheckError{Pos: e.Pos, Message: "head() takes 1 argument"}
		}
		argType, err := c.checkExpr(e.Args[0])
		if err != nil {
			return nil, err
		}
		if argType.Kind != ast.TypeList {
			return nil, &CheckError{Pos: e.Pos, Message: "head() requires a list"}
		}
		return argType.Elem, nil

	case "tail":
		if len(e.Args) != 1 {
			return nil, &CheckError{Pos: e.Pos, Message: "tail() takes 1 argument"}
		}
		argType, err := c.checkExpr(e.Args[0])
		if err != nil {
			return nil, err
		}
		if argType.Kind != ast.TypeList {
			return nil, &CheckError{Pos: e.Pos, Message: "tail() requires a list"}
		}
		return argType, nil

	case "append":
		if len(e.Args) != 2 {
			return nil, &CheckError{Pos: e.Pos, Message: "append() takes 2 arguments: list and element"}
		}
		listType, err := c.checkExpr(e.Args[0])
		if err != nil {
			return nil, err
		}
		if listType.Kind != ast.TypeList {
			return nil, &CheckError{Pos: e.Pos, Message: "append() first argument must be a list"}
		}
		elemType, err := c.checkExpr(e.Args[1])
		if err != nil {
			return nil, err
		}
		if !c.typesCompatible(listType.Elem, elemType) {
			return nil, &CheckError{Pos: e.Pos, Message: fmt.Sprintf("append() type mismatch: list<%s> cannot hold %s", listType.Elem, elemType)}
		}
		return listType, nil

	case "contains":
		if len(e.Args) != 2 {
			return nil, &CheckError{Pos: e.Pos, Message: "contains() takes 2 arguments"}
		}
		if _, err := c.checkExpr(e.Args[0]); err != nil {
			return nil, err
		}
		if _, err := c.checkExpr(e.Args[1]); err != nil {
			return nil, err
		}
		return &ast.Type{Kind: ast.TypeBoolean}, nil

	case "range":
		if len(e.Args) != 2 {
			return nil, &CheckError{Pos: e.Pos, Message: "range() takes 2 arguments: start, end"}
		}
		for _, arg := range e.Args {
			argType, err := c.checkExpr(arg)
			if err != nil {
				return nil, err
			}
			if argType.Kind != ast.TypeNumber {
				return nil, &CheckError{Pos: e.Pos, Message: "range() arguments must be int"}
			}
		}
		return &ast.Type{Kind: ast.TypeList, Elem: &ast.Type{Kind: ast.TypeNumber}}, nil

	case "exit":
		if len(e.Args) > 1 {
			return nil, &CheckError{Pos: e.Pos, Message: "exit() takes 0 or 1 argument"}
		}
		if len(e.Args) == 1 {
			argType, err := c.checkExpr(e.Args[0])
			if err != nil {
				return nil, err
			}
			if argType.Kind != ast.TypeNumber && argType.Kind != ast.TypeStatus {
				return nil, &CheckError{Pos: e.Pos, Message: "exit() argument must be int or status"}
			}
		}
		return &ast.Type{Kind: ast.TypeVoid}, nil

	case "env":
		if len(e.Args) < 1 || len(e.Args) > 2 {
			return nil, &CheckError{Pos: e.Pos, Message: "env() takes 1 or 2 arguments: env(NAME) or env(NAME, default)"}
		}
		if _, err := c.checkExpr(e.Args[0]); err != nil {
			return nil, err
		}
		if len(e.Args) == 2 {
			if _, err := c.checkExpr(e.Args[1]); err != nil {
				return nil, err
			}
		}
		return &ast.Type{Kind: ast.TypeString}, nil

	case "console.log", "console.error":
		for _, arg := range e.Args {
			if _, err := c.checkExpr(arg); err != nil {
				return nil, err
			}
		}
		return &ast.Type{Kind: ast.TypeVoid}, nil

	case "to_str", "String":
		if len(e.Args) != 1 {
			return nil, &CheckError{Pos: e.Pos, Message: "str() takes 1 argument"}
		}
		if _, err := c.checkExpr(e.Args[0]); err != nil {
			return nil, err
		}
		return &ast.Type{Kind: ast.TypeString}, nil

	case "to_int":
		if len(e.Args) != 1 {
			return nil, &CheckError{Pos: e.Pos, Message: "int() takes 1 argument"}
		}
		if _, err := c.checkExpr(e.Args[0]); err != nil {
			return nil, err
		}
		return &ast.Type{Kind: ast.TypeNumber}, nil

	case "Number.parseInt", "Number.parseFloat":
		if len(e.Args) < 1 || len(e.Args) > 2 {
			return nil, &CheckError{Pos: e.Pos, Message: e.Name + "() takes 1 or 2 arguments"}
		}
		for _, arg := range e.Args {
			if _, err := c.checkExpr(arg); err != nil {
				return nil, err
			}
		}
		return &ast.Type{Kind: ast.TypeNumber}, nil

	case "Array.from":
		if len(e.Args) != 1 {
			return nil, &CheckError{Pos: e.Pos, Message: "Array.from() takes 1 argument"}
		}
		obj, ok := e.Args[0].(*ast.ObjectLit)
		if !ok {
			return nil, &CheckError{Pos: e.Pos, Message: "Array.from() currently supports only { length: number }"}
		}
		foundLength := false
		for _, field := range obj.Fields {
			if field.Key != "length" {
				continue
			}
			foundLength = true
			lengthType, err := c.checkExpr(field.Value)
			if err != nil {
				return nil, err
			}
			if lengthType.Kind != ast.TypeNumber {
				return nil, &CheckError{Pos: field.Pos, Message: fmt.Sprintf("Array.from({ length }) requires number length, got %s", lengthType)}
			}
		}
		if !foundLength {
			return nil, &CheckError{Pos: e.Pos, Message: "Array.from() requires a length field"}
		}
		return &ast.Type{Kind: ast.TypeList, Elem: &ast.Type{Kind: ast.TypeNumber}}, nil

	case "Array.of":
		if len(e.Args) == 0 {
			return &ast.Type{Kind: ast.TypeList, Elem: &ast.Type{Kind: ast.TypeString}}, nil
		}
		firstType, err := c.checkExpr(e.Args[0])
		if err != nil {
			return nil, err
		}
		for _, arg := range e.Args[1:] {
			argType, err := c.checkExpr(arg)
			if err != nil {
				return nil, err
			}
			if !c.typesCompatible(firstType, argType) {
				return nil, &CheckError{Pos: e.Pos, Message: fmt.Sprintf("Array.of() elements must have consistent type, got %s and %s", firstType, argType)}
			}
		}
		return &ast.Type{Kind: ast.TypeList, Elem: firstType}, nil

	case "Number.isFinite", "Number.isInteger":
		if len(e.Args) != 1 {
			return nil, &CheckError{Pos: e.Pos, Message: e.Name + "() takes 1 argument"}
		}
		if _, err := c.checkExpr(e.Args[0]); err != nil {
			return nil, err
		}
		return &ast.Type{Kind: ast.TypeBoolean}, nil

	case "Number.isSafeInteger":
		if len(e.Args) != 1 {
			return nil, &CheckError{Pos: e.Pos, Message: "Number.isSafeInteger() takes 1 argument"}
		}
		if _, err := c.checkExpr(e.Args[0]); err != nil {
			return nil, err
		}
		return &ast.Type{Kind: ast.TypeBoolean}, nil

	case "Number.isNaN":
		if len(e.Args) != 1 {
			return nil, &CheckError{Pos: e.Pos, Message: "Number.isNaN() takes 1 argument"}
		}
		if _, err := c.checkExpr(e.Args[0]); err != nil {
			return nil, err
		}
		return &ast.Type{Kind: ast.TypeBoolean}, nil

	case "concat":
		if len(e.Args) != 2 {
			return nil, &CheckError{Pos: e.Pos, Message: "concat() takes 2 list arguments"}
		}
		aType, err := c.checkExpr(e.Args[0])
		if err != nil {
			return nil, err
		}
		bType, err := c.checkExpr(e.Args[1])
		if err != nil {
			return nil, err
		}
		if aType.Kind != ast.TypeList {
			return nil, &CheckError{Pos: e.Pos, Message: fmt.Sprintf("concat() first argument must be list, got %s", aType)}
		}
		if bType.Kind != ast.TypeList {
			return nil, &CheckError{Pos: e.Pos, Message: fmt.Sprintf("concat() second argument must be list, got %s", bType)}
		}
		return aType, nil
	}

	return nil, &CheckError{Pos: e.Pos, Message: fmt.Sprintf("unknown builtin %q", e.Name)}
}

func (c *Checker) checkMethodCall(e *ast.MethodCallExpr) (*ast.Type, error) {
	if ident, ok := e.Receiver.(*ast.IdentExpr); ok && ident.Name == "args" {
		return c.checkArgsMethod(e)
	}
	if ident, ok := e.Receiver.(*ast.IdentExpr); ok && ident.Name == "Math" {
		return c.checkMathMethod(e)
	}

	recvType, err := c.checkExpr(e.Receiver)
	if err != nil {
		return nil, err
	}
	if recvType.Kind == ast.TypeList && (e.Method == "map" || e.Method == "filter" || e.Method == "reduce" || e.Method == "findIndex") {
		return c.checkListMethod(e, recvType)
	}
	for _, arg := range e.Args {
		if _, err := c.checkExpr(arg); err != nil {
			return nil, err
		}
	}

	switch recvType.Kind {
	case ast.TypeSet:
		return c.checkSetMethod(e, recvType)
	case ast.TypeList:
		return c.checkListMethod(e, recvType)
	case ast.TypeString:
		return c.checkStringMethod(e)
	case ast.TypeNumber:
		return c.checkNumberMethod(e)
	case ast.TypeCommand:
		return c.checkCommandMethod(e)
	}
	return nil, &CheckError{Pos: e.Pos, Message: fmt.Sprintf("type %s has no methods", recvType)}
}

func (c *Checker) checkArgsMethod(e *ast.MethodCallExpr) (*ast.Type, error) {
	strType := &ast.Type{Kind: ast.TypeString}
	switch e.Method {
	case "argv":
		if len(e.Args) != 0 {
			return nil, &CheckError{Pos: e.Pos, Message: "args.argv() takes no arguments"}
		}
		return &ast.Type{Kind: ast.TypeList, Elem: strType}, nil
	case "positional":
		if len(e.Args) != 1 {
			return nil, &CheckError{Pos: e.Pos, Message: "args.positional() takes 1 argument"}
		}
		if _, err := c.checkExpr(e.Args[0]); err != nil {
			return nil, err
		}
		return strType, nil
	case "option":
		if len(e.Args) < 1 || len(e.Args) > 2 {
			return nil, &CheckError{Pos: e.Pos, Message: "args.option() takes 1 or 2 arguments"}
		}
		for _, arg := range e.Args {
			if _, err := c.checkExpr(arg); err != nil {
				return nil, err
			}
		}
		return strType, nil
	case "flag":
		if len(e.Args) < 1 || len(e.Args) > 2 {
			return nil, &CheckError{Pos: e.Pos, Message: "args.flag() takes 1 or 2 arguments"}
		}
		for _, arg := range e.Args {
			if _, err := c.checkExpr(arg); err != nil {
				return nil, err
			}
		}
		return &ast.Type{Kind: ast.TypeBoolean}, nil
	}
	return nil, &CheckError{Pos: e.Pos, Message: fmt.Sprintf("args has no method %q", e.Method)}
}

func (c *Checker) checkSetMethod(e *ast.MethodCallExpr, setType *ast.Type) (*ast.Type, error) {
	switch e.Method {
	case "has":
		if len(e.Args) != 1 {
			return nil, &CheckError{Pos: e.Pos, Message: "Set.has() takes 1 argument"}
		}
		return &ast.Type{Kind: ast.TypeBoolean}, nil
	case "add":
		if len(e.Args) != 1 {
			return nil, &CheckError{Pos: e.Pos, Message: "Set.add() takes 1 argument"}
		}
		return &ast.Type{Kind: ast.TypeVoid}, nil
	}
	return nil, &CheckError{Pos: e.Pos, Message: fmt.Sprintf("Set has no method %q", e.Method)}
}

func (c *Checker) checkCommandMethod(e *ast.MethodCallExpr) (*ast.Type, error) {
	cmdType := &ast.Type{Kind: ast.TypeCommand}
	strType := &ast.Type{Kind: ast.TypeString}
	listStrType := &ast.Type{Kind: ast.TypeList, Elem: strType}

	switch e.Method {
	case "pipe":
		if len(e.Args) != 1 {
			return nil, &CheckError{Pos: e.Pos, Message: "pipe() takes 1 argument"}
		}
		if _, err := c.checkExpr(e.Args[0]); err != nil {
			return nil, err
		}
		return cmdType, nil
	case "run":
		if len(e.Args) != 0 {
			return nil, &CheckError{Pos: e.Pos, Message: "run() takes no arguments"}
		}
		return cmdType, nil
	case "readStdout":
		if len(e.Args) != 0 {
			return nil, &CheckError{Pos: e.Pos, Message: "readStdout() takes no arguments"}
		}
		return strType, nil
	case "readStdoutLines":
		if len(e.Args) != 0 {
			return nil, &CheckError{Pos: e.Pos, Message: "readStdoutLines() takes no arguments"}
		}
		return listStrType, nil
	case "readStderr":
		if len(e.Args) != 0 {
			return nil, &CheckError{Pos: e.Pos, Message: "readStderr() takes no arguments"}
		}
		return strType, nil
	case "exitCode":
		if len(e.Args) != 0 {
			return nil, &CheckError{Pos: e.Pos, Message: "exitCode() takes no arguments"}
		}
		return &ast.Type{Kind: ast.TypeNumber}, nil
	case "clone":
		if len(e.Args) != 0 {
			return nil, &CheckError{Pos: e.Pos, Message: "clone() takes no arguments"}
		}
		return cmdType, nil
	case "stdout":
		if len(e.Args) < 1 || len(e.Args) > 2 {
			return nil, &CheckError{Pos: e.Pos, Message: "stdout() takes 1 or 2 arguments: (path[, \"append\"])"}
		}
		return cmdType, nil
	case "stderr":
		if len(e.Args) != 1 {
			return nil, &CheckError{Pos: e.Pos, Message: "stderr() takes 1 argument: \"null\", \"&1\", or a file path"}
		}
		return cmdType, nil
	case "workdir":
		if len(e.Args) != 1 {
			return nil, &CheckError{Pos: e.Pos, Message: "workdir() takes 1 argument: path"}
		}
		return cmdType, nil
	case "env":
		if len(e.Args) != 2 {
			return nil, &CheckError{Pos: e.Pos, Message: "env() on command takes 2 arguments: (name, value)"}
		}
		name, err := commandEnvName(e.Args[0])
		if err != nil {
			return nil, &CheckError{Pos: e.Pos, Message: err.Error()}
		}
		if !isShellIdentifier(name) {
			return nil, &CheckError{Pos: e.Pos, Message: fmt.Sprintf("invalid command env name %q", name)}
		}
		return cmdType, nil
	}
	return nil, &CheckError{Pos: e.Pos, Message: fmt.Sprintf("command has no method %q (available: run, pipe, readStdout, readStdoutLines, readStderr, exitCode, stdout, stderr, workdir, env, clone)", e.Method)}
}

func (c *Checker) checkListMethod(e *ast.MethodCallExpr, listType *ast.Type) (*ast.Type, error) {
	nargs := len(e.Args)
	strType := &ast.Type{Kind: ast.TypeString}
	intType := &ast.Type{Kind: ast.TypeNumber}
	boolType := &ast.Type{Kind: ast.TypeBoolean}

	switch e.Method {
	case "push", "unshift":
		if nargs != 1 {
			return nil, &CheckError{Pos: e.Pos, Message: e.Method + "() takes 1 argument"}
		}
		return listType, nil
	case "pop":
		if nargs != 0 {
			return nil, &CheckError{Pos: e.Pos, Message: "pop() takes no arguments"}
		}
		return listType, nil
	case "shift":
		if nargs != 0 {
			return nil, &CheckError{Pos: e.Pos, Message: "shift() takes no arguments"}
		}
		return listType, nil
	case "concat":
		if nargs != 1 {
			return nil, &CheckError{Pos: e.Pos, Message: "concat() takes 1 argument"}
		}
		return listType, nil
	case "slice":
		if nargs < 1 || nargs > 2 {
			return nil, &CheckError{Pos: e.Pos, Message: "slice() takes 1 or 2 arguments"}
		}
		return listType, nil
	case "join":
		if nargs != 1 {
			return nil, &CheckError{Pos: e.Pos, Message: "join() takes 1 argument (separator)"}
		}
		return strType, nil
	case "includes":
		if nargs != 1 {
			return nil, &CheckError{Pos: e.Pos, Message: "includes() takes 1 argument"}
		}
		return boolType, nil
	case "indexOf":
		if nargs != 1 {
			return nil, &CheckError{Pos: e.Pos, Message: "indexOf() takes 1 argument"}
		}
		return intType, nil
	case "findIndex":
		if nargs != 1 {
			return nil, &CheckError{Pos: e.Pos, Message: "findIndex() takes 1 arrow callback"}
		}
		if _, err := c.checkArrowCallback(e.Args[0], listType.Elem); err != nil {
			return nil, err
		}
		return intType, nil
	case "lastIndexOf":
		if nargs != 1 {
			return nil, &CheckError{Pos: e.Pos, Message: "lastIndexOf() takes 1 argument"}
		}
		return intType, nil
	case "reverse":
		if nargs != 0 {
			return nil, &CheckError{Pos: e.Pos, Message: "reverse() takes no arguments"}
		}
		return listType, nil
	case "map":
		if nargs != 1 {
			return nil, &CheckError{Pos: e.Pos, Message: "map() takes 1 arrow callback"}
		}
		bodyType, err := c.checkArrowCallback(e.Args[0], listType.Elem)
		if err != nil {
			return nil, err
		}
		return &ast.Type{Kind: ast.TypeList, Elem: bodyType}, nil
	case "filter":
		if nargs != 1 {
			return nil, &CheckError{Pos: e.Pos, Message: "filter() takes 1 arrow callback"}
		}
		if _, err := c.checkArrowCallback(e.Args[0], listType.Elem); err != nil {
			return nil, err
		}
		return listType, nil
	case "reduce":
		if nargs != 2 {
			return nil, &CheckError{Pos: e.Pos, Message: "reduce() takes 2 arguments: callback and initial value"}
		}
		arrow, ok := e.Args[0].(*ast.ArrowExpr)
		if !ok {
			return nil, &CheckError{Pos: e.Pos, Message: "reduce() callback must be an arrow expression"}
		}
		if len(arrow.Params) != 2 {
			return nil, &CheckError{Pos: arrow.Pos, Message: "reduce() callback must take 2 parameters (accumulator, current)"}
		}
		initType, err := c.checkExpr(e.Args[1])
		if err != nil {
			return nil, err
		}
		accType := initType
		if arrow.Params[0].Type != nil {
			accType = arrow.Params[0].Type
		}
		old := c.scope
		inner := newScope(c.scope)
		inner.define(arrow.Params[0].Name, accType)
		elemT := listType.Elem
		if len(arrow.Params) > 1 {
			if arrow.Params[1].Type != nil {
				elemT = arrow.Params[1].Type
			}
			inner.define(arrow.Params[1].Name, elemT)
		}
		c.scope = inner
		if arrow.BlockBody != nil {
			prevFn := c.currentFn
			prevInFn := c.inFunction
			c.currentFn = &FnSig{Params: arrow.Params, ReturnType: accType}
			c.inFunction = true
			if err := c.checkBlock(arrow.BlockBody); err != nil {
				c.currentFn = prevFn
				c.inFunction = prevInFn
				c.scope = old
				return nil, err
			}
			c.currentFn = prevFn
			c.inFunction = prevInFn
		} else {
			if _, err := c.checkExpr(arrow.Body); err != nil {
				c.scope = old
				return nil, err
			}
		}
		c.scope = old
		return accType, nil
	}
	return nil, &CheckError{Pos: e.Pos, Message: fmt.Sprintf("list has no method %q", e.Method)}
}

func (c *Checker) checkStandaloneArrow(e *ast.ArrowExpr) (*ast.Type, error) {
	return nil, &CheckError{Pos: e.Pos, Message: "arrow expressions can only be used as list callbacks"}
}

func (c *Checker) checkArrowCallback(expr ast.Expression, elemType *ast.Type) (*ast.Type, error) {
	arrow, ok := expr.(*ast.ArrowExpr)
	if !ok {
		return nil, &CheckError{Pos: ast.Pos{}, Message: "list callback must be an arrow expression"}
	}
	return c.checkArrowWithParamType(arrow, elemType)
}

func (c *Checker) checkArrowWithParamType(e *ast.ArrowExpr, elemType *ast.Type) (*ast.Type, error) {
	if len(e.Params) < 1 || len(e.Params) > 2 {
		return nil, &CheckError{Pos: e.Pos, Message: "arrow callbacks take 1 or 2 parameters"}
	}
	param := e.Params[0]
	if param.Type != nil && !c.typesCompatible(param.Type, elemType) {
		return nil, &CheckError{Pos: param.Pos, Message: fmt.Sprintf("arrow parameter %q expects %s, got %s", param.Name, param.Type, elemType)}
	}
	old := c.scope
	inner := newScope(c.scope)
	pt := elemType
	if param.Type != nil {
		pt = param.Type
	}
	inner.define(param.Name, pt)
	if len(e.Params) == 2 {
		p2 := e.Params[1]
		p2t := &ast.Type{Kind: ast.TypeNumber}
		if p2.Type != nil {
			if !c.typesCompatible(p2.Type, p2t) {
				return nil, &CheckError{Pos: p2.Pos, Message: fmt.Sprintf("arrow index parameter %q expects %s, got %s", p2.Name, p2.Type, p2t)}
			}
			p2t = p2.Type
		}
		inner.define(p2.Name, p2t)
	}
	c.scope = inner
	var bodyType *ast.Type
	var err error
	if e.BlockBody != nil {
		bodyType, err = c.checkCallbackBlock(e.BlockBody)
	} else {
		bodyType, err = c.checkExpr(e.Body)
	}
	c.scope = old
	if err != nil {
		return nil, err
	}
	return bodyType, nil
}

func (c *Checker) checkCallbackBlock(block *ast.Block) (*ast.Type, error) {
	outer := c.scope
	c.scope = newScope(outer)
	var returnType *ast.Type
	for _, stmt := range block.Statements {
		t, err := c.checkCallbackStmt(stmt)
		if err != nil {
			c.scope = outer
			return nil, err
		}
		if t == nil {
			continue
		}
		if returnType == nil {
			returnType = t
		} else if !c.typesCompatible(returnType, t) {
			c.scope = outer
			return nil, &CheckError{Pos: ast.Pos{}, Message: fmt.Sprintf("callback returns inconsistent types %s and %s", returnType, t)}
		}
	}
	c.scope = outer
	if returnType == nil {
		returnType = &ast.Type{Kind: ast.TypeVoid}
	}
	return returnType, nil
}

func (c *Checker) checkCallbackStmt(stmt ast.Statement) (*ast.Type, error) {
	switch s := stmt.(type) {
	case *ast.ReturnStmt:
		if s.Value == nil {
			return &ast.Type{Kind: ast.TypeVoid}, nil
		}
		return c.checkExpr(s.Value)
	case *ast.IfStmt:
		if _, err := c.checkExpr(s.Condition); err != nil {
			return nil, err
		}
		var returnType *ast.Type
		merge := func(t *ast.Type, err error) error {
			if err != nil || t == nil {
				return err
			}
			if returnType == nil {
				returnType = t
				return nil
			}
			if !c.typesCompatible(returnType, t) {
				return &CheckError{Pos: s.Pos, Message: fmt.Sprintf("callback returns inconsistent types %s and %s", returnType, t)}
			}
			return nil
		}
		if err := merge(c.checkCallbackBlock(s.Then)); err != nil {
			return nil, err
		}
		for _, ei := range s.ElseIfs {
			if _, err := c.checkExpr(ei.Condition); err != nil {
				return nil, err
			}
			if err := merge(c.checkCallbackBlock(ei.Body)); err != nil {
				return nil, err
			}
		}
		if s.Else != nil {
			if err := merge(c.checkCallbackBlock(s.Else)); err != nil {
				return nil, err
			}
		}
		return returnType, nil
	default:
		return nil, c.checkStmt(stmt)
	}
}

func (c *Checker) checkStringMethod(e *ast.MethodCallExpr) (*ast.Type, error) {
	nargs := len(e.Args)
	strType := &ast.Type{Kind: ast.TypeString}
	boolType := &ast.Type{Kind: ast.TypeBoolean}
	listStrType := &ast.Type{Kind: ast.TypeList, Elem: strType}
	intType := &ast.Type{Kind: ast.TypeNumber}

	switch e.Method {
	case "split":
		if nargs != 1 {
			return nil, &CheckError{Pos: e.Pos, Message: "split() takes 1 argument (separator)"}
		}
		return listStrType, nil
	case "trim":
		if nargs != 0 {
			return nil, &CheckError{Pos: e.Pos, Message: "trim() takes no arguments"}
		}
		return strType, nil
	case "trimStart", "trimEnd":
		if nargs != 0 {
			return nil, &CheckError{Pos: e.Pos, Message: e.Method + "() takes no arguments"}
		}
		return strType, nil
	case "toUpperCase":
		if nargs != 0 {
			return nil, &CheckError{Pos: e.Pos, Message: "toUpperCase() takes no arguments"}
		}
		return strType, nil
	case "toLowerCase":
		if nargs != 0 {
			return nil, &CheckError{Pos: e.Pos, Message: "toLowerCase() takes no arguments"}
		}
		return strType, nil
	case "includes":
		if err := c.checkStringSearchMethodArgs(e, "includes() takes 1 or 2 arguments"); err != nil {
			return nil, err
		}
		return boolType, nil
	case "startsWith":
		if err := c.checkStringSearchMethodArgs(e, "startsWith() takes 1 or 2 arguments"); err != nil {
			return nil, err
		}
		return boolType, nil
	case "endsWith":
		if err := c.checkStringSearchMethodArgs(e, "endsWith() takes 1 or 2 arguments"); err != nil {
			return nil, err
		}
		return boolType, nil
	case "replace":
		if nargs != 2 {
			return nil, &CheckError{Pos: e.Pos, Message: "replace() takes 2 arguments (search, replacement)"}
		}
		return strType, nil
	case "replaceAll":
		if nargs != 2 {
			return nil, &CheckError{Pos: e.Pos, Message: "replaceAll() takes 2 arguments (search, replacement)"}
		}
		return strType, nil
	case "padStart", "padEnd":
		if nargs < 1 || nargs > 2 {
			return nil, &CheckError{Pos: e.Pos, Message: e.Method + "() takes 1 or 2 arguments (length[, fillChar])"}
		}
		return strType, nil
	case "repeat":
		if nargs != 1 {
			return nil, &CheckError{Pos: e.Pos, Message: "repeat() takes 1 argument (count)"}
		}
		return strType, nil
	case "at":
		if nargs != 1 {
			return nil, &CheckError{Pos: e.Pos, Message: "at() takes 1 argument (index)"}
		}
		return strType, nil
	case "charAt":
		if nargs != 1 {
			return nil, &CheckError{Pos: e.Pos, Message: "charAt() takes 1 argument"}
		}
		argType, err := c.checkExpr(e.Args[0])
		if err != nil {
			return nil, err
		}
		if argType.Kind != ast.TypeNumber {
			return nil, &CheckError{Pos: e.Pos, Message: "charAt() argument must be number"}
		}
		return strType, nil
	case "indexOf":
		if err := c.checkStringSearchMethodArgs(e, "indexOf() takes 1 or 2 arguments"); err != nil {
			return nil, err
		}
		return intType, nil
	case "lastIndexOf":
		if err := c.checkStringSearchMethodArgs(e, "lastIndexOf() takes 1 or 2 arguments"); err != nil {
			return nil, err
		}
		return intType, nil
	case "slice":
		if nargs < 1 || nargs > 2 {
			return nil, &CheckError{Pos: e.Pos, Message: "slice() takes 1 or 2 arguments"}
		}
		return strType, nil
	case "substring":
		if nargs < 1 || nargs > 2 {
			return nil, &CheckError{Pos: e.Pos, Message: "substring() takes 1 or 2 arguments"}
		}
		for _, arg := range e.Args {
			argType, err := c.checkExpr(arg)
			if err != nil {
				return nil, err
			}
			if argType.Kind != ast.TypeNumber {
				return nil, &CheckError{Pos: e.Pos, Message: "substring() arguments must be numbers"}
			}
		}
		return strType, nil
	case "concat":
		if nargs < 1 {
			return nil, &CheckError{Pos: e.Pos, Message: "concat() takes at least 1 argument"}
		}
		return strType, nil
	}
	return nil, &CheckError{Pos: e.Pos, Message: fmt.Sprintf("string has no method %q", e.Method)}
}

func (c *Checker) checkStringSearchMethodArgs(e *ast.MethodCallExpr, arityMessage string) error {
	if len(e.Args) < 1 || len(e.Args) > 2 {
		return &CheckError{Pos: e.Pos, Message: arityMessage}
	}
	if len(e.Args) == 2 {
		argType, err := c.checkExpr(e.Args[1])
		if err != nil {
			return err
		}
		if argType.Kind != ast.TypeNumber {
			return &CheckError{Pos: e.Pos, Message: e.Method + "() second argument must be number"}
		}
	}
	return nil
}

func (c *Checker) checkMathMethod(e *ast.MethodCallExpr) (*ast.Type, error) {
	nargs := len(e.Args)
	want := 1
	switch e.Method {
	case "min", "max", "pow":
		want = 2
	case "round", "floor", "ceil", "abs", "sqrt", "trunc", "sign":
		want = 1
	default:
		return nil, &CheckError{Pos: e.Pos, Message: fmt.Sprintf("Math has no method %q", e.Method)}
	}
	if nargs != want {
		return nil, &CheckError{Pos: e.Pos, Message: fmt.Sprintf("Math.%s() takes %d argument(s)", e.Method, want)}
	}
	for _, arg := range e.Args {
		argType, err := c.checkExpr(arg)
		if err != nil {
			return nil, err
		}
		if argType.Kind != ast.TypeNumber {
			return nil, &CheckError{Pos: e.Pos, Message: fmt.Sprintf("Math.%s() arguments must be numbers", e.Method)}
		}
	}
	return &ast.Type{Kind: ast.TypeNumber}, nil
}

func (c *Checker) checkNumberMethod(e *ast.MethodCallExpr) (*ast.Type, error) {
	nargs := len(e.Args)
	strType := &ast.Type{Kind: ast.TypeString}

	switch e.Method {
	case "toString":
		if nargs != 0 {
			return nil, &CheckError{Pos: e.Pos, Message: "toString() takes no arguments"}
		}
		return strType, nil
	case "toFixed":
		if nargs > 1 {
			return nil, &CheckError{Pos: e.Pos, Message: "toFixed() takes 0 or 1 arguments"}
		}
		if nargs == 1 {
			argType, err := c.checkExpr(e.Args[0])
			if err != nil {
				return nil, err
			}
			if argType.Kind != ast.TypeNumber {
				return nil, &CheckError{Pos: e.Pos, Message: "toFixed() digits must be a number"}
			}
		}
		return strType, nil
	}
	return nil, &CheckError{Pos: e.Pos, Message: fmt.Sprintf("number has no method %q", e.Method)}
}

func commandEnvName(expr ast.Expression) (string, error) {
	var name string
	switch e := expr.(type) {
	case *ast.StringLit:
		name = e.Value
	case *ast.RawStringLit:
		name = e.Value
	default:
		return "", fmt.Errorf("command env name must be a string literal")
	}
	return name, nil
}

func isShellIdentifier(name string) bool {
	if name == "" {
		return false
	}
	for i, r := range name {
		if r == '_' || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
			continue
		}
		if i > 0 && r >= '0' && r <= '9' {
			continue
		}
		return false
	}
	return true
}

func (c *Checker) checkTernary(e *ast.TernaryExpr) (*ast.Type, error) {
	condType, err := c.checkExpr(e.Condition)
	if err != nil {
		return nil, err
	}
	if condType.Kind != ast.TypeBoolean && condType.Kind != ast.TypeStatus && condType.Kind != ast.TypeCommand {
		return nil, &CheckError{Pos: e.Pos, Message: fmt.Sprintf("ternary condition must be bool or status, got %s", condType)}
	}
	thenType, err := c.checkExpr(e.Then)
	if err != nil {
		return nil, err
	}
	elseType, err := c.checkExpr(e.Else)
	if err != nil {
		return nil, err
	}
	if thenType.Equal(elseType) {
		return thenType, nil
	}
	if !c.strict {
		if thenType.Kind == ast.TypeString || elseType.Kind == ast.TypeString {
			return &ast.Type{Kind: ast.TypeString}, nil
		}
		if thenType.Kind == ast.TypeNumber && elseType.Kind == ast.TypeNumber {
			return &ast.Type{Kind: ast.TypeNumber}, nil
		}
		if thenType.Kind == ast.TypeBoolean && elseType.Kind == ast.TypeBoolean {
			return &ast.Type{Kind: ast.TypeBoolean}, nil
		}
		return thenType, nil
	}
	return nil, &CheckError{Pos: e.Pos, Message: fmt.Sprintf("ternary branches must have compatible types, got %s and %s", thenType, elseType)}
}

func (c *Checker) checkProperty(e *ast.PropertyExpr) (*ast.Type, error) {
	if ident, ok := e.Receiver.(*ast.IdentExpr); ok {
		if pt, ok := c.classProps[ident.Name+"."+e.Property]; ok {
			return pt, nil
		}
	}
	recvType, err := c.checkExpr(e.Receiver)
	if err != nil {
		return nil, err
	}
	switch e.Property {
	case "length":
		return &ast.Type{Kind: ast.TypeNumber}, nil
	}
	if recvType.Kind == ast.TypeObject {
		return &ast.Type{Kind: ast.TypeString}, nil
	}
	return nil, &CheckError{Pos: e.Pos, Message: fmt.Sprintf("type %s has no property %q", recvType, e.Property)}
}

func (c *Checker) checkIndexExpr(e *ast.IndexExpr) (*ast.Type, error) {
	exprType, err := c.checkExpr(e.Expr)
	if err != nil {
		return nil, err
	}
	if exprType.Kind == ast.TypeList {
		indexType, err := c.checkExpr(e.Index)
		if err != nil {
			return nil, err
		}
		if indexType.Kind != ast.TypeNumber {
			return nil, &CheckError{Pos: e.Pos, Message: "list index must be int"}
		}
		return exprType.Elem, nil
	}
	if exprType.Kind == ast.TypeObject {
		if _, err := c.checkExpr(e.Index); err != nil {
			return nil, err
		}
		if exprType.Elem != nil {
			return exprType.Elem, nil
		}
		return &ast.Type{Kind: ast.TypeString}, nil
	}
	return nil, &CheckError{Pos: e.Pos, Message: fmt.Sprintf("index access requires list<T> or object, got %s", exprType)}
}

func (c *Checker) checkRange(e *ast.RangeExpr) (*ast.Type, error) {
	startType, err := c.checkExpr(e.Start)
	if err != nil {
		return nil, err
	}
	endType, err := c.checkExpr(e.End)
	if err != nil {
		return nil, err
	}
	if startType.Kind != ast.TypeNumber || endType.Kind != ast.TypeNumber {
		return nil, &CheckError{Pos: e.Pos, Message: "range() requires int bounds"}
	}
	return &ast.Type{Kind: ast.TypeList, Elem: &ast.Type{Kind: ast.TypeNumber}}, nil
}

func (c *Checker) checkPropagate(e *ast.PropagateExpr) (*ast.Type, error) {
	inner, err := c.checkExpr(e.Expr)
	if err != nil {
		return nil, err
	}
	return inner, nil
}

func (c *Checker) typesCompatible(expected, got *ast.Type) bool {
	if expected == nil || got == nil {
		return true
	}
	if expected.Kind == got.Kind {
		if expected.Kind == ast.TypeList || expected.Kind == ast.TypeSet {
			return c.typesCompatible(expected.Elem, got.Elem)
		}
		return true
	}
	if c.strict {
		return got.Kind == ast.TypeCommand
	}
	if expected.Kind == ast.TypeString && got.Kind == ast.TypeNumber {
		return true
	}
	if expected.Kind == ast.TypeNumber && got.Kind == ast.TypeString {
		return true
	}
	if expected.Kind == ast.TypeList && got.Kind == ast.TypeString {
		return true
	}
	if got.Kind == ast.TypeCommand {
		return true
	}
	return false
}
