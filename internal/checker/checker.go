package checker

import (
	"fmt"

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
	fns        map[string]*FnSig
	scope      *Scope
	inFunction bool
	inLoop     bool
	inCallback bool
	consts     map[string]bool
}

func New() *Checker {
	return &Checker{
		fns:    make(map[string]*FnSig),
		scope:  newScope(nil),
		consts: make(map[string]bool),
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
	c.predeclareTopLevel(prog.Statements)
	if err := c.checkSemanticStmts(prog.Statements); err != nil {
		return err
	}
	varTypes := c.semanticVarTypes()
	if err := ValidateFetchSurfaceWithTypes(prog.Statements, varTypes); err != nil {
		return err
	}
	if err := ValidateObjectSurfaceWithTypes(prog.Statements, varTypes); err != nil {
		return err
	}
	if err := ValidateForEachSurfaceWithTypes(prog.Statements, varTypes); err != nil {
		return err
	}
	return nil
}

func (c *Checker) semanticVarTypes() map[string]*ast.Type {
	out := make(map[string]*ast.Type)
	for scope := c.scope; scope != nil; scope = scope.parent {
		for name, typ := range scope.vars {
			if _, exists := out[name]; !exists {
				out[name] = typ
			}
		}
	}
	return out
}

func (c *Checker) predeclareTopLevel(stmts []ast.Statement) {
	for _, stmt := range stmts {
		switch s := stmt.(type) {
		case *ast.LetDecl:
			typ := s.TypeAnnot
			if typ == nil && s.Value != nil {
				typ = s.Value.GetType()
			}
			c.scope.define(s.Name, typ)
			if s.IsConst {
				c.consts[s.Name] = true
			}
		case *ast.DestructureDecl:
			for _, name := range s.Names {
				c.scope.define(name, &ast.Type{Kind: ast.TypeString})
				if s.IsConst {
					c.consts[name] = true
				}
			}
		case *ast.ClassDecl:
			c.scope.define(s.Name, &ast.Type{Kind: ast.TypeObject})
		}
	}
}

func (c *Checker) checkSemanticStmts(stmts []ast.Statement) error {
	for _, stmt := range stmts {
		if err := c.checkSemanticStmt(stmt); err != nil {
			return err
		}
	}
	return nil
}

func (c *Checker) checkSemanticBlock(block *ast.Block) error {
	if block == nil {
		return nil
	}
	prev := c.scope
	c.scope = newScope(prev)
	defer func() { c.scope = prev }()
	return c.checkSemanticStmts(block.Statements)
}

func (c *Checker) checkSemanticStmt(stmt ast.Statement) error {
	switch s := stmt.(type) {
	case nil, *ast.ImportDecl, *ast.DeclareStmt, *ast.DeclareFnStmt:
		return nil
	case *ast.BreakStmt:
		if !c.inLoop && !c.inCallback {
			return &CheckError{Pos: s.Pos, Message: "'break' outside of loop"}
		}
		return nil
	case *ast.ContinueStmt:
		if !c.inLoop && !c.inCallback {
			return &CheckError{Pos: s.Pos, Message: "'continue' outside of loop"}
		}
		return nil
	case *ast.LetDecl:
		if err := c.checkSemanticExpr(s.Value); err != nil {
			return err
		}
		typ := s.TypeAnnot
		if typ == nil {
			typ = c.semanticExprType(s.Value)
		}
		c.scope.define(s.Name, typ)
		if s.IsConst {
			c.consts[s.Name] = true
		}
	case *ast.DestructureDecl:
		if err := c.checkSemanticExpr(s.Value); err != nil {
			return err
		}
		for _, name := range s.Names {
			c.scope.define(name, &ast.Type{Kind: ast.TypeString})
			if s.IsConst {
				c.consts[name] = true
			}
		}
	case *ast.Assignment:
		if c.consts[s.Name] {
			return &CheckError{Pos: s.Pos, Message: fmt.Sprintf("cannot assign to const %q", s.Name)}
		}
		if _, ok := c.scope.lookup(s.Name); !ok {
			return &CheckError{Pos: s.Pos, Message: fmt.Sprintf("variable %q not declared", s.Name)}
		}
		if _, ok := s.Value.(*ast.ArrowExpr); ok {
			return &CheckError{Pos: s.Pos, Message: "arrow expressions can only be used as list callbacks"}
		}
		return c.checkSemanticExpr(s.Value)
	case *ast.IndexAssignStmt:
		if c.consts[s.Name] {
			return &CheckError{Pos: s.Pos, Message: fmt.Sprintf("cannot assign to const %q", s.Name)}
		}
		if _, ok := c.scope.lookup(s.Name); !ok {
			return &CheckError{Pos: s.Pos, Message: fmt.Sprintf("variable %q not declared", s.Name)}
		}
		if err := c.checkSemanticExpr(s.Index); err != nil {
			return err
		}
		return c.checkSemanticExpr(s.Value)
	case *ast.PropertyAssignStmt:
		if s.Object != "this" {
			if c.consts[s.Object] {
				return &CheckError{Pos: s.Pos, Message: fmt.Sprintf("cannot assign to const %q", s.Object)}
			}
			if _, ok := c.scope.lookup(s.Object); !ok {
				return &CheckError{Pos: s.Pos, Message: fmt.Sprintf("variable %q not declared", s.Object)}
			}
		}
		return c.checkSemanticExpr(s.Value)
	case *ast.FnDecl:
		prev := c.scope
		prevInFunction := c.inFunction
		c.scope = newScope(prev)
		c.inFunction = true
		for _, param := range s.Params {
			c.scope.define(param.Name, param.Type)
		}
		err := c.checkSemanticStmts(s.Body.Statements)
		c.scope = prev
		c.inFunction = prevInFunction
		return err
	case *ast.ClassDecl:
		c.scope.define(s.Name, &ast.Type{Kind: ast.TypeObject})
		fieldNames := make(map[string]bool)
		accessorNames := make(map[string]map[ast.ClassAccessorKind]bool)
		for _, prop := range s.Properties {
			fieldNames[prop.Name] = true
			if err := c.checkSemanticExpr(prop.Value); err != nil {
				return err
			}
		}
		for _, prop := range s.StaticProps {
			fieldNames[prop.Name] = true
			if err := c.checkSemanticExpr(prop.Value); err != nil {
				return err
			}
		}
		for _, accessor := range s.Accessors {
			if fieldNames[accessor.Name] {
				return &CheckError{Pos: accessor.Pos, Message: fmt.Sprintf("class accessor %q conflicts with field", accessor.Name)}
			}
			if accessorNames[accessor.Name] == nil {
				accessorNames[accessor.Name] = make(map[ast.ClassAccessorKind]bool)
			}
			if accessorNames[accessor.Name][accessor.Kind] {
				return &CheckError{Pos: accessor.Pos, Message: fmt.Sprintf("duplicate %s accessor %q", accessor.Kind, accessor.Name)}
			}
			accessorNames[accessor.Name][accessor.Kind] = true
		}
		for _, method := range s.Methods {
			for name := range accessorNames {
				if method.Name == "get_"+name || method.Name == "set_"+name {
					return &CheckError{Pos: method.Pos, Message: fmt.Sprintf("method %q conflicts with accessor %q", method.Name, name)}
				}
			}
		}
		if s.Constructor != nil {
			if err := c.checkSemanticClassMethod(s.Constructor, true); err != nil {
				return err
			}
		}
		for i := range s.Methods {
			method := &s.Methods[i]
			if semanticBodyReturnsValue(method.Body.Statements) && classBodyMutatesThis(method.Body.Statements) {
				return &CheckError{Pos: method.Pos, Message: fmt.Sprintf("class method %q returns a value and cannot assign to this properties", method.Name)}
			}
			if err := c.checkSemanticClassMethod(&s.Methods[i], false); err != nil {
				return err
			}
		}
		for i := range s.Accessors {
			accessor := &s.Accessors[i]
			if accessor.Kind == ast.AccessorGet && len(accessor.Params) != 0 {
				return &CheckError{Pos: accessor.Pos, Message: fmt.Sprintf("getter %q must not take parameters", accessor.Name)}
			}
			if accessor.Kind == ast.AccessorSet && len(accessor.Params) != 1 {
				return &CheckError{Pos: accessor.Pos, Message: fmt.Sprintf("setter %q must take exactly one parameter", accessor.Name)}
			}
			if accessor.Kind == ast.AccessorSet && accessor.ReturnType != nil && accessor.ReturnType.Kind != ast.TypeVoid {
				return &CheckError{Pos: accessor.Pos, Message: fmt.Sprintf("setter %q must not declare a return type", accessor.Name)}
			}
			if accessor.Kind == ast.AccessorGet && classBodyMutatesThis(accessor.Body.Statements) {
				return &CheckError{Pos: accessor.Pos, Message: fmt.Sprintf("getter %q must not assign to this properties", accessor.Name)}
			}
			method := &ast.ClassMethod{Pos: accessor.Pos, Name: string(accessor.Kind) + "_" + accessor.Name, IsStatic: accessor.IsStatic, Params: accessor.Params, Body: accessor.Body}
			if err := c.checkSemanticClassMethod(method, false); err != nil {
				return err
			}
		}
	case *ast.Block:
		return c.checkSemanticBlock(s)
	case *ast.IfStmt:
		if err := c.checkSemanticExpr(s.Condition); err != nil {
			return err
		}
		if err := c.checkSemanticBlock(s.Then); err != nil {
			return err
		}
		for _, elseif := range s.ElseIfs {
			if err := c.checkSemanticExpr(elseif.Condition); err != nil {
				return err
			}
			if err := c.checkSemanticBlock(elseif.Body); err != nil {
				return err
			}
		}
		return c.checkSemanticBlock(s.Else)
	case *ast.ForStmt:
		if err := c.checkSemanticExpr(s.Iterator); err != nil {
			return err
		}
		prev := c.scope
		prevInLoop := c.inLoop
		c.scope = newScope(prev)
		c.inLoop = true
		c.scope.define(s.VarName, &ast.Type{Kind: ast.TypeString})
		err := c.checkSemanticStmts(s.Body.Statements)
		c.scope = prev
		c.inLoop = prevInLoop
		return err
	case *ast.CStyleForStmt:
		prev := c.scope
		prevInLoop := c.inLoop
		c.scope = newScope(prev)
		if assign, ok := s.Init.(*ast.Assignment); ok {
			c.scope.define(assign.Name, &ast.Type{Kind: ast.TypeNumber})
		}
		if err := c.checkSemanticStmt(s.Init); err != nil {
			c.scope = prev
			return err
		}
		if err := c.checkSemanticExpr(s.Condition); err != nil {
			c.scope = prev
			return err
		}
		if err := c.checkSemanticStmt(s.Update); err != nil {
			c.scope = prev
			return err
		}
		c.inLoop = true
		err := c.checkSemanticStmts(s.Body.Statements)
		c.scope = prev
		c.inLoop = prevInLoop
		return err
	case *ast.WhileStmt:
		if err := c.checkSemanticExpr(s.Condition); err != nil {
			return err
		}
		prevInLoop := c.inLoop
		c.inLoop = true
		err := c.checkSemanticBlock(s.Body)
		c.inLoop = prevInLoop
		return err
	case *ast.TryStmt:
		if err := c.checkSemanticBlock(s.Body); err != nil {
			return err
		}
		prev := c.scope
		c.scope = newScope(prev)
		if s.CatchVar != "" {
			c.scope.define(s.CatchVar, &ast.Type{Kind: ast.TypeStatus})
		}
		err := c.checkSemanticStmts(s.Catch.Statements)
		c.scope = prev
		return err
	case *ast.SwitchStmt:
		if err := c.checkSemanticExpr(s.Value); err != nil {
			return err
		}
		prevInLoop := c.inLoop
		c.inLoop = true
		defer func() { c.inLoop = prevInLoop }()
		for _, swCase := range s.Cases {
			if !swCase.IsDefault {
				if err := c.checkSemanticExpr(swCase.Value); err != nil {
					return err
				}
			}
			if err := c.checkSemanticBlock(swCase.Body); err != nil {
				return err
			}
		}
	case *ast.ReturnStmt:
		if !c.inFunction && !c.inCallback {
			return &CheckError{Pos: s.Pos, Message: "'return' outside of function"}
		}
		return c.checkSemanticExpr(s.Value)
	case *ast.ExitStmt:
		return c.checkSemanticExpr(s.Code)
	case *ast.ExprStmt:
		return c.checkSemanticExpr(s.Expr)
	}
	return nil
}

func (c *Checker) checkSemanticClassMethod(method *ast.ClassMethod, isConstructor bool) error {
	prev := c.scope
	prevInFunction := c.inFunction
	c.scope = newScope(prev)
	c.inFunction = true
	if !method.IsStatic || isConstructor {
		c.scope.define("this", &ast.Type{Kind: ast.TypeObject})
	}
	for _, param := range method.Params {
		c.scope.define(param.Name, param.Type)
	}
	err := c.checkSemanticStmts(method.Body.Statements)
	c.scope = prev
	c.inFunction = prevInFunction
	return err
}

func (c *Checker) checkSemanticExpr(expr ast.Expression) error {
	if expr == nil {
		return nil
	}
	switch e := expr.(type) {
	case *ast.IntLit, *ast.FloatLit, *ast.StringLit, *ast.BoolLit, *ast.UndefinedLit, *ast.NullLit, *ast.ThisExpr:
		return nil
	case *ast.IdentExpr:
		if isSemanticGlobalIdent(e.Name) {
			return nil
		}
		if _, ok := c.fns[e.Name]; ok {
			return nil
		}
		if _, ok := c.scope.lookup(e.Name); !ok {
			return &CheckError{Pos: e.Pos, Message: fmt.Sprintf("variable %q not declared", e.Name)}
		}
	case *ast.TemplateLit:
		for _, part := range e.Exprs {
			if err := c.checkSemanticExpr(part); err != nil {
				return err
			}
		}
	case *ast.ListLit:
		for _, elem := range e.Elements {
			if err := c.checkSemanticExpr(elem); err != nil {
				return err
			}
		}
	case *ast.ObjectLit:
		for _, field := range e.Fields {
			if err := c.checkSemanticExpr(field.Value); err != nil {
				return err
			}
		}
	case *ast.NewExpr:
		if e.ClassName == "Set" && len(e.Args) != 0 {
			return &CheckError{Pos: e.Pos, Message: "Set constructor takes no runtime arguments"}
		}
		for _, arg := range e.Args {
			if err := c.checkSemanticExpr(arg); err != nil {
				return err
			}
		}
	case *ast.FnCallExpr:
		if _, ok := c.fns[e.Name]; !ok {
			if _, ok := c.scope.lookup(e.Name); !ok {
				return &CheckError{Pos: e.Pos, Message: fmt.Sprintf("function %q not declared", e.Name)}
			}
		}
		for _, arg := range e.Args {
			if err := c.checkSemanticExpr(arg); err != nil {
				return err
			}
		}
	case *ast.BuiltinCallExpr:
		if err := c.checkBuiltinArity(e); err != nil {
			return err
		}
		for _, arg := range e.Args {
			if err := c.checkSemanticExpr(arg); err != nil {
				return err
			}
		}
		if e.Name == "JSON.stringify" {
			return c.validateJSONStringifyArg(e.Pos, e.Args[0])
		}
	case *ast.MethodCallExpr:
		if err := c.checkMethodArity(e); err != nil {
			return err
		}
		if err := c.checkSemanticExpr(e.Receiver); err != nil {
			return err
		}
		for _, arg := range e.Args {
			if err := c.checkSemanticExpr(arg); err != nil {
				return err
			}
		}
	case *ast.PropertyExpr:
		if err := c.checkSemanticExpr(e.Receiver); err != nil {
			return err
		}
		if ident, ok := e.Receiver.(*ast.IdentExpr); ok {
			switch ident.Name {
			case "process":
				if e.Property != "env" {
					return &CheckError{Pos: e.Pos, Message: fmt.Sprintf("process has no property %q", e.Property)}
				}
			case "Besht":
				switch e.Property {
				case "fs", "strings", "args", "iter":
				default:
					return &CheckError{Pos: e.Pos, Message: fmt.Sprintf("Besht has no property %q", e.Property)}
				}
			}
		}
		if recvType := c.semanticExprType(e.Receiver); recvType != nil && recvType.Kind == ast.TypeFetchResponse {
			return &CheckError{Pos: e.Pos, Message: fmt.Sprintf("FetchResponse has no property %q; status, ok, headers, json(), and body are not supported yet", e.Property)}
		}
		return nil
	case *ast.IndexExpr:
		if err := c.checkSemanticExpr(e.Expr); err != nil {
			return err
		}
		return c.checkSemanticExpr(e.Index)
	case *ast.BinaryExpr:
		if err := c.checkSemanticExpr(e.Left); err != nil {
			return err
		}
		return c.checkSemanticExpr(e.Right)
	case *ast.TernaryExpr:
		if err := c.checkSemanticExpr(e.Condition); err != nil {
			return err
		}
		if err := c.checkSemanticExpr(e.Then); err != nil {
			return err
		}
		return c.checkSemanticExpr(e.Else)
	case *ast.UnaryExpr:
		return c.checkSemanticExpr(e.Expr)
	case *ast.UpdateExpr:
		if c.consts[e.Name] {
			return &CheckError{Pos: e.Pos, Message: fmt.Sprintf("cannot assign to const %q", e.Name)}
		}
		if _, ok := c.scope.lookup(e.Name); !ok {
			return &CheckError{Pos: e.Pos, Message: fmt.Sprintf("variable %q not declared", e.Name)}
		}
	case *ast.CmdExpr:
		for i, arg := range e.Args {
			if spread, ok := arg.(*ast.SpreadExpr); ok && i == 0 && len(e.Args) > 1 {
				return &CheckError{Pos: spread.Pos, Message: "command-name spread must be the only $() argument"}
			}
		}
		for _, arg := range e.Args {
			if err := c.checkSemanticExpr(arg); err != nil {
				return err
			}
		}
	case *ast.PipeExpr:
		if err := c.checkSemanticExpr(e.Left); err != nil {
			return err
		}
		return c.checkSemanticExpr(e.Right)
	case *ast.PropagateExpr:
		return c.checkSemanticExpr(e.Expr)
	case *ast.ArrowExpr:
		prev := c.scope
		prevInFunction := c.inFunction
		prevInCallback := c.inCallback
		c.scope = newScope(prev)
		c.inFunction = true
		c.inCallback = true
		for _, param := range e.Params {
			c.scope.define(param.Name, param.Type)
		}
		var err error
		if e.Body != nil {
			err = c.checkSemanticExpr(e.Body)
		} else {
			err = c.checkSemanticBlock(e.BlockBody)
		}
		c.scope = prev
		c.inFunction = prevInFunction
		c.inCallback = prevInCallback
		return err
	case *ast.SpreadExpr:
		return c.checkSemanticExpr(e.Expr)
	case *ast.AsExpr:
		return c.checkSemanticExpr(e.Expr)
	case *ast.RangeExpr:
		if err := c.checkSemanticExpr(e.Start); err != nil {
			return err
		}
		return c.checkSemanticExpr(e.End)
	}
	return nil
}

func isSemanticGlobalIdent(name string) bool {
	switch name {
	case "Besht", "process", "Math", "Number", "Object", "Array", "Boolean", "JSON", "console", "Set":
		return true
	}
	return false
}

func (c *Checker) checkBuiltinArity(e *ast.BuiltinCallExpr) error {
	switch e.Name {
	case "fetch":
		if len(e.Args) != 1 {
			return &CheckError{Pos: e.Pos, Message: "fetch() takes 1 URL argument; options are not supported yet"}
		}
	case "Boolean", "Array.isArray":
		if len(e.Args) != 1 {
			return &CheckError{Pos: e.Pos, Message: fmt.Sprintf("%s() takes 1 argument", e.Name)}
		}
	case "Number.parseInt", "Number.parseFloat":
		if len(e.Args) < 1 || len(e.Args) > 2 {
			return &CheckError{Pos: e.Pos, Message: e.Name + "() takes 1 or 2 arguments"}
		}
	case "Number.isFinite", "Number.isInteger", "Number.isSafeInteger", "Number.isNaN":
		if len(e.Args) != 1 {
			return &CheckError{Pos: e.Pos, Message: e.Name + "() takes 1 argument"}
		}
	case "Array.from":
		if len(e.Args) != 1 {
			return &CheckError{Pos: e.Pos, Message: "Array.from() takes 1 argument"}
		}
	case "Object.keys", "Object.values", "Object.entries":
		if len(e.Args) != 1 {
			return &CheckError{Pos: e.Pos, Message: e.Name + "() takes 1 argument"}
		}
	case "Object.hasOwn":
		if len(e.Args) != 2 {
			return &CheckError{Pos: e.Pos, Message: "Object.hasOwn() takes 2 arguments"}
		}
	case "JSON.stringify":
		if len(e.Args) != 1 {
			return &CheckError{Pos: e.Pos, Message: "JSON.stringify() takes 1 argument"}
		}
	}
	return nil
}

func (c *Checker) checkMethodArity(e *ast.MethodCallExpr) error {
	if builtinName, ok := ast.BeshtMethodBuiltinName(e.Receiver, e.Method); ok {
		want := 1
		if builtinName == "Besht.iter.range" {
			want = 2
		}
		if len(e.Args) != want {
			return &CheckError{Pos: e.Pos, Message: fmt.Sprintf("%s.%s() takes %d argument%s", beshtReceiverName(e.Receiver), e.Method, want, pluralSuffix(want))}
		}
		return nil
	}
	if ast.IsBeshtArgsReceiver(e.Receiver) {
		switch e.Method {
		case "argv":
			if len(e.Args) != 0 {
				return &CheckError{Pos: e.Pos, Message: "Besht.args.argv() takes no arguments"}
			}
		case "positional":
			if len(e.Args) != 1 {
				return &CheckError{Pos: e.Pos, Message: "Besht.args.positional() takes 1 argument"}
			}
		case "option":
			if len(e.Args) < 1 || len(e.Args) > 2 {
				return &CheckError{Pos: e.Pos, Message: "Besht.args.option() takes 1 or 2 arguments"}
			}
		case "flag":
			if len(e.Args) < 1 || len(e.Args) > 2 {
				return &CheckError{Pos: e.Pos, Message: "Besht.args.flag() takes 1 or 2 arguments"}
			}
		default:
			return &CheckError{Pos: e.Pos, Message: fmt.Sprintf("Besht.args has no method %q", e.Method)}
		}
		return nil
	}
	if group, ok := ast.BeshtGroupReceiver(e.Receiver); ok {
		return &CheckError{Pos: e.Pos, Message: fmt.Sprintf("Besht.%s has no method %q", group, e.Method)}
	}
	if ident, ok := e.Receiver.(*ast.IdentExpr); ok && ident.Name == "Math" {
		return c.checkMathMethodArity(e)
	}
	if ident, ok := e.Receiver.(*ast.IdentExpr); ok && ident.Name == "process" {
		if e.Method != "exit" {
			return &CheckError{Pos: e.Pos, Message: fmt.Sprintf("process has no method %q", e.Method)}
		}
		if len(e.Args) > 1 {
			return &CheckError{Pos: e.Pos, Message: "process.exit() takes 0 or 1 argument"}
		}
		return nil
	}
	recvType := c.semanticExprType(e.Receiver)
	if recvType == nil {
		return nil
	}
	switch recvType.Kind {
	case ast.TypeCommand:
		return c.checkCommandMethodArity(e)
	case ast.TypeFetchResponse:
		if e.Method != "text" {
			return &CheckError{Pos: e.Pos, Message: fmt.Sprintf("FetchResponse has no method %q; this fetch() slice only supports text()", e.Method)}
		}
		if len(e.Args) != 0 {
			return &CheckError{Pos: e.Pos, Message: "FetchResponse.text() takes no arguments"}
		}
	case ast.TypeSet:
		switch e.Method {
		case "has", "add":
			if len(e.Args) != 1 {
				return &CheckError{Pos: e.Pos, Message: fmt.Sprintf("Set.%s() takes 1 argument", e.Method)}
			}
		default:
			return &CheckError{Pos: e.Pos, Message: fmt.Sprintf("Set has no method %q", e.Method)}
		}
	case ast.TypeList:
		return c.checkListMethodArity(e)
	case ast.TypeString:
		return c.checkStringMethodArity(e)
	case ast.TypeNumber:
		return c.checkNumberMethodArity(e)
	case ast.TypeBoolean, ast.TypeStatus:
		if e.Method != "toString" {
			return &CheckError{Pos: e.Pos, Message: fmt.Sprintf("type %s has no methods", recvType)}
		}
		if len(e.Args) != 0 {
			return &CheckError{Pos: e.Pos, Message: "toString() takes no arguments"}
		}
	}
	return nil
}

func (c *Checker) checkMathMethodArity(e *ast.MethodCallExpr) error {
	want := 1
	switch e.Method {
	case "min", "max", "pow":
		want = 2
	case "round", "floor", "ceil", "abs", "sqrt", "trunc", "sign":
	default:
		return &CheckError{Pos: e.Pos, Message: fmt.Sprintf("Math has no method %q", e.Method)}
	}
	if len(e.Args) != want {
		return &CheckError{Pos: e.Pos, Message: fmt.Sprintf("Math.%s() takes %d argument(s)", e.Method, want)}
	}
	return nil
}

func (c *Checker) checkCommandMethodArity(e *ast.MethodCallExpr) error {
	switch e.Method {
	case "pipe":
		if len(e.Args) != 1 {
			return &CheckError{Pos: e.Pos, Message: "pipe() takes 1 argument"}
		}
	case "run", "readStdout", "readStdoutLines", "readStderr", "exitCode", "clone":
		if len(e.Args) != 0 {
			return &CheckError{Pos: e.Pos, Message: e.Method + "() takes no arguments"}
		}
	case "stdout":
		if len(e.Args) < 1 || len(e.Args) > 2 {
			return &CheckError{Pos: e.Pos, Message: "stdout() takes 1 or 2 arguments: (path[, \"append\"])"}
		}
	case "stderr":
		if len(e.Args) != 1 {
			return &CheckError{Pos: e.Pos, Message: "stderr() takes 1 argument: \"null\", \"&1\", or a file path"}
		}
	case "workdir":
		if len(e.Args) != 1 {
			return &CheckError{Pos: e.Pos, Message: "workdir() takes 1 argument: path"}
		}
	case "env":
		if len(e.Args) != 2 {
			return &CheckError{Pos: e.Pos, Message: "env() on command takes 2 arguments: (name, value)"}
		}
		name, err := commandEnvName(e.Args[0])
		if err != nil {
			return &CheckError{Pos: e.Pos, Message: err.Error()}
		}
		if !isShellIdentifier(name) {
			return &CheckError{Pos: e.Pos, Message: fmt.Sprintf("invalid command env name %q", name)}
		}
	default:
		return &CheckError{Pos: e.Pos, Message: fmt.Sprintf("command has no method %q (available: run, pipe, readStdout, readStdoutLines, readStderr, exitCode, stdout, stderr, workdir, env, clone)", e.Method)}
	}
	return nil
}

func (c *Checker) checkListMethodArity(e *ast.MethodCallExpr) error {
	switch e.Method {
	case "push", "unshift", "includes", "indexOf", "lastIndexOf", "join":
		if len(e.Args) != 1 {
			return &CheckError{Pos: e.Pos, Message: e.Method + "() takes 1 argument"}
		}
	case "pop", "shift", "reverse", "toString":
		if len(e.Args) != 0 {
			if e.Method == "toString" {
				return &CheckError{Pos: e.Pos, Message: "toString() takes no arguments"}
			}
			return &CheckError{Pos: e.Pos, Message: e.Method + "() takes no arguments"}
		}
	case "concat":
		if len(e.Args) != 1 {
			return &CheckError{Pos: e.Pos, Message: "concat() takes 1 argument"}
		}
	case "slice":
		if len(e.Args) < 1 || len(e.Args) > 2 {
			return &CheckError{Pos: e.Pos, Message: "slice() takes 1 or 2 arguments"}
		}
	case "map", "filter", "some", "every", "find", "findIndex":
		if len(e.Args) != 1 {
			return &CheckError{Pos: e.Pos, Message: e.Method + "() takes 1 arrow callback"}
		}
		arrow, ok := callbackArrowExpr(e.Args[0])
		if !ok && !c.isCallbackValue(e.Args[0]) {
			return &CheckError{Pos: e.Pos, Message: "list callback must be an arrow expression"}
		}
		if ok {
			if len(arrow.Params) < 1 || len(arrow.Params) > 2 {
				return &CheckError{Pos: arrow.Pos, Message: "arrow callbacks take 1 or 2 parameters"}
			}
			if (e.Method == "some" || e.Method == "every" || e.Method == "find") && arrow.BlockBody != nil {
				return &CheckError{Pos: arrow.Pos, Message: e.Method + "() predicate callback must be expression-bodied"}
			}
		}
	case "forEach":
		if len(e.Args) != 1 {
			return &CheckError{Pos: e.Pos, Message: "forEach() takes 1 arrow callback"}
		}
		arrow, ok := callbackArrowExpr(e.Args[0])
		if !ok && !c.isCallbackValue(e.Args[0]) {
			return &CheckError{Pos: e.Pos, Message: "forEach() callback must be an arrow expression"}
		}
		if ok {
			if len(arrow.Params) < 1 || len(arrow.Params) > 2 {
				return &CheckError{Pos: arrow.Pos, Message: "arrow callbacks take 1 or 2 parameters"}
			}
		}
	case "reduce":
		if len(e.Args) != 2 {
			return &CheckError{Pos: e.Pos, Message: "reduce() takes 2 arguments: callback and initial value"}
		}
		arrow, ok := callbackArrowExpr(e.Args[0])
		if !ok && !c.isCallbackValue(e.Args[0]) {
			return &CheckError{Pos: e.Pos, Message: "reduce() callback must be an arrow expression"}
		}
		if ok {
			if len(arrow.Params) != 2 {
				return &CheckError{Pos: arrow.Pos, Message: "reduce() callback must take 2 parameters (accumulator, current)"}
			}
		}
	default:
		return &CheckError{Pos: e.Pos, Message: fmt.Sprintf("list has no method %q", e.Method)}
	}
	return nil
}

func callbackArrowExpr(expr ast.Expression) (*ast.ArrowExpr, bool) {
	switch e := expr.(type) {
	case *ast.ArrowExpr:
		return e, true
	case *ast.AsExpr:
		return callbackArrowExpr(e.Expr)
	default:
		return nil, false
	}
}

func (c *Checker) isCallbackValue(expr ast.Expression) bool {
	switch e := expr.(type) {
	case *ast.AsExpr:
		return c.isCallbackValue(e.Expr)
	case *ast.IdentExpr:
		if _, ok := c.fns[e.Name]; ok {
			return true
		}
		_, ok := c.scope.lookup(e.Name)
		return ok
	default:
		t := c.semanticExprType(expr)
		return t != nil && t.Kind == ast.TypeFunction
	}
}

func (c *Checker) checkStringMethodArity(e *ast.MethodCallExpr) error {
	switch e.Method {
	case "toString", "trim", "trimStart", "trimEnd", "toUpperCase", "toLowerCase":
		if len(e.Args) != 0 {
			if e.Method == "toString" {
				return &CheckError{Pos: e.Pos, Message: "toString() takes no arguments"}
			}
			return &CheckError{Pos: e.Pos, Message: e.Method + "() takes no arguments"}
		}
	case "split", "repeat", "at", "charAt":
		if len(e.Args) != 1 {
			return &CheckError{Pos: e.Pos, Message: e.Method + "() takes 1 argument"}
		}
	case "includes", "startsWith", "endsWith", "indexOf", "lastIndexOf", "slice", "substring", "padStart", "padEnd":
		if len(e.Args) < 1 || len(e.Args) > 2 {
			return &CheckError{Pos: e.Pos, Message: e.Method + "() takes 1 or 2 arguments"}
		}
	case "replace", "replaceAll":
		if len(e.Args) != 2 {
			return &CheckError{Pos: e.Pos, Message: e.Method + "() takes 2 arguments"}
		}
	case "concat":
		if len(e.Args) < 1 {
			return &CheckError{Pos: e.Pos, Message: "concat() takes at least 1 argument"}
		}
	default:
		return &CheckError{Pos: e.Pos, Message: fmt.Sprintf("string has no method %q", e.Method)}
	}
	return nil
}

func (c *Checker) checkNumberMethodArity(e *ast.MethodCallExpr) error {
	switch e.Method {
	case "toString":
		if len(e.Args) != 0 {
			return &CheckError{Pos: e.Pos, Message: "toString() takes no arguments"}
		}
	case "toFixed":
		if len(e.Args) > 1 {
			return &CheckError{Pos: e.Pos, Message: "toFixed() takes 0 or 1 arguments"}
		}
	default:
		return &CheckError{Pos: e.Pos, Message: fmt.Sprintf("number has no method %q", e.Method)}
	}
	return nil
}

func (c *Checker) semanticExprType(expr ast.Expression) *ast.Type {
	strType := &ast.Type{Kind: ast.TypeString}
	numType := &ast.Type{Kind: ast.TypeNumber}
	boolType := &ast.Type{Kind: ast.TypeBoolean}
	voidType := &ast.Type{Kind: ast.TypeVoid}
	listStrType := &ast.Type{Kind: ast.TypeList, Elem: strType}

	switch e := expr.(type) {
	case nil:
		return nil
	case *ast.AsExpr:
		if e.Type != nil {
			return e.Type
		}
		return c.semanticExprType(e.Expr)
	case *ast.IntLit, *ast.FloatLit, *ast.UpdateExpr:
		return numType
	case *ast.StringLit, *ast.TemplateLit:
		return strType
	case *ast.BoolLit:
		return boolType
	case *ast.NullLit, *ast.UndefinedLit:
		return strType
	case *ast.IdentExpr:
		if typ, ok := c.scope.lookup(e.Name); ok {
			return typ
		}
		if sig, ok := c.fns[e.Name]; ok {
			return &ast.Type{Kind: ast.TypeFunction, Return: sig.ReturnType}
		}
		return strType
	case *ast.ListLit:
		elemType := strType
		if len(e.Elements) > 0 {
			elemType = c.semanticExprType(e.Elements[0])
			if elemType == nil {
				elemType = strType
			}
		}
		return &ast.Type{Kind: ast.TypeList, Elem: elemType}
	case *ast.ObjectLit, *ast.ThisExpr:
		return &ast.Type{Kind: ast.TypeObject}
	case *ast.NewExpr:
		if e.ClassName == "Set" {
			elemType := strType
			if len(e.TypeArgs) > 0 && e.TypeArgs[0] != nil {
				elemType = e.TypeArgs[0]
			}
			return &ast.Type{Kind: ast.TypeSet, Elem: elemType}
		}
		return &ast.Type{Kind: ast.TypeObject}
	case *ast.CmdExpr, *ast.PipeExpr:
		return &ast.Type{Kind: ast.TypeCommand}
	case *ast.PropagateExpr:
		return c.semanticExprType(e.Expr)
	case *ast.FnCallExpr:
		if sig, ok := c.fns[e.Name]; ok && sig.ReturnType != nil {
			return sig.ReturnType
		}
		if typ, ok := c.scope.lookup(e.Name); ok && typ != nil && typ.Kind == ast.TypeFunction {
			return typ.Return
		}
		return strType
	case *ast.BuiltinCallExpr:
		switch e.Name {
		case "fetch":
			return &ast.Type{Kind: ast.TypeFetchResponse}
		case "Boolean", "Array.isArray", "Object.hasOwn", "Number.isFinite", "Number.isInteger", "Number.isSafeInteger", "Number.isNaN":
			return boolType
		case "Number.parseInt", "Number.parseFloat":
			return numType
		case "Array.from", "Array.of", "Object.keys", "Object.values":
			return listStrType
		case "Object.entries":
			return &ast.Type{Kind: ast.TypeList, Elem: listStrType}
		case "JSON.stringify":
			return strType
		case "console.log", "console.error":
			return voidType
		}
		return strType
	case *ast.MethodCallExpr:
		if builtinName, ok := ast.BeshtMethodBuiltinName(e.Receiver, e.Method); ok {
			if builtinName == "Besht.iter.range" {
				return &ast.Type{Kind: ast.TypeList, Elem: numType}
			}
			return boolType
		}
		if ast.IsBeshtArgsReceiver(e.Receiver) {
			switch e.Method {
			case "argv":
				return listStrType
			case "flag":
				return boolType
			case "positional", "option":
				return strType
			}
		}
		if ident, ok := e.Receiver.(*ast.IdentExpr); ok {
			if ident.Name == "Math" {
				return numType
			}
			if ident.Name == "process" && e.Method == "exit" {
				return voidType
			}
		}
		recvType := c.semanticExprType(e.Receiver)
		if recvType == nil {
			return strType
		}
		switch recvType.Kind {
		case ast.TypeCommand:
			switch e.Method {
			case "readStdout", "readStderr":
				return strType
			case "readStdoutLines":
				return listStrType
			case "exitCode":
				return numType
			default:
				return recvType
			}
		case ast.TypeFetchResponse:
			if e.Method == "text" {
				return strType
			}
		case ast.TypeSet:
			if e.Method == "has" {
				return boolType
			}
			if e.Method == "add" {
				return voidType
			}
		case ast.TypeList:
			switch e.Method {
			case "length":
				return numType
			case "join", "toString":
				return strType
			case "includes":
				return boolType
			case "indexOf", "lastIndexOf", "findIndex":
				return numType
			case "some", "every":
				return boolType
			case "find":
				return recvType.Elem
			case "map":
				if len(e.Args) == 1 {
					if t := c.callbackReturnType(e.Args[0]); t != nil && t.Kind != ast.TypeVoid {
						return &ast.Type{Kind: ast.TypeList, Elem: t}
					}
				}
				return &ast.Type{Kind: ast.TypeList, Elem: strType}
			case "pop", "shift", "push", "unshift", "concat", "slice", "filter", "reverse":
				return recvType
			case "reduce":
				if len(e.Args) == 2 {
					return c.semanticExprType(e.Args[1])
				}
			}
		case ast.TypeString:
			switch e.Method {
			case "includes", "startsWith", "endsWith":
				return boolType
			case "indexOf", "lastIndexOf":
				return numType
			case "split":
				return listStrType
			default:
				return strType
			}
		case ast.TypeNumber:
			if e.Method == "toString" || e.Method == "toFixed" {
				return strType
			}
			return numType
		case ast.TypeBoolean, ast.TypeStatus:
			if e.Method == "toString" {
				return strType
			}
		}
		return strType
	case *ast.PropertyExpr:
		if ident, ok := e.Receiver.(*ast.IdentExpr); ok && ident.Name == "process" && e.Property == "env" {
			return &ast.Type{Kind: ast.TypeObject}
		}
		recvType := c.semanticExprType(e.Receiver)
		if recvType != nil && (recvType.Kind == ast.TypeList || recvType.Kind == ast.TypeString) && e.Property == "length" {
			return numType
		}
		return nil
	case *ast.IndexExpr:
		recvType := c.semanticExprType(e.Expr)
		if recvType != nil && recvType.Kind == ast.TypeList && recvType.Elem != nil {
			return recvType.Elem
		}
		return strType
	case *ast.BinaryExpr:
		switch e.Op {
		case "==", "!=", "===", "!==", "<", ">", "<=", ">=", "&&", "||":
			return boolType
		case "-", "*", "/", "%":
			return numType
		case "??":
			if left := c.semanticExprType(e.Left); left != nil {
				return left
			}
			return c.semanticExprType(e.Right)
		case "+":
			left := c.semanticExprType(e.Left)
			right := c.semanticExprType(e.Right)
			if left != nil && right != nil && left.Kind == ast.TypeNumber && right.Kind == ast.TypeNumber {
				return numType
			}
			return strType
		}
	case *ast.TernaryExpr:
		return c.semanticExprType(e.Then)
	case *ast.UnaryExpr:
		if e.Op == "!" {
			return boolType
		}
		return c.semanticExprType(e.Expr)
	case *ast.ArrowExpr:
		var params []*ast.Type
		for _, param := range e.Params {
			if param.Type != nil {
				params = append(params, param.Type)
			} else {
				params = append(params, strType)
			}
		}
		return &ast.Type{Kind: ast.TypeFunction, Params: params, Return: c.arrowReturnType(e)}
	case *ast.SpreadExpr:
		return c.semanticExprType(e.Expr)
	case *ast.RangeExpr:
		return &ast.Type{Kind: ast.TypeList, Elem: numType}
	}
	return strType
}

func (c *Checker) callbackReturnType(expr ast.Expression) *ast.Type {
	switch e := expr.(type) {
	case *ast.AsExpr:
		if e.Type != nil && e.Type.Kind == ast.TypeFunction {
			return e.Type.Return
		}
		return c.callbackReturnType(e.Expr)
	case *ast.ArrowExpr:
		return c.arrowReturnType(e)
	case *ast.IdentExpr:
		if sig, ok := c.fns[e.Name]; ok {
			return sig.ReturnType
		}
		if typ, ok := c.scope.lookup(e.Name); ok && typ != nil && typ.Kind == ast.TypeFunction {
			return typ.Return
		}
	default:
		if typ := c.semanticExprType(expr); typ != nil && typ.Kind == ast.TypeFunction {
			return typ.Return
		}
	}
	return nil
}

func (c *Checker) arrowReturnType(e *ast.ArrowExpr) *ast.Type {
	if e == nil {
		return &ast.Type{Kind: ast.TypeString}
	}
	if e.ReturnType != nil {
		return e.ReturnType
	}
	if e.Body != nil {
		if t := c.semanticExprType(e.Body); t != nil {
			return t
		}
		return &ast.Type{Kind: ast.TypeString}
	}
	if t := c.blockReturnType(e.BlockBody); t != nil {
		return t
	}
	return &ast.Type{Kind: ast.TypeVoid}
}

func (c *Checker) blockReturnType(block *ast.Block) *ast.Type {
	if block == nil {
		return nil
	}
	for _, stmt := range block.Statements {
		switch s := stmt.(type) {
		case *ast.ReturnStmt:
			if s.Value == nil {
				return &ast.Type{Kind: ast.TypeVoid}
			}
			return c.semanticExprType(s.Value)
		case *ast.IfStmt:
			if t := c.blockReturnType(s.Then); t != nil {
				return t
			}
			for _, elseif := range s.ElseIfs {
				if t := c.blockReturnType(elseif.Body); t != nil {
					return t
				}
			}
			if t := c.blockReturnType(s.Else); t != nil {
				return t
			}
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
			for _, accessor := range fn.Accessors {
				retType := accessor.ReturnType
				if accessor.Kind == ast.AccessorSet || retType == nil {
					retType = &ast.Type{Kind: ast.TypeVoid}
				}
				fns[fn.Name+"__"+string(accessor.Kind)+"_"+accessor.Name] = &FnSig{Params: accessor.Params, ReturnType: retType}
			}
		}
	}
}

func classBodyMutatesThis(stmts []ast.Statement) bool {
	for _, stmt := range stmts {
		switch s := stmt.(type) {
		case *ast.PropertyAssignStmt:
			if s.Object == "this" {
				return true
			}
		case *ast.Block:
			if classBodyMutatesThis(s.Statements) {
				return true
			}
		case *ast.IfStmt:
			if classBodyMutatesThis(s.Then.Statements) || (s.Else != nil && classBodyMutatesThis(s.Else.Statements)) {
				return true
			}
			for _, ei := range s.ElseIfs {
				if classBodyMutatesThis(ei.Body.Statements) {
					return true
				}
			}
		case *ast.ForStmt:
			if classBodyMutatesThis(s.Body.Statements) {
				return true
			}
		case *ast.CStyleForStmt:
			if classStmtMutatesThis(s.Init) || classStmtMutatesThis(s.Update) || classBodyMutatesThis(s.Body.Statements) {
				return true
			}
		case *ast.WhileStmt:
			if classBodyMutatesThis(s.Body.Statements) {
				return true
			}
		case *ast.TryStmt:
			if classBodyMutatesThis(s.Body.Statements) || classBodyMutatesThis(s.Catch.Statements) {
				return true
			}
		case *ast.SwitchStmt:
			for _, swCase := range s.Cases {
				if classBodyMutatesThis(swCase.Body.Statements) {
					return true
				}
			}
		}
	}
	return false
}

func classStmtMutatesThis(stmt ast.Statement) bool {
	if stmt == nil {
		return false
	}
	return classBodyMutatesThis([]ast.Statement{stmt})
}

func semanticBodyReturnsValue(stmts []ast.Statement) bool {
	for _, stmt := range stmts {
		switch s := stmt.(type) {
		case *ast.ReturnStmt:
			if s.Value != nil {
				return true
			}
		case *ast.Block:
			if semanticBodyReturnsValue(s.Statements) {
				return true
			}
		case *ast.IfStmt:
			if semanticBodyReturnsValue(s.Then.Statements) || (s.Else != nil && semanticBodyReturnsValue(s.Else.Statements)) {
				return true
			}
			for _, ei := range s.ElseIfs {
				if semanticBodyReturnsValue(ei.Body.Statements) {
					return true
				}
			}
		case *ast.ForStmt:
			if semanticBodyReturnsValue(s.Body.Statements) {
				return true
			}
		case *ast.CStyleForStmt:
			if semanticStmtReturnsValue(s.Init) || semanticStmtReturnsValue(s.Update) || semanticBodyReturnsValue(s.Body.Statements) {
				return true
			}
		case *ast.WhileStmt:
			if semanticBodyReturnsValue(s.Body.Statements) {
				return true
			}
		case *ast.TryStmt:
			if semanticBodyReturnsValue(s.Body.Statements) || semanticBodyReturnsValue(s.Catch.Statements) {
				return true
			}
		case *ast.SwitchStmt:
			for _, swCase := range s.Cases {
				if semanticBodyReturnsValue(swCase.Body.Statements) {
					return true
				}
			}
		}
	}
	return false
}

func semanticStmtReturnsValue(stmt ast.Statement) bool {
	if stmt == nil {
		return false
	}
	return semanticBodyReturnsValue([]ast.Statement{stmt})
}

func beshtReceiverName(expr ast.Expression) string {
	if group, ok := ast.BeshtGroupReceiver(expr); ok {
		return "Besht." + group
	}
	return "Besht"
}

func (c *Checker) validateJSONStringifyArg(pos ast.Pos, expr ast.Expression) error {
	if obj, ok := unwrapJSONStringifyAs(expr).(*ast.ObjectLit); ok {
		for _, field := range obj.Fields {
			if unsupportedJSONStringifyObjectValue(c.semanticExprType(field.Value)) {
				return &CheckError{Pos: pos, Message: "JSON.stringify() only supports scalar object values"}
			}
		}
	}
	typ := c.semanticExprType(expr)
	if !isJSONStringifyType(typ) {
		return &CheckError{Pos: pos, Message: fmt.Sprintf("JSON.stringify() cannot encode %s", typ)}
	}
	return nil
}

func unwrapJSONStringifyAs(expr ast.Expression) ast.Expression {
	for {
		as, ok := expr.(*ast.AsExpr)
		if !ok {
			return expr
		}
		expr = as.Expr
	}
}

func isJSONStringifyType(t *ast.Type) bool {
	if t == nil {
		return true
	}
	switch t.Kind {
	case ast.TypeString, ast.TypeNumber, ast.TypeBoolean, ast.TypeObject:
		return true
	case ast.TypeList:
		if t.Elem == nil {
			return true
		}
		return isJSONStringifyScalarType(t.Elem)
	}
	return false
}

func unsupportedJSONStringifyObjectValue(t *ast.Type) bool {
	return !isJSONStringifyScalarType(t)
}

func isJSONStringifyScalarType(t *ast.Type) bool {
	if t == nil {
		return true
	}
	switch t.Kind {
	case ast.TypeString, ast.TypeNumber, ast.TypeBoolean:
		return true
	}
	return false
}

func pluralSuffix(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func validateObjectKey(key string) error {
	if key == "" {
		return fmt.Errorf("object keys must not be empty")
	}
	for _, r := range key {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			continue
		}
		return fmt.Errorf("object key %q is not supported; keys must match [A-Za-z0-9_]", key)
	}
	return nil
}

func isProcessEnvExpr(expr ast.Expression) bool {
	prop, ok := expr.(*ast.PropertyExpr)
	if !ok || prop.Property != "env" {
		return false
	}
	ident, ok := prop.Receiver.(*ast.IdentExpr)
	return ok && ident.Name == "process"
}

func commandEnvName(expr ast.Expression) (string, error) {
	var name string
	switch e := expr.(type) {
	case *ast.StringLit:
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
