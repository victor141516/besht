package codegen

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/victor141516/besht/internal/ast"
)

const nullishSentinelVar = "_BESHT_NULLISH_SENTINEL"

type Generator struct {
	sb             strings.Builder
	indent         int
	currentFn      string
	fnReturnMap    map[string]*ast.Type
	varTypeMap     map[string]*ast.Type // codegen-level var → inferred type (list detection)
	inFunction     bool
	inLoop         bool
	topLevel       bool
	paramMap       map[string]string // besht param name → mangled shell var name
	globalVarMap   map[string]string // top-level/imported besht name → shell var name
	objAliasMap    map[string]objectRef
	objFieldsMap   map[string][]string  // object var name → list of field names
	objPropTypeMap map[string]*ast.Type // "varName.propName" → property type
	stringConstMap map[string]string
	staticListMap  map[string][]string
	fnParamTypes   map[string]*ast.Type // current fn param name → type annotation
	fnParamNames   map[string][]string  // function name → param names (in order)
	classMap       map[string]*ast.ClassDecl
	varClassMap    map[string]string
	currentClass   string
	currentThisVar string
	floatVars      map[string]bool
	listLenMap     map[string]string
	runtimeHelpers map[string]bool
	argsOptions    map[string]bool
	argsFlags      map[string]bool
	NoCheck        bool
	NoSourceMap    bool
	cmdAnalysis    *CmdAnalysis
	cmdScope       *cmdScope
	cmdChains      map[string]ast.Expression
	reduceReturns  []reduceReturnContext
	mapReturns     []mapReturnContext
}

type reduceReturnContext struct {
	accVar      string
	accParam    string
	accIsObject bool
}

type mapReturnContext struct {
	indexVar string
}

type objectRef struct {
	StaticName              string
	SlotExpr                string
	RootName                string
	UnsupportedScalarValues bool
}

func cloneObjectRefMap(in map[string]objectRef) map[string]objectRef {
	out := make(map[string]objectRef, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneObjectFieldsMap(in map[string][]string) map[string][]string {
	out := make(map[string][]string, len(in))
	for k, v := range in {
		out[k] = append([]string(nil), v...)
	}
	return out
}

func cloneTypeMap(in map[string]*ast.Type) map[string]*ast.Type {
	out := make(map[string]*ast.Type, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneStringMap(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

type callbackLoopCtx struct {
	ParamVars   []string
	ParamByName map[string]string
}

type callbackBinding struct {
	name     string
	old      string
	hadOld   bool
	oldAlias objectRef
	hadAlias bool
}

type Options struct {
	NoCheck                   bool
	NoSourceMap               bool
	ResolveTsImports          bool
	AllowExternalShellImports bool
}

func New() *Generator {
	return &Generator{
		fnReturnMap:    make(map[string]*ast.Type),
		varTypeMap:     make(map[string]*ast.Type),
		paramMap:       make(map[string]string),
		globalVarMap:   make(map[string]string),
		objAliasMap:    make(map[string]objectRef),
		objFieldsMap:   make(map[string][]string),
		objPropTypeMap: make(map[string]*ast.Type),
		stringConstMap: make(map[string]string),
		staticListMap:  make(map[string][]string),
		fnParamTypes:   make(map[string]*ast.Type),
		fnParamNames:   make(map[string][]string),
		classMap:       make(map[string]*ast.ClassDecl),
		varClassMap:    make(map[string]string),
		floatVars:      make(map[string]bool),
		listLenMap:     make(map[string]string),
		runtimeHelpers: make(map[string]bool),
		argsOptions:    make(map[string]bool),
		argsFlags:      make(map[string]bool),
		cmdScope:       newCmdScope(nil),
		cmdChains:      make(map[string]ast.Expression),
		topLevel:       true,
	}
}

func Generate(prog *ast.Program) (string, error) {
	g := New()
	return g.generate(prog)
}

func nullishSentinelRef() string {
	return "\"$" + nullishSentinelVar + "\""
}

func isProcessEnvObject(expr ast.Expression) bool {
	prop, ok := expr.(*ast.PropertyExpr)
	if !ok || prop.Property != "env" {
		return false
	}
	ident, ok := prop.Receiver.(*ast.IdentExpr)
	return ok && ident.Name == "process"
}

func isProcessEnvProperty(expr ast.Expression) (*ast.PropertyExpr, bool) {
	prop, ok := expr.(*ast.PropertyExpr)
	if !ok || !isProcessEnvObject(prop.Receiver) {
		return nil, false
	}
	return prop, true
}

func (g *Generator) collectArgsSchema(stmts []ast.Statement) {
	walkArgsStatements(stmts, g.collectArgsSchemaExpr)
}

func statementsUseArgs(stmts []ast.Statement) bool {
	found := false
	walkArgsStatements(stmts, func(expr ast.Expression) {
		if found {
			return
		}
		if isArgsMethodCall(expr) {
			found = true
		}
	})
	return found
}

func walkArgsStatements(stmts []ast.Statement, visit func(ast.Expression)) {
	for _, stmt := range stmts {
		walkArgsStmt(stmt, visit)
	}
}

func walkArgsStmt(stmt ast.Statement, visit func(ast.Expression)) {
	switch s := stmt.(type) {
	case nil:
		return
	case *ast.ImportDecl:
		return
	case *ast.LetDecl:
		walkArgsExpr(s.Value, visit)
	case *ast.DestructureDecl:
		walkArgsExpr(s.Value, visit)
	case *ast.Assignment:
		walkArgsExpr(s.Value, visit)
	case *ast.IndexAssignStmt:
		walkArgsExpr(s.Index, visit)
		walkArgsExpr(s.Value, visit)
	case *ast.PropertyAssignStmt:
		walkArgsExpr(s.Value, visit)
	case *ast.ExprStmt:
		walkArgsExpr(s.Expr, visit)
	case *ast.ReturnStmt:
		walkArgsExpr(s.Value, visit)
	case *ast.ExitStmt:
		walkArgsExpr(s.Code, visit)
	case *ast.IfStmt:
		walkArgsExpr(s.Condition, visit)
		walkArgsStmt(s.Then, visit)
		for _, elseif := range s.ElseIfs {
			walkArgsExpr(elseif.Condition, visit)
			walkArgsStmt(elseif.Body, visit)
		}
		walkArgsStmt(s.Else, visit)
	case *ast.WhileStmt:
		walkArgsExpr(s.Condition, visit)
		walkArgsStmt(s.Body, visit)
	case *ast.ForStmt:
		walkArgsExpr(s.Iterator, visit)
		walkArgsStmt(s.Body, visit)
	case *ast.CStyleForStmt:
		walkArgsStmt(s.Init, visit)
		walkArgsExpr(s.Condition, visit)
		walkArgsStmt(s.Update, visit)
		walkArgsStmt(s.Body, visit)
	case *ast.TryStmt:
		walkArgsStmt(s.Body, visit)
		walkArgsStmt(s.Catch, visit)
	case *ast.SwitchStmt:
		walkArgsExpr(s.Value, visit)
		for _, swCase := range s.Cases {
			if !swCase.IsDefault {
				walkArgsExpr(swCase.Value, visit)
			}
			walkArgsStmt(swCase.Body, visit)
		}
	case *ast.FnDecl:
		walkArgsStmt(s.Body, visit)
	case *ast.ClassDecl:
		for _, prop := range s.Properties {
			walkArgsExpr(prop.Value, visit)
		}
		for _, prop := range s.StaticProps {
			walkArgsExpr(prop.Value, visit)
		}
		if s.Constructor != nil {
			walkArgsStmt(s.Constructor.Body, visit)
		}
		for _, method := range s.Methods {
			walkArgsStmt(method.Body, visit)
		}
		for _, accessor := range s.Accessors {
			walkArgsStmt(accessor.Body, visit)
		}
	case *ast.Block:
		if s == nil {
			return
		}
		walkArgsStatements(s.Statements, visit)
	case *ast.DeclareStmt, *ast.DeclareFnStmt, *ast.BreakStmt, *ast.ContinueStmt:
		return
	}
}

func isArgsMethodCall(expr ast.Expression) bool {
	e, ok := expr.(*ast.MethodCallExpr)
	if !ok {
		return false
	}
	return ast.IsBeshtArgsReceiver(e.Receiver)
}

func isArgsArgvCall(expr ast.Expression) bool {
	call, ok := expr.(*ast.MethodCallExpr)
	return ok && call.Method == "argv" && isArgsMethodCall(call)
}

func walkArgsExpr(expr ast.Expression, visit func(ast.Expression)) {
	if expr == nil {
		return
	}
	visit(expr)
	switch e := expr.(type) {
	case *ast.IntLit, *ast.FloatLit, *ast.StringLit, *ast.RawStringLit, *ast.BoolLit, *ast.UndefinedLit, *ast.NullLit, *ast.IdentExpr, *ast.ThisExpr, *ast.UpdateExpr:
		return
	case *ast.TemplateLit:
		for _, expr := range e.Exprs {
			walkArgsExpr(expr, visit)
		}
	case *ast.MethodCallExpr:
		walkArgsExpr(e.Receiver, visit)
		for _, arg := range e.Args {
			walkArgsExpr(arg, visit)
		}
	case *ast.BinaryExpr:
		walkArgsExpr(e.Left, visit)
		walkArgsExpr(e.Right, visit)
	case *ast.PipeExpr:
		walkArgsExpr(e.Left, visit)
		walkArgsExpr(e.Right, visit)
	case *ast.UnaryExpr:
		walkArgsExpr(e.Expr, visit)
	case *ast.TernaryExpr:
		walkArgsExpr(e.Condition, visit)
		walkArgsExpr(e.Then, visit)
		walkArgsExpr(e.Else, visit)
	case *ast.FnCallExpr:
		for _, arg := range e.Args {
			walkArgsExpr(arg, visit)
		}
	case *ast.BuiltinCallExpr:
		for _, arg := range e.Args {
			walkArgsExpr(arg, visit)
		}
	case *ast.RangeExpr:
		walkArgsExpr(e.Start, visit)
		walkArgsExpr(e.End, visit)
	case *ast.CmdExpr:
		for _, arg := range e.Args {
			walkArgsExpr(arg, visit)
		}
	case *ast.PropagateExpr:
		walkArgsExpr(e.Expr, visit)
	case *ast.ListLit:
		for _, elem := range e.Elements {
			walkArgsExpr(elem, visit)
		}
	case *ast.ObjectLit:
		for _, field := range e.Fields {
			walkArgsExpr(field.Value, visit)
		}
	case *ast.ArrowExpr:
		walkArgsExpr(e.Body, visit)
		walkArgsStmt(e.BlockBody, visit)
	case *ast.IndexExpr:
		walkArgsExpr(e.Expr, visit)
		walkArgsExpr(e.Index, visit)
	case *ast.PropertyExpr:
		walkArgsExpr(e.Receiver, visit)
	case *ast.AsExpr:
		walkArgsExpr(e.Expr, visit)
	case *ast.SpreadExpr:
		walkArgsExpr(e.Expr, visit)
	case *ast.NewExpr:
		for _, arg := range e.Args {
			walkArgsExpr(arg, visit)
		}
	}
}

func (g *Generator) collectArgsSchemaExpr(expr ast.Expression) {
	e, ok := expr.(*ast.MethodCallExpr)
	if !ok || !isArgsMethodCall(expr) {
		return
	}
	if len(e.Args) >= 1 {
		if lit, ok := e.Args[0].(*ast.StringLit); ok && lit.Value != "" {
			switch e.Method {
			case "option":
				g.argsOptions["--"+lit.Value] = true
			case "flag":
				g.argsFlags["--"+lit.Value] = true
			}
		}
	}
	if len(e.Args) >= 2 {
		if lit, ok := e.Args[1].(*ast.StringLit); ok && lit.Value != "" {
			switch e.Method {
			case "option":
				g.argsOptions["-"+lit.Value] = true
			case "flag":
				g.argsFlags["-"+lit.Value] = true
			}
		}
	}
}

func (g *Generator) collectObjectTypes(stmts []ast.Statement) {
	for _, stmt := range stmts {
		switch s := stmt.(type) {
		case *ast.LetDecl:
			if obj, ok := s.Value.(*ast.ObjectLit); ok {
				varName := s.Name
				for _, field := range obj.Fields {
					pt := g.inferExprType(field.Value)
					if pt != nil {
						g.objPropTypeMap[varName+"."+field.Key] = pt
					}
				}
			}
		case *ast.FnDecl:
			g.collectObjectTypes(s.Body.Statements)
		case *ast.ClassDecl:
			for _, method := range s.Methods {
				g.collectObjectTypes(method.Body.Statements)
			}
			for _, accessor := range s.Accessors {
				g.collectObjectTypes(accessor.Body.Statements)
			}
			if s.Constructor != nil {
				g.collectObjectTypes(s.Constructor.Body.Statements)
			}
		case *ast.IfStmt:
			g.collectObjectTypes(s.Then.Statements)
			if s.Else != nil {
				g.collectObjectTypes(s.Else.Statements)
			}
		case *ast.WhileStmt:
			g.collectObjectTypes(s.Body.Statements)
		case *ast.ForStmt:
			g.collectObjectTypes(s.Body.Statements)
		case *ast.CStyleForStmt:
			g.collectObjectTypes(s.Body.Statements)
		case *ast.TryStmt:
			g.collectObjectTypes(s.Body.Statements)
			if s.Catch != nil {
				g.collectObjectTypes(s.Catch.Statements)
			}
		}
	}
}

func (g *Generator) inferListLiteralType(e *ast.ListLit) *ast.Type {
	if t := e.GetType(); t != nil {
		return t
	}
	if len(e.Elements) == 0 {
		return &ast.Type{Kind: ast.TypeList, Elem: typeString}
	}
	for _, elem := range e.Elements {
		elemType := g.inferReceiverType(elem)
		if spread, ok := elem.(*ast.SpreadExpr); ok {
			spreadType := g.inferReceiverType(spread.Expr)
			if spreadType != nil && spreadType.Kind == ast.TypeList && spreadType.Elem != nil {
				elemType = spreadType.Elem
			}
		}
		if elemType != nil {
			return &ast.Type{Kind: ast.TypeList, Elem: elemType}
		}
	}
	return &ast.Type{Kind: ast.TypeList, Elem: typeString}
}

func (g *Generator) inferExprType(expr ast.Expression) *ast.Type {
	t := expr.GetType()
	if t != nil {
		return t
	}
	switch e := expr.(type) {
	case *ast.StringLit, *ast.RawStringLit, *ast.TemplateLit:
		return typeString
	case *ast.UndefinedLit, *ast.NullLit:
		return typeString
	case *ast.IntLit, *ast.FloatLit:
		return typeNumber
	case *ast.BoolLit:
		return &ast.Type{Kind: ast.TypeBoolean}
	case *ast.ListLit:
		return &ast.Type{Kind: ast.TypeList, Elem: g.inferListElemType(e)}
	case *ast.IdentExpr:
		if vt, ok := g.varTypeMap[e.Name]; ok {
			return vt
		}
	case *ast.BinaryExpr:
		if e.Op == "??" {
			lt := g.inferExprType(e.Left)
			if lt != nil {
				return lt
			}
			return g.inferExprType(e.Right)
		}
		if e.Op == "+" {
			lt := g.inferExprType(e.Left)
			rt := g.inferExprType(e.Right)
			if (lt != nil && lt.Kind == ast.TypeString) || (rt != nil && rt.Kind == ast.TypeString) {
				return typeString
			}
			return typeNumber
		}
		return &ast.Type{Kind: ast.TypeBoolean}
	case *ast.ObjectLit:
		return &ast.Type{Kind: ast.TypeObject}
	case *ast.NewExpr:
		if e.ClassName == "Set" {
			elem := typeString
			if len(e.TypeArgs) > 0 {
				elem = e.TypeArgs[0]
			}
			return &ast.Type{Kind: ast.TypeSet, Elem: elem}
		}
		return &ast.Type{Kind: ast.TypeObject}
	case *ast.ThisExpr:
		return &ast.Type{Kind: ast.TypeObject}
	case *ast.UnaryExpr:
		if e.Op == "!" {
			return &ast.Type{Kind: ast.TypeBoolean}
		}
		return typeNumber
	}
	return nil
}

func (g *Generator) inferListElemType(e *ast.ListLit) *ast.Type {
	if len(e.Elements) == 0 {
		return typeString
	}
	if spread, ok := e.Elements[0].(*ast.SpreadExpr); ok {
		if t := g.inferReceiverType(spread.Expr); t != nil && t.Kind == ast.TypeList {
			return t.Elem
		}
	}
	if t := g.inferReceiverType(e.Elements[0]); t != nil {
		return t
	}
	return typeString
}

const (
	beshtRuntimeHelperStartsWith = `_bst_starts_with() { case "$1" in "$2"*) return 0;; *) return 1;; esac; }
`
	beshtRuntimeHelperEndsWith = `_bst_ends_with()   { case "$1" in *"$2") return 0;; *) return 1;; esac; }
`
	beshtRuntimeHelperIncludes = `_bst_includes()    { case "$1" in *"$2"*) return 0;; *) return 1;; esac; }
`
	beshtRuntimeHelperNullish = `_BESHT_NULLISH_SENTINEL=__BESHT_NULLISH_$$
`
	beshtRuntimeHelperArgs = `_bst_args_has() { case "
$1
" in *"
$2
"*) return 0;; *) return 1;; esac; }
_bst_shell_quote() { printf "'"; printf '%s' "$1" | sed "s/'/'\\\\''/g"; printf "'"; }
_bst_args_call() {
  _bst_fn=$1; shift
  _bst_call_i=1
  while [ "$_bst_call_i" -le "$_bst_argc" ]; do
    eval "_bst_call_v=\${_bst_arg_${_bst_call_i}}"
    set -- "$@" "$_bst_call_v"
    _bst_call_i=$(( _bst_call_i + 1 ))
  done
  "$_bst_fn" "$@"
}
_bst_args_argv() {
  _bst_options=$1; _bst_flags=$2; shift 2
  while [ "$#" -gt 0 ]; do
    case "$1" in
      --) shift; while [ "$#" -gt 0 ]; do printf '%s\n' "$1"; shift; done; return;;
      --*=*) _bst_name=${1%%=*}; if _bst_args_has "$_bst_options" "$_bst_name" || _bst_args_has "$_bst_flags" "$_bst_name"; then shift; else printf '%s\n' "$1"; shift; fi;;
      --*|-[!-]) if _bst_args_has "$_bst_options" "$1"; then shift; [ "$#" -gt 0 ] && shift; elif _bst_args_has "$_bst_flags" "$1"; then shift; else printf '%s\n' "$1"; shift; fi;;
      *) printf '%s\n' "$1"; shift;;
    esac
  done
}
_bst_args_argc() {
  _bst_options=$1; _bst_flags=$2; shift 2; _bst_count=0
  while [ "$#" -gt 0 ]; do
    case "$1" in
      --) shift; while [ "$#" -gt 0 ]; do _bst_count=$(( _bst_count + 1 )); shift; done; break;;
      --*=*) _bst_name=${1%%=*}; if _bst_args_has "$_bst_options" "$_bst_name" || _bst_args_has "$_bst_flags" "$_bst_name"; then shift; else _bst_count=$(( _bst_count + 1 )); shift; fi;;
      --*|-[!-]) if _bst_args_has "$_bst_options" "$1"; then shift; [ "$#" -gt 0 ] && shift; elif _bst_args_has "$_bst_flags" "$1"; then shift; else _bst_count=$(( _bst_count + 1 )); shift; fi;;
      *) _bst_count=$(( _bst_count + 1 )); shift;;
    esac
  done
  printf '%s' "$_bst_count"
}
_bst_args_positional() {
  _bst_n=$1; _bst_options=$2; _bst_flags=$3; shift 3; _bst_i=1
  while [ "$#" -gt 0 ]; do
    case "$1" in
      --) shift; while [ "$#" -gt 0 ]; do if [ "$_bst_i" -eq "$_bst_n" ]; then printf '%s' "$1"; return; fi; _bst_i=$(( _bst_i + 1 )); shift; done; break;;
      --*=*) _bst_name=${1%%=*}; if _bst_args_has "$_bst_options" "$_bst_name" || _bst_args_has "$_bst_flags" "$_bst_name"; then shift; else if [ "$_bst_i" -eq "$_bst_n" ]; then printf '%s' "$1"; return; fi; _bst_i=$(( _bst_i + 1 )); shift; fi;;
      --*|-[!-]) if _bst_args_has "$_bst_options" "$1"; then shift; [ "$#" -gt 0 ] && shift; elif _bst_args_has "$_bst_flags" "$1"; then shift; else if [ "$_bst_i" -eq "$_bst_n" ]; then printf '%s' "$1"; return; fi; _bst_i=$(( _bst_i + 1 )); shift; fi;;
      *) if [ "$_bst_i" -eq "$_bst_n" ]; then printf '%s' "$1"; return; fi; _bst_i=$(( _bst_i + 1 )); shift;;
    esac
  done
  printf '%s' "$` + nullishSentinelVar + `"
}
_bst_args_option() {
  _bst_long=$1; _bst_short=$2; shift 2
  while [ "$#" -gt 0 ]; do
    case "$1" in
      --) printf '%s' "$` + nullishSentinelVar + `"; return;;
      --"$_bst_long"=*) printf '%s' "${1#*=}"; return;;
      --"$_bst_long") shift; if [ "$#" -gt 0 ]; then printf '%s' "$1"; else printf '%s' "$` + nullishSentinelVar + `"; fi; return;;
    esac
    if [ -n "$_bst_short" ] && [ "$1" = "-$_bst_short" ]; then
      shift; if [ "$#" -gt 0 ]; then printf '%s' "$1"; else printf '%s' "$` + nullishSentinelVar + `"; fi; return
    fi
    shift
  done
  printf '%s' "$` + nullishSentinelVar + `"
}
_bst_args_flag() {
  _bst_long=$1; _bst_short=$2; shift 2
  while [ "$#" -gt 0 ]; do
    if [ "$1" = "--" ]; then printf 0; return; fi
    if [ "$1" = "--$_bst_long" ]; then printf 1; return; fi
    if [ -n "$_bst_short" ] && [ "$1" = "-$_bst_short" ]; then printf 1; return; fi
    shift
  done
  printf 0
}
`
)

const beshtRuntimeArgsSnapshot = `_bst_argc=$#
_bst_capture_i=1
for _bst_arg do
  case "$_bst_arg" in *'
'*) printf '[besht] args values cannot contain newlines\n' >&2; exit 1;; esac
  eval "_bst_arg_${_bst_capture_i}=$(_bst_shell_quote "$_bst_arg")"
  _bst_capture_i=$(( _bst_capture_i + 1 ))
done
unset _bst_capture_i _bst_arg
`

func (g *Generator) requireRuntimeHelper(name string) {
	if g.runtimeHelpers == nil {
		g.runtimeHelpers = make(map[string]bool)
	}
	g.runtimeHelpers[name] = true
}

func runtimeHelpersSource(helpers map[string]bool) string {
	var sb strings.Builder
	if helpers["startsWith"] {
		sb.WriteString(beshtRuntimeHelperStartsWith)
	}
	if helpers["endsWith"] {
		sb.WriteString(beshtRuntimeHelperEndsWith)
	}
	if helpers["includes"] {
		sb.WriteString(beshtRuntimeHelperIncludes)
	}
	if helpers["nullish"] || helpers["args"] {
		sb.WriteString(beshtRuntimeHelperNullish)
	}
	if helpers["args"] {
		sb.WriteString(beshtRuntimeHelperArgs)
	}
	return sb.String()
}

const beshtCheckBlock = `_r=$(printf 'hello:world' | grep -F 'hello' | sed 's/hello/goodbye/' 2>/dev/null) || {
  printf '[besht] FATAL: required utilities (printf/grep/sed) are not working correctly\n' >&2
  exit 1
}
[ "$_r" = 'goodbye:world' ] || {
  printf '[besht] FATAL: utility pipeline produced unexpected output (got: %s)\n' "$_r" >&2
  exit 1
}
unset _r
`

func (g *Generator) generate(prog *ast.Program) (string, error) {
	g.cmdAnalysis = AnalyzeProgram(prog.Statements)
	for _, w := range g.cmdAnalysis.Warnings {
		fmt.Fprintf(os.Stderr, "besht: warning: %s: %s\n", w.Pos, w.Message)
	}
	for _, e := range g.cmdAnalysis.Errors {
		return "", fmt.Errorf("%s: %s", e.Pos, e.Message)
	}

	g.collectGlobalBindings(prog.Statements)
	for _, stmt := range prog.Statements {
		switch fn := stmt.(type) {
		case *ast.FnDecl:
			retType := inferFunctionReturnType(fn.Body.Statements)
			g.fnReturnMap[fn.Name] = retType
			var pnames []string
			for _, p := range fn.Params {
				pnames = append(pnames, p.Name)
			}
			g.fnParamNames[fn.Name] = pnames
			if g.fnReturnsFloat(fn.Body.Statements) {
				g.floatVars[fn.Name] = true
			}
		case *ast.ClassDecl:
			g.registerClass(fn)
		}
	}
	g.collectObjectTypes(prog.Statements)
	g.collectArgsSchema(prog.Statements)

	for _, stmt := range prog.Statements {
		if err := g.genStmt(stmt); err != nil {
			return "", err
		}
	}

	body := g.sb.String()
	var out strings.Builder
	out.WriteString("#!/bin/sh\n")
	out.WriteString("# Generated by besht — do not edit by hand\n")
	out.WriteString("\n")
	out.WriteString(runtimeHelpersSource(g.runtimeHelpers))
	if g.runtimeHelpers["args"] {
		out.WriteString(beshtRuntimeArgsSnapshot)
	}
	if !g.NoCheck {
		out.WriteString(beshtCheckBlock)
	}
	out.WriteString("\n")
	out.WriteString(body)
	return out.String(), nil
}

func (g *Generator) collectGlobalBindings(stmts []ast.Statement) {
	if g.globalVarMap == nil {
		g.globalVarMap = make(map[string]string)
	}
	for _, stmt := range stmts {
		if s, ok := stmt.(*ast.LetDecl); ok {
			if _, exists := g.globalVarMap[s.Name]; !exists {
				g.globalVarMap[s.Name] = s.Name
			}
		}
	}
}

func (g *Generator) line(s string) {
	if s == "" {
		g.sb.WriteString("\n")
		return
	}
	g.sb.WriteString(strings.Repeat("    ", g.indent))
	g.sb.WriteString(s)
	g.sb.WriteString("\n")
}

func (g *Generator) raw(s string) {
	g.sb.WriteString(s)
}

func (g *Generator) push() { g.indent++ }
func (g *Generator) pop()  { g.indent-- }

func (g *Generator) genSourceComment(pos ast.Pos) {
	if pos.File == "" || g.NoSourceMap {
		return
	}
	g.line(fmt.Sprintf("# besht:%s:%d:%d", sanitizeShellComment(pos.File), pos.Line, pos.Column))
}

func (g *Generator) genStmt(stmt ast.Statement) error {
	if _, ok := stmt.(*ast.ClassDecl); !ok {
		g.genSourceComment(stmtPos(stmt))
	}
	switch s := stmt.(type) {
	case *ast.ImportDecl:
		return nil
	case *ast.LetDecl:
		return g.genLetDecl(s)
	case *ast.DestructureDecl:
		return g.genDestructureDecl(s)
	case *ast.Assignment:
		return g.genAssignment(s)
	case *ast.IndexAssignStmt:
		return g.genIndexAssign(s)
	case *ast.PropertyAssignStmt:
		return g.genPropertyAssign(s)
	case *ast.ClassDecl:
		return g.genClassDecl(s)
	case *ast.FnDecl:
		return g.genFnDecl(s)
	case *ast.IfStmt:
		return g.genIf(s)
	case *ast.SwitchStmt:
		return g.genSwitch(s)
	case *ast.ForStmt:
		return g.genFor(s)
	case *ast.WhileStmt:
		return g.genWhile(s)
	case *ast.TryStmt:
		return g.genTry(s)
	case *ast.ReturnStmt:
		return g.genReturn(s)
	case *ast.ExitStmt:
		return g.genExit(s)
	case *ast.ExprStmt:
		return g.genExprStmt(s)
	case *ast.Block:
		for _, child := range s.Statements {
			if err := g.genStmt(child); err != nil {
				return err
			}
		}
		return nil
	case *ast.CStyleForStmt:
		return g.genCStyleFor(s)
	case *ast.DeclareStmt:
		return nil
	case *ast.DeclareFnStmt:
		return nil
	case *ast.BreakStmt:
		g.line("break")
		return nil
	case *ast.ContinueStmt:
		g.line("continue")
		return nil
	}
	return fmt.Errorf("codegen: unknown statement type %T", stmt)
}

func (g *Generator) genLetDecl(s *ast.LetDecl) error {
	varName := g.resolveVarName(s.Name)
	g.paramMap[s.Name] = varName
	delete(g.objAliasMap, s.Name)

	if newExpr, ok := s.Value.(*ast.NewExpr); ok {
		return g.genNewLetDecl(varName, s.Name, newExpr)
	}

	if g.isReduceCall(s.Value) {
		return g.genReduceLetDecl(varName, s.Name, s.Value)
	}

	if chained, ok := g.reduceReceiverChain(s.Value, varName+"_reduce"); ok {
		if err := g.genReduceLetDecl(varName+"_reduce", varName+"_reduce", chained.reduce); err != nil {
			return err
		}
		val, err := g.genExprRHS(chained.rewritten, s.TypeAnnot)
		if err != nil {
			return err
		}
		g.line(fmt.Sprintf("%s=%s", varName, val))
		if s.TypeAnnot != nil {
			g.varTypeMap[varName] = s.TypeAnnot
		} else if inferred := g.inferReceiverType(chained.rewritten); inferred != nil {
			g.varTypeMap[varName] = inferred
		}
		return nil
	}

	if obj, ok := s.Value.(*ast.ObjectLit); ok {
		var fields []string
		for _, field := range obj.Fields {
			if err := validateStaticObjectKey(field.Key); err != nil {
				return err
			}
			fields = append(fields, field.Key)
			val, err := g.genExprRHS(field.Value, nil)
			if err != nil {
				return err
			}
			pt := g.inferReceiverType(field.Value)
			if pt != nil {
				g.objPropTypeMap[varName+"."+field.Key] = pt
			}
			g.line(fmt.Sprintf("%s=%s", objectPropVar(varName, field.Key), val))
		}
		g.objFieldsMap[varName] = fields
		g.varTypeMap[varName] = &ast.Type{Kind: ast.TypeObject}
		delete(g.objAliasMap, s.Name)
		g.line(fmt.Sprintf("%s=%s", varName, shellQuote(varName)))
		g.emitObjectKeysInit(varName, fields)
		return nil
	}

	if ok, err := g.genFetchResponseBinding(varName, s.Value); ok || err != nil {
		return err
	}

	// Lazy command chain (no .run() inline): just register the identity, no code.
	if g.isLazyCommandBinding(s.Value) && g.cmdAnalysis != nil {
		g.varTypeMap[varName] = &ast.Type{Kind: ast.TypeCommand}
		if g.cmdChains == nil {
			g.cmdChains = make(map[string]ast.Expression)
		}
		g.cmdChains[s.Name] = s.Value
		if g.cmdScope != nil {
			id := -1
			if me, ok := s.Value.(*ast.MethodCallExpr); ok && me.Method == "clone" {
				id = g.cmdAnalysis.resolveIdentityExpr(s.Value, g.cmdScope)
			}
			if id < 0 {
				if root := findRootCmd(s.Value); root != nil {
					if rootID, ok := g.cmdAnalysis.nodeToID[root]; ok {
						id = rootID
					}
				}
			}
			if id < 0 {
				id = g.cmdAnalysis.resolveIdentityExpr(s.Value, g.cmdScope)
			}
			if id >= 0 {
				if ident := g.cmdAnalysis.identity(id); ident != nil && ident.VarName == "" {
					ident.VarName = s.Name
				}
				g.cmdScope.define(s.Name, id)
			}
		}
		return nil
	}

	// Inline .run() chain (e.g. let x = $(...).run().readStdout()):
	// emit the run call first, then read the captured value var.
	if containsRunCall(s.Value) && g.cmdAnalysis != nil {
		if err := g.emitInlineRunChain(s.Value); err != nil {
			return err
		}
		id := g.cmdAnalysis.resolveIdentityExpr(s.Value, g.cmdScope)
		if id >= 0 {
			ident := g.cmdAnalysis.identity(id)
			if ident != nil {
				valueVar := ident.CaptureVarName(g.resolveVarName)
				if me, ok := s.Value.(*ast.MethodCallExpr); ok && me.Method == "exitCode" {
					valueVar = ident.ExitCodeVarName(g.resolveVarName)
				}
				// genRunCall already emitted the value assignment using valueVar.
				// Only add an alias if the let variable name differs from valueVar.
				if varName != valueVar {
					g.line(fmt.Sprintf("%s=\"$%s\"", varName, valueVar))
				}
				return nil
			}
		}
	}

	refValue, hasObjectRefValue := g.objectRefValue(s.Value)
	val := refValue
	if !hasObjectRefValue {
		var err error
		val, err = g.genExprRHS(s.Value, s.TypeAnnot)
		if err != nil {
			return err
		}
	}
	g.paramMap[s.Name] = varName
	// Infer type from annotation or value for method/operator dispatch
	if s.TypeAnnot != nil {
		g.varTypeMap[varName] = s.TypeAnnot
	} else if _, isLit := s.Value.(*ast.ListLit); isLit {
		if inferred := g.inferReceiverType(s.Value); inferred != nil {
			g.varTypeMap[varName] = inferred
		} else {
			g.varTypeMap[varName] = &ast.Type{Kind: ast.TypeList, Elem: &ast.Type{Kind: ast.TypeString}}
		}
	} else if _, isStr := s.Value.(*ast.StringLit); isStr {
		g.varTypeMap[varName] = &ast.Type{Kind: ast.TypeString}
	} else if _, isTmpl := s.Value.(*ast.TemplateLit); isTmpl {
		g.varTypeMap[varName] = &ast.Type{Kind: ast.TypeString}
	} else if _, isRaw := s.Value.(*ast.RawStringLit); isRaw {
		g.varTypeMap[varName] = &ast.Type{Kind: ast.TypeString}
	} else if _, isInt := s.Value.(*ast.IntLit); isInt {
		g.varTypeMap[varName] = &ast.Type{Kind: ast.TypeNumber}
	} else if _, isFloat := s.Value.(*ast.FloatLit); isFloat {
		g.varTypeMap[varName] = &ast.Type{Kind: ast.TypeNumber}
		g.floatVars[varName] = true
	} else if t := g.inferReceiverType(s.Value); t != nil {
		g.varTypeMap[varName] = t
	}
	if ref, ok := g.objectRefForBinding(varName, s.Value); ok {
		g.objAliasMap[s.Name] = ref
	} else {
		delete(g.objAliasMap, s.Name)
	}
	if g.isFloatExpr(s.Value) {
		g.floatVars[varName] = true
	}
	if value, ok := stringLiteralValue(s.Value); ok {
		if g.stringConstMap == nil {
			g.stringConstMap = make(map[string]string)
		}
		g.stringConstMap[varName] = value
	} else {
		delete(g.stringConstMap, varName)
	}
	if g.listLenMap == nil {
		g.listLenMap = make(map[string]string)
	}
	if emptyArrayOf(s.Value) {
		g.listLenMap[varName] = "0"
	} else if isArgsArgvCall(s.Value) {
		g.listLenMap[varName] = g.argsArgcExpr()
	} else {
		delete(g.listLenMap, varName)
	}
	g.updateStaticListBinding(varName, s.Value)
	g.line(fmt.Sprintf("%s=%s", varName, val))
	return nil
}

func (g *Generator) objectRefForBinding(varName string, expr ast.Expression) (objectRef, bool) {
	ref, ok := g.resolveObjectRef(expr)
	if !ok {
		return objectRef{}, false
	}
	if ref.StaticName != "" {
		if ref.RootName == "" {
			ref.RootName = ref.StaticName
		}
		ref.UnsupportedScalarValues = g.objectRefHasUnsupportedScalarValues(ref)
		return ref, true
	}
	rootName := ref.RootName
	if rootName == "" {
		rootName = varName
	}
	return objectRef{SlotExpr: fmt.Sprintf("\"$%s\"", varName), RootName: rootName, UnsupportedScalarValues: g.objectRefHasUnsupportedScalarValues(ref)}, true
}

func (g *Generator) objectRefValue(expr ast.Expression) (string, bool) {
	ref, ok := g.resolveObjectRef(expr)
	if !ok {
		return "", false
	}
	if ref.StaticName != "" {
		return shellQuote(ref.StaticName), true
	}
	return ref.SlotExpr, true
}

func (g *Generator) isLazyCommandBinding(expr ast.Expression) bool {
	if isCmdExprOrChain(expr) {
		return true
	}
	if containsRunCall(expr) || g.cmdAnalysis == nil || g.cmdScope == nil {
		return false
	}
	me, ok := expr.(*ast.MethodCallExpr)
	if !ok {
		return false
	}
	switch me.Method {
	case "readStdout", "readStdoutLines", "readStderr", "exitCode":
		return false
	}
	return g.cmdAnalysis.resolveIdentityExpr(expr, g.cmdScope) >= 0
}

func (g *Generator) genDestructureDecl(s *ast.DestructureDecl) error {
	tmp := fmt.Sprintf("_destructure_%d_%d", s.Pos.Line, s.Pos.Column)
	if containsRunCall(s.Value) && g.cmdAnalysis != nil && !isImmediateRunTerminalCall(s.Value) {
		if err := g.emitInlineRunChain(s.Value); err != nil {
			return err
		}
	}
	refValue, hasObjectRefValue := g.objectRefValue(s.Value)
	val := refValue
	if !hasObjectRefValue {
		var err error
		val, err = g.genExprRHS(s.Value, nil)
		if err != nil {
			return err
		}
	}
	g.line(fmt.Sprintf("%s=%s", tmp, val))
	elemType := typeString
	if valueType := g.inferReceiverType(s.Value); valueType != nil && valueType.Kind == ast.TypeList && valueType.Elem != nil {
		elemType = valueType.Elem
	}
	for i, name := range s.Names {
		varName := g.resolveVarName(name)
		g.paramMap[name] = varName
		g.varTypeMap[varName] = elemType
		g.line(fmt.Sprintf("%s=$(printf '%%s\\n' \"$%s\" | sed -n '%dp')", varName, tmp, i+1))
	}
	return nil
}

func emptyArrayOf(expr ast.Expression) bool {
	builtin, ok := expr.(*ast.BuiltinCallExpr)
	return ok && builtin.Name == "Array.of" && len(builtin.Args) == 0
}

type reduceReceiverChain struct {
	reduce    *ast.MethodCallExpr
	rewritten ast.Expression
}

func (g *Generator) reduceReceiverChain(expr ast.Expression, reduceVar string) (reduceReceiverChain, bool) {
	me, ok := expr.(*ast.MethodCallExpr)
	if !ok {
		return reduceReceiverChain{}, false
	}
	reduce, ok := me.Receiver.(*ast.MethodCallExpr)
	if !ok || !g.isReduceCall(reduce) {
		return reduceReceiverChain{}, false
	}
	clone := *me
	clone.Receiver = &ast.IdentExpr{Pos: reduce.Pos, Name: reduceVar}
	return reduceReceiverChain{reduce: reduce, rewritten: &clone}, true
}

func (g *Generator) registerClass(c *ast.ClassDecl) {
	g.classMap[c.Name] = c
	for _, prop := range c.Properties {
		pt := prop.Type
		if pt == nil {
			pt = typeString
		}
		g.objPropTypeMap[c.Name+"."+prop.Name] = pt
	}
	for _, prop := range c.StaticProps {
		pt := prop.Type
		if pt == nil && prop.Value != nil {
			pt = g.inferExprType(prop.Value)
		}
		if pt == nil {
			pt = typeString
		}
		staticVar := classPropVar(c.Name, prop.Name)
		g.varTypeMap[staticVar] = pt
		if pt.Kind == ast.TypeObject && pt.Elem != nil {
			g.objPropTypeMap[staticVar+".*"] = pt.Elem
		}
	}
	for _, method := range c.Methods {
		retType := inferFunctionReturnType(method.Body.Statements)
		fnName := classMethodName(c.Name, method.Name)
		g.fnReturnMap[fnName] = retType
		if g.fnReturnsFloat(method.Body.Statements) {
			g.floatVars[fnName] = true
		}
	}
	for _, accessor := range c.Accessors {
		methodName := string(accessor.Kind) + "_" + accessor.Name
		retType := inferFunctionReturnType(accessor.Body.Statements)
		if accessor.Kind == ast.AccessorSet {
			retType = &ast.Type{Kind: ast.TypeVoid}
		}
		fnName := classMethodName(c.Name, methodName)
		g.fnReturnMap[fnName] = retType
		if g.fnReturnsFloat(accessor.Body.Statements) {
			g.floatVars[fnName] = true
		}
		if accessor.Kind == ast.AccessorGet {
			g.objPropTypeMap[c.Name+"."+accessor.Name] = retType
		}
	}
	if c.Constructor != nil {
		g.fnReturnMap[classMethodName(c.Name, "constructor")] = &ast.Type{Kind: ast.TypeVoid}
	}
}

func classAccessor(c *ast.ClassDecl, name string, kind ast.ClassAccessorKind, static bool) (*ast.ClassAccessor, bool) {
	if c == nil {
		return nil, false
	}
	for i := range c.Accessors {
		accessor := &c.Accessors[i]
		if accessor.Name == name && accessor.Kind == kind && accessor.IsStatic == static {
			return accessor, true
		}
	}
	return nil, false
}

func (g *Generator) genNewLetDecl(varName, sourceName string, e *ast.NewExpr) error {
	if e.ClassName == "Set" {
		if len(e.Args) != 0 {
			return fmt.Errorf("Set constructor takes no runtime arguments")
		}
		if len(e.TypeArgs) > 1 {
			return fmt.Errorf("Set constructor takes at most 1 type argument")
		}
		g.line(fmt.Sprintf("%s=\"\"", varName))
		elem := typeString
		if len(e.TypeArgs) > 0 {
			elem = e.TypeArgs[0]
		}
		g.varTypeMap[varName] = &ast.Type{Kind: ast.TypeSet, Elem: elem}
		return nil
	}
	className := g.resolveClassName(e.ClassName)
	classDecl := g.classMap[className]
	if classDecl == nil {
		classDecl = g.classMap[e.ClassName]
	}
	g.line(fmt.Sprintf("%s=%s", varName, shellQuote(varName)))
	args, err := g.genFnArgs(e.Args)
	if err != nil {
		return err
	}
	ctor := classMethodName(className, "constructor")
	callArgs := append([]string{fmt.Sprintf("\"$%s\"", varName)}, args...)
	g.line(fmt.Sprintf("%s %s", ctor, strings.Join(callArgs, " ")))
	g.varTypeMap[varName] = &ast.Type{Kind: ast.TypeObject}
	g.varClassMap[varName] = className
	g.varClassMap[sourceName] = className
	if classDecl != nil {
		var fields []string
		for _, prop := range classDecl.Properties {
			fields = append(fields, prop.Name)
			pt := prop.Type
			if pt != nil {
				g.objPropTypeMap[varName+"."+prop.Name] = pt
			}
		}
		g.objFieldsMap[varName] = fields
		g.emitObjectKeysInit(varName, fields)
	}
	return nil
}

func (g *Generator) isReduceCall(expr ast.Expression) bool {
	if asExpr, ok := expr.(*ast.AsExpr); ok {
		return g.isReduceCall(asExpr.Expr)
	}
	if me, ok := expr.(*ast.MethodCallExpr); ok {
		if me.Method == "reduce" {
			recvType := g.inferReceiverType(me.Receiver)
			return recvType != nil && recvType.Kind == ast.TypeList
		}
	}
	return false
}

func (g *Generator) withCallbackParams(arrow *ast.ArrowExpr, overrides map[string]string, body func(callbackLoopCtx) error) error {
	bindings := make([]callbackBinding, 0, len(arrow.Params))
	ctx := callbackLoopCtx{ParamVars: make([]string, len(arrow.Params)), ParamByName: make(map[string]string, len(arrow.Params))}
	for i, param := range arrow.Params {
		name := param.Name
		binding := callbackParamNameAt(arrow, i)
		if overrides != nil {
			if override, ok := overrides[name]; ok {
				binding = override
			}
		}
		old, hadOld := g.paramMap[name]
		oldAlias, hadAlias := g.objAliasMap[name]
		bindings = append(bindings, callbackBinding{name: name, old: old, hadOld: hadOld, oldAlias: oldAlias, hadAlias: hadAlias})
		g.paramMap[name] = binding
		delete(g.objAliasMap, name)
		ctx.ParamVars[i] = binding
		ctx.ParamByName[name] = binding
	}
	defer func() {
		for i := len(bindings) - 1; i >= 0; i-- {
			binding := bindings[i]
			if binding.hadOld {
				g.paramMap[binding.name] = binding.old
			} else {
				delete(g.paramMap, binding.name)
			}
			if binding.hadAlias {
				g.objAliasMap[binding.name] = binding.oldAlias
			} else {
				delete(g.objAliasMap, binding.name)
			}
		}
	}()
	return body(ctx)
}

func (g *Generator) withReduceReturn(ctx reduceReturnContext, body func() error) error {
	g.reduceReturns = append(g.reduceReturns, ctx)
	defer func() {
		g.reduceReturns = g.reduceReturns[:len(g.reduceReturns)-1]
	}()
	return body()
}

func (g *Generator) currentReduceReturn() (reduceReturnContext, bool) {
	if len(g.reduceReturns) == 0 {
		return reduceReturnContext{}, false
	}
	return g.reduceReturns[len(g.reduceReturns)-1], true
}

func (g *Generator) withMapReturn(ctx mapReturnContext, body func() error) error {
	g.mapReturns = append(g.mapReturns, ctx)
	defer func() {
		g.mapReturns = g.mapReturns[:len(g.mapReturns)-1]
	}()
	return body()
}

func (g *Generator) currentMapReturn() (mapReturnContext, bool) {
	if len(g.mapReturns) == 0 {
		return mapReturnContext{}, false
	}
	return g.mapReturns[len(g.mapReturns)-1], true
}

func (g *Generator) genReduceLetDecl(varName, sourceName string, expr ast.Expression) error {
	me := expr.(*ast.MethodCallExpr)
	arrow, ok := me.Args[0].(*ast.ArrowExpr)
	if !ok {
		return fmt.Errorf("reduce() callback must be an arrow expression")
	}
	if len(arrow.Params) != 2 {
		return fmt.Errorf("reduce() callback must take 2 parameters")
	}

	initVal := me.Args[1]
	if obj, ok := initVal.(*ast.ObjectLit); ok {
		var fields []string
		for _, field := range obj.Fields {
			if err := validateStaticObjectKey(field.Key); err != nil {
				return err
			}
			fields = append(fields, field.Key)
			val, err := g.genExprRHS(field.Value, nil)
			if err != nil {
				return err
			}
			g.line(fmt.Sprintf("%s=%s", objectPropVar(varName, field.Key), val))
		}
		g.objFieldsMap[varName] = fields
		g.varTypeMap[varName] = &ast.Type{Kind: ast.TypeObject}
		g.line(fmt.Sprintf("%s=%s", varName, shellQuote(varName)))
		g.emitObjectKeysInit(varName, fields)
	} else {
		val, err := g.genExprRHS(initVal, nil)
		if err != nil {
			return err
		}
		g.line(varName + "=" + val)
		g.varTypeMap[varName] = g.inferReceiverType(initVal)
		if g.varTypeMap[varName] == nil {
			g.varTypeMap[varName] = typeString
		}
	}

	recv, err := g.genExprValue(me.Receiver)
	if err != nil {
		return err
	}
	recvArg := ensureArgSafe(recv)

	heredocTag := fmt.Sprintf("_EOF_RED_%d_%d", arrow.Pos.Line, arrow.Pos.Column)

	accType := g.inferReceiverType(initVal)
	if accType != nil {
		g.varTypeMap[varName] = accType
	}
	reduceCtx := reduceReturnContext{accVar: varName, accParam: arrow.Params[0].Name, accIsObject: accType != nil && accType.Kind == ast.TypeObject}
	return g.withReduceReturn(reduceCtx, func() error {
		return g.withCallbackParams(arrow, map[string]string{arrow.Params[0].Name: varName}, func(ctx callbackLoopCtx) error {
			curMangled := ctx.ParamVars[1]
			g.line(fmt.Sprintf("while IFS= read -r %s; do", curMangled))
			g.push()

			if arrow.BlockBody != nil {
				for _, stmt := range arrow.BlockBody.Statements {
					if err := g.genStmt(stmt); err != nil {
						g.pop()
						return err
					}
				}
			} else {
				val, err := g.genExprRHS(arrow.Body, nil)
				if err != nil {
					g.pop()
					return err
				}
				g.line(varName + "=" + val)
			}

			g.pop()
			g.line("done << " + heredocTag)
			g.line("$(printf '" + "%s" + "\\n' " + recvArg + ")")
			g.line(heredocTag)
			return nil
		})
	})
}

func (g *Generator) genAssignment(s *ast.Assignment) error {
	varName := g.resolveVarName(s.Name)
	if ok, err := g.genFetchResponseBinding(varName, s.Value); ok || err != nil {
		return err
	}

	val, err := g.genExprRHS(s.Value, nil)
	if err != nil {
		return err
	}
	if g.listLenMap == nil {
		g.listLenMap = make(map[string]string)
	}
	if emptyArrayOf(s.Value) {
		g.listLenMap[varName] = "0"
	} else {
		delete(g.listLenMap, varName)
	}
	if g.isFloatExpr(s.Value) {
		g.floatVars[varName] = true
	} else {
		delete(g.floatVars, varName)
	}
	if value, ok := stringLiteralValue(s.Value); ok {
		if g.stringConstMap == nil {
			g.stringConstMap = make(map[string]string)
		}
		g.stringConstMap[varName] = value
	} else {
		delete(g.stringConstMap, varName)
	}
	g.updateStaticListBinding(varName, s.Value)
	g.line(fmt.Sprintf("%s=%s", varName, val))
	if ref, ok := g.objectRefForBinding(varName, s.Value); ok {
		g.objAliasMap[s.Name] = ref
	} else {
		delete(g.objAliasMap, s.Name)
	}
	return nil
}

func (g *Generator) genFetchResponseBinding(varName string, expr ast.Expression) (bool, error) {
	switch e := expr.(type) {
	case *ast.AsExpr:
		return g.genFetchResponseBinding(varName, e.Expr)
	case *ast.BuiltinCallExpr:
		if e.Name != "fetch" {
			return false, nil
		}
		body, err := g.genFetchCall(e)
		if err != nil {
			return true, err
		}
		g.line(fmt.Sprintf("%s=%s", varName, shellQuote(varName)))
		g.line(fmt.Sprintf("%s=%s", objectPropVar(varName, "body"), body))
		g.varTypeMap[varName] = &ast.Type{Kind: ast.TypeFetchResponse}
		g.objPropTypeMap[varName+".body"] = typeString
		return true, nil
	case *ast.IdentExpr:
		sourceName := g.resolveVarName(e.Name)
		if typ := g.varTypeMap[sourceName]; typ == nil || typ.Kind != ast.TypeFetchResponse {
			return false, nil
		}
		g.line(fmt.Sprintf("%s=%s", varName, shellQuote(varName)))
		g.line(fmt.Sprintf("%s=\"$%s\"", objectPropVar(varName, "body"), objectPropVar(sourceName, "body")))
		g.varTypeMap[varName] = &ast.Type{Kind: ast.TypeFetchResponse}
		g.objPropTypeMap[varName+".body"] = typeString
		return true, nil
	}
	return false, nil
}

func (g *Generator) resolveVarName(name string) string {
	if mangled, ok := g.paramMap[name]; ok {
		return mangled
	}
	if global, ok := g.globalVarMap[name]; ok {
		return global
	}
	if g.inFunction {
		mangled := fnParamVar(g.currentFn, name)
		return mangled
	}
	return name
}

func (g *Generator) genFnDecl(s *ast.FnDecl) error {
	fnName := mangle(s.Name)
	g.line(fmt.Sprintf("%s() {", fnName))
	g.push()

	prevFn := g.currentFn
	prevInFn := g.inFunction
	prevParamMap := g.paramMap
	prevObjAliasMap := g.objAliasMap
	prevFnParamTypes := g.fnParamTypes
	prevObjFieldsMap := g.objFieldsMap
	prevObjPropTypeMap := g.objPropTypeMap
	prevStringConstMap := g.stringConstMap
	g.currentFn = s.Name
	g.inFunction = true
	g.paramMap = make(map[string]string)
	g.objAliasMap = cloneObjectRefMap(prevObjAliasMap)
	g.fnParamTypes = make(map[string]*ast.Type)
	g.objFieldsMap = cloneObjectFieldsMap(prevObjFieldsMap)
	g.objPropTypeMap = cloneTypeMap(prevObjPropTypeMap)
	g.stringConstMap = cloneStringMap(prevStringConstMap)

	for i, param := range s.Params {
		varName := fnParamVar(s.Name, param.Name)
		g.paramMap[param.Name] = varName
		delete(g.objAliasMap, param.Name)
		if param.Type != nil {
			g.fnParamTypes[param.Name] = param.Type
			g.varTypeMap[varName] = param.Type
		}
		g.line(fmt.Sprintf("%s=\"$%d\"", varName, i+1))
	}

	if len(s.Params) > 0 {
		g.line("")
	}

	for _, stmt := range s.Body.Statements {
		if err := g.genStmt(stmt); err != nil {
			return err
		}
	}

	g.currentFn = prevFn
	g.inFunction = prevInFn
	g.paramMap = prevParamMap
	g.objAliasMap = prevObjAliasMap
	g.fnParamTypes = prevFnParamTypes
	g.objFieldsMap = prevObjFieldsMap
	g.objPropTypeMap = prevObjPropTypeMap
	g.stringConstMap = prevStringConstMap
	g.pop()
	g.line("}")
	g.line("")
	return nil
}

func (g *Generator) genClassDecl(c *ast.ClassDecl) error {
	g.registerClass(c)
	for _, prop := range c.Properties {
		g.line(fmt.Sprintf("%s() { _bst_slot=$1; %s; eval \"printf '%%s' \\\"\\${_obj_${_bst_slot}_%s}\\\"\"; }", classMethodName(c.Name, "get_"+prop.Name), computedKeyValidation("_bst_slot"), prop.Name))
		g.line(fmt.Sprintf("%s() { _bst_slot=$1; %s; eval \"_obj_${_bst_slot}_%s=\\\"\\$2\\\"\"; }", classMethodName(c.Name, "set_"+prop.Name), computedKeyValidation("_bst_slot"), prop.Name))
	}
	if len(c.Properties) > 0 {
		g.line("")
	}
	for _, prop := range c.StaticProps {
		if obj, ok := prop.Value.(*ast.ObjectLit); ok {
			staticVar := classPropVar(c.Name, prop.Name)
			var fields []string
			for _, field := range obj.Fields {
				if err := validateStaticObjectKey(field.Key); err != nil {
					return err
				}
				fields = append(fields, field.Key)
				val, err := g.genExprRHS(field.Value, nil)
				if err != nil {
					return err
				}
				if pt := g.inferReceiverType(field.Value); pt != nil {
					g.objPropTypeMap[staticVar+"."+field.Key] = pt
				}
				g.line(fmt.Sprintf("%s=%s", objectPropVar(staticVar, field.Key), val))
			}
			g.objFieldsMap[staticVar] = fields
			g.emitObjectKeysInit(staticVar, fields)
			g.line(fmt.Sprintf("%s=%s", staticVar, shellQuote(staticVar)))
			continue
		}
		val := `""`
		if prop.Value != nil {
			var err error
			val, err = g.genExprRHS(prop.Value, prop.Type)
			if err != nil {
				return err
			}
		}
		g.line(fmt.Sprintf("%s=%s", classPropVar(c.Name, prop.Name), val))
	}
	if len(c.StaticProps) > 0 {
		g.line("")
	}
	for i := range c.Accessors {
		if err := g.genClassAccessorDecl(c, &c.Accessors[i]); err != nil {
			return err
		}
	}
	if c.Constructor != nil {
		if err := g.genClassMethodDecl(c, c.Constructor, true); err != nil {
			return err
		}
	} else {
		g.line(fmt.Sprintf("%s() {", classMethodName(c.Name, "constructor")))
		g.push()
		g.line("return 0")
		g.pop()
		g.line("}")
		g.line("")
	}
	for i := range c.Methods {
		if err := g.genClassMethodDecl(c, &c.Methods[i], false); err != nil {
			return err
		}
	}
	return nil
}

func (g *Generator) genClassAccessorDecl(c *ast.ClassDecl, accessor *ast.ClassAccessor) error {
	if accessor.Kind == ast.AccessorGet && methodMutatesThis(accessor.Body.Statements) {
		return fmt.Errorf("getter %q must not assign to this properties", accessor.Name)
	}
	method := &ast.ClassMethod{Pos: accessor.Pos, Name: string(accessor.Kind) + "_" + accessor.Name, IsStatic: accessor.IsStatic, Params: accessor.Params, ReturnType: accessor.ReturnType, Body: accessor.Body}
	return g.genClassMethodDecl(c, method, false)
}

func (g *Generator) genClassMethodDecl(c *ast.ClassDecl, method *ast.ClassMethod, isConstructor bool) error {
	fnName := classMethodName(c.Name, method.Name)
	retType := inferFunctionReturnType(method.Body.Statements)
	if !isConstructor && retType.Kind != ast.TypeVoid && methodMutatesThis(method.Body.Statements) {
		return fmt.Errorf("class method %q returns a value and cannot assign to this properties", method.Name)
	}
	g.genSourceComment(method.Pos)
	g.line(fmt.Sprintf("%s() {", fnName))
	g.push()
	prevFn := g.currentFn
	prevInFn := g.inFunction
	prevParamMap := g.paramMap
	prevFnParamTypes := g.fnParamTypes
	prevObjAliasMap := g.objAliasMap
	prevObjFieldsMap := g.objFieldsMap
	prevObjPropTypeMap := g.objPropTypeMap
	prevStringConstMap := g.stringConstMap
	prevClass := g.currentClass
	prevThis := g.currentThisVar
	g.currentFn = fnName
	g.inFunction = true
	g.paramMap = make(map[string]string)
	g.fnParamTypes = make(map[string]*ast.Type)
	g.objAliasMap = cloneObjectRefMap(prevObjAliasMap)
	g.objFieldsMap = cloneObjectFieldsMap(prevObjFieldsMap)
	g.objPropTypeMap = cloneTypeMap(prevObjPropTypeMap)
	g.stringConstMap = cloneStringMap(prevStringConstMap)
	g.currentClass = c.Name
	g.currentThisVar = ""
	argOffset := 1
	if !method.IsStatic || isConstructor {
		thisVar := fnParamVar(fnName, "this")
		g.currentThisVar = thisVar
		g.paramMap["this"] = thisVar
		delete(g.objAliasMap, "this")
		g.varTypeMap[thisVar] = &ast.Type{Kind: ast.TypeObject}
		g.varClassMap[thisVar] = c.Name
		g.line(fmt.Sprintf("%s=\"$1\"", thisVar))
		argOffset = 2
	}
	for i, param := range method.Params {
		varName := fnParamVar(fnName, param.Name)
		g.paramMap[param.Name] = varName
		delete(g.objAliasMap, param.Name)
		if param.Type != nil {
			g.fnParamTypes[param.Name] = param.Type
			g.varTypeMap[varName] = param.Type
		}
		g.line(fmt.Sprintf("%s=\"$%d\"", varName, i+argOffset))
	}
	if len(method.Params) > 0 || g.currentThisVar != "" {
		g.line("")
	}
	for _, stmt := range method.Body.Statements {
		if err := g.genStmt(stmt); err != nil {
			return err
		}
	}
	g.currentFn = prevFn
	g.inFunction = prevInFn
	g.paramMap = prevParamMap
	g.fnParamTypes = prevFnParamTypes
	g.objAliasMap = prevObjAliasMap
	g.objFieldsMap = prevObjFieldsMap
	g.objPropTypeMap = prevObjPropTypeMap
	g.stringConstMap = prevStringConstMap
	g.currentClass = prevClass
	g.currentThisVar = prevThis
	g.pop()
	g.line("}")
	g.line("")
	return nil
}

func (g *Generator) genIf(s *ast.IfStmt) error {
	cond, err := g.genCondition(s.Condition)
	if err != nil {
		return err
	}
	g.line(fmt.Sprintf("if %s; then", cond))
	g.push()
	for _, stmt := range s.Then.Statements {
		if err := g.genStmt(stmt); err != nil {
			return err
		}
	}
	g.pop()

	for _, ei := range s.ElseIfs {
		eiCond, err := g.genCondition(ei.Condition)
		if err != nil {
			return err
		}
		g.line(fmt.Sprintf("elif %s; then", eiCond))
		g.push()
		for _, stmt := range ei.Body.Statements {
			if err := g.genStmt(stmt); err != nil {
				return err
			}
		}
		g.pop()
	}

	if s.Else != nil {
		g.line("else")
		g.push()
		for _, stmt := range s.Else.Statements {
			if err := g.genStmt(stmt); err != nil {
				return err
			}
		}
		g.pop()
	}
	g.line("fi")
	return nil
}

func (g *Generator) genFor(s *ast.ForStmt) error {
	switch iter := s.Iterator.(type) {
	case *ast.BuiltinCallExpr:
		if iter.Name == "Besht.iter.range" {
			return g.genForRange(s, iter)
		}
	case *ast.IdentExpr:
		if words, ok := g.staticListMap[g.resolveVarName(iter.Name)]; ok {
			return g.genForStaticList(s, words)
		}
		return g.genForList(s, fmt.Sprintf("\"$%s\"", iter.Name), g.listLengthExpr(iter))
	case *ast.CmdExpr:
		pipeline, redirect, err := g.genCmdPipeline(iter)
		if err != nil {
			return err
		}
		return g.genForShell(s, formatCmdForBare(pipeline, redirect))
	case *ast.MethodCallExpr:
		if builtin, ok := beshtBuiltinCall(iter); ok && builtin.Name == "Besht.iter.range" {
			return g.genForRange(s, builtin)
		}
		pipeline, redirect, err := g.genCmdChain(iter)
		if err != nil {
			return err
		}
		return g.genForShell(s, formatCmdForBare(pipeline, redirect))
	case *ast.FnCallExpr:
		callStr, err := g.genFnCallCapture(iter)
		if err != nil {
			return err
		}
		return g.genForShell(s, callStr)
	case *ast.ListLit:
		if words, ok := staticForListWords(iter); ok {
			return g.genForStaticList(s, words)
		}
	}
	iterStr, err := g.genExprValue(s.Iterator)
	if err != nil {
		return err
	}
	return g.genForList(s, iterStr, g.listLengthExpr(s.Iterator))
}

func (g *Generator) genCStyleFor(s *ast.CStyleForStmt) error {
	if err := g.genStmt(s.Init); err != nil {
		return err
	}
	cond, err := g.genCondition(s.Condition)
	if err != nil {
		return err
	}
	g.line(fmt.Sprintf("while %s; do", cond))
	g.push()
	for _, stmt := range s.Body.Statements {
		if err := g.genStmt(stmt); err != nil {
			return err
		}
	}
	if err := g.genStmt(s.Update); err != nil {
		return err
	}
	g.pop()
	g.line("done")
	return nil
}

func (g *Generator) genForRange(s *ast.ForStmt, r *ast.BuiltinCallExpr) error {
	if len(r.Args) != 2 {
		return fmt.Errorf("Besht.iter.range() takes 2 arguments")
	}
	startStr, err := g.genExprValue(r.Args[0])
	if err != nil {
		return err
	}
	endStr, err := g.genExprValue(r.Args[1])
	if err != nil {
		return err
	}

	iVar := g.declareLoopVar(s.VarName)
	g.line(fmt.Sprintf("%s=%s", iVar, startStr))
	g.line(fmt.Sprintf("while [ \"$%s\" -le %s ]; do", iVar, endStr))
	g.push()
	for _, stmt := range s.Body.Statements {
		if err := g.genStmt(stmt); err != nil {
			return err
		}
	}
	g.line(fmt.Sprintf("%s=$(( %s + 1 ))", iVar, iVar))
	g.pop()
	g.line("done")
	g.undeclareLoopVar(s.VarName)
	return nil
}

func staticForListWords(list *ast.ListLit) ([]string, bool) {
	words := make([]string, 0, len(list.Elements))
	for _, elem := range list.Elements {
		switch e := elem.(type) {
		case *ast.StringLit:
			if strings.Contains(e.Value, "\n") {
				return nil, false
			}
			words = append(words, shellQuote(e.Value))
		case *ast.RawStringLit:
			if strings.Contains(e.Value, "\n") {
				return nil, false
			}
			words = append(words, shellQuote(e.Value))
		case *ast.IntLit:
			words = append(words, shellQuote(strconv.FormatInt(e.Value, 10)))
		case *ast.FloatLit:
			words = append(words, shellQuote(e.Value))
		case *ast.BoolLit:
			if e.Value {
				words = append(words, shellQuote("1"))
			} else {
				words = append(words, shellQuote("0"))
			}
		default:
			return nil, false
		}
	}
	return words, true
}

func staticScalarListValues(list *ast.ListLit) ([]string, bool) {
	values := make([]string, 0, len(list.Elements))
	for _, elem := range list.Elements {
		value, ok := staticScalarValue(elem)
		if !ok || strings.Contains(value, "\n") {
			return nil, false
		}
		values = append(values, value)
	}
	return values, true
}

func staticScalarValue(expr ast.Expression) (string, bool) {
	switch e := expr.(type) {
	case *ast.StringLit:
		return e.Value, true
	case *ast.RawStringLit:
		return e.Value, true
	case *ast.IntLit:
		return strconv.FormatInt(e.Value, 10), true
	case *ast.FloatLit:
		return e.Value, true
	case *ast.BoolLit:
		if e.Value {
			return "1", true
		}
		return "0", true
	default:
		return "", false
	}
}

func staticForListWordsExpr(expr ast.Expression) ([]string, bool) {
	switch e := expr.(type) {
	case *ast.ListLit:
		return staticForListWords(e)
	case *ast.AsExpr:
		return staticForListWordsExpr(e.Expr)
	default:
		return nil, false
	}
}

func (g *Generator) updateStaticListBinding(varName string, expr ast.Expression) {
	if g.staticListMap == nil {
		g.staticListMap = make(map[string][]string)
	}
	if words, ok := staticForListWordsExpr(expr); ok {
		g.staticListMap[varName] = words
		return
	}
	delete(g.staticListMap, varName)
}

func staticListLiteralValue(list *ast.ListLit) (string, bool) {
	values := make([]string, 0, len(list.Elements))
	for _, elem := range list.Elements {
		switch e := elem.(type) {
		case *ast.StringLit:
			if strings.Contains(e.Value, "\n") {
				return "", false
			}
			values = append(values, e.Value)
		case *ast.RawStringLit:
			if strings.Contains(e.Value, "\n") {
				return "", false
			}
			values = append(values, e.Value)
		case *ast.IntLit:
			values = append(values, strconv.FormatInt(e.Value, 10))
		case *ast.FloatLit:
			values = append(values, e.Value)
		case *ast.BoolLit:
			if e.Value {
				values = append(values, "1")
			} else {
				values = append(values, "0")
			}
		default:
			return "", false
		}
	}
	return strings.Join(values, "\n"), true
}

func staticStringByteLength(expr ast.Expression) (int, bool) {
	switch e := expr.(type) {
	case *ast.StringLit:
		return len(e.Value), true
	case *ast.RawStringLit:
		return len(e.Value), true
	case *ast.TemplateLit:
		if len(e.Exprs) == 0 {
			return len(strings.Join(e.Parts, "")), true
		}
	case *ast.AsExpr:
		return staticStringByteLength(e.Expr)
	}
	return 0, false
}

func staticScalarListLength(expr ast.Expression) (int, bool) {
	switch e := expr.(type) {
	case *ast.ListLit:
		for _, elem := range e.Elements {
			switch elem.(type) {
			case *ast.StringLit, *ast.RawStringLit, *ast.IntLit, *ast.FloatLit, *ast.BoolLit:
				continue
			default:
				return 0, false
			}
		}
		return len(e.Elements), true
	case *ast.AsExpr:
		return staticScalarListLength(e.Expr)
	}
	return 0, false
}

func (g *Generator) genForStaticList(s *ast.ForStmt, words []string) error {
	if len(words) == 0 {
		return nil
	}
	iVar := g.declareLoopVar(s.VarName)
	g.line(fmt.Sprintf("for %s in %s; do", iVar, strings.Join(words, " ")))
	g.push()
	for _, stmt := range s.Body.Statements {
		if err := g.genStmt(stmt); err != nil {
			return err
		}
	}
	g.pop()
	g.line("done")
	g.undeclareLoopVar(s.VarName)
	return nil
}

func (g *Generator) genForList(s *ast.ForStmt, listExpr, listLen string) error {
	iVar := g.declareLoopVar(s.VarName)
	dataVar := fmt.Sprintf("_forlist_%d_%d", s.Pos.Line, s.Pos.Column)
	g.line(fmt.Sprintf("%s=%s", dataVar, listExpr))
	if listLen != "" {
		idxVar := fmt.Sprintf("_foridx_%d_%d", s.Pos.Line, s.Pos.Column)
		lenVar := fmt.Sprintf("_forlen_%d_%d", s.Pos.Line, s.Pos.Column)
		g.line(fmt.Sprintf("%s=%s", lenVar, listLen))
		g.line(fmt.Sprintf("%s=1", idxVar))
		g.line(fmt.Sprintf("while [ \"$%s\" -le \"$%s\" ]; do", idxVar, lenVar))
		g.push()
		g.line(fmt.Sprintf("%s=$(printf '%%s\\n' \"$%s\" | sed -n \"${%s}p\")", iVar, dataVar, idxVar))
		g.line(fmt.Sprintf("%s=$(( %s + 1 ))", idxVar, idxVar))
		for _, stmt := range s.Body.Statements {
			if err := g.genStmt(stmt); err != nil {
				return err
			}
		}
		g.pop()
		g.line("done")
		g.undeclareLoopVar(s.VarName)
		return nil
	}
	g.line(fmt.Sprintf("while IFS= read -r %s; do", iVar))
	g.push()
	for _, stmt := range s.Body.Statements {
		if err := g.genStmt(stmt); err != nil {
			return err
		}
	}
	g.pop()
	g.line(fmt.Sprintf("done <<__BESHT_FOR_%d_%d", s.Pos.Line, s.Pos.Column))
	g.raw(fmt.Sprintf("$%s\n", dataVar))
	g.raw(fmt.Sprintf("__BESHT_FOR_%d_%d\n", s.Pos.Line, s.Pos.Column))
	g.undeclareLoopVar(s.VarName)
	return nil
}

func (g *Generator) genForShell(s *ast.ForStmt, shellBody string) error {
	iVar := g.declareLoopVar(s.VarName)
	g.line(fmt.Sprintf("{ %s; } | while IFS= read -r %s; do", shellBody, iVar))
	g.push()
	for _, stmt := range s.Body.Statements {
		if err := g.genStmt(stmt); err != nil {
			return err
		}
	}
	g.pop()
	g.line("done")
	g.undeclareLoopVar(s.VarName)
	return nil
}

func (g *Generator) declareLoopVar(name string) string {
	mangled := g.resolveVarName(name)
	g.paramMap[name] = mangled
	return mangled
}

func (g *Generator) undeclareLoopVar(name string) {
	delete(g.paramMap, name)
}

func (g *Generator) genWhile(s *ast.WhileStmt) error {
	cond, err := g.genCondition(s.Condition)
	if err != nil {
		return err
	}
	g.line(fmt.Sprintf("while %s; do", cond))
	g.push()
	for _, stmt := range s.Body.Statements {
		if err := g.genStmt(stmt); err != nil {
			return err
		}
	}
	g.pop()
	g.line("done")
	return nil
}

func (g *Generator) genSwitch(s *ast.SwitchStmt) error {
	value, err := g.genExprValue(s.Value)
	if err != nil {
		return err
	}
	g.line(fmt.Sprintf("case %s in", value))
	g.push()
	for _, swCase := range s.Cases {
		if swCase.IsDefault {
			g.line("*)")
		} else {
			caseValue, err := g.genExprValue(swCase.Value)
			if err != nil {
				return err
			}
			g.line(fmt.Sprintf("%s)", caseValue))
		}
		g.push()
		for _, stmt := range swCase.Body.Statements {
			if _, ok := stmt.(*ast.BreakStmt); ok {
				continue
			}
			if err := g.genStmt(stmt); err != nil {
				return err
			}
		}
		g.line(";;")
		g.pop()
	}
	g.pop()
	g.line("esac")
	return nil
}

func (g *Generator) genTry(s *ast.TryStmt) error {
	statusVar := fmt.Sprintf("_try_status_%d_%d", s.Pos.Line, s.Pos.Column)
	g.line("(")
	g.push()
	g.line("set -e")
	for _, stmt := range s.Body.Statements {
		if err := g.genStmt(stmt); err != nil {
			return err
		}
	}
	g.pop()
	g.line(")")
	g.line(fmt.Sprintf("%s=$?", statusVar))
	g.line(fmt.Sprintf("if [ \"$%s\" -ne 0 ]; then", statusVar))
	g.push()
	catchVar := g.resolveVarName(s.CatchVar)
	g.paramMap[s.CatchVar] = catchVar
	g.varTypeMap[catchVar] = &ast.Type{Kind: ast.TypeStatus}
	g.line(fmt.Sprintf("%s=\"$%s\"", catchVar, statusVar))
	for _, stmt := range s.Catch.Statements {
		if err := g.genStmt(stmt); err != nil {
			return err
		}
	}
	delete(g.paramMap, s.CatchVar)
	g.pop()
	g.line("fi")
	return nil
}

func (g *Generator) genReturn(s *ast.ReturnStmt) error {
	if mapCtx, ok := g.currentMapReturn(); ok {
		if s.Value != nil {
			val, err := g.genExprRHS(s.Value, nil)
			if err != nil {
				return err
			}
			g.line(fmt.Sprintf("printf '%%s\n' %s", ensureArgSafe(val)))
		}
		if mapCtx.indexVar != "" {
			g.line(fmt.Sprintf("%s=$(( %s + 1 ))", mapCtx.indexVar, mapCtx.indexVar))
		}
		g.line("continue")
		return nil
	}
	if reduceCtx, ok := g.currentReduceReturn(); ok {
		if s.Value == nil {
			return nil
		}
		if reduceCtx.accIsObject {
			if ident, ok := s.Value.(*ast.IdentExpr); ok && ident.Name == reduceCtx.accParam {
				return nil
			}
			return fmt.Errorf("reduce() object accumulator callbacks must return the accumulator")
		}
		val, err := g.genExprRHS(s.Value, nil)
		if err != nil {
			return err
		}
		g.line(reduceCtx.accVar + "=" + val)
		return nil
	}

	if s.Value == nil {
		g.line("return 0")
		return nil
	}

	retType := g.fnReturnMap[g.currentFn]
	if retType != nil && retType.Kind == ast.TypeVoid {
		g.line("return 0")
		return nil
	}

	if containsRunCall(s.Value) && g.cmdAnalysis != nil && !isImmediateRunTerminalCall(s.Value) {
		if err := g.emitInlineRunChain(s.Value); err != nil {
			return err
		}
	}

	val, err := g.genExprRHS(s.Value, retType)
	if err != nil {
		return err
	}

	if retType != nil && (retType.Kind == ast.TypeString || retType.Kind == ast.TypeNumber || retType.Kind == ast.TypeList) {
		if strings.HasPrefix(val, "$(") {
			g.line(fmt.Sprintf("printf '%%s' \"%s\"", val))
		} else if strings.HasPrefix(val, "\"") || strings.HasPrefix(val, "'") || strings.HasPrefix(val, "$") {
			g.line(fmt.Sprintf("printf '%%s' %s", val))
		} else {
			g.line(fmt.Sprintf("printf '%%s' \"%s\"", val))
		}
	} else {
		g.line(fmt.Sprintf("printf '%%s' %s", val))
	}
	g.line("return 0")
	return nil
}

func (g *Generator) genExit(s *ast.ExitStmt) error {
	if s.Code == nil {
		g.line("exit 0")
		return nil
	}
	codeStr, err := g.genExprValue(s.Code)
	if err != nil {
		return err
	}
	g.line(fmt.Sprintf("exit %s", codeStr))
	return nil
}

func (g *Generator) genProcessExit(e *ast.MethodCallExpr) error {
	if len(e.Args) > 1 {
		return fmt.Errorf("process.exit() takes 0 or 1 argument")
	}
	if len(e.Args) == 0 {
		g.line("exit 0")
		return nil
	}
	codeStr, err := g.genExprValue(e.Args[0])
	if err != nil {
		return err
	}
	g.line(fmt.Sprintf("exit %s", codeStr))
	return nil
}

func (g *Generator) genExprStmt(s *ast.ExprStmt) error {
	stmtExpr := unwrapAsExpr(s.Expr)
	switch e := stmtExpr.(type) {
	case *ast.CmdExpr:
		// Bare $() as a statement with no .run() — emit nothing (warning already issued)
		return nil
	case *ast.MethodCallExpr:
		if ident, ok := e.Receiver.(*ast.IdentExpr); ok && ident.Name == "process" && e.Method == "exit" {
			return g.genProcessExit(e)
		}
		if e.Optional {
			if className, ok := g.receiverClassName(e.Receiver); ok {
				return g.genOptionalClassMethodStmt(className, e)
			}
			return g.genDiscardExprStmt(s.Expr)
		}
		if className, ok := g.receiverClassName(e.Receiver); ok {
			return g.genClassMethodStmt(className, e)
		}
		if recvType := g.inferReceiverType(e.Receiver); recvType != nil && recvType.Kind == ast.TypeSet && e.Method == "add" {
			val, err := g.genSetMethod(e)
			if err != nil {
				return err
			}
			g.line(val)
			return nil
		}
		if e.Method == "run" {
			return g.genRunCall(e)
		}
		recvType := g.inferReceiverType(e.Receiver)
		if recvType != nil && recvType.Kind == ast.TypeList {
			switch e.Method {
			case "forEach":
				return g.genListForEachStmt(e)
			case "push", "pop", "shift", "unshift", "concat", "slice", "reverse":
				if ident, ok := e.Receiver.(*ast.IdentExpr); ok {
					recvVar := g.resolveVarName(ident.Name)
					val, err := g.genListMethod(fmt.Sprintf("\"$%s\"", recvVar), e)
					if err != nil {
						return err
					}
					g.line(fmt.Sprintf("%s=%s", recvVar, val))
					return nil
				}
			}
		}
		if isLazyCmdChain(e) {
			return nil
		}
		isCmd := isCmdReceiver(e.Receiver) || e.Method == "pipe" || e.Method == "stdout" || e.Method == "stderr" || e.Method == "workdir" || e.Method == "readStdoutLines" || e.Method == "readStdout" || e.Method == "readStderr"
		if isCmd {
			pipeline, redirect, err := g.genCmdChain(e)
			if err != nil {
				return err
			}
			g.line(formatCmdForBare(pipeline, redirect))
			return nil
		}
		if containsOptionalChainExpr(s.Expr) {
			return g.genDiscardExprStmt(s.Expr)
		}
		val, err := g.genExprValue(s.Expr)
		if err != nil {
			return err
		}
		g.line(val)
		return nil
	case *ast.FnCallExpr:
		retType, hasRet := g.fnReturnMap[e.Name]
		isVoid := !hasRet || (retType != nil && retType.Kind == ast.TypeVoid)

		argStrs, err := g.genFnArgs(e.Args)
		if err != nil {
			return err
		}
		fnName := mangle(e.Name)
		if isVoid {
			g.line(fmt.Sprintf("%s %s", fnName, strings.Join(argStrs, " ")))
		} else {
			g.line(fmt.Sprintf("%s %s", fnName, strings.Join(argStrs, " ")))
		}
		return nil
	case *ast.BuiltinCallExpr:
		stmt, err := g.genBuiltinStmt(e)
		if err != nil {
			return err
		}
		if stmt != "" {
			g.line(stmt)
		}
		return nil
	}
	if containsOptionalChainExpr(s.Expr) {
		return g.genDiscardExprStmt(s.Expr)
	}
	val, err := g.genExprValue(s.Expr)
	if err != nil {
		return err
	}
	if val != "" {
		g.line(val)
	}
	return nil
}

func unwrapAsExpr(expr ast.Expression) ast.Expression {
	for {
		asExpr, ok := expr.(*ast.AsExpr)
		if !ok {
			return expr
		}
		expr = asExpr.Expr
	}
}

func containsOptionalChainExpr(expr ast.Expression) bool {
	switch e := expr.(type) {
	case *ast.PropertyExpr:
		return e.Optional || containsOptionalChainExpr(e.Receiver)
	case *ast.IndexExpr:
		return e.Optional || containsOptionalChainExpr(e.Expr) || containsOptionalChainExpr(e.Index)
	case *ast.MethodCallExpr:
		if e.Optional || containsOptionalChainExpr(e.Receiver) {
			return true
		}
		for _, arg := range e.Args {
			if containsOptionalChainExpr(arg) {
				return true
			}
		}
	case *ast.BinaryExpr:
		return containsOptionalChainExpr(e.Left) || containsOptionalChainExpr(e.Right)
	case *ast.TernaryExpr:
		return containsOptionalChainExpr(e.Condition) || containsOptionalChainExpr(e.Then) || containsOptionalChainExpr(e.Else)
	case *ast.UnaryExpr:
		return containsOptionalChainExpr(e.Expr)
	case *ast.PropagateExpr:
		return containsOptionalChainExpr(e.Expr)
	case *ast.SpreadExpr:
		return containsOptionalChainExpr(e.Expr)
	case *ast.AsExpr:
		return containsOptionalChainExpr(e.Expr)
	case *ast.ListLit:
		for _, elem := range e.Elements {
			if containsOptionalChainExpr(elem) {
				return true
			}
		}
	case *ast.ObjectLit:
		for _, field := range e.Fields {
			if containsOptionalChainExpr(field.Value) {
				return true
			}
		}
	case *ast.TemplateLit:
		for _, expr := range e.Exprs {
			if containsOptionalChainExpr(expr) {
				return true
			}
		}
	case *ast.FnCallExpr:
		for _, arg := range e.Args {
			if containsOptionalChainExpr(arg) {
				return true
			}
		}
	case *ast.BuiltinCallExpr:
		for _, arg := range e.Args {
			if containsOptionalChainExpr(arg) {
				return true
			}
		}
	}
	return false
}

func (g *Generator) genDiscardExprStmt(expr ast.Expression) error {
	val, err := g.genExprValue(expr)
	if err != nil {
		return err
	}
	if val != "" {
		g.line(": " + ensureArgSafe(val))
	}
	return nil
}

func (g *Generator) genBuiltinStmt(e *ast.BuiltinCallExpr) (string, error) {
	switch e.Name {
	case "console.log":
		if len(e.Args) == 1 && g.isObjectArg(e.Args[0]) {
			return g.genConsoleLogObject(e.Args[0])
		}
		if len(e.Args) == 1 && g.isListArg(e.Args[0]) {
			return g.genConsoleList(e.Args[0], "stdout")
		}
		parts, err := g.genArgs(e.Args)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("printf '%%s\\n' %s", strings.Join(parts, " ")), nil
	case "console.error":
		if len(e.Args) == 1 && g.isObjectArg(e.Args[0]) {
			return g.genConsoleErrorObject(e.Args[0])
		}
		if len(e.Args) == 1 && g.isListArg(e.Args[0]) {
			return g.genConsoleList(e.Args[0], "stderr")
		}
		parts, err := g.genArgs(e.Args)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("printf '%%s\\n' %s >&2", strings.Join(parts, " ")), nil
	}
	return "", nil
}

func (g *Generator) isObjectArg(expr ast.Expression) bool {
	t := g.inferReceiverType(expr)
	return t != nil && t.Kind == ast.TypeObject
}

func (g *Generator) isListArg(expr ast.Expression) bool {
	t := g.inferReceiverType(expr)
	return t != nil && t.Kind == ast.TypeList
}

func (g *Generator) genConsoleList(expr ast.Expression, dest string) (string, error) {
	val, err := g.genExprValue(expr)
	if err != nil {
		return "", err
	}
	redirect := ""
	if dest == "stderr" {
		redirect = " >&2"
	}
	return fmt.Sprintf(`{ printf '[ '; if [ -n %s ]; then printf '%%s\n' %s | awk 'BEGIN{first=1}{if(!first)printf ", "; printf "%%s",$0; first=0}'; fi; printf ' ]\n'; }%s`, val, val, redirect), nil
}

func (g *Generator) genConsoleLogObject(expr ast.Expression) (string, error) {
	varName := g.resolveObjectVarName(expr)
	if varName == "" {
		return "", fmt.Errorf("cannot log object: unable to resolve variable name")
	}
	return g.genObjectPrintCode(varName, "stdout"), nil
}

func (g *Generator) genConsoleErrorObject(expr ast.Expression) (string, error) {
	varName := g.resolveObjectVarName(expr)
	if varName == "" {
		return "", fmt.Errorf("cannot log object: unable to resolve variable name")
	}
	return g.genObjectPrintCode(varName, "stderr"), nil
}

func (g *Generator) resolveObjectVarName(expr ast.Expression) string {
	if ref, ok := g.resolveObjectRef(expr); ok && ref.StaticName != "" {
		return ref.StaticName
	}
	return ""
}

func (g *Generator) resolveObjectRef(expr ast.Expression) (objectRef, bool) {
	if as, ok := expr.(*ast.AsExpr); ok {
		return g.resolveObjectRef(as.Expr)
	}
	if ident, ok := expr.(*ast.IdentExpr); ok {
		if ref, ok := g.objAliasMap[ident.Name]; ok {
			return ref, true
		}
		if paramName, ok := g.paramMap[ident.Name]; ok {
			if typ, ok := g.fnParamTypes[ident.Name]; ok && typ.Kind == ast.TypeObject {
				return objectRef{SlotExpr: fmt.Sprintf("\"$%s\"", paramName), RootName: ident.Name}, true
			}
		}
		varName := g.resolveVarName(ident.Name)
		if typ, ok := g.varTypeMap[varName]; ok && typ.Kind == ast.TypeObject {
			return objectRef{StaticName: varName, RootName: varName}, true
		}
		if typ, ok := g.varTypeMap[ident.Name]; ok && typ.Kind == ast.TypeObject {
			return objectRef{StaticName: ident.Name, RootName: ident.Name}, true
		}
	}
	if prop, ok := expr.(*ast.PropertyExpr); ok {
		if ident, ok := prop.Receiver.(*ast.IdentExpr); ok {
			classDecl := g.classMap[g.resolveClassName(ident.Name)]
			if classDecl == nil {
				classDecl = g.classMap[ident.Name]
			}
			if classDecl != nil {
				staticVar := classPropVar(classDecl.Name, prop.Property)
				if typ, ok := g.varTypeMap[staticVar]; ok && typ.Kind == ast.TypeObject {
					return objectRef{StaticName: staticVar, RootName: staticVar}, true
				}
			}
		}
	}
	return objectRef{}, false
}

func objectKeysLiteral(fields []string) string {
	return shellQuote(strings.Join(uniqueStrings(fields), " "))
}

func validateStaticObjectKey(key string) error {
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

func uniqueStrings(values []string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, value := range values {
		if seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func appendUniqueString(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func stringLiteralValue(expr ast.Expression) (string, bool) {
	switch e := expr.(type) {
	case *ast.AsExpr:
		return stringLiteralValue(e.Expr)
	case *ast.StringLit:
		return e.Value, true
	case *ast.RawStringLit:
		return e.Value, true
	}
	return "", false
}

func (g *Generator) emitObjectKeysInit(varName string, fields []string) {
	g.line(fmt.Sprintf("_objkeys_%s=%s", varName, objectKeysLiteral(fields)))
}

func (g *Generator) emitObjectKeyAppend(varName, key string) {
	g.line(fmt.Sprintf("case \" $_objkeys_%s \" in *\" %s \"*) ;; *) _objkeys_%s=\"${_objkeys_%s} %s\" ;; esac", varName, key, varName, varName, key))
}

func (g *Generator) emitObjectKeyAppendRef(ref objectRef, key string, pos ast.Pos) {
	if ref.StaticName != "" {
		g.emitObjectKeyAppend(ref.StaticName, key)
		return
	}
	slotVar := fmt.Sprintf("_objs_%d_%d", pos.Line, pos.Column)
	g.line(slotVar + "=" + ref.SlotExpr)
	g.line(computedKeyValidation(slotVar))
	g.line("eval \"_bst_obj_keys=\\\"\\${_objkeys_${" + slotVar + "}}\\\"\"")
	g.line("case \" $_bst_obj_keys \" in *\" " + key + " \"*) ;; *) _bst_obj_keys=\"${_bst_obj_keys} " + key + "\"; eval \"_objkeys_${" + slotVar + "}=\\\"\\$_bst_obj_keys\\\"\" ;; esac")
}

func objectKeysExpr(varName string) string {
	return fmt.Sprintf("$(if [ -n \"$_objkeys_%s\" ]; then printf '%%s\\n' $_objkeys_%s; fi)", varName, varName)
}

func objectKeysSlotExpr(slotExpr string) string {
	return fmt.Sprintf("$(_bst_obj_slot=%s; %s; eval \"_bst_obj_keys=\\\"\\${_objkeys_${_bst_obj_slot}}\\\"\"; if [ -n \"$_bst_obj_keys\" ]; then printf '%%s\\n' $_bst_obj_keys; fi)", slotExpr, computedKeyValidation("_bst_obj_slot"))
}

func objectKeysRefExpr(ref objectRef) string {
	if ref.StaticName != "" {
		return objectKeysExpr(ref.StaticName)
	}
	return objectKeysSlotExpr(ref.SlotExpr)
}

func (g *Generator) objectBooleanValueCase(varName string) string {
	var fields []string
	for _, field := range g.objFieldsMap[varName] {
		if typ := g.objPropTypeMap[varName+"."+field]; typ != nil && typ.Kind == ast.TypeBoolean {
			fields = append(fields, fmt.Sprintf("[ \"$_bst_obj_key\" = %s ]", shellQuote(field)))
		}
	}
	if len(fields) == 0 {
		return ""
	}
	return fmt.Sprintf("if %s; then if [ \"$_bst_obj_value\" = 1 ]; then _bst_obj_value=true; else _bst_obj_value=false; fi; fi; ", strings.Join(fields, " || "))
}

func (g *Generator) objectValuesLoop(varName string) string {
	return fmt.Sprintf("for _bst_obj_key in $_objkeys_%s; do %s; eval \"_bst_obj_value=\\\"\\${_obj_%s_${_bst_obj_key}}\\\"\"; %sprintf '%%s\\n' \"$_bst_obj_value\"; done", varName, computedKeyValidation("_bst_obj_key"), varName, g.objectBooleanValueCase(varName))
}

func (g *Generator) objectValuesExpr(varName string) string {
	return fmt.Sprintf("$(%s)", g.objectValuesLoop(varName))
}

func objectValuesSlotExpr(slotExpr string) string {
	return fmt.Sprintf("$(_bst_obj_slot=%s; %s; eval \"_bst_obj_keys=\\\"\\${_objkeys_${_bst_obj_slot}}\\\"\"; for _bst_obj_key in $_bst_obj_keys; do %s; eval \"printf '%%s\\n' \\\"\\${_obj_${_bst_obj_slot}_${_bst_obj_key}}\\\"\"; done)", slotExpr, computedKeyValidation("_bst_obj_slot"), computedKeyValidation("_bst_obj_key"))
}

func (g *Generator) objectValuesRefExpr(ref objectRef) string {
	if ref.StaticName != "" {
		return g.objectValuesExpr(ref.StaticName)
	}
	return objectValuesSlotExpr(ref.SlotExpr)
}

func (g *Generator) objectEntriesLoop(varName string) string {
	return fmt.Sprintf("for _bst_obj_key in $_objkeys_%s; do %s; eval \"_bst_obj_value=\\\"\\${_obj_%s_${_bst_obj_key}}\\\"\"; %sprintf '%%s\\037%%s\\n' \"$_bst_obj_key\" \"$_bst_obj_value\"; done", varName, computedKeyValidation("_bst_obj_key"), varName, g.objectBooleanValueCase(varName))
}

func (g *Generator) objectEntriesExpr(varName string) string {
	return fmt.Sprintf("$(%s)", g.objectEntriesLoop(varName))
}

func objectEntriesSlotExpr(slotExpr string) string {
	return fmt.Sprintf("$(_bst_obj_slot=%s; %s; eval \"_bst_obj_keys=\\\"\\${_objkeys_${_bst_obj_slot}}\\\"\"; for _bst_obj_key in $_bst_obj_keys; do %s; eval \"_bst_obj_value=\\\"\\${_obj_${_bst_obj_slot}_${_bst_obj_key}}\\\"\"; printf '%%s\\037%%s\\n' \"$_bst_obj_key\" \"$_bst_obj_value\"; done)", slotExpr, computedKeyValidation("_bst_obj_slot"), computedKeyValidation("_bst_obj_key"))
}

func (g *Generator) objectEntriesRefExpr(ref objectRef) string {
	if ref.StaticName != "" {
		return g.objectEntriesExpr(ref.StaticName)
	}
	return objectEntriesSlotExpr(ref.SlotExpr)
}

func unsupportedObjectValueType(t *ast.Type) bool {
	if t == nil {
		return false
	}
	switch t.Kind {
	case ast.TypeList, ast.TypeSet, ast.TypeObject, ast.TypeCommand, ast.TypeFetchResponse:
		return true
	}
	return false
}

func objectScalarValuesError(api string) error {
	return fmt.Errorf("%s() only supports scalar object values", api)
}

func (g *Generator) validateObjectLiteralScalarValues(api string, obj *ast.ObjectLit) error {
	for _, field := range obj.Fields {
		if unsupportedObjectValueType(g.inferReceiverType(field.Value)) {
			return objectScalarValuesError(api)
		}
	}
	return nil
}

func (g *Generator) validateObjectRefScalarValues(api string, ref objectRef) error {
	if ref.UnsupportedScalarValues {
		return objectScalarValuesError(api)
	}
	if ref.StaticName == "" {
		return nil
	}
	if unsupportedObjectValueType(g.objPropTypeMap[ref.StaticName+".*"]) {
		return objectScalarValuesError(api)
	}
	for _, field := range g.objFieldsMap[ref.StaticName] {
		if unsupportedObjectValueType(g.objPropTypeMap[ref.StaticName+"."+field]) {
			return objectScalarValuesError(api)
		}
	}
	return nil
}

func (g *Generator) objectRefHasUnsupportedScalarValues(ref objectRef) bool {
	if ref.UnsupportedScalarValues {
		return true
	}
	if ref.RootName != "" {
		if rootRef, ok := g.objAliasMap[ref.RootName]; ok && rootRef.UnsupportedScalarValues {
			return true
		}
	}
	if ref.StaticName == "" {
		return false
	}
	if unsupportedObjectValueType(g.objPropTypeMap[ref.StaticName+".*"]) {
		return true
	}
	for _, field := range g.objFieldsMap[ref.StaticName] {
		if unsupportedObjectValueType(g.objPropTypeMap[ref.StaticName+"."+field]) {
			return true
		}
	}
	return false
}

func (g *Generator) recordObjectStaticFieldType(varName, field string, value ast.Expression) {
	pt := g.inferReceiverType(value)
	if pt != nil {
		g.objPropTypeMap[varName+"."+field] = pt
	} else {
		delete(g.objPropTypeMap, varName+"."+field)
	}
	g.objFieldsMap[varName] = appendUniqueString(g.objFieldsMap[varName], field)
}

func (g *Generator) recordObjectAssignmentType(ref objectRef, field string, value ast.Expression, dynamicField bool) objectRef {
	pt := g.inferReceiverType(value)
	if ref.StaticName != "" {
		if dynamicField {
			for _, knownField := range g.objFieldsMap[ref.StaticName] {
				if typ := g.objPropTypeMap[ref.StaticName+"."+knownField]; typ != nil && typ.Kind == ast.TypeBoolean {
					g.objPropTypeMap[ref.StaticName+"."+knownField] = typeString
				}
			}
			if unsupportedObjectValueType(pt) {
				g.objPropTypeMap[ref.StaticName+".*"] = pt
			}
		} else {
			g.recordObjectStaticFieldType(ref.StaticName, field, value)
			if unsupportedObjectValueType(pt) {
				g.objPropTypeMap[ref.StaticName+".*"] = pt
			}
		}
		ref.UnsupportedScalarValues = g.objectRefHasUnsupportedScalarValues(ref)
		return ref
	}
	if unsupportedObjectValueType(pt) {
		ref.UnsupportedScalarValues = true
		g.markObjectRootUnsupported(ref)
	}
	return ref
}

func (g *Generator) updateObjectAliasRef(name string, ref objectRef) {
	if ref.RootName == "" {
		if ref.StaticName != "" {
			ref.RootName = ref.StaticName
		} else {
			ref.RootName = name
		}
	}
	if _, ok := g.objAliasMap[name]; ok || ref.UnsupportedScalarValues || ref.RootName != "" {
		g.objAliasMap[name] = ref
	}
}

func (g *Generator) markObjectRootUnsupported(ref objectRef) {
	if ref.RootName == "" {
		return
	}
	rootRef := ref
	if existing, ok := g.objAliasMap[ref.RootName]; ok {
		rootRef = existing
	}
	rootRef.RootName = ref.RootName
	rootRef.UnsupportedScalarValues = true
	g.objAliasMap[ref.RootName] = rootRef
}

func (g *Generator) staticObjectKeyExpr(expr ast.Expression) (string, bool) {
	switch e := expr.(type) {
	case *ast.AsExpr:
		return g.staticObjectKeyExpr(e.Expr)
	case *ast.StringLit:
		return e.Value, true
	case *ast.RawStringLit:
		return e.Value, true
	case *ast.IdentExpr:
		value, ok := g.stringConstMap[g.resolveVarName(e.Name)]
		return value, ok
	}
	return "", false
}

func objectHasOwnMembershipExpr(keysExpr, keyExpr string) string {
	return fmt.Sprintf("$(_bst_obj_key=%s; if [ -z \"$_bst_obj_key\" ] || printf '%%s\\n' \"$_bst_obj_key\" | grep -q '[^A-Za-z0-9_]'; then printf 0; elif printf '%%s\\n' %s | grep -qxF -- \"$_bst_obj_key\"; then printf 1; else printf 0; fi)", ensureArgSafe(keyExpr), keysExpr)
}

func objectHasOwnExpr(varName, keyExpr string) string {
	return objectHasOwnMembershipExpr("$_objkeys_"+varName, keyExpr)
}

func objectHasOwnSlotExpr(slotExpr, keyExpr string) string {
	return fmt.Sprintf("$(_bst_obj_slot=%s; %s; eval \"_bst_obj_keys=\\\"\\${_objkeys_${_bst_obj_slot}}\\\"\"; _bst_obj_key=%s; if [ -z \"$_bst_obj_key\" ] || printf '%%s\\n' \"$_bst_obj_key\" | grep -q '[^A-Za-z0-9_]'; then printf 0; elif printf '%%s\\n' $_bst_obj_keys | grep -qxF -- \"$_bst_obj_key\"; then printf 1; else printf 0; fi)", slotExpr, computedKeyValidation("_bst_obj_slot"), ensureArgSafe(keyExpr))
}

func objectHasOwnRefExpr(ref objectRef, keyExpr string) string {
	if ref.StaticName != "" {
		return objectHasOwnExpr(ref.StaticName, keyExpr)
	}
	return objectHasOwnSlotExpr(ref.SlotExpr, keyExpr)
}

func (g *Generator) genObjectKeys(expr ast.Expression) (string, error) {
	if as, ok := expr.(*ast.AsExpr); ok {
		return g.genObjectKeys(as.Expr)
	}
	if obj, ok := expr.(*ast.ObjectLit); ok {
		return g.genObjectLiteralKeys(obj)
	}
	if ref, ok := g.resolveObjectRef(expr); ok {
		if ref.StaticName == "" {
			return objectKeysRefExpr(ref), nil
		}
		if _, ok := g.objFieldsMap[ref.StaticName]; ok {
			return objectKeysRefExpr(ref), nil
		}
		if typ, ok := g.varTypeMap[ref.StaticName]; ok && typ.Kind == ast.TypeObject {
			return objectKeysRefExpr(ref), nil
		}
	}
	return "", fmt.Errorf("Object.keys() requires an object literal or named object")
}

func (g *Generator) genObjectValues(expr ast.Expression) (string, error) {
	if as, ok := expr.(*ast.AsExpr); ok {
		return g.genObjectValues(as.Expr)
	}
	if obj, ok := expr.(*ast.ObjectLit); ok {
		if err := g.validateObjectLiteralScalarValues("Object.values", obj); err != nil {
			return "", err
		}
		return g.genObjectLiteralValues(obj)
	}
	if ref, ok := g.resolveObjectRef(expr); ok {
		if err := g.validateObjectRefScalarValues("Object.values", ref); err != nil {
			return "", err
		}
		if ref.StaticName == "" {
			return g.objectValuesRefExpr(ref), nil
		}
		if _, ok := g.objFieldsMap[ref.StaticName]; ok {
			return g.objectValuesRefExpr(ref), nil
		}
		if typ, ok := g.varTypeMap[ref.StaticName]; ok && typ.Kind == ast.TypeObject {
			return g.objectValuesRefExpr(ref), nil
		}
	}
	return "", fmt.Errorf("Object.values() requires an object literal or named object")
}

func (g *Generator) genObjectEntries(expr ast.Expression) (string, error) {
	if as, ok := expr.(*ast.AsExpr); ok {
		return g.genObjectEntries(as.Expr)
	}
	if obj, ok := expr.(*ast.ObjectLit); ok {
		if err := g.validateObjectLiteralScalarValues("Object.entries", obj); err != nil {
			return "", err
		}
		return g.genObjectLiteralEntries(obj)
	}
	if ref, ok := g.resolveObjectRef(expr); ok {
		if err := g.validateObjectRefScalarValues("Object.entries", ref); err != nil {
			return "", err
		}
		if ref.StaticName == "" {
			return g.objectEntriesRefExpr(ref), nil
		}
		if _, ok := g.objFieldsMap[ref.StaticName]; ok {
			return g.objectEntriesRefExpr(ref), nil
		}
		if typ, ok := g.varTypeMap[ref.StaticName]; ok && typ.Kind == ast.TypeObject {
			return g.objectEntriesRefExpr(ref), nil
		}
	}
	return "", fmt.Errorf("Object.entries() requires an object literal or named object")
}

func (g *Generator) genObjectHasOwn(objExpr, keyExpr ast.Expression) (string, error) {
	if as, ok := objExpr.(*ast.AsExpr); ok {
		return g.genObjectHasOwn(as.Expr, keyExpr)
	}
	keyStr, err := g.genExprValue(keyExpr)
	if err != nil {
		return "", err
	}
	if obj, ok := objExpr.(*ast.ObjectLit); ok {
		return g.genObjectLiteralHasOwn(obj, keyStr)
	}
	if ref, ok := g.resolveObjectRef(objExpr); ok {
		if ref.StaticName == "" {
			return objectHasOwnRefExpr(ref, keyStr), nil
		}
		if _, ok := g.objFieldsMap[ref.StaticName]; ok {
			return objectHasOwnRefExpr(ref, keyStr), nil
		}
		if typ, ok := g.varTypeMap[ref.StaticName]; ok && typ.Kind == ast.TypeObject {
			return objectHasOwnRefExpr(ref, keyStr), nil
		}
	}
	return "", fmt.Errorf("Object.hasOwn() requires an object literal or named object")
}

func (g *Generator) genObjectLiteralHasOwn(obj *ast.ObjectLit, keyStr string) (string, error) {
	tmp := fmt.Sprintf("_objlit_%d_%d", obj.Pos.Line, obj.Pos.Column)
	var fields []string
	var cmds []string
	for _, field := range obj.Fields {
		if err := validateStaticObjectKey(field.Key); err != nil {
			return "", err
		}
		fields = append(fields, field.Key)
	}
	cmds = append(cmds, fmt.Sprintf("_objkeys_%s=%s", tmp, objectKeysLiteral(fields)))
	for _, field := range obj.Fields {
		val, err := g.genExprRHS(field.Value, nil)
		if err != nil {
			return "", err
		}
		cmds = append(cmds, fmt.Sprintf("%s=%s", objectPropVar(tmp, field.Key), val))
	}
	cmds = append(cmds, strings.TrimPrefix(strings.TrimSuffix(objectHasOwnExpr(tmp, keyStr), ")"), "$("))
	return fmt.Sprintf("$(%s)", strings.Join(cmds, "; ")), nil
}

func (g *Generator) genObjectLiteralKeys(obj *ast.ObjectLit) (string, error) {
	tmp := fmt.Sprintf("_objlit_%d_%d", obj.Pos.Line, obj.Pos.Column)
	var fields []string
	var cmds []string
	for _, field := range obj.Fields {
		if err := validateStaticObjectKey(field.Key); err != nil {
			return "", err
		}
		fields = append(fields, field.Key)
	}
	cmds = append(cmds, fmt.Sprintf("_objkeys_%s=%s", tmp, objectKeysLiteral(fields)))
	for _, field := range obj.Fields {
		val, err := g.genExprRHS(field.Value, nil)
		if err != nil {
			return "", err
		}
		cmds = append(cmds, fmt.Sprintf("%s=%s", objectPropVar(tmp, field.Key), val))
	}
	cmds = append(cmds, fmt.Sprintf("if [ -n \"$_objkeys_%s\" ]; then printf '%%s\\n' $_objkeys_%s; fi", tmp, tmp))
	return fmt.Sprintf("$(%s)", strings.Join(cmds, "; ")), nil
}

func (g *Generator) genObjectLiteralValues(obj *ast.ObjectLit) (string, error) {
	tmp := fmt.Sprintf("_objlit_%d_%d", obj.Pos.Line, obj.Pos.Column)
	cmds, err := g.genObjectLiteralTempCommands(tmp, obj)
	if err != nil {
		return "", err
	}
	cmds = append(cmds, g.objectValuesLoop(tmp))
	return fmt.Sprintf("$(%s)", strings.Join(cmds, "; ")), nil
}

func (g *Generator) genObjectLiteralEntries(obj *ast.ObjectLit) (string, error) {
	tmp := fmt.Sprintf("_objlit_%d_%d", obj.Pos.Line, obj.Pos.Column)
	cmds, err := g.genObjectLiteralTempCommands(tmp, obj)
	if err != nil {
		return "", err
	}
	cmds = append(cmds, g.objectEntriesLoop(tmp))
	return fmt.Sprintf("$(%s)", strings.Join(cmds, "; ")), nil
}

func (g *Generator) genObjectLiteralTempCommands(tmp string, obj *ast.ObjectLit) ([]string, error) {
	var fields []string
	var cmds []string
	for _, field := range obj.Fields {
		if err := validateStaticObjectKey(field.Key); err != nil {
			return nil, err
		}
		fields = append(fields, field.Key)
	}
	cmds = append(cmds, fmt.Sprintf("_objkeys_%s=%s", tmp, objectKeysLiteral(fields)))
	g.objFieldsMap[tmp] = fields
	for _, field := range obj.Fields {
		val, err := g.genExprRHS(field.Value, nil)
		if err != nil {
			return nil, err
		}
		if pt := g.inferReceiverType(field.Value); pt != nil {
			g.objPropTypeMap[tmp+"."+field.Key] = pt
		} else {
			delete(g.objPropTypeMap, tmp+"."+field.Key)
		}
		cmds = append(cmds, fmt.Sprintf("%s=%s", objectPropVar(tmp, field.Key), val))
	}
	return cmds, nil
}

func (g *Generator) genObjectPrintCode(varName, dest string) string {
	redirect := ""
	if dest == "stderr" {
		redirect = " >&2"
	}
	if fields, ok := g.objFieldsMap[varName]; ok && len(fields) > 0 {
		var parts []string
		for i, field := range fields {
			propVar := objectPropVar(varName, field)
			propType := g.objPropTypeMap[varName+"."+field]
			if i > 0 {
				parts = append(parts, ", ")
			}
			if propType != nil && propType.Kind == ast.TypeBoolean {
				parts = append(parts, field+": $(if [ \"$"+propVar+"\" = 1 ]; then printf true; else printf false; fi)")
			} else {
				parts = append(parts, field+": \"$"+propVar+"\"")
			}
		}
		return "printf '{ " + strings.Join(parts, "") + " }\\n'" + redirect
	}
	var lines []string
	lines = append(lines, "printf '{\\n'"+redirect)
	lines = append(lines, "for _bst_k in $_objkeys_"+varName+"; do")
	lines = append(lines, "  if [ -n \"$_bst_k\" ]; then")
	lines = append(lines, "    "+computedKeyValidation("_bst_k"))
	lines = append(lines, "    eval \"_bst_v=\\\"\\${_obj_"+varName+"_${_bst_k}}\\\"\"")
	lines = append(lines, "    printf '  %s: %s,\\n' \"$_bst_k\" \"$_bst_v\""+redirect)
	lines = append(lines, "  fi")
	lines = append(lines, "done")
	lines = append(lines, "printf '}\\n'"+redirect)
	return strings.Join(lines, "\n")
}

func (g *Generator) genExprRHS(expr ast.Expression, targetType *ast.Type) (string, error) {
	switch e := expr.(type) {
	case *ast.StringLit:
		return shellQuote(e.Value), nil
	case *ast.RawStringLit:
		return shellQuote(e.Value), nil
	case *ast.TemplateLit:
		return g.genTemplateLiteral(e)
	case *ast.IntLit:
		return fmt.Sprintf("%d", e.Value), nil
	case *ast.FloatLit:
		return e.Value, nil
	case *ast.BoolLit:
		if e.Value {
			return "1", nil
		}
		return "0", nil
	case *ast.UndefinedLit, *ast.NullLit:
		g.requireRuntimeHelper("nullish")
		return nullishSentinelRef(), nil
	case *ast.ListLit:
		return g.genListLiteral(e)
	case *ast.ObjectLit:
		return `""`, nil
	case *ast.NewExpr:
		return `""`, nil
	case *ast.ThisExpr:
		if g.currentThisVar != "" {
			return fmt.Sprintf("\"$%s\"", g.currentThisVar), nil
		}
		return `""`, nil
	case *ast.IdentExpr:
		varName := g.resolveVarName(e.Name)
		return fmt.Sprintf("\"$%s\"", varName), nil
	case *ast.CmdExpr:
		// CmdExpr in value position: the command is lazy, no code here.
		// Codegen happens when .run() is called.
		// If we can find the capture var from the analysis, return it for chaining.
		if g.cmdAnalysis != nil {
			if id, ok := g.cmdAnalysis.nodeToID[e]; ok {
				ident := g.cmdAnalysis.identity(id)
				if ident != nil && ident.HasRun && (ident.UsesText || ident.UsesLines) {
					varName := ident.CaptureVarName(g.resolveVarName)
					return fmt.Sprintf("\"$%s\"", varName), nil
				}
			}
		}
		return `""`, nil
	case *ast.FnCallExpr:
		return g.genFnCallCapture(e)
	case *ast.BinaryExpr:
		return g.genBinaryRHS(e, targetType)
	case *ast.TernaryExpr:
		return g.genTernaryRHS(e, targetType)
	case *ast.UnaryExpr:
		return g.genUnaryRHS(e)
	case *ast.UpdateExpr:
		return g.genUpdateRHS(e), nil
	case *ast.BuiltinCallExpr:
		return g.genBuiltinCapture(e)
	case *ast.PropagateExpr:
		return g.genPropagateRHS(e)
	case *ast.IndexExpr:
		if e.Optional {
			return g.genOptionalIndexExpr(e)
		}
		return g.genIndexExpr(e)
	case *ast.MethodCallExpr:
		if builtin, ok := beshtBuiltinCall(e); ok {
			return g.genBuiltinCapture(builtin)
		}
		if e.Optional {
			return g.genOptionalMethodCall(e)
		}
		return g.genMethodCall(e)
	case *ast.ArrowExpr:
		return "", fmt.Errorf("arrow expressions can only be used as list callbacks")
	case *ast.PropertyExpr:
		if e.Optional {
			return g.genOptionalProperty(e)
		}
		return g.genProperty(e)
	case *ast.SpreadExpr:
		return g.genExprValue(e.Expr)
	case *ast.AsExpr:
		return g.genExprRHS(e.Expr, e.Type)
	}
	return "", fmt.Errorf("codegen: unknown expression type %T", expr)
}

func (g *Generator) genExprValue(expr ast.Expression) (string, error) {
	return g.genExprRHS(expr, nil)
}

func (g *Generator) genNullishValue(expr ast.Expression) (string, error) {
	if idx, ok := expr.(*ast.IndexExpr); ok {
		if idx.Optional {
			return g.genOptionalIndexExpr(idx)
		}
		return g.genNullishIndexExpr(idx)
	}
	return g.genExprValue(expr)
}

func isNullishLiteral(expr ast.Expression) bool {
	switch expr.(type) {
	case *ast.UndefinedLit, *ast.NullLit:
		return true
	}
	return false
}

func nullishAwareTruthyCondition(val string) string {
	return fmt.Sprintf("{ _bst_cond=%s; [ \"$_bst_cond\" != \"$%s\" ] && [ -n \"$_bst_cond\" ] && [ \"$_bst_cond\" != 0 ]; }", val, nullishSentinelVar)
}

func (g *Generator) withTempReceiver(name string, pos ast.Pos, receiver ast.Expression, fn func(ast.Expression) (string, error)) (string, error) {
	recvType := g.inferReceiverType(receiver)
	oldType, hadOldType := g.varTypeMap[name]
	oldClass, hadOldClass := g.varClassMap[name]
	oldParam, hadOldParam := g.paramMap[name]
	g.paramMap[name] = name
	if recvType != nil {
		g.varTypeMap[name] = recvType
	}
	if className, ok := g.receiverClassName(receiver); ok {
		g.varClassMap[name] = className
	}
	defer func() {
		if hadOldParam {
			g.paramMap[name] = oldParam
		} else {
			delete(g.paramMap, name)
		}
		if hadOldType {
			g.varTypeMap[name] = oldType
		} else {
			delete(g.varTypeMap, name)
		}
		if hadOldClass {
			g.varClassMap[name] = oldClass
		} else {
			delete(g.varClassMap, name)
		}
	}()
	return fn(&ast.IdentExpr{Pos: pos, Name: name})
}

func (g *Generator) genOptionalShell(pos ast.Pos, receiver ast.Expression, body func(tmpName string) (string, error)) (string, error) {
	g.requireRuntimeHelper("nullish")
	recv, err := g.genNullishValue(receiver)
	if err != nil {
		return "", err
	}
	tmpName := fmt.Sprintf("_bst_opt_recv_%d_%d", pos.Line, pos.Column)
	outName := fmt.Sprintf("_bst_opt_out_%d_%d", pos.Line, pos.Column)
	val, err := body(tmpName)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("$(%s=%s; if [ \"$%s\" = \"$%s\" ]; then printf '%%s' \"$%s\"; else %s=%s; printf '%%s' \"$%s\"; fi)", tmpName, recv, tmpName, nullishSentinelVar, nullishSentinelVar, outName, val, outName), nil
}

func (g *Generator) genOptionalProperty(e *ast.PropertyExpr) (string, error) {
	return g.genOptionalShell(e.Pos, e.Receiver, func(tmpName string) (string, error) {
		if isNullishLiteral(e.Receiver) && e.Property != "length" {
			return `""`, nil
		}
		if ident, ok := e.Receiver.(*ast.IdentExpr); ok && e.Property != "length" {
			varName := g.resolveVarName(ident.Name)
			_, hasObjectType := g.varTypeMap[varName]
			_, hasAlias := g.objAliasMap[ident.Name]
			_, hasClass := g.varClassMap[varName]
			if hasObjectType || hasAlias || hasClass {
				clone := *e
				clone.Optional = false
				clone.Receiver = ident
				return g.genProperty(&clone)
			}
			return fmt.Sprintf("\"$%s\"", objectPropVar(varName, e.Property)), nil
		}
		return g.withTempReceiver(tmpName, e.Pos, e.Receiver, func(recv ast.Expression) (string, error) {
			clone := *e
			clone.Optional = false
			clone.Receiver = recv
			return g.genProperty(&clone)
		})
	})
}

func (g *Generator) genOptionalIndexExpr(e *ast.IndexExpr) (string, error) {
	return g.genOptionalShell(e.Pos, e.Expr, func(tmpName string) (string, error) {
		recvType := g.inferReceiverType(e.Expr)
		if recvType != nil && recvType.Kind == ast.TypeObject {
			clone := *e
			clone.Optional = false
			return g.genComputedPropertyAccess(&clone)
		}
		return g.withTempReceiver(tmpName, e.Pos, e.Expr, func(recv ast.Expression) (string, error) {
			clone := *e
			clone.Optional = false
			clone.Expr = recv
			return g.genNullishIndexExpr(&clone)
		})
	})
}

func (g *Generator) genOptionalMethodCall(e *ast.MethodCallExpr) (string, error) {
	return g.genOptionalShell(e.Pos, e.Receiver, func(tmpName string) (string, error) {
		return g.withTempReceiver(tmpName, e.Pos, e.Receiver, func(recv ast.Expression) (string, error) {
			clone := *e
			clone.Optional = false
			clone.Receiver = recv
			return g.genMethodCall(&clone)
		})
	})
}

func (g *Generator) genListLiteral(e *ast.ListLit) (string, error) {
	if len(e.Elements) == 0 {
		return "\"\"", nil
	}
	if value, ok := staticListLiteralValue(e); ok {
		return shellQuote(value), nil
	}
	var cmds []string
	for _, elem := range e.Elements {
		if spread, ok := elem.(*ast.SpreadExpr); ok {
			val, err := g.genExprValue(spread.Expr)
			if err != nil {
				return "", err
			}
			cmds = append(cmds, fmt.Sprintf("if [ -n %s ]; then printf '%%s\\n' %s; fi", val, val))
			continue
		}
		val, err := g.genExprValue(elem)
		if err != nil {
			return "", err
		}
		if elemType := g.inferReceiverType(elem); elemType != nil && elemType.Kind == ast.TypeList {
			cmds = append(cmds, fmt.Sprintf("printf '%%s\\n' %s | awk 'NR>1{printf \"\\037\"}{printf \"%%s\",$0}'; printf '\\n'", ensureArgSafe(val)))
			continue
		}
		cmds = append(cmds, fmt.Sprintf("printf '%%s\\n' %s", val))
	}
	return fmt.Sprintf("$( { %s; } )", strings.Join(cmds, "; ")), nil
}

func (g *Generator) genFnCallCapture(e *ast.FnCallExpr) (string, error) {
	argStrs, err := g.genFnArgs(e.Args)
	if err != nil {
		return "", err
	}
	fnName := mangle(e.Name)
	if len(argStrs) == 0 {
		return fmt.Sprintf("$(%s)", fnName), nil
	}
	return fmt.Sprintf("$(%s %s)", fnName, strings.Join(argStrs, " ")), nil
}

func (g *Generator) genBooleanCapture(expr ast.Expression) (string, error) {
	switch e := expr.(type) {
	case *ast.AsExpr:
		return g.genBooleanCapture(e.Expr)
	case *ast.BoolLit:
		if e.Value {
			return "1", nil
		}
		return "0", nil
	case *ast.UndefinedLit, *ast.NullLit:
		return "0", nil
	case *ast.StringLit:
		if e.Value == "" {
			return "0", nil
		}
		return "1", nil
	case *ast.RawStringLit:
		if e.Value == "" {
			return "0", nil
		}
		return "1", nil
	case *ast.IntLit:
		if e.Value == 0 {
			return "0", nil
		}
		return "1", nil
	case *ast.FloatLit:
		f, err := strconv.ParseFloat(e.Value, 64)
		if err == nil && f == 0 {
			return "0", nil
		}
		return "1", nil
	case *ast.ListLit, *ast.ObjectLit, *ast.NewExpr:
		return "1", nil
	}
	typ := g.inferReceiverType(expr)
	if typ != nil && (typ.Kind == ast.TypeList || typ.Kind == ast.TypeObject || typ.Kind == ast.TypeSet) {
		return "1", nil
	}
	val, err := g.genNullishValue(expr)
	if err != nil {
		return "", err
	}
	switch {
	case typ != nil && typ.Kind == ast.TypeBoolean:
		return fmt.Sprintf("$(if [ %s = 1 ]; then printf 1; else printf 0; fi)", stripQuotes(val)), nil
	case typ != nil && typ.Kind == ast.TypeString:
		g.requireRuntimeHelper("nullish")
		return fmt.Sprintf("$(_bst_bool=%s; if [ \"$_bst_bool\" != \"$%s\" ] && [ -n \"$_bst_bool\" ]; then printf 1; else printf 0; fi)", val, nullishSentinelVar), nil
	case typ != nil && (typ.Kind == ast.TypeNumber || typ.Kind == ast.TypeStatus):
		return fmt.Sprintf("$(awk %s 'BEGIN{if ((_x + 0) == 0) printf 0; else printf 1}')", awkArg("_x", val)), nil
	default:
		g.requireRuntimeHelper("nullish")
		return fmt.Sprintf("$(_bst_bool=%s; if [ \"$_bst_bool\" != \"$%s\" ] && [ -n \"$_bst_bool\" ] && [ \"$_bst_bool\" != 0 ]; then printf 1; else printf 0; fi)", val, nullishSentinelVar), nil
	}
}

func (g *Generator) genArgs(args []ast.Expression) ([]string, error) {
	var out []string
	for _, arg := range args {
		val, err := g.genExprValue(arg)
		if err != nil {
			return nil, err
		}
		if g.isBooleanExpr(arg) {
			val = fmt.Sprintf("$(if [ %s = 1 ]; then printf true; else printf false; fi)", stripQuotes(val))
		}
		out = append(out, ensureArgSafe(val))
	}
	return out, nil
}

func (g *Generator) genFnArgs(args []ast.Expression) ([]string, error) {
	var out []string
	for _, arg := range args {
		val, err := g.genExprValue(arg)
		if err != nil {
			return nil, err
		}
		out = append(out, ensureArgSafe(val))
	}
	return out, nil
}

func (g *Generator) genBinaryRHS(e *ast.BinaryExpr, targetType *ast.Type) (string, error) {
	leftStr, err := g.genExprValue(e.Left)
	if err != nil {
		return "", err
	}
	rightStr, err := g.genExprValue(e.Right)
	if err != nil {
		return "", err
	}

	switch e.Op {
	case "==", "!=", "===", "!==", ">", "<", ">=", "<=":
		cond, err := g.genCondition(e)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("$(if %s; then printf 1; else printf 0; fi)", cond), nil
	case "??":
		g.requireRuntimeHelper("nullish")
		leftStr, err = g.genNullishValue(e.Left)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("$(_bst_l=%s; if [ \"$_bst_l\" = \"$%s\" ]; then _bst_r=%s; printf '%%s' \"$_bst_r\"; else printf '%%s' \"$_bst_l\"; fi)", leftStr, nullishSentinelVar, rightStr), nil
	case "||":
		return "$( _l=" + leftStr + "; if [ -n \"$_l\" ] && [ \"$_l\" != \"0\" ]; then printf '%s' \"$_l\"; else printf '%s' " + rightStr + "; fi )", nil
	case "&&":
		return "$( _l=" + leftStr + "; if [ -n \"$_l\" ] && [ \"$_l\" != \"0\" ]; then printf '%s' " + rightStr + "; else printf '%s' \"$_l\"; fi )", nil
	case "+":
		lType := e.Left.GetType()
		rType := e.Right.GetType()
		if lType == nil {
			lType = g.inferReceiverType(e.Left)
		}
		if rType == nil {
			rType = g.inferReceiverType(e.Right)
		}
		isStrNode := func(n ast.Expression) bool {
			switch n.(type) {
			case *ast.StringLit, *ast.RawStringLit, *ast.TemplateLit:
				return true
			}
			return false
		}
		isFloat := func(n ast.Expression) bool {
			_, ok := n.(*ast.FloatLit)
			return ok
		}
		isStr := (lType != nil && lType.Kind == ast.TypeString) ||
			(rType != nil && rType.Kind == ast.TypeString) ||
			isStrNode(e.Left) || isStrNode(e.Right)
		if isStr {
			lInner := strInner(leftStr)
			if g.isBooleanExpr(e.Left) {
				lInner = fmt.Sprintf("$(if [ %s = 1 ]; then printf true; else printf false; fi)", stripQuotes(leftStr))
			}
			rInner := strInner(rightStr)
			if g.isBooleanExpr(e.Right) {
				rInner = fmt.Sprintf("$(if [ %s = 1 ]; then printf true; else printf false; fi)", stripQuotes(rightStr))
			}
			return fmt.Sprintf("\"%s%s\"", lInner, rInner), nil
		}
		lClean := stripQuotes(leftStr)
		rClean := stripQuotes(rightStr)
		if isFloat(e.Left) || isFloat(e.Right) || g.isFloatExpr(e.Left) || g.isFloatExpr(e.Right) {
			if needsAwkCapture(lClean) || needsAwkCapture(rClean) {
				lArg := awkArg("_a", lClean)
				rArg := awkArg("_b", rClean)
				return "$(awk " + lArg + " " + rArg + " 'BEGIN{OFMT=\"%.17g\";printf \"%.16g\", _a + _b}')", nil
			}
			return fmt.Sprintf("$(awk -v _a=%s -v _b=%s 'BEGIN{OFMT=\"%%.17g\";printf \"%%.16g\", _a + _b}')", lClean, rClean), nil
		}
		if strings.HasPrefix(lClean, "$((") || strings.HasPrefix(rClean, "$((") {
			return fmt.Sprintf("$(( %s %s %s ))", arithmeticOperand(lClean), e.Op, arithmeticOperand(rClean)), nil
		}
		if needsAwkCapture(lClean) || needsAwkCapture(rClean) {
			lArg := awkArg("_a", lClean)
			rArg := awkArg("_b", rClean)
			return "$(awk " + lArg + " " + rArg + " 'BEGIN{printf \"%d\", _a + _b}')", nil
		}
		return fmt.Sprintf("$(( %s + %s ))", lClean, rClean), nil
	case "-", "*", "/", "%":
		lClean := stripQuotes(leftStr)
		rClean := stripQuotes(rightStr)
		if up, ok := e.Left.(*ast.UpdateExpr); ok {
			varName := g.resolveVarName(up.Name)
			return fmt.Sprintf("$(( (%s = %s %s 1) %s %s ))", varName, varName, up.Op, e.Op, rClean), nil
		}
		if e.Op == "/" {
			return fmt.Sprintf("$(awk -v _a=%s -v _b=%s 'BEGIN{OFMT=\"%%.17g\";printf \"%%.16g\", _a / _b}')", lClean, rClean), nil
		}
		if _, ok := e.Left.(*ast.FloatLit); ok {
			return fmt.Sprintf("$(awk -v _a=%s -v _b=%s 'BEGIN{OFMT=\"%%.17g\";printf \"%%.16g\", _a %s _b}')", lClean, rClean, e.Op), nil
		}
		if _, ok := e.Right.(*ast.FloatLit); ok {
			return fmt.Sprintf("$(awk -v _a=%s -v _b=%s 'BEGIN{OFMT=\"%%.17g\";printf \"%%.16g\", _a %s _b}')", lClean, rClean, e.Op), nil
		}
		if g.isFloatExpr(e.Left) || g.isFloatExpr(e.Right) {
			if needsAwkCapture(lClean) || needsAwkCapture(rClean) {
				lArg := awkArg("_a", lClean)
				rArg := awkArg("_b", rClean)
				return "$(awk " + lArg + " " + rArg + " 'BEGIN{OFMT=\"%.17g\";printf \"%.16g\", _a " + e.Op + " _b}')", nil
			}
			return fmt.Sprintf("$(awk -v _a=%s -v _b=%s 'BEGIN{OFMT=\"%%.17g\";printf \"%%.16g\", _a %s _b}')", lClean, rClean, e.Op), nil
		}
		if strings.HasPrefix(lClean, "$((") || strings.HasPrefix(rClean, "$((") {
			return fmt.Sprintf("$(( %s %s %s ))", arithmeticOperand(lClean), e.Op, arithmeticOperand(rClean)), nil
		}
		if needsAwkCapture(lClean) || needsAwkCapture(rClean) {
			lArg := awkArg("_a", lClean)
			rArg := awkArg("_b", rClean)
			return "$(awk " + lArg + " " + rArg + " 'BEGIN{printf \"%d\", _a " + e.Op + " _b}')", nil
		}
		return fmt.Sprintf("$(( %s %s %s ))", lClean, e.Op, rClean), nil
	}
	return "", fmt.Errorf("binary op %q cannot appear on RHS directly; use in conditions", e.Op)
}

func (g *Generator) genUpdateRHS(e *ast.UpdateExpr) string {
	varName := g.resolveVarName(e.Name)
	return fmt.Sprintf("$(( %s = %s %s 1 ))", varName, varName, e.Op)
}

func (g *Generator) genUnaryRHS(e *ast.UnaryExpr) (string, error) {
	inner, err := g.genExprValue(e.Expr)
	if err != nil {
		return "", err
	}
	if e.Op == "-" {
		clean := stripQuotes(inner)
		if _, ok := e.Expr.(*ast.FloatLit); ok {
			return fmt.Sprintf("$(awk -v _x=%s 'BEGIN{OFMT=\"%%.17g\";printf \"%%.16g\", -_x}')", clean), nil
		}
		return fmt.Sprintf("$(( -%s ))", clean), nil
	}
	if e.Op == "!" {
		cond, err := g.genCondition(e)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("$(if %s; then printf 1; else printf 0; fi)", cond), nil
	}
	return "", fmt.Errorf("unknown unary operator %q", e.Op)
}

func arrayFromLengthArg(e *ast.BuiltinCallExpr) (ast.Expression, bool) {
	if len(e.Args) != 1 {
		return nil, false
	}
	obj, ok := e.Args[0].(*ast.ObjectLit)
	if !ok || len(obj.Fields) != 1 || obj.Fields[0].Key != "length" {
		return nil, false
	}
	return obj.Fields[0].Value, true
}

func (g *Generator) genBuiltinCapture(e *ast.BuiltinCallExpr) (string, error) {
	switch e.Name {
	case "Besht.fs.isFile", "Besht.fs.isDir", "Besht.fs.isReadable", "Besht.fs.isWritable", "Besht.fs.isExecutable", "Besht.strings.isEmpty", "Besht.strings.isNonEmpty":
		if len(e.Args) != 1 {
			return "", fmt.Errorf("%s() takes 1 argument", e.Name)
		}
		cond, err := g.genBuiltinCondition(e)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("$(if %s; then printf 1; else printf 0; fi)", cond), nil

	case "console.log", "console.error":
		parts, err := g.genArgs(e.Args)
		if err != nil {
			return "", err
		}
		if e.Name == "console.error" {
			return fmt.Sprintf("$(printf '%%s\\n' %s >&2)", strings.Join(parts, " ")), nil
		}
		return fmt.Sprintf("$(printf '%%s\\n' %s)", strings.Join(parts, " ")), nil

	case "Boolean":
		return g.genBooleanCapture(e.Args[0])

	case "Number.parseInt":
		argStr, err := g.genExprValue(e.Args[0])
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("$(( %s + 0 ))", stripQuotes(argStr)), nil

	case "Number.parseFloat":
		argStr, err := g.genExprValue(e.Args[0])
		if err != nil {
			return "", err
		}
		return stripQuotes(argStr), nil

	case "Number.isFinite":
		return "1", nil

	case "Number.isSafeInteger":
		if argType := g.inferReceiverType(e.Args[0]); argType != nil && argType.Kind == ast.TypeString {
			return "0", nil
		}
		argStr, err := g.genExprValue(e.Args[0])
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("$(awk %s 'BEGIN{_s=\"\" _x; if (_s !~ /^-?[0-9]+(\\.0*)?$/) { printf 0; exit } sub(/^-/, \"\", _s); sub(/\\.0*$/, \"\", _s); sub(/^0+/, \"\", _s); if (_s == \"\") _s=\"0\"; _lim=\"9007199254740991\"; _ok=1; if (length(_s) > length(_lim)) _ok=0; else if (length(_s) == length(_lim)) { for (i=1; i<=length(_lim); i++) { _d=substr(_s,i,1)+0; _l=substr(_lim,i,1)+0; if (_d > _l) { _ok=0; break } if (_d < _l) break } } printf _ok ? 1 : 0}')", awkArg("_x", argStr)), nil

	case "Number.isNaN":
		return "0", nil

	case "Array.from":
		lengthExpr, ok := arrayFromLengthArg(e)
		if !ok {
			return "", fmt.Errorf("Array.from() currently supports only { length: expr }")
		}
		lengthStr, err := g.genExprValue(lengthExpr)
		if err != nil {
			return "", err
		}
		idx := fmt.Sprintf("_arrfrom_%d_%d", e.Pos.Line, e.Pos.Column)
		return fmt.Sprintf("$(%s=0; while [ \"$%s\" -lt %s ]; do printf '%%s\n' \"$%s\"; %s=$(( %s + 1 )); done)", idx, idx, stripQuotes(lengthStr), idx, idx, idx), nil

	case "Array.of":
		return g.genListLiteral(&ast.ListLit{Pos: e.Pos, Elements: e.Args})

	case "Besht.iter.range":
		if len(e.Args) != 2 {
			return "", fmt.Errorf("Besht.iter.range() takes 2 arguments")
		}
		startStr, err := g.genExprValue(e.Args[0])
		if err != nil {
			return "", err
		}
		endStr, err := g.genExprValue(e.Args[1])
		if err != nil {
			return "", err
		}
		idx := fmt.Sprintf("_range_%d_%d", e.Pos.Line, e.Pos.Column)
		return fmt.Sprintf("$(%s=%s; while [ \"$%s\" -le %s ]; do printf '%%s\n' \"$%s\"; %s=$(( %s + 1 )); done)", idx, stripQuotes(startStr), idx, stripQuotes(endStr), idx, idx, idx), nil

	case "Array.isArray":
		if argType := g.inferReceiverType(e.Args[0]); argType != nil && argType.Kind == ast.TypeList {
			return "1", nil
		}
		return "0", nil

	case "Object.keys":
		return g.genObjectKeys(e.Args[0])

	case "Object.values":
		return g.genObjectValues(e.Args[0])

	case "Object.entries":
		return g.genObjectEntries(e.Args[0])

	case "Object.hasOwn":
		return g.genObjectHasOwn(e.Args[0], e.Args[1])

	case "Number.isInteger":
		argStr, err := g.genExprValue(e.Args[0])
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("$(if printf '%%s\\n' %s | grep -q '[.]'; then printf 0; else printf 1; fi)", argStr), nil

	}
	return "", fmt.Errorf("builtin %q cannot be used in value position", e.Name)
}

func (g *Generator) genProperty(e *ast.PropertyExpr) (string, error) {
	if prop, ok := isProcessEnvProperty(e); ok {
		g.requireRuntimeHelper("nullish")
		name := prop.Property
		return fmt.Sprintf("$(if [ -n \"${%s+x}\" ]; then printf '%%s' \"$%s\"; else printf '%%s' \"$%s\"; fi)", name, name, nullishSentinelVar), nil
	}
	if isProcessEnvObject(e) {
		return "", fmt.Errorf("process.env cannot be used as a value; access a variable like process.env.HOME")
	}
	if _, ok := e.Receiver.(*ast.ThisExpr); ok {
		if g.currentThisVar == "" {
			return "", fmt.Errorf("this.%s used outside class method", e.Property)
		}
		if _, ok := classAccessor(g.classMap[g.currentClass], e.Property, ast.AccessorGet, false); ok {
			return fmt.Sprintf("$(%s \"$%s\")", classMethodName(g.currentClass, "get_"+e.Property), g.currentThisVar), nil
		}
		if _, ok := classAccessor(g.classMap[g.currentClass], e.Property, ast.AccessorSet, false); ok {
			return "", fmt.Errorf("property %q has a setter but no getter", e.Property)
		}
		return fmt.Sprintf("$(%s \"$%s\")", classMethodName(g.currentClass, "get_"+e.Property), g.currentThisVar), nil
	}
	if ident, ok := e.Receiver.(*ast.IdentExpr); ok {
		if classDecl := g.classMap[g.resolveClassName(ident.Name)]; classDecl != nil {
			if _, ok := classAccessor(classDecl, e.Property, ast.AccessorGet, true); ok {
				return fmt.Sprintf("$(%s)", classMethodName(classDecl.Name, "get_"+e.Property)), nil
			}
			if _, ok := classAccessor(classDecl, e.Property, ast.AccessorSet, true); ok {
				return "", fmt.Errorf("static property %q has a setter but no getter", e.Property)
			}
			for _, prop := range classDecl.StaticProps {
				if prop.Name == e.Property {
					return fmt.Sprintf("\"$%s\"", classPropVar(classDecl.Name, e.Property)), nil
				}
			}
		}
		varName := g.resolveVarName(ident.Name)
		if className, ok := g.varClassMap[varName]; ok {
			classDecl := g.classMap[className]
			if _, ok := classAccessor(classDecl, e.Property, ast.AccessorGet, false); ok {
				return fmt.Sprintf("$(%s \"$%s\")", classMethodName(className, "get_"+e.Property), varName), nil
			}
			if _, ok := classAccessor(classDecl, e.Property, ast.AccessorSet, false); ok {
				return "", fmt.Errorf("property %q has a setter but no getter", e.Property)
			}
		}
		if ref, ok := g.objAliasMap[ident.Name]; ok {
			return g.genObjectRefProperty(ref, e.Property, e.Pos), nil
		}
		if _, ok := g.objPropTypeMap[varName+"."+e.Property]; ok {
			return fmt.Sprintf("\"$%s\"", objectPropVar(varName, e.Property)), nil
		}
		if global, ok := g.globalVarMap[ident.Name]; ok && global == varName {
			if _, hasProp := g.objPropTypeMap[ident.Name+"."+e.Property]; hasProp {
				return fmt.Sprintf("\"$%s\"", objectPropVar(varName, e.Property)), nil
			}
		}
		if vt, ok := g.varTypeMap[varName]; ok && vt.Kind == ast.TypeObject {
			return fmt.Sprintf("\"$%s\"", objectPropVar(varName, e.Property)), nil
		}
		if _, isParam := g.paramMap[ident.Name]; isParam {
			if _, hasProp := g.objPropTypeMap[ident.Name+"."+e.Property]; hasProp {
				return fmt.Sprintf("\"$%s\"", objectPropVar(ident.Name, e.Property)), nil
			}
		}
	}
	if e.Property == "length" {
		if n, ok := staticScalarListLength(e.Receiver); ok {
			return strconv.Itoa(n), nil
		}
		if n, ok := staticStringByteLength(e.Receiver); ok {
			return strconv.Itoa(n), nil
		}
	}
	recv, err := g.genExprValue(e.Receiver)
	if err != nil {
		return "", err
	}
	recvType := g.inferReceiverType(e.Receiver)
	switch e.Property {
	case "length":
		if recvType != nil && recvType.Kind == ast.TypeList {
			if known := g.listLengthExpr(e.Receiver); known != "" {
				return known, nil
			}
			if emptyArrayOf(e.Receiver) {
				return "0", nil
			}
			return fmt.Sprintf("$(printf '%%s\\n' %s | wc -l | tr -d ' ')", ensureArgSafe(recv)), nil
		}
		return fmt.Sprintf("$(printf '%%s' %s | wc -c | tr -d ' ')", recv), nil
	}
	return "", fmt.Errorf("unknown property %q", e.Property)
}

func (g *Generator) genObjectRefProperty(ref objectRef, property string, pos ast.Pos) string {
	if ref.StaticName != "" {
		return fmt.Sprintf("\"$%s\"", objectPropVar(ref.StaticName, property))
	}
	slotVar := fmt.Sprintf("_objs_%d_%d", pos.Line, pos.Column)
	return "$(" + slotVar + "=" + ref.SlotExpr + "; " + computedKeyValidation(slotVar) + "; eval \"printf '" + "%s" + "' \\\"\\${_obj_${" + slotVar + "}_" + property + "}\\\"\")"
}

func (g *Generator) genPropertyAssign(s *ast.PropertyAssignStmt) error {
	if err := validateStaticObjectKey(s.Property); err != nil {
		return err
	}
	val, err := g.genExprRHS(s.Value, nil)
	if err != nil {
		return err
	}
	if s.Object == "this" {
		if g.currentThisVar == "" {
			return fmt.Errorf("this.%s used outside class method", s.Property)
		}
		if _, ok := classAccessor(g.classMap[g.currentClass], s.Property, ast.AccessorSet, false); ok {
			g.line(fmt.Sprintf("%s \"$%s\" %s", classMethodName(g.currentClass, "set_"+s.Property), g.currentThisVar, ensureArgSafe(val)))
			return nil
		}
		if _, ok := classAccessor(g.classMap[g.currentClass], s.Property, ast.AccessorGet, false); ok {
			return fmt.Errorf("property %q has a getter but no setter", s.Property)
		}
		g.line(fmt.Sprintf("%s \"$%s\" %s", classMethodName(g.currentClass, "set_"+s.Property), g.currentThisVar, ensureArgSafe(val)))
		return nil
	}
	if classDecl, ok := g.classMap[g.resolveClassName(s.Object)]; ok {
		if _, ok := classAccessor(classDecl, s.Property, ast.AccessorSet, true); ok {
			g.line(fmt.Sprintf("%s %s", classMethodName(classDecl.Name, "set_"+s.Property), ensureArgSafe(val)))
			return nil
		}
		if _, ok := classAccessor(classDecl, s.Property, ast.AccessorGet, true); ok {
			return fmt.Errorf("static property %q has a getter but no setter", s.Property)
		}
		g.line(fmt.Sprintf("%s=%s", classPropVar(classDecl.Name, s.Property), val))
		return nil
	}
	if className, ok := g.varClassMap[g.resolveVarName(s.Object)]; ok {
		if _, ok := classAccessor(g.classMap[className], s.Property, ast.AccessorSet, false); ok {
			g.line(fmt.Sprintf("%s \"$%s\" %s", classMethodName(className, "set_"+s.Property), g.resolveVarName(s.Object), ensureArgSafe(val)))
			return nil
		}
		if _, ok := classAccessor(g.classMap[className], s.Property, ast.AccessorGet, false); ok {
			return fmt.Errorf("property %q has a getter but no setter", s.Property)
		}
	}
	ref, ok := g.resolveObjectRef(&ast.IdentExpr{Pos: s.Pos, Name: s.Object})
	if !ok {
		ref = objectRef{StaticName: g.resolveVarName(s.Object)}
	}
	ref = g.recordObjectAssignmentType(ref, s.Property, s.Value, false)
	g.updateObjectAliasRef(s.Object, ref)
	if ref.StaticName != "" {
		g.line(fmt.Sprintf("%s=%s", objectPropVar(ref.StaticName, s.Property), val))
	} else {
		slotVar := fmt.Sprintf("_objs_%d_%d", s.Pos.Line, s.Pos.Column)
		valVar := fmt.Sprintf("_objv_%d_%d", s.Pos.Line, s.Pos.Column)
		g.line(slotVar + "=" + ref.SlotExpr)
		g.line(computedKeyValidation(slotVar))
		g.line(valVar + "=" + val)
		g.line("eval \"_obj_${" + slotVar + "}_" + s.Property + "=\\\"\\$" + valVar + "\\\"\"")
	}
	g.emitObjectKeyAppendRef(ref, s.Property, s.Pos)
	return nil
}

// genRunCall emits shell code for a .run() call.
// It consults the analysis to decide: capture or bare.
func (g *Generator) genRunCall(me *ast.MethodCallExpr) error {
	if g.cmdAnalysis == nil {
		pipeline, redirect, err := g.genCmdChain(me)
		if err != nil {
			return err
		}
		g.line(formatCmdForBare(pipeline, redirect))
		return nil
	}

	id := g.cmdAnalysis.resolveIdentityExpr(me.Receiver, g.cmdScope)
	ident := g.cmdAnalysis.identity(id)

	chainExpr := me.Receiver
	if receiver, ok := me.Receiver.(*ast.IdentExpr); ok {
		if chain, ok := g.cmdChains[receiver.Name]; ok {
			chainExpr = chain
		} else if ident != nil && ident.FullChain != nil {
			chainExpr = ident.FullChain
		}
	}
	pipeline, redirect, err := g.genCmdChainExpr(chainExpr)
	if err != nil {
		return err
	}

	if ident == nil || (!ident.UsesText && !ident.UsesLines && !ident.UsesReadStderr && !ident.UsesExitCode) {
		// Side-effect only — bare command
		g.line(formatCmdForBare(pipeline, redirect))
		return nil
	}

	captureVar := ident.CaptureVarName(g.resolveVarName)

	if ident.UsesReadStderr {
		g.line(fmt.Sprintf("%s=%s", captureVar, commandSubstitution(formatCmdForStderrCapture(pipeline, redirect))))
	} else if ident.UsesText || ident.UsesLines {
		g.line(fmt.Sprintf("%s=%s", captureVar, commandSubstitution(formatCmdForCapture(pipeline, redirect))))
	} else {
		g.line(formatCmdForBare(pipeline, redirect))
	}

	if ident.UsesExitCode {
		exitVar := ident.ExitCodeVarName(g.resolveVarName)
		g.line(fmt.Sprintf("%s=$?", exitVar))
	}
	return nil
}

func isCmdExprOrChain(expr ast.Expression) bool {
	return isLazyCmdChain(expr)
}

// isLazyCmdChain returns true if expr is a pure pipeline declaration
// (no .run() in the chain) — meaning it's still unexecuted.
func containsRunCall(expr ast.Expression) bool {
	me, ok := expr.(*ast.MethodCallExpr)
	if !ok {
		return false
	}
	if me.Method == "run" {
		return true
	}
	return containsRunCall(me.Receiver)
}

func isTerminalCommandReadMethod(method string) bool {
	switch method {
	case "readStdout", "readStdoutLines", "readStderr", "exitCode":
		return true
	default:
		return false
	}
}

func immediateRunReceiver(e *ast.MethodCallExpr) (*ast.MethodCallExpr, bool) {
	runCall, ok := e.Receiver.(*ast.MethodCallExpr)
	if !ok || runCall.Method != "run" {
		return nil, false
	}
	return runCall, true
}

func isImmediateRunTerminalCall(expr ast.Expression) bool {
	me, ok := expr.(*ast.MethodCallExpr)
	if !ok || !isTerminalCommandReadMethod(me.Method) {
		return false
	}
	_, ok = immediateRunReceiver(me)
	return ok
}

func (g *Generator) emitInlineRunChain(expr ast.Expression) error {
	me, ok := expr.(*ast.MethodCallExpr)
	if !ok {
		return nil
	}
	if me.Method == "run" {
		return g.genRunCall(me)
	}
	return g.emitInlineRunChain(me.Receiver)
}

func isLazyCmdChain(expr ast.Expression) bool {
	switch e := expr.(type) {
	case *ast.CmdExpr:
		return true
	case *ast.MethodCallExpr:
		switch e.Method {
		case "pipe", "stdout", "stderr", "workdir", "env":
			return isLazyCmdChain(e.Receiver)
		case "run", "readStdout", "readStdoutLines", "readStderr", "exitCode":
			return false
		case "clone":
			return true
		}
	}
	return false
}

func isCmdReceiver(expr ast.Expression) bool {
	switch expr.(type) {
	case *ast.CmdExpr:
		return true
	case *ast.MethodCallExpr:
		t := expr.GetType()
		if t != nil && t.Kind == ast.TypeCommand {
			return true
		}
		me := expr.(*ast.MethodCallExpr)
		return isCmdReceiver(me.Receiver)
	}
	t := expr.GetType()
	return t != nil && t.Kind == ast.TypeCommand
}

var typeString = &ast.Type{Kind: ast.TypeString}
var typeNumber = &ast.Type{Kind: ast.TypeNumber}

func (g *Generator) inferReceiverType(expr ast.Expression) *ast.Type {
	t := expr.GetType()
	if t != nil {
		return t
	}
	switch e := expr.(type) {
	case *ast.AsExpr:
		return e.Type
	case *ast.IdentExpr:
		varName := g.resolveVarName(e.Name)
		if vt, ok := g.varTypeMap[varName]; ok {
			return vt
		}
		if vt, ok := g.fnParamTypes[e.Name]; ok {
			return vt
		}
	case *ast.ListLit:
		if t := e.GetType(); t != nil {
			return t
		}
		return &ast.Type{Kind: ast.TypeList, Elem: g.inferListElemType(e)}
	case *ast.StringLit, *ast.RawStringLit, *ast.TemplateLit:
		return typeString
	case *ast.IntLit, *ast.FloatLit:
		return typeNumber
	case *ast.BoolLit:
		return &ast.Type{Kind: ast.TypeBoolean}
	case *ast.NewExpr:
		if e.ClassName == "Set" {
			elem := typeString
			if len(e.TypeArgs) > 0 {
				elem = e.TypeArgs[0]
			}
			return &ast.Type{Kind: ast.TypeSet, Elem: elem}
		}
		return &ast.Type{Kind: ast.TypeObject}
	case *ast.ObjectLit:
		return &ast.Type{Kind: ast.TypeObject}
	case *ast.ThisExpr:
		return &ast.Type{Kind: ast.TypeObject}
	case *ast.UnaryExpr:
		if e.Op == "!" {
			return &ast.Type{Kind: ast.TypeBoolean}
		}
		return typeNumber
	case *ast.UpdateExpr:
		return typeNumber
	case *ast.BuiltinCallExpr:
		switch e.Name {
		case "fetch":
			return &ast.Type{Kind: ast.TypeFetchResponse}
		case "Boolean", "Number.isFinite", "Number.isInteger", "Number.isSafeInteger", "Number.isNaN", "Array.isArray", "Object.hasOwn":
			return &ast.Type{Kind: ast.TypeBoolean}
		case "Object.keys", "Object.values":
			return &ast.Type{Kind: ast.TypeList, Elem: typeString}
		case "Object.entries":
			return &ast.Type{Kind: ast.TypeList, Elem: &ast.Type{Kind: ast.TypeList, Elem: typeString}}
		case "Number.parseInt", "Number.parseFloat":
			return typeNumber
		case "Array.from":
			return &ast.Type{Kind: ast.TypeList, Elem: typeNumber}
		case "Array.of":
			if len(e.Args) == 0 {
				return &ast.Type{Kind: ast.TypeList, Elem: typeString}
			}
			elem := g.inferReceiverType(e.Args[0])
			if elem == nil {
				elem = typeString
			}
			return &ast.Type{Kind: ast.TypeList, Elem: elem}
		}
	case *ast.PropertyExpr:
		if _, ok := isProcessEnvProperty(e); ok {
			return typeString
		}
		if isProcessEnvObject(e) {
			return &ast.Type{Kind: ast.TypeObject}
		}
		if _, ok := e.Receiver.(*ast.ThisExpr); ok && g.currentClass != "" {
			if _, ok := classAccessor(g.classMap[g.currentClass], e.Property, ast.AccessorGet, false); ok {
				if pt, ok := g.fnReturnMap[classMethodName(g.currentClass, "get_"+e.Property)]; ok {
					return pt
				}
			}
			if pt, ok := g.objPropTypeMap[g.currentClass+"."+e.Property]; ok {
				return pt
			}
		}
		if ident, ok := e.Receiver.(*ast.IdentExpr); ok {
			if classDecl := g.classMap[g.resolveClassName(ident.Name)]; classDecl != nil {
				if _, ok := classAccessor(classDecl, e.Property, ast.AccessorGet, true); ok {
					if pt, ok := g.fnReturnMap[classMethodName(classDecl.Name, "get_"+e.Property)]; ok {
						return pt
					}
				}
				for _, prop := range classDecl.StaticProps {
					if prop.Name == e.Property {
						if prop.Type != nil {
							return prop.Type
						}
						return g.inferExprType(prop.Value)
					}
				}
			}
			varName := g.resolveVarName(ident.Name)
			if className, ok := g.varClassMap[varName]; ok {
				if _, ok := classAccessor(g.classMap[className], e.Property, ast.AccessorGet, false); ok {
					if pt, ok := g.fnReturnMap[classMethodName(className, "get_"+e.Property)]; ok {
						return pt
					}
				}
			}
			if pt, ok := g.objPropTypeMap[varName+"."+e.Property]; ok {
				return pt
			}
			if global, ok := g.globalVarMap[ident.Name]; ok && global == varName {
				if pt, ok := g.objPropTypeMap[ident.Name+"."+e.Property]; ok {
					return pt
				}
			}
			if _, isParam := g.paramMap[ident.Name]; isParam {
				if pt, ok := g.objPropTypeMap[ident.Name+"."+e.Property]; ok {
					return pt
				}
			}
			if ref, ok := g.objAliasMap[ident.Name]; ok && ref.StaticName != "" {
				if pt, ok := g.objPropTypeMap[ref.StaticName+"."+e.Property]; ok {
					return pt
				}
			}
		}
	case *ast.IndexExpr:
		containerType := g.inferReceiverType(e.Expr)
		if containerType != nil && containerType.Kind == ast.TypeList {
			return containerType.Elem
		}
		if containerType != nil && containerType.Kind == ast.TypeObject && containerType.Elem != nil {
			return containerType.Elem
		}
		if containerType != nil && containerType.Kind == ast.TypeString {
			return typeString
		}
	case *ast.BinaryExpr:
		switch e.Op {
		case "??":
			if _, ok := e.Left.(*ast.UndefinedLit); ok {
				return g.inferReceiverType(e.Right)
			}
			if _, ok := e.Left.(*ast.NullLit); ok {
				return g.inferReceiverType(e.Right)
			}
			lt := g.inferReceiverType(e.Left)
			if lt != nil {
				return lt
			}
			return g.inferReceiverType(e.Right)
		case "==", "!=", "===", "!==", ">", "<", ">=", "<=", "&&", "||":
			return &ast.Type{Kind: ast.TypeBoolean}
		}
		if e.Op == "+" {
			lt := g.inferReceiverType(e.Left)
			rt := g.inferReceiverType(e.Right)
			if (lt != nil && lt.Kind == ast.TypeString) || (rt != nil && rt.Kind == ast.TypeString) {
				return typeString
			}
		}
	case *ast.MethodCallExpr:
		if builtin, ok := beshtBuiltinCall(e); ok {
			if builtin.Name == "Besht.iter.range" {
				return &ast.Type{Kind: ast.TypeList, Elem: typeNumber}
			}
			return &ast.Type{Kind: ast.TypeBoolean}
		}
		if ast.IsBeshtArgsReceiver(e.Receiver) {
			switch e.Method {
			case "argv":
				return &ast.Type{Kind: ast.TypeList, Elem: typeString}
			case "positional", "option":
				return typeString
			case "flag":
				return &ast.Type{Kind: ast.TypeBoolean}
			}
		}
		if className, ok := g.receiverClassName(e.Receiver); ok {
			if rt, ok := g.fnReturnMap[classMethodName(className, e.Method)]; ok {
				return rt
			}
		}
		recvType := g.inferReceiverType(e.Receiver)
		if recvType != nil && recvType.Kind == ast.TypeFetchResponse && e.Method == "text" {
			return typeString
		}
		if recvType != nil && recvType.Kind == ast.TypeList {
			switch e.Method {
			case "push", "pop", "shift", "unshift", "concat", "slice", "reverse", "filter":
				return recvType
			case "map":
				if len(e.Args) == 1 {
					if arrow, ok := e.Args[0].(*ast.ArrowExpr); ok {
						if arrow.Body != nil {
							if t := g.inferReceiverType(arrow.Body); t != nil {
								return &ast.Type{Kind: ast.TypeList, Elem: t}
							}
						}
						if arrow.BlockBody != nil {
							if t := g.inferMapBlockReturnType(arrow.BlockBody); t != nil {
								return &ast.Type{Kind: ast.TypeList, Elem: t}
							}
						}
					}
				}
				return &ast.Type{Kind: ast.TypeList, Elem: typeString}
			case "reduce":
				if len(e.Args) == 2 {
					return g.inferReceiverType(e.Args[1])
				}
				return &ast.Type{Kind: ast.TypeObject}
			case "join":
				return typeString
			case "includes", "some", "every":
				return &ast.Type{Kind: ast.TypeBoolean}
			case "find":
				return recvType.Elem
			case "indexOf", "lastIndexOf", "findIndex", "length":
				return typeNumber
			}
		}
		if recvType != nil && recvType.Kind == ast.TypeSet {
			switch e.Method {
			case "has":
				return &ast.Type{Kind: ast.TypeBoolean}
			case "add":
				return recvType
			}
		}
		switch e.Method {
		case "split":
			return &ast.Type{Kind: ast.TypeList, Elem: typeString}
		case "trim", "trimStart", "trimEnd", "toUpperCase", "toLowerCase",
			"replace", "replaceAll", "slice", "substring", "at", "charAt", "concat", "padStart", "padEnd", "toString", "toFixed", "join":
			return typeString
		case "indexOf", "lastIndexOf", "length":
			return typeNumber
		case "includes", "startsWith", "endsWith":
			return &ast.Type{Kind: ast.TypeBoolean}
		case "push", "pop", "shift", "unshift", "reverse":
			return &ast.Type{Kind: ast.TypeList, Elem: &ast.Type{Kind: ast.TypeString}}
		}
	case *ast.TernaryExpr:
		thenType := g.inferReceiverType(e.Then)
		elseType := g.inferReceiverType(e.Else)
		if thenType != nil {
			return thenType
		}
		return elseType
	}
	return nil
}

func (g *Generator) isBooleanExpr(expr ast.Expression) bool {
	switch e := expr.(type) {
	case *ast.BoolLit:
		return true
	case *ast.IdentExpr:
		if vt := g.inferReceiverType(e); vt != nil {
			return vt.Kind == ast.TypeBoolean
		}
	case *ast.UnaryExpr:
		return e.Op == "!"
	case *ast.BinaryExpr:
		switch e.Op {
		case "??":
			if t := g.inferReceiverType(e); t != nil {
				return t.Kind == ast.TypeBoolean
			}
		case "==", "!=", "===", "!==", ">", "<", ">=", "<=", "&&", "||":
			return true
		}
	case *ast.PropertyExpr:
		pt := g.inferReceiverType(expr)
		return pt != nil && pt.Kind == ast.TypeBoolean
	case *ast.BuiltinCallExpr:
		switch e.Name {
		case "Boolean", "Number.isFinite", "Number.isInteger", "Number.isSafeInteger", "Number.isNaN", "Array.isArray", "Object.hasOwn":
			return true
		}
	case *ast.MethodCallExpr:
		if ast.IsBeshtArgsReceiver(e.Receiver) && e.Method == "flag" {
			return true
		}
		if className, ok := g.receiverClassName(e.Receiver); ok {
			if rt, ok := g.fnReturnMap[classMethodName(className, e.Method)]; ok {
				return rt != nil && rt.Kind == ast.TypeBoolean
			}
		}
		switch e.Method {
		case "includes", "startsWith", "endsWith", "has", "some", "every":
			return true
		}
	case *ast.FnCallExpr:
		if rt, ok := g.fnReturnMap[e.Name]; ok {
			return rt != nil && rt.Kind == ast.TypeBoolean
		}
	}
	return false
}

func (g *Generator) isFloatExpr(expr ast.Expression) bool {
	switch e := expr.(type) {
	case *ast.FloatLit:
		return true
	case *ast.IdentExpr:
		return g.floatVars[g.resolveVarName(e.Name)]
	case *ast.BinaryExpr:
		return e.Op == "/" || g.isFloatExpr(e.Left) || g.isFloatExpr(e.Right)
	case *ast.FnCallExpr:
		return g.floatVars[e.Name]
	case *ast.MethodCallExpr:
		if ident, ok := e.Receiver.(*ast.IdentExpr); ok && ident.Name == "Math" {
			return true
		}
	}
	return false
}

func (g *Generator) fnReturnsFloat(stmts []ast.Statement) bool {
	for _, stmt := range stmts {
		switch s := stmt.(type) {
		case *ast.ReturnStmt:
			return s.Value != nil && g.isFloatExpr(s.Value)
		case *ast.IfStmt:
			if g.fnReturnsFloat(s.Then.Statements) || (s.Else != nil && g.fnReturnsFloat(s.Else.Statements)) {
				return true
			}
		}
	}
	return false
}

func (g *Generator) genMethodCall(e *ast.MethodCallExpr) (string, error) {
	if builtin, ok := beshtBuiltinCall(e); ok {
		return g.genBuiltinCapture(builtin)
	}
	if ident, ok := e.Receiver.(*ast.IdentExpr); ok && ident.Name == "process" && e.Method == "exit" {
		return "", fmt.Errorf("process.exit() cannot be used in value position")
	}
	if ast.IsBeshtArgsReceiver(e.Receiver) {
		return g.genArgsMethod(e)
	}
	if className, ok := g.receiverClassName(e.Receiver); ok {
		return g.genClassMethodCall(className, e)
	}
	if recvType := g.inferReceiverType(e.Receiver); recvType != nil && recvType.Kind == ast.TypeFetchResponse {
		return g.genFetchResponseMethod(e)
	}
	if val, ok, err := g.genStaticListJoinMethod(e); ok || err != nil {
		return val, err
	}
	// Handle terminal command methods that read captured output
	if isTerminalCommandReadMethod(e.Method) {
		if runCall, ok := immediateRunReceiver(e); ok {
			id := -1
			var ident *CmdIdentity
			if g.cmdAnalysis != nil {
				id = g.cmdAnalysis.resolveIdentityExpr(e.Receiver, g.cmdScope)
				ident = g.cmdAnalysis.identity(id)
			}
			needsCapturedRun := e.Method == "exitCode" || (ident != nil && (ident.VarName != "" || ident.UsesExitCode))
			if needsCapturedRun {
				if err := g.genRunCall(runCall); err != nil {
					return "", err
				}
				if ident != nil {
					if e.Method == "exitCode" {
						exitVar := ident.ExitCodeVarName(g.resolveVarName)
						return fmt.Sprintf("\"$%s\"", exitVar), nil
					}
					captureVar := ident.CaptureVarName(g.resolveVarName)
					return fmt.Sprintf("\"$%s\"", captureVar), nil
				}
			}
			pipeline, redirect, err := g.genCmdChainExpr(runCall.Receiver)
			if err != nil {
				return "", err
			}
			if e.Method == "readStderr" {
				return commandSubstitution(formatCmdForStderrCapture(pipeline, redirect)), nil
			}
			return commandSubstitution(formatCmdForCapture(pipeline, redirect)), nil
		}
		id := -1
		if g.cmdAnalysis != nil {
			id = g.cmdAnalysis.resolveIdentityExpr(e.Receiver, g.cmdScope)
		}
		if id >= 0 {
			ident := g.cmdAnalysis.identity(id)
			if ident != nil {
				if e.Method == "exitCode" {
					exitVar := ident.ExitCodeVarName(g.resolveVarName)
					return fmt.Sprintf("\"$%s\"", exitVar), nil
				}
				captureVar := ident.CaptureVarName(g.resolveVarName)
				return fmt.Sprintf("\"$%s\"", captureVar), nil
			}
		}
	}

	// .run() in value position (e.g. $("whoami").run().text()): emit run as side-effect inline
	if e.Method == "run" {
		if err := g.genRunCall(e); err != nil {
			return "", err
		}
		// Return the receiver expression — allows .run().text() chaining
		return g.genExprValue(e.Receiver)
	}

	recvType := g.inferReceiverType(e.Receiver)
	isCmd := (recvType != nil && recvType.Kind == ast.TypeCommand) || isCmdReceiver(e.Receiver)

	if isCmd {
		pipeline, redirect, err := g.genCmdChain(e)
		if err != nil {
			return "", err
		}
		return commandSubstitution(formatCmdForCapture(pipeline, redirect)), nil
	}

	if ident, ok := e.Receiver.(*ast.IdentExpr); ok && ident.Name == "Math" {
		return g.genMathMethod(e)
	}

	recv, err := g.genExprValue(e.Receiver)
	if err != nil {
		return "", err
	}

	isListMethod := recvType != nil && recvType.Kind == ast.TypeList
	isStringMethod := recvType != nil && recvType.Kind == ast.TypeString
	isNumberMethod := recvType != nil && recvType.Kind == ast.TypeNumber
	isBooleanMethod := recvType != nil && recvType.Kind == ast.TypeBoolean
	isStatusMethod := recvType != nil && recvType.Kind == ast.TypeStatus
	if e.Method == "toString" && (isStringMethod || isBooleanMethod || isStatusMethod) {
		if isBooleanMethod {
			return fmt.Sprintf("$(if [ %s = 1 ]; then printf true; else printf false; fi)", stripQuotes(recv)), nil
		}
		return fmt.Sprintf("$(printf '%%s' %s)", recv), nil
	}
	if isNumberMethod || (recvType == nil && (e.Method == "toString" || e.Method == "toFixed")) {
		switch e.Method {
		case "toString":
			return fmt.Sprintf("$(printf '%%s' %s)", recv), nil
		case "toFixed":
			digits := "0"
			if len(e.Args) > 0 {
				arg, err := g.genExprValue(e.Args[0])
				if err != nil {
					return "", err
				}
				digits = stripQuotes(arg)
			}
			return fmt.Sprintf("$(awk -v _x=%s -v _n=%s 'BEGIN{OFMT=\"%%.17g\";printf \"%%.*f\", _n, _x}')", recv, digits), nil
		}
	}
	if !isStringMethod && !isListMethod && recvType == nil {
		switch e.Method {
		case "trim", "trimStart", "trimEnd", "toUpperCase", "toLowerCase",
			"replace", "replaceAll", "split", "includes", "startsWith",
			"endsWith", "indexOf", "lastIndexOf", "slice", "substring", "at", "charAt", "padStart", "padEnd",
			"repeat", "concat":
			isStringMethod = true
		}
	}
	if isStringMethod {
		return g.genStringMethod(recv, e)
	}
	if recvType != nil && recvType.Kind == ast.TypeSet {
		if e.Method == "add" {
			return "", fmt.Errorf("Set.add() is only supported as a statement")
		}
		return g.genSetMethod(e)
	}
	if isListMethod {
		return g.genListMethod(recv, e)
	}
	// Default: try string methods (most common when type is unknown)
	return g.genStringMethod(recv, e)
}

func (g *Generator) genFetchResponseMethod(e *ast.MethodCallExpr) (string, error) {
	if e.Method != "text" {
		return "", fmt.Errorf("FetchResponse has no method %q; this fetch() slice only supports text()", e.Method)
	}
	if len(e.Args) != 0 {
		return "", fmt.Errorf("FetchResponse.text() takes no arguments")
	}
	if fetchCall, ok := e.Receiver.(*ast.BuiltinCallExpr); ok && fetchCall.Name == "fetch" {
		return g.genFetchCall(fetchCall)
	}
	if ident, ok := e.Receiver.(*ast.IdentExpr); ok {
		varName := g.resolveVarName(ident.Name)
		return fmt.Sprintf("\"$%s\"", objectPropVar(varName, "body")), nil
	}
	return "", fmt.Errorf("FetchResponse.text() receiver must be fetch(url) or a fetch response variable")
}

func (g *Generator) genFetchCall(e *ast.BuiltinCallExpr) (string, error) {
	if len(e.Args) != 1 {
		return "", fmt.Errorf("fetch() takes 1 URL argument; options are not supported yet")
	}
	url, err := g.genExprValue(e.Args[0])
	if err != nil {
		return "", err
	}
	return commandSubstitution("curl -sS -- " + ensureArgSafe(cmdArgQuote(url))), nil
}

func shellWordList(values map[string]bool) string {
	if len(values) == 0 {
		return shellQuote("")
	}
	keys := make([]string, 0, len(values))
	for v := range values {
		keys = append(keys, v)
	}
	sort.Strings(keys)
	return shellQuote(strings.Join(keys, "\n"))
}

func (g *Generator) genArgsMethod(e *ast.MethodCallExpr) (string, error) {
	switch e.Method {
	case "argv":
		g.requireRuntimeHelper("args")
		if len(e.Args) != 0 {
			return "", fmt.Errorf("Besht.args.argv() takes no arguments")
		}
		return fmt.Sprintf("$(_bst_args_call _bst_args_argv %s %s)", shellWordList(g.argsOptions), shellWordList(g.argsFlags)), nil
	case "positional":
		if len(e.Args) != 1 {
			return "", fmt.Errorf("Besht.args.positional() takes 1 argument")
		}
		idx, err := g.genExprValue(e.Args[0])
		if err != nil {
			return "", err
		}
		if g.canInlineTopLevelPositionalArg() {
			return g.genInlineTopLevelPositionalArg(idx), nil
		}
		g.requireRuntimeHelper("args")
		return fmt.Sprintf("$(_bst_args_call _bst_args_positional %s %s %s)", ensureArgSafe(idx), shellWordList(g.argsOptions), shellWordList(g.argsFlags)), nil
	case "option":
		g.requireRuntimeHelper("args")
		if len(e.Args) < 1 || len(e.Args) > 2 {
			return "", fmt.Errorf("Besht.args.option() takes 1 or 2 arguments")
		}
		longName, err := g.genExprValue(e.Args[0])
		if err != nil {
			return "", err
		}
		shortName := shellQuote("")
		if len(e.Args) == 2 {
			shortName, err = g.genExprValue(e.Args[1])
			if err != nil {
				return "", err
			}
		}
		return fmt.Sprintf("$(_bst_args_call _bst_args_option %s %s)", ensureArgSafe(longName), ensureArgSafe(shortName)), nil
	case "flag":
		g.requireRuntimeHelper("args")
		if len(e.Args) < 1 || len(e.Args) > 2 {
			return "", fmt.Errorf("Besht.args.flag() takes 1 or 2 arguments")
		}
		longName, err := g.genExprValue(e.Args[0])
		if err != nil {
			return "", err
		}
		shortName := shellQuote("")
		if len(e.Args) == 2 {
			shortName, err = g.genExprValue(e.Args[1])
			if err != nil {
				return "", err
			}
		}
		return fmt.Sprintf("$(_bst_args_call _bst_args_flag %s %s)", ensureArgSafe(longName), ensureArgSafe(shortName)), nil
	}
	return "", fmt.Errorf("Besht.args has no method %q", e.Method)
}

func (g *Generator) canInlineTopLevelPositionalArg() bool {
	return !g.inFunction && len(g.argsOptions) == 0 && len(g.argsFlags) == 0
}

func (g *Generator) genInlineTopLevelPositionalArg(idx string) string {
	g.requireRuntimeHelper("nullish")
	return fmt.Sprintf("$(_bst_n=%s; _bst_i=1; for _bst_a do if [ \"$_bst_a\" = '--' ]; then continue; fi; if [ \"$_bst_i\" -eq \"$_bst_n\" ]; then printf '%%s' \"$_bst_a\"; exit 0; fi; _bst_i=$(( _bst_i + 1 )); done; printf '%%s' \"$%s\")", ensureArgSafe(idx), nullishSentinelVar)
}

func (g *Generator) argsArgcExpr() string {
	g.requireRuntimeHelper("args")
	return fmt.Sprintf("$(_bst_args_call _bst_args_argc %s %s)", shellWordList(g.argsOptions), shellWordList(g.argsFlags))
}

func (g *Generator) listLengthExpr(expr ast.Expression) string {
	if ident, ok := expr.(*ast.IdentExpr); ok {
		if known, ok := g.listLenMap[g.resolveVarName(ident.Name)]; ok {
			return known
		}
	}
	if isArgsArgvCall(expr) {
		return g.argsArgcExpr()
	}
	return ""
}

func (g *Generator) receiverClassName(receiver ast.Expression) (string, bool) {
	if ident, ok := receiver.(*ast.IdentExpr); ok {
		if classDecl := g.classMap[g.resolveClassName(ident.Name)]; classDecl != nil {
			return classDecl.Name, true
		}
		varName := g.resolveVarName(ident.Name)
		if className, ok := g.varClassMap[varName]; ok {
			return className, true
		}
		if className, ok := g.varClassMap[ident.Name]; ok {
			return className, true
		}
	}
	if _, ok := receiver.(*ast.ThisExpr); ok && g.currentClass != "" {
		return g.currentClass, true
	}
	return "", false
}

func (g *Generator) genClassMethodCall(className string, e *ast.MethodCallExpr) (string, error) {
	args, err := g.genFnArgs(e.Args)
	if err != nil {
		return "", err
	}
	if ident, ok := e.Receiver.(*ast.IdentExpr); ok {
		if classDecl := g.classMap[g.resolveClassName(ident.Name)]; classDecl != nil {
			call := classMethodName(classDecl.Name, e.Method)
			if len(args) == 0 {
				return fmt.Sprintf("$(%s)", call), nil
			}
			return fmt.Sprintf("$(%s %s)", call, strings.Join(args, " ")), nil
		}
	}
	recv, err := g.genExprValue(e.Receiver)
	if err != nil {
		return "", err
	}
	callArgs := append([]string{recv}, args...)
	return fmt.Sprintf("$(%s %s)", classMethodName(className, e.Method), strings.Join(callArgs, " ")), nil
}

func (g *Generator) genClassMethodStmt(className string, e *ast.MethodCallExpr) error {
	line, err := g.classMethodStmtLine(className, e)
	if err != nil {
		return err
	}
	g.line(line)
	return nil
}

func (g *Generator) classMethodStmtLine(className string, e *ast.MethodCallExpr) (string, error) {
	args, err := g.genFnArgs(e.Args)
	if err != nil {
		return "", err
	}
	if ident, ok := e.Receiver.(*ast.IdentExpr); ok {
		if classDecl := g.classMap[g.resolveClassName(ident.Name)]; classDecl != nil {
			if len(args) == 0 {
				return classMethodName(classDecl.Name, e.Method), nil
			}
			return fmt.Sprintf("%s %s", classMethodName(classDecl.Name, e.Method), strings.Join(args, " ")), nil
		}
	}
	recv, err := g.genExprValue(e.Receiver)
	if err != nil {
		return "", err
	}
	callArgs := append([]string{recv}, args...)
	return fmt.Sprintf("%s %s", classMethodName(className, e.Method), strings.Join(callArgs, " ")), nil
}

func (g *Generator) genOptionalClassMethodStmt(className string, e *ast.MethodCallExpr) error {
	g.requireRuntimeHelper("nullish")
	recv, err := g.genNullishValue(e.Receiver)
	if err != nil {
		return err
	}
	tmpName := fmt.Sprintf("_bst_opt_recv_%d_%d", e.Pos.Line, e.Pos.Column)
	line, err := g.withTempReceiver(tmpName, e.Pos, e.Receiver, func(recvExpr ast.Expression) (string, error) {
		clone := *e
		clone.Optional = false
		clone.Receiver = recvExpr
		return g.classMethodStmtLine(className, &clone)
	})
	if err != nil {
		return err
	}
	g.line(fmt.Sprintf("%s=%s", tmpName, recv))
	g.line(fmt.Sprintf("if [ \"$%s\" != \"$%s\" ]; then", tmpName, nullishSentinelVar))
	g.push()
	g.line(line)
	g.pop()
	g.line("fi")
	return nil
}

func (g *Generator) resolveClassName(name string) string {
	return name
}

func (g *Generator) genArgs2(args []ast.Expression) ([]string, error) {
	return g.genArgs(args)
}

func (g *Generator) genListMethod(recv string, e *ast.MethodCallExpr) (string, error) {
	arg0 := func() (string, error) {
		if len(e.Args) == 0 {
			return "", fmt.Errorf("%s() requires an argument", e.Method)
		}
		return g.genExprValue(e.Args[0])
	}
	switch e.Method {
	case "push":
		a0, err := arg0()
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("$(if [ -n %s ]; then printf '%%s\\n%%s' %s %s; else printf '%%s' %s; fi)", recv, recv, a0, a0), nil

	case "unshift":
		a0, err := arg0()
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("$(if [ -n %s ]; then printf '%%s\\n%%s' %s %s; else printf '%%s' %s; fi)", ensureArgSafe(recv), ensureArgSafe(a0), ensureArgSafe(recv), ensureArgSafe(a0)), nil

	case "pop":
		return fmt.Sprintf("$(printf '%%s\\n' %s | head -n -1)", recv), nil

	case "shift":
		return fmt.Sprintf("$(printf '%%s\\n' %s | tail -n +2)", recv), nil

	case "concat":
		a0, err := arg0()
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("$(printf '%%s\\n%%s' %s %s)", recv, a0), nil

	case "slice":
		startStr, err := g.genExprValue(e.Args[0])
		if err != nil {
			return "", err
		}
		startClean := stripQuotes(startStr)
		if len(e.Args) == 1 {
			return fmt.Sprintf("$(printf '%%s\\n' %s | tail -n +$(( %s + 1 )))", recv, startClean), nil
		}
		endStr, err := g.genExprValue(e.Args[1])
		if err != nil {
			return "", err
		}
		negativeEnd := "n+e"
		if isNewlineSplit(e.Receiver) {
			negativeEnd = "n+e+1"
		}
		return fmt.Sprintf("$(printf '%%s\\n' %s | awk %s %s '{a[NR]=$0}END{n=NR; s=int(_s); e=int(_e); if(s<0)s=n+s; if(e<0)e=%s; if(s<0)s=0; if(e<0)e=0; if(s>n)s=n; if(e>n)e=n; for(i=s+1;i<=e;i++) print a[i]}')", ensureArgSafe(recv), awkArg("_s", startStr), awkArg("_e", endStr), negativeEnd), nil

	case "join":
		a0, err := arg0()
		if err != nil {
			return "", err
		}
		return g.genListJoin(recv, e, stripQuotes(a0)), nil

	case "toString":
		if len(e.Args) != 0 {
			return "", fmt.Errorf("toString() takes no arguments")
		}
		return g.genListJoin(recv, e, ","), nil

	case "includes":
		a0, err := arg0()
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("$(printf '%%s\\n' %s | grep -qxF %s && printf 1 || printf 0)", recv, a0), nil

	case "indexOf":
		a0, err := arg0()
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("$(printf '%%s\\n' %s | awk %s 'BEGIN{found=-1}{if(found<0 && $0 == _needle) found=NR-1}END{printf \"%%s\", found}')", ensureArgSafe(recv), awkArg("_needle", a0)), nil

	case "findIndex":
		return g.genListFindIndex(recv, e)

	case "some":
		return g.genListSomeEvery(recv, e, true)

	case "every":
		return g.genListSomeEvery(recv, e, false)

	case "find":
		return g.genListFind(recv, e)

	case "lastIndexOf":
		a0, err := arg0()
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("$(printf '%%s\\n' %s | awk %s 'BEGIN{found=-1}{if ($0 == _needle) found=NR-1}END{printf \"%%s\", found}')", ensureArgSafe(recv), awkArg("_needle", a0)), nil

	case "reverse":
		return fmt.Sprintf("$(printf '%%s\\n' %s | tail -r 2>/dev/null || printf '%%s\\n' %s | awk 'BEGIN{OFMT=\"%%.17g\";i=0}{a[i++]=$0}END{while(i--)print a[i]}')", recv, recv), nil

	case "map":
		return g.genListMap(recv, e)

	case "filter":
		return g.genListFilter(recv, e)

	case "reduce":
		return "", fmt.Errorf("reduce() must be used in a let/const declaration")

	case "forEach":
		return "", fmt.Errorf("forEach() must be used as a statement")
	}
	return "", fmt.Errorf("list has no method %q", e.Method)
}

func (g *Generator) genStaticListJoinMethod(e *ast.MethodCallExpr) (string, bool, error) {
	list, ok := e.Receiver.(*ast.ListLit)
	if !ok {
		return "", false, nil
	}
	values, ok := staticScalarListValues(list)
	if !ok {
		return "", false, nil
	}

	switch e.Method {
	case "join":
		if len(e.Args) != 1 {
			return "", true, fmt.Errorf("join() requires an argument")
		}
		sep, ok := staticScalarValue(e.Args[0])
		if !ok {
			return "", false, nil
		}
		return shellQuote(strings.Join(values, sep)), true, nil
	case "toString":
		if len(e.Args) != 0 {
			return "", true, fmt.Errorf("toString() takes no arguments")
		}
		return shellQuote(strings.Join(values, ",")), true, nil
	default:
		return "", false, nil
	}
}

func (g *Generator) genListForEachStmt(e *ast.MethodCallExpr) error {
	arrow, err := listArrowArg(e)
	if err != nil {
		return err
	}
	recv, err := g.genExprValue(e.Receiver)
	if err != nil {
		return err
	}
	dataVar := fmt.Sprintf("_foreach_%d_%d", e.Pos.Line, e.Pos.Column)
	idxVar := fmt.Sprintf("_foreach_idx_%d_%d", e.Pos.Line, e.Pos.Column)
	tag := fmt.Sprintf("__BESHT_FOREACH_%d_%d", e.Pos.Line, e.Pos.Column)
	g.line(fmt.Sprintf("%s=%s", dataVar, recv))
	g.line(fmt.Sprintf("if [ -n \"$%s\" ]; then", dataVar))
	g.push()
	err = g.withCallbackParams(arrow, nil, func(ctx callbackLoopCtx) error {
		var oldElemType *ast.Type
		hadOldElemType := false
		if recvType := g.inferReceiverType(e.Receiver); recvType != nil && recvType.Kind == ast.TypeList && recvType.Elem != nil && len(arrow.Params) > 0 {
			oldElemType, hadOldElemType = g.fnParamTypes[arrow.Params[0].Name]
			g.fnParamTypes[arrow.Params[0].Name] = recvType.Elem
			defer func() {
				if hadOldElemType {
					g.fnParamTypes[arrow.Params[0].Name] = oldElemType
				} else {
					delete(g.fnParamTypes, arrow.Params[0].Name)
				}
			}()
		}
		var oldIndexType *ast.Type
		hadOldIndexType := false
		if len(arrow.Params) > 1 {
			oldIndexType, hadOldIndexType = g.fnParamTypes[arrow.Params[1].Name]
			g.fnParamTypes[arrow.Params[1].Name] = typeNumber
			defer func() {
				if hadOldIndexType {
					g.fnParamTypes[arrow.Params[1].Name] = oldIndexType
				} else {
					delete(g.fnParamTypes, arrow.Params[1].Name)
				}
			}()
		}
		param := ctx.ParamVars[0]
		indexVar := ""
		if len(ctx.ParamVars) > 1 {
			indexVar = ctx.ParamVars[1]
			g.line(fmt.Sprintf("%s=0", idxVar))
		}
		g.line(fmt.Sprintf("while IFS= read -r %s; do", param))
		g.push()
		if recvType := g.inferReceiverType(e.Receiver); recvType != nil && recvType.Kind == ast.TypeList && recvType.Elem != nil && recvType.Elem.Kind == ast.TypeList {
			g.line(fmt.Sprintf("%s=$(printf '%%s' \"$%s\" | tr '\\037' '\\n')", param, param))
		}
		if indexVar != "" {
			g.line(fmt.Sprintf("%s=$%s", indexVar, idxVar))
		}
		if arrow.BlockBody != nil {
			for _, stmt := range arrow.BlockBody.Statements {
				if err := g.genForEachCallbackStmt(stmt); err != nil {
					return err
				}
			}
		} else {
			if err := g.genExprStmt(&ast.ExprStmt{Pos: arrow.Pos, Expr: arrow.Body}); err != nil {
				return err
			}
		}
		if indexVar != "" {
			g.line(fmt.Sprintf("%s=$(( %s + 1 ))", idxVar, idxVar))
		}
		g.pop()
		g.line("done <<" + tag)
		g.raw(fmt.Sprintf("$%s\n", dataVar))
		g.raw(tag + "\n")
		return nil
	})
	g.pop()
	g.line("fi")
	return err
}

func (g *Generator) genForEachCallbackStmt(stmt ast.Statement) error {
	switch s := stmt.(type) {
	case *ast.ReturnStmt:
		return fmt.Errorf("forEach() callback does not support return")
	case *ast.BreakStmt:
		return fmt.Errorf("forEach() callback does not support break")
	case *ast.ContinueStmt:
		return fmt.Errorf("forEach() callback does not support continue")
	case *ast.IfStmt:
		cond, err := g.genCondition(s.Condition)
		if err != nil {
			return err
		}
		g.line(fmt.Sprintf("if %s; then", cond))
		g.push()
		if err := g.genForEachCallbackBlock(s.Then); err != nil {
			return err
		}
		g.pop()
		for _, ei := range s.ElseIfs {
			eiCond, err := g.genCondition(ei.Condition)
			if err != nil {
				return err
			}
			g.line(fmt.Sprintf("elif %s; then", eiCond))
			g.push()
			if err := g.genForEachCallbackBlock(ei.Body); err != nil {
				return err
			}
			g.pop()
		}
		if s.Else != nil {
			g.line("else")
			g.push()
			if err := g.genForEachCallbackBlock(s.Else); err != nil {
				return err
			}
			g.pop()
		}
		g.line("fi")
		return nil
	default:
		return g.genStmt(stmt)
	}
}

func (g *Generator) genForEachCallbackBlock(block *ast.Block) error {
	if block == nil {
		return nil
	}
	for _, stmt := range block.Statements {
		if err := g.genForEachCallbackStmt(stmt); err != nil {
			return err
		}
	}
	return nil
}

func (g *Generator) genListJoin(recv string, e *ast.MethodCallExpr, sep string) string {
	if sep == "\n" {
		if listLen := g.listLengthExpr(e.Receiver); listLen != "" {
			return fmt.Sprintf("$(printf '%%s\n' %s | awk %s 'NR<=_len{if(NR>1)printf \"\\n\"; if($0==\"__BESHT_NL__\")printf \"\\n\"; else printf \"%%s\",$0}END{for(i=NR+1;i<=_len;i++)printf \"\\n\"}')", ensureArgSafe(recv), awkArg("_len", listLen))
		}
		return fmt.Sprintf("$(printf '%%s\n' %s | awk 'NR>1{printf \"\\n\"}{if($0==\"__BESHT_NL__\")printf \"\\n\"; else printf \"%%s\",$0}')", ensureArgSafe(recv))
	}
	sepEscaped := strings.ReplaceAll(sep, "'", "'\"'\"'")
	if listLen := g.listLengthExpr(e.Receiver); listLen != "" {
		return fmt.Sprintf("$(printf '%%s\n' %s | awk -v s='%s' %s 'NR<=_len{if(NR>1)printf s; if($0==\"__BESHT_NL__\")printf \"\\n\"; else printf \"%%s\",$0}END{for(i=NR+1;i<=_len;i++)printf s}')", ensureArgSafe(recv), sepEscaped, awkArg("_len", listLen))
	}
	return fmt.Sprintf("$(printf '%%s\n' %s | awk -v s='%s' 'NR>1{printf s}{if($0==\"__BESHT_NL__\")printf \"\\n\"; else printf \"%%s\",$0}')", ensureArgSafe(recv), sepEscaped)
}

func isNewlineSplit(expr ast.Expression) bool {
	call, ok := expr.(*ast.MethodCallExpr)
	if !ok || call.Method != "split" || len(call.Args) != 1 {
		return false
	}
	arg, ok := call.Args[0].(*ast.StringLit)
	return ok && arg.Value == "\n"
}

func shellSuffix(s string) string {
	if s == "" {
		return ""
	}
	return "; " + s
}

func (g *Generator) genListMap(recv string, e *ast.MethodCallExpr) (string, error) {
	arrow, err := listArrowArg(e)
	if err != nil {
		return "", err
	}
	var out string
	err = g.withCallbackParams(arrow, nil, func(ctx callbackLoopCtx) error {
		param := ctx.ParamVars[0]
		prefix := ""
		loopPrefix := ""
		if len(ctx.ParamVars) > 1 {
			counterVar := fmt.Sprintf("_cb_%d_%d_index", arrow.Pos.Line, arrow.Pos.Column)
			indexVar := ctx.ParamVars[1]
			prefix = counterVar + "=0; "
			loopPrefix = fmt.Sprintf("%s=$%s; %s=$(( %s + 1 )); ", indexVar, counterVar, counterVar, counterVar)
		}
		if arrow.BlockBody != nil {
			body, err := g.genMapCallbackBlockSource(arrow.BlockBody, "")
			if err != nil {
				return err
			}
			out = fmt.Sprintf("$(%sprintf '%%s\\n' %s | while IFS= read -r %s; do %s%s done)", prefix, ensureArgSafe(recv), param, loopPrefix, body)
			return nil
		}
		body, err := g.genExprValue(arrow.Body)
		if err != nil {
			return err
		}
		bodyType := g.inferReceiverType(arrow.Body)
		if bodyType != nil && bodyType.Kind == ast.TypeList {
			out = fmt.Sprintf("$(%sprintf '%%s\\n' %s | while IFS= read -r %s; do %sprintf '%%s\\n' %s | awk 'NR>1{printf \"\\037\"}{printf \"%%s\",$0}'; printf '\\n'; done)", prefix, ensureArgSafe(recv), param, loopPrefix, ensureArgSafe(body))
		} else {
			out = fmt.Sprintf("$(%sprintf '%%s\\n' %s | while IFS= read -r %s; do %sprintf '%%s\\n' %s; done)", prefix, ensureArgSafe(recv), param, loopPrefix, ensureArgSafe(body))
		}
		return nil
	})
	return out, err
}

func (g *Generator) genListFilter(recv string, e *ast.MethodCallExpr) (string, error) {
	arrow, err := listArrowArg(e)
	if err != nil {
		return "", err
	}
	var out string
	err = g.withCallbackParams(arrow, nil, func(ctx callbackLoopCtx) error {
		cond, err := g.genCondition(arrow.Body)
		if err != nil {
			return err
		}
		param := ctx.ParamVars[0]
		indexVar := ""
		if len(ctx.ParamVars) > 1 {
			indexVar = ctx.ParamVars[1]
		}
		prefix := ""
		inc := ""
		if indexVar != "" {
			prefix = indexVar + "=0; "
			inc = indexVar + "=$(( " + indexVar + " + 1 ))"
		}
		out = fmt.Sprintf("$(%sprintf '%%s\\n' %s | while IFS= read -r %s; do if %s; then printf '%%s\\n' \"$%s\"; fi%s; done)", prefix, ensureArgSafe(recv), param, cond, param, shellSuffix(inc))
		return nil
	})
	return out, err
}

func (g *Generator) genListFindIndex(recv string, e *ast.MethodCallExpr) (string, error) {
	arrow, err := listArrowArg(e)
	if err != nil {
		return "", err
	}
	var out string
	err = g.withCallbackParams(arrow, nil, func(ctx callbackLoopCtx) error {
		cond, err := g.genCondition(arrow.Body)
		if err != nil {
			return err
		}
		param := ctx.ParamVars[0]
		idx := fmt.Sprintf("_findidx_%d_%d", arrow.Pos.Line, arrow.Pos.Column)
		dataVar := fmt.Sprintf("_finddata_%d_%d", arrow.Pos.Line, arrow.Pos.Column)
		indexAssign := ""
		if len(ctx.ParamVars) > 1 {
			indexAssign = ctx.ParamVars[1] + "=$" + idx + "; "
		}
		out = fmt.Sprintf("$(%s=%s; %s=0; _found=-1; while IFS= read -r %s; do %sif [ $_found -lt 0 ] && %s; then _found=$%s; break; fi; %s=$(( %s + 1 )); done <<__BESHT_FIND_%d_%d\n$%s\n__BESHT_FIND_%d_%d\nprintf '%%s' \"$_found\")", dataVar, recv, idx, param, indexAssign, cond, idx, idx, idx, arrow.Pos.Line, arrow.Pos.Column, dataVar, arrow.Pos.Line, arrow.Pos.Column)
		return nil
	})
	return out, err
}

func (g *Generator) genListSomeEvery(recv string, e *ast.MethodCallExpr, some bool) (string, error) {
	arrow, err := listArrowArg(e)
	if err != nil {
		return "", err
	}
	if arrow.BlockBody != nil {
		return "", fmt.Errorf("%s() predicate callback must be expression-bodied", e.Method)
	}
	var out string
	err = g.withCallbackParams(arrow, nil, func(ctx callbackLoopCtx) error {
		cond, err := g.genCondition(arrow.Body)
		if err != nil {
			return err
		}
		param := ctx.ParamVars[0]
		idx := fmt.Sprintf("_listpred_%d_%d_index", arrow.Pos.Line, arrow.Pos.Column)
		dataVar := fmt.Sprintf("_listpred_%d_%d_data", arrow.Pos.Line, arrow.Pos.Column)
		resultVar := fmt.Sprintf("_listpred_%d_%d_result", arrow.Pos.Line, arrow.Pos.Column)
		indexAssign := ""
		if len(ctx.ParamVars) > 1 {
			indexAssign = ctx.ParamVars[1] + "=$" + idx + "; "
		}
		initial := "1"
		body := fmt.Sprintf("%sif %s; then :; else %s=0; break; fi;", indexAssign, cond, resultVar)
		if some {
			initial = "0"
			body = fmt.Sprintf("%sif %s; then %s=1; break; fi;", indexAssign, cond, resultVar)
		}
		out = fmt.Sprintf("$(%s=%s; %s=%s; %s=0; if [ -n \"$%s\" ]; then while IFS= read -r %s; do %s %s=$(( %s + 1 )); done <<__BESHT_LIST_PRED_%d_%d\n$%s\n__BESHT_LIST_PRED_%d_%d\nfi; printf '%%s' \"$%s\")", dataVar, recv, resultVar, initial, idx, dataVar, param, body, idx, idx, arrow.Pos.Line, arrow.Pos.Column, dataVar, arrow.Pos.Line, arrow.Pos.Column, resultVar)
		return nil
	})
	return out, err
}

func (g *Generator) genListFind(recv string, e *ast.MethodCallExpr) (string, error) {
	arrow, err := listArrowArg(e)
	if err != nil {
		return "", err
	}
	if arrow.BlockBody != nil {
		return "", fmt.Errorf("find() predicate callback must be expression-bodied")
	}
	g.requireRuntimeHelper("nullish")
	var out string
	err = g.withCallbackParams(arrow, nil, func(ctx callbackLoopCtx) error {
		cond, err := g.genCondition(arrow.Body)
		if err != nil {
			return err
		}
		param := ctx.ParamVars[0]
		idx := fmt.Sprintf("_listfind_%d_%d_index", arrow.Pos.Line, arrow.Pos.Column)
		dataVar := fmt.Sprintf("_listfind_%d_%d_data", arrow.Pos.Line, arrow.Pos.Column)
		resultVar := fmt.Sprintf("_listfind_%d_%d_result", arrow.Pos.Line, arrow.Pos.Column)
		indexAssign := ""
		if len(ctx.ParamVars) > 1 {
			indexAssign = ctx.ParamVars[1] + "=$" + idx + "; "
		}
		out = fmt.Sprintf("$(%s=%s; %s=$%s; %s=0; if [ -n \"$%s\" ]; then while IFS= read -r %s; do %sif %s; then %s=$%s; break; fi; %s=$(( %s + 1 )); done <<__BESHT_LIST_FIND_%d_%d\n$%s\n__BESHT_LIST_FIND_%d_%d\nfi; printf '%%s' \"$%s\")", dataVar, recv, resultVar, nullishSentinelVar, idx, dataVar, param, indexAssign, cond, resultVar, param, idx, idx, arrow.Pos.Line, arrow.Pos.Column, dataVar, arrow.Pos.Line, arrow.Pos.Column, resultVar)
		return nil
	})
	return out, err
}

func (g *Generator) genSetMethod(e *ast.MethodCallExpr) (string, error) {
	ident, ok := e.Receiver.(*ast.IdentExpr)
	if !ok {
		return "", fmt.Errorf("Set.%s() requires an identifier receiver", e.Method)
	}
	setVar := g.resolveVarName(ident.Name)
	arg, err := g.genExprValue(e.Args[0])
	if err != nil {
		return "", err
	}
	arg = ensureArgSafe(arg)
	switch e.Method {
	case "has":
		return fmt.Sprintf("$([ -n \"$%s\" ] && printf '%%s\\n' \"$%s\" | grep -qxF -- %s && printf 1 || printf 0)", setVar, setVar, arg), nil
	case "add":
		return fmt.Sprintf("%s=$( { if [ -n \"$%s\" ]; then printf '%%s\n' \"$%s\"; fi; printf '%%s\n' %s; } | awk '!seen[$0]++')", setVar, setVar, setVar, arg), nil
	}
	return "", fmt.Errorf("Set has no method %q", e.Method)
}

func (g *Generator) genMapCallbackStmtSource(stmt ast.Statement, indexVar string) (string, error) {
	switch s := stmt.(type) {
	case *ast.ReturnStmt:
		inc := ""
		if indexVar != "" {
			inc = indexVar + "=$(( " + indexVar + " + 1 ))"
		}
		if s.Value == nil {
			return shellSuffix(inc) + "; continue", nil
		}
		val, err := g.genExprValue(s.Value)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("printf '%%s\\n' %s%s; continue;", ensureArgSafe(val), inc), nil
	case *ast.IfStmt:
		cond, err := g.genCondition(s.Condition)
		if err != nil {
			return "", err
		}
		thenSrc, err := g.genMapCallbackBlockSource(s.Then, indexVar)
		if err != nil {
			return "", err
		}
		pieces := []string{fmt.Sprintf("if %s; then %s", cond, thenSrc)}
		for _, elseif := range s.ElseIfs {
			elseifCond, err := g.genCondition(elseif.Condition)
			if err != nil {
				return "", err
			}
			body, err := g.genMapCallbackBlockSource(elseif.Body, indexVar)
			if err != nil {
				return "", err
			}
			pieces = append(pieces, fmt.Sprintf("elif %s; then %s", elseifCond, body))
		}
		if s.Else != nil {
			body, err := g.genMapCallbackBlockSource(s.Else, indexVar)
			if err != nil {
				return "", err
			}
			pieces = append(pieces, "else "+body)
		}
		pieces = append(pieces, "fi")
		return strings.Join(pieces, " "), nil
	case *ast.ExprStmt:
		return "", fmt.Errorf("unsupported expression statement in map callback: %T", s.Expr)
	case *ast.Assignment:
		val, err := g.genExprRHS(s.Value, nil)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s=%s", g.resolveVarName(s.Name), val), nil
	}
	return "", fmt.Errorf("unsupported statement in map callback: %T", stmt)
}

func (g *Generator) genMapCallbackBlockSource(block *ast.Block, indexVar string) (string, error) {
	var parts []string
	for _, stmt := range block.Statements {
		src, err := g.genMapCallbackStmtSource(stmt, indexVar)
		if err != nil {
			return "", err
		}
		if src != "" {
			parts = append(parts, src)
		}
	}
	return strings.Join(parts, "; "), nil
}

func (g *Generator) inferMapBlockReturnType(block *ast.Block) *ast.Type {
	for _, stmt := range block.Statements {
		switch s := stmt.(type) {
		case *ast.ReturnStmt:
			if s.Value == nil {
				return &ast.Type{Kind: ast.TypeVoid}
			}
			return g.inferReceiverType(s.Value)
		case *ast.IfStmt:
			if t := g.inferMapBlockReturnType(s.Then); t != nil {
				return t
			}
			for _, elseif := range s.ElseIfs {
				if t := g.inferMapBlockReturnType(elseif.Body); t != nil {
					return t
				}
			}
			if s.Else != nil {
				if t := g.inferMapBlockReturnType(s.Else); t != nil {
					return t
				}
			}
		}
	}
	return nil
}

func listArrowArg(e *ast.MethodCallExpr) (*ast.ArrowExpr, error) {
	if len(e.Args) != 1 {
		return nil, fmt.Errorf("%s() takes 1 arrow callback", e.Method)
	}
	arrow, ok := e.Args[0].(*ast.ArrowExpr)
	if !ok {
		return nil, fmt.Errorf("%s() callback must be an arrow expression", e.Method)
	}
	if len(arrow.Params) < 1 || len(arrow.Params) > 2 {
		return nil, fmt.Errorf("arrow callbacks take 1 or 2 parameters")
	}
	return arrow, nil
}

func singleArrowArg(e *ast.MethodCallExpr) (*ast.ArrowExpr, error) {
	if len(e.Args) != 1 {
		return nil, fmt.Errorf("%s() takes 1 arrow callback", e.Method)
	}
	arrow, ok := e.Args[0].(*ast.ArrowExpr)
	if !ok {
		return nil, fmt.Errorf("%s() callback must be an arrow expression", e.Method)
	}
	if len(arrow.Params) < 1 || len(arrow.Params) > 2 {
		return nil, fmt.Errorf("arrow callbacks take 1 or 2 parameters")
	}
	return arrow, nil
}

func callbackParamName(e *ast.ArrowExpr) string {
	return callbackParamNameAt(e, 0)
}

func callbackParamNameAt(e *ast.ArrowExpr, index int) string {
	name := e.Params[index].Name
	return fmt.Sprintf("_cb_%d_%d_%s", e.Pos.Line, e.Pos.Column+index, mangle(name))
}

func (g *Generator) genMathMethod(e *ast.MethodCallExpr) (string, error) {
	genArg := func(i int) (string, error) {
		if i >= len(e.Args) {
			return "", fmt.Errorf("Math.%s requires at least %d argument(s)", e.Method, i+1)
		}
		v, err := g.genExprValue(e.Args[i])
		if err != nil {
			return "", err
		}
		return stripQuotes(v), nil
	}
	switch e.Method {
	case "min":
		a, err := genArg(0)
		if err != nil {
			return "", err
		}
		b, err := genArg(1)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("$(awk -v _a=%s -v _b=%s 'BEGIN{OFMT=\"%%.17g\";printf \"%%.16g\", (_a+0 < _b+0) ? _a : _b}')", a, b), nil

	case "max":
		a, err := genArg(0)
		if err != nil {
			return "", err
		}
		b, err := genArg(1)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("$(awk -v _a=%s -v _b=%s 'BEGIN{OFMT=\"%%.17g\";printf \"%%.16g\", (_a+0 > _b+0) ? _a : _b}')", a, b), nil

	case "round":
		a, err := genArg(0)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("$(awk -v _x=%s 'BEGIN{OFMT=\"%%.17g\";_y=_x+0.5; printf \"%%.16g\", int(_y)-((_y<int(_y))?1:0)}')", a), nil

	case "floor":
		a, err := genArg(0)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("$(awk -v _x=%s 'BEGIN{OFMT=\"%%.17g\";printf \"%%.16g\", int(_x) - (_x < int(_x) ? 1 : 0)}')", a), nil

	case "ceil":
		a, err := genArg(0)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("$(awk -v _x=%s 'BEGIN{OFMT=\"%%.17g\";printf \"%%.16g\", int(_x) + (_x > int(_x) ? 1 : 0)}')", a), nil

	case "trunc":
		a, err := g.genExprValue(e.Args[0])
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("$(awk %s 'BEGIN{OFMT=\"%%.17g\";_t=int(_x);printf \"%%.16g\", (_t == 0) ? 0 : _t}')", awkArg("_x", a)), nil

	case "sign":
		a, err := g.genExprValue(e.Args[0])
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("$(awk %s 'BEGIN{OFMT=\"%%.17g\";_n=_x+0;printf \"%%d\", (_n > 0) ? 1 : ((_n < 0) ? -1 : 0)}')", awkArg("_x", a)), nil

	case "abs":
		a, err := genArg(0)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("$(awk -v _x=%s 'BEGIN{OFMT=\"%%.17g\";printf \"%%.16g\", (_x < 0) ? -_x : _x}')", a), nil

	case "pow":
		a, err := genArg(0)
		if err != nil {
			return "", err
		}
		b, err := genArg(1)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("$(awk -v _a=%s -v _b=%s 'BEGIN{OFMT=\"%%.17g\";printf \"%%.16g\", _a ^ _b}')", a, b), nil

	case "sqrt":
		a, err := genArg(0)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("$(awk -v _x=%s 'BEGIN{OFMT=\"%%.17g\";printf \"%%.16g\", sqrt(_x)}')", a), nil
	}
	return "", fmt.Errorf("Math has no method %q", e.Method)
}

func (g *Generator) genStringMethod(recv string, e *ast.MethodCallExpr) (string, error) {
	arg0 := func() (string, error) {
		if len(e.Args) == 0 {
			return "", fmt.Errorf("%s() requires an argument", e.Method)
		}
		return g.genExprValue(e.Args[0])
	}
	switch e.Method {
	case "split":
		a0, err := arg0()
		if err != nil {
			return "", err
		}
		sep := stripQuotes(a0)
		if sep == "" {
			return fmt.Sprintf("$(printf '%%s' %s | awk 'BEGIN{ORS=\"\"}{if(NR>1)print \"__BESHT_NL__\\n\"; for(i=1;i<=length($0);i++) print substr($0,i,1) \"\\n\"}')", ensureArgSafe(recv)), nil
		}
		return fmt.Sprintf("$(printf '%%s' %s | tr '%s' '\\n')", ensureArgSafe(recv), sep), nil

	case "trim":
		return fmt.Sprintf("$(printf '%%s' %s | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')", ensureArgSafe(recv)), nil

	case "trimStart":
		return fmt.Sprintf("$(printf '%%s' %s | sed 's/^[[:space:]]*//')", ensureArgSafe(recv)), nil

	case "trimEnd":
		return fmt.Sprintf("$(printf '%%s' %s | sed 's/[[:space:]]*$//')", ensureArgSafe(recv)), nil

	case "toUpperCase":
		return fmt.Sprintf("$(printf '%%s' %s | tr '[:lower:]' '[:upper:]')", ensureArgSafe(recv)), nil

	case "toLowerCase":
		return fmt.Sprintf("$(printf '%%s' %s | tr '[:upper:]' '[:lower:]')", ensureArgSafe(recv)), nil

	case "includes":
		a0, err := arg0()
		if err != nil {
			return "", err
		}
		if len(e.Args) == 2 {
			pos, err := g.genExprValue(e.Args[1])
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("$(awk %s %s %s 'BEGIN{_len=length(_s);_p=int(_pos);if(_p<0)_p=0;if(_p>_len)_p=_len;_n=length(_needle);if(_n==0){printf 1;exit}_tail=substr(_s,_p+1);printf (index(_tail,_needle)>0)?1:0}')", awkArg("_s", recv), awkArg("_needle", a0), awkArg("_pos", pos)), nil
		}
		g.requireRuntimeHelper("includes")
		return fmt.Sprintf("$(_bst_includes %s %s && printf 1 || printf 0)", ensureArgSafe(recv), a0), nil

	case "startsWith":
		a0, err := arg0()
		if err != nil {
			return "", err
		}
		if len(e.Args) == 2 {
			pos, err := g.genExprValue(e.Args[1])
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("$(awk %s %s %s 'BEGIN{_len=length(_s);_p=int(_pos);if(_p<0)_p=0;if(_p>_len)_p=_len;_n=length(_needle);printf (substr(_s,_p+1,_n)==_needle)?1:0}')", awkArg("_s", recv), awkArg("_needle", a0), awkArg("_pos", pos)), nil
		}
		g.requireRuntimeHelper("startsWith")
		return fmt.Sprintf("$(_bst_starts_with %s %s && printf 1 || printf 0)", ensureArgSafe(recv), a0), nil

	case "endsWith":
		a0, err := arg0()
		if err != nil {
			return "", err
		}
		if len(e.Args) == 2 {
			lengthArg, err := g.genExprValue(e.Args[1])
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("$(awk %s %s %s 'BEGIN{_slen=length(_s);_end=int(_len);if(_end<0)_end=0;if(_end>_slen)_end=_slen;_n=length(_needle);_prefix=substr(_s,1,_end);printf (substr(_prefix,length(_prefix)-_n+1)==_needle)?1:0}')", awkArg("_s", recv), awkArg("_needle", a0), awkArg("_len", lengthArg)), nil
		}
		g.requireRuntimeHelper("endsWith")
		return fmt.Sprintf("$(_bst_ends_with %s %s && printf 1 || printf 0)", ensureArgSafe(recv), a0), nil

	case "replace":
		a0, err := g.genExprValue(e.Args[0])
		if err != nil {
			return "", err
		}
		a1, err := g.genExprValue(e.Args[1])
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("$(printf '%%s' %s | sed \"s/%s/%s/\")", ensureArgSafe(recv), stripQuotes(a0), stripQuotes(a1)), nil

	case "replaceAll":
		a0, err := g.genExprValue(e.Args[0])
		if err != nil {
			return "", err
		}
		a1, err := g.genExprValue(e.Args[1])
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("$(printf '%%s' %s | sed \"s/%s/%s/g\")", ensureArgSafe(recv), stripQuotes(a0), stripQuotes(a1)), nil

	case "padStart":
		lenArg, err := g.genExprValue(e.Args[0])
		if err != nil {
			return "", err
		}
		fill := " "
		if len(e.Args) == 2 {
			fillArg, err := g.genExprValue(e.Args[1])
			if err != nil {
				return "", err
			}
			fill = stripQuotes(fillArg)
		}
		return fmt.Sprintf("$(printf '%%%ss' %s | tr ' ' '%s')", stripQuotes(lenArg), recv, fill), nil

	case "padEnd":
		lenArg, err := g.genExprValue(e.Args[0])
		if err != nil {
			return "", err
		}
		fill := " "
		if len(e.Args) == 2 {
			fillArg, err := g.genExprValue(e.Args[1])
			if err != nil {
				return "", err
			}
			fill = stripQuotes(fillArg)
		}
		return fmt.Sprintf("$(printf '%%-*s' %s %s | tr ' ' '%s')", stripQuotes(lenArg), recv, fill), nil

	case "repeat":
		a0, err := arg0()
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("$(awk -v _s=%s -v _n=%s 'BEGIN{OFMT=\"%%.17g\";r=\"\"; for(i=0;i<_n;i++) r=r _s; printf \"%%s\",r}')", ensureArgSafe(recv), stripQuotes(a0)), nil

	case "indexOf":
		a0, err := arg0()
		if err != nil {
			return "", err
		}
		if len(e.Args) == 2 {
			pos, err := g.genExprValue(e.Args[1])
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("$(awk %s %s %s 'BEGIN{_len=length(_s);_p=int(_pos);if(_p<0)_p=0;if(_p>_len)_p=_len;_n=length(_needle);if(_n==0){printf \"%%d\", _p; exit}_tail=substr(_s,_p+1);_found=index(_tail,_needle);printf \"%%d\", (_found>0)?_p+_found-1:-1}')", awkArg("_s", recv), awkArg("_needle", a0), awkArg("_pos", pos)), nil
		}
		return fmt.Sprintf("$(awk %s %s 'BEGIN{_n=length(_needle); if(_n==0){printf \"0\"; exit} p=index(_s,_needle)-1; print (p<0)?-1:p}')", awkArg("_s", recv), awkArg("_needle", a0)), nil

	case "lastIndexOf":
		a0, err := arg0()
		if err != nil {
			return "", err
		}
		posArg := ""
		posExpr := "length(_s)"
		if len(e.Args) == 2 {
			pos, err := g.genExprValue(e.Args[1])
			if err != nil {
				return "", err
			}
			posArg = " " + awkArg("_pos", pos)
			posExpr = "_pos"
		}
		return fmt.Sprintf("$(awk %s %s%s 'BEGIN{found=-1; _len=length(_s); n=length(_needle); _p=int(%s); if(_p<0)_p=0; if(_p>_len)_p=_len; if(n==0){printf \"%%d\", _p; exit} limit=_len-n; if(_p<limit) limit=_p; for(i=0;i<=limit;i++){if(substr(_s,i+1,n)==_needle) found=i} printf \"%%d\", found}')", awkArg("_s", recv), awkArg("_needle", a0), posArg, posExpr), nil

	case "at":
		a0, err := arg0()
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("$(printf '%%s' %s | cut -c$(( %s + 1 )))", ensureArgSafe(recv), stripQuotes(a0)), nil

	case "charAt":
		a0, err := arg0()
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("$(awk %s %s 'BEGIN{_i=int(_idx); if (_i < 0 || _i >= length(_s)) exit; printf \"%%s\", substr(_s, _i + 1, 1)}')", awkArg("_s", recv), awkArg("_idx", a0)), nil

	case "slice":
		startStr, err := g.genExprValue(e.Args[0])
		if err != nil {
			return "", err
		}
		startClean := stripQuotes(startStr)
		if len(e.Args) == 1 {
			return fmt.Sprintf("$(printf '%%s' %s | cut -c$(( %s + 1 ))-)", ensureArgSafe(recv), startClean), nil
		}
		endStr, err := g.genExprValue(e.Args[1])
		if err != nil {
			return "", err
		}
		endClean := stripQuotes(endStr)
		return fmt.Sprintf("$(printf '%%s' %s | cut -c$(( %s + 1 ))-$(( %s )))", ensureArgSafe(recv), startClean, endClean), nil

	case "substring":
		startStr, err := g.genExprValue(e.Args[0])
		if err != nil {
			return "", err
		}
		endArg := ""
		endExpr := "length(_s)"
		if len(e.Args) == 2 {
			endStr, err := g.genExprValue(e.Args[1])
			if err != nil {
				return "", err
			}
			endArg = " " + awkArg("_end", endStr)
			endExpr = "_end"
		}
		return fmt.Sprintf("$(awk %s %s%s 'BEGIN{OFMT=\"%%.17g\";_len=length(_s);_a=int(_start);_b=int(%s);if(_a<0)_a=0;if(_b<0)_b=0;if(_a>_len)_a=_len;if(_b>_len)_b=_len;if(_a>_b){_t=_a;_a=_b;_b=_t}printf \"%%s\", substr(_s,_a+1,_b-_a)}')", awkArg("_s", recv), awkArg("_start", startStr), endArg, endExpr), nil

	case "concat":
		parts := []string{strInner(recv)}
		for _, arg := range e.Args {
			a, err := g.genExprValue(arg)
			if err != nil {
				return "", err
			}
			parts = append(parts, strInner(a))
		}
		return fmt.Sprintf("\"%s\"", strings.Join(parts, "")), nil
	}
	return "", fmt.Errorf("string has no method %q", e.Method)
}

func (g *Generator) genIndexExpr(e *ast.IndexExpr) (string, error) {
	if prop, ok := e.Expr.(*ast.PropertyExpr); ok {
		if ident, ok := prop.Receiver.(*ast.IdentExpr); ok {
			if classDecl := g.classMap[g.resolveClassName(ident.Name)]; classDecl != nil {
				for _, staticProp := range classDecl.StaticProps {
					if staticProp.Name == prop.Property && staticProp.Value != nil {
						if _, ok := staticProp.Value.(*ast.ObjectLit); !ok {
							break
						}
						return g.genComputedObjectAccess(classPropVar(classDecl.Name, prop.Property), e)
					}
				}
			}
			className := ident.Name
			if g.currentClass == ident.Name || strings.HasSuffix(g.currentClass, "__"+ident.Name) {
				className = g.currentClass
			}
			if elemType, ok := g.objPropTypeMap[classPropVar(className, prop.Property)+".*"]; ok && elemType != nil {
				return g.genComputedObjectAccess(classPropVar(className, prop.Property), e)
			}
		}
	}
	recvType := g.inferReceiverType(e.Expr)
	if recvType != nil && recvType.Kind == ast.TypeObject {
		return g.genComputedPropertyAccess(e)
	}
	listStr, err := g.genExprValue(e.Expr)
	if err != nil {
		return "", err
	}
	indexStr, err := g.genExprValue(e.Index)
	if err != nil {
		return "", err
	}
	indexClean := stripQuotes(indexStr)
	listArg := ensureArgSafe(listStr)
	if recvType := g.inferReceiverType(e.Expr); recvType != nil && recvType.Kind == ast.TypeString {
		return fmt.Sprintf("$(printf '%%s' %s | cut -c$(( %s + 1 )))", ensureArgSafe(listStr), indexClean), nil
	}
	if recvType := g.inferReceiverType(e.Expr); recvType != nil && recvType.Kind == ast.TypeList && recvType.Elem != nil && recvType.Elem.Kind == ast.TypeList {
		return fmt.Sprintf("$(printf '%%s\\n' %s | sed -n \"$(( %s + 1 ))p\" | tr '\\037' '\\n')", listArg, indexClean), nil
	}
	return fmt.Sprintf("$(printf '%%s\\n' %s | sed -n \"$(( %s + 1 ))p\")", listArg, indexClean), nil
}

func (g *Generator) genNullishIndexExpr(e *ast.IndexExpr) (string, error) {
	g.requireRuntimeHelper("nullish")
	recvType := g.inferReceiverType(e.Expr)
	if recvType != nil && recvType.Kind == ast.TypeObject {
		return g.genComputedPropertyAccess(e)
	}
	listStr, err := g.genExprValue(e.Expr)
	if err != nil {
		return "", err
	}
	indexStr, err := g.genExprValue(e.Index)
	if err != nil {
		return "", err
	}
	indexClean := stripQuotes(indexStr)
	listArg := ensureArgSafe(listStr)
	listLen := g.listLengthExpr(e.Expr)
	if recvType != nil && recvType.Kind == ast.TypeString {
		return fmt.Sprintf("$(_bst_s=$%s; printf '%%s' %s | awk -v null=\"$_bst_s\" -v n=$(( %s + 1 )) 'BEGIN{out=null}{if(length($0)>=n) out=substr($0,n,1)}END{printf \"%%s\", out}')", nullishSentinelVar, ensureArgSafe(listStr), indexClean), nil
	}
	if listLen != "" {
		if recvType != nil && recvType.Kind == ast.TypeList && recvType.Elem != nil && recvType.Elem.Kind == ast.TypeList {
			return fmt.Sprintf("$(_bst_s=$%s; printf '%%s\\n' %s | awk -v null=\"$_bst_s\" -v n=$(( %s + 1 )) %s 'BEGIN{out=null}{if(NR==n) out=$0}END{if(n<=_len && out==null) out=\"\"; printf \"%%s\", out}' | tr '\\037' '\\n')", nullishSentinelVar, listArg, indexClean, awkArg("_len", listLen)), nil
		}
		return fmt.Sprintf("$(_bst_s=$%s; printf '%%s\\n' %s | awk -v null=\"$_bst_s\" -v n=$(( %s + 1 )) %s 'BEGIN{out=null}{if(NR==n) out=$0}END{if(n<=_len && out==null) out=\"\"; printf \"%%s\", out}')", nullishSentinelVar, listArg, indexClean, awkArg("_len", listLen)), nil
	}
	if recvType != nil && recvType.Kind == ast.TypeList && recvType.Elem != nil && recvType.Elem.Kind == ast.TypeList {
		return fmt.Sprintf("$(_bst_s=$%s; printf '%%s\\n' %s | awk -v null=\"$_bst_s\" -v n=$(( %s + 1 )) 'NR==n{found=1; printf \"%%s\", $0}END{if(!found) printf \"%%s\", null}' | tr '\\037' '\\n')", nullishSentinelVar, listArg, indexClean), nil
	}
	return fmt.Sprintf("$(_bst_s=$%s; printf '%%s\\n' %s | awk -v null=\"$_bst_s\" -v n=$(( %s + 1 )) 'NR==n{found=1; printf \"%%s\", $0}END{if(!found) printf \"%%s\", null}')", nullishSentinelVar, listArg, indexClean), nil
}

func (g *Generator) genComputedPropertyAccess(e *ast.IndexExpr) (string, error) {
	ref, ok := g.resolveObjectRef(e.Expr)
	if !ok {
		return "", fmt.Errorf("computed property access requires a simple identifier as receiver")
	}
	return g.genComputedObjectRefAccess(ref, e)
}

func (g *Generator) genComputedObjectRefAccess(ref objectRef, e *ast.IndexExpr) (string, error) {
	if ref.StaticName != "" {
		return g.genComputedObjectAccess(ref.StaticName, e)
	}
	keyStr, err := g.genExprValue(e.Index)
	if err != nil {
		return "", err
	}
	slotVar := fmt.Sprintf("_objs_%d_%d", e.Pos.Line, e.Pos.Column)
	keyVar := fmt.Sprintf("_objk_%d_%d", e.Pos.Line, e.Pos.Column)
	return "$(" + slotVar + "=" + ref.SlotExpr + "; " + computedKeyValidation(slotVar) + "; " + keyVar + "=" + ensureArgSafe(keyStr) + "; " + computedKeyValidation(keyVar) + "; eval \"printf '" + "%s" + "' \\\"\\${_obj_${" + slotVar + "}_${" + keyVar + "}}\\\"\")", nil
}

func (g *Generator) genComputedObjectAccess(objVar string, e *ast.IndexExpr) (string, error) {
	keyStr, err := g.genExprValue(e.Index)
	if err != nil {
		return "", err
	}
	keyVar := fmt.Sprintf("_objk_%d_%d", e.Pos.Line, e.Pos.Column)
	return "$(" + keyVar + "=" + ensureArgSafe(keyStr) + "; " + computedKeyValidation(keyVar) + "; eval \"printf '" + "%s" + "' \\\"\\${_obj_" + objVar + "_${" + keyVar + "}}\\\"\")", nil
}

func (g *Generator) genIndexAssign(s *ast.IndexAssignStmt) error {
	listVar := g.resolveVarName(s.Name)
	vt, isObj := g.varTypeMap[listVar]
	if !isObj && g.inFunction {
		if existing, ok := g.fnParamTypes[s.Name]; ok && existing.Kind == ast.TypeObject {
			vt = existing
			isObj = true
		}
	}
	if isObj && vt.Kind == ast.TypeObject {
		return g.genComputedPropertyAssign(s)
	}
	indexStr, err := g.genExprValue(s.Index)
	if err != nil {
		return err
	}
	val, err := g.genExprValue(s.Value)
	if err != nil {
		return err
	}
	delete(g.staticListMap, listVar)
	g.line(fmt.Sprintf("%s=$(printf '%%s\\n' \"$%s\" | awk -v n=$(( %s + 1 )) -v v=%s '{if (NR==n) print v; else print}')", listVar, listVar, stripQuotes(indexStr), val))
	return nil
}

func (g *Generator) genComputedPropertyAssign(s *ast.IndexAssignStmt) error {
	ref, ok := g.resolveObjectRef(&ast.IdentExpr{Pos: s.Pos, Name: s.Name})
	if !ok {
		ref = objectRef{StaticName: g.resolveVarName(s.Name)}
	}
	keyStr, err := g.genExprValue(s.Index)
	if err != nil {
		return err
	}
	val, err := g.genExprRHS(s.Value, nil)
	if err != nil {
		return err
	}
	if field, ok := g.staticObjectKeyExpr(s.Index); ok {
		ref = g.recordObjectAssignmentType(ref, field, s.Value, false)
	} else {
		ref = g.recordObjectAssignmentType(ref, "", s.Value, true)
	}
	g.updateObjectAliasRef(s.Name, ref)
	if ref.StaticName != "" {
		return g.genComputedStaticPropertyAssign(ref.StaticName, s, keyStr, val)
	}
	return g.genComputedDynamicPropertyAssign(ref, s, keyStr, val)
}

func (g *Generator) genComputedStaticPropertyAssign(objVar string, s *ast.IndexAssignStmt, keyStr, val string) error {
	keyVar := fmt.Sprintf("_objk_%d_%d", s.Pos.Line, s.Pos.Column)
	valVar := fmt.Sprintf("_objv_%d_%d", s.Pos.Line, s.Pos.Column)
	g.line(keyVar + "=" + ensureArgSafe(keyStr))
	g.line(computedKeyValidation(keyVar))
	g.line(valVar + "=" + val)
	g.line("eval \"_obj_" + objVar + "_${" + keyVar + "}=\\\"\\$" + valVar + "\\\"\"")
	g.line("case \" $_objkeys_" + objVar + " \" in *\" ${" + keyVar + "} \"*) ;; *) _objkeys_" + objVar + "=\"${_objkeys_" + objVar + "} ${" + keyVar + "}\" ;; esac")
	return nil
}

func (g *Generator) genComputedDynamicPropertyAssign(ref objectRef, s *ast.IndexAssignStmt, keyStr, val string) error {
	slotVar := fmt.Sprintf("_objs_%d_%d", s.Pos.Line, s.Pos.Column)
	keyVar := fmt.Sprintf("_objk_%d_%d", s.Pos.Line, s.Pos.Column)
	valVar := fmt.Sprintf("_objv_%d_%d", s.Pos.Line, s.Pos.Column)
	g.line(slotVar + "=" + ref.SlotExpr)
	g.line(computedKeyValidation(slotVar))
	g.line(keyVar + "=" + ensureArgSafe(keyStr))
	g.line(computedKeyValidation(keyVar))
	g.line(valVar + "=" + val)
	g.line("eval \"_obj_${" + slotVar + "}_${" + keyVar + "}=\\\"\\$" + valVar + "\\\"\"")
	g.line("eval \"_bst_obj_keys=\\\"\\${_objkeys_${" + slotVar + "}}\\\"\"")
	g.line("case \" $_bst_obj_keys \" in *\" ${" + keyVar + "} \"*) ;; *) _bst_obj_keys=\"${_bst_obj_keys} ${" + keyVar + "}\"; eval \"_objkeys_${" + slotVar + "}=\\\"\\$_bst_obj_keys\\\"\" ;; esac")
	return nil
}

func computedKeyValidation(varName string) string {
	return "if [ -z \"$" + varName + "\" ] || printf '%s\\n' \"$" + varName + "\" | grep -q '[^A-Za-z0-9_]'; then printf '[besht] invalid object key: %s\\n' \"$" + varName + "\" >&2; exit 1; fi"
}

func (g *Generator) genTernaryRHS(e *ast.TernaryExpr, targetType *ast.Type) (string, error) {
	cond, err := g.genCondition(e.Condition)
	if err != nil {
		return "", err
	}
	thenVal, err := g.genExprRHS(e.Then, targetType)
	if err != nil {
		return "", err
	}
	elseVal, err := g.genExprRHS(e.Else, targetType)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("$(if %s; then printf '%%s' %s; else printf '%%s' %s; fi)", cond, thenVal, elseVal), nil
}

func (g *Generator) genPropagateRHS(e *ast.PropagateExpr) (string, error) {
	inner, err := g.genExprValue(e.Expr)
	if err != nil {
		return "", err
	}
	suffix := "|| return $?"
	if !g.inFunction {
		suffix = "|| exit $?"
	}
	return fmt.Sprintf("%s %s", inner, suffix), nil
}

func (g *Generator) genCondition(expr ast.Expression) (string, error) {
	switch e := expr.(type) {
	case *ast.BinaryExpr:
		if e.Op == "??" {
			val, err := g.genBinaryRHS(e, nil)
			if err != nil {
				return "", err
			}
			return truthyCondition(val), nil
		}
		return g.genBinaryCondition(e)
	case *ast.UnaryExpr:
		if e.Op == "!" {
			inner, err := g.genCondition(e.Expr)
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("! %s", inner), nil
		}
	case *ast.BuiltinCallExpr:
		return g.genBuiltinCondition(e)
	case *ast.IdentExpr:
		varName := g.resolveVarName(e.Name)
		if t := g.inferReceiverType(e); t != nil && t.Kind == ast.TypeBoolean {
			return fmt.Sprintf("[ \"$%s\" = 1 ]", varName), nil
		}
		return fmt.Sprintf("[ -n \"$%s\" ] && [ \"$%s\" != 0 ]", varName, varName), nil
	case *ast.BoolLit:
		if e.Value {
			return "true", nil
		}
		return "false", nil
	case *ast.CmdExpr:
		pipeline, redirect, err := g.genCmdPipeline(e)
		if err != nil {
			return "", err
		}
		return formatCmdForBare(pipeline, redirect), nil
	case *ast.FnCallExpr:
		call, err := g.genFnCallCapture(e)
		if err != nil {
			return "", err
		}
		return truthyCondition(call), nil
	case *ast.MethodCallExpr:
		if builtin, ok := beshtBuiltinCall(e); ok {
			return g.genBuiltinCondition(builtin)
		}
		if e.Optional {
			val, err := g.genExprValue(e)
			if err != nil {
				return "", err
			}
			return nullishAwareTruthyCondition(val), nil
		}
		return g.genMethodCondition(e)
	case *ast.PropertyExpr:
		if _, ok := isProcessEnvProperty(e); ok {
			val, err := g.genProperty(e)
			if err != nil {
				return "", err
			}
			return nullishAwareTruthyCondition(val), nil
		}
		if e.Optional {
			val, err := g.genExprValue(e)
			if err != nil {
				return "", err
			}
			return nullishAwareTruthyCondition(val), nil
		}
		val, err := g.genProperty(e)
		if err != nil {
			return "", err
		}
		return truthyCondition(val), nil
	case *ast.IndexExpr:
		if e.Optional {
			val, err := g.genExprValue(e)
			if err != nil {
				return "", err
			}
			return nullishAwareTruthyCondition(val), nil
		}
	}
	val, err := g.genExprValue(expr)
	if err != nil {
		return "", err
	}
	return truthyCondition(val), nil
}

func beshtBuiltinCall(e *ast.MethodCallExpr) (*ast.BuiltinCallExpr, bool) {
	builtinName, ok := ast.BeshtMethodBuiltinName(e.Receiver, e.Method)
	if !ok {
		return nil, false
	}
	return &ast.BuiltinCallExpr{Pos: e.Pos, Name: builtinName, Args: e.Args}, true
}

func (g *Generator) genMethodCondition(e *ast.MethodCallExpr) (string, error) {
	if ast.IsBeshtArgsReceiver(e.Receiver) {
		val, err := g.genMethodCall(e)
		if err != nil {
			return "", err
		}
		return truthyCondition(val), nil
	}
	recv, err := g.genExprValue(e.Receiver)
	if err != nil {
		return "", err
	}
	recvType := g.inferReceiverType(e.Receiver)
	isListType := recvType != nil && recvType.Kind == ast.TypeList

	if !isListType {
		if recvType != nil && recvType.Kind == ast.TypeSet && e.Method == "has" {
			a0, err := g.genExprValue(e.Args[0])
			if err != nil {
				return "", err
			}
			a0 = ensureArgSafe(a0)
			recv = ensureArgSafe(recv)
			return fmt.Sprintf("[ -n %s ] && printf '%%s\\n' %s | grep -qxF -- %s", recv, recv, a0), nil
		}
		switch e.Method {
		case "startsWith":
			if len(e.Args) == 2 {
				val, err := g.genMethodCall(e)
				if err != nil {
					return "", err
				}
				return fmt.Sprintf("[ %s = 1 ]", val), nil
			}
			a0, err := g.genExprValue(e.Args[0])
			if err != nil {
				return "", err
			}
			g.requireRuntimeHelper("startsWith")
			return fmt.Sprintf("_bst_starts_with %s %s", recv, a0), nil
		case "endsWith":
			if len(e.Args) == 2 {
				val, err := g.genMethodCall(e)
				if err != nil {
					return "", err
				}
				return fmt.Sprintf("[ %s = 1 ]", val), nil
			}
			a0, err := g.genExprValue(e.Args[0])
			if err != nil {
				return "", err
			}
			g.requireRuntimeHelper("endsWith")
			return fmt.Sprintf("_bst_ends_with %s %s", recv, a0), nil
		case "includes":
			if len(e.Args) == 2 {
				val, err := g.genMethodCall(e)
				if err != nil {
					return "", err
				}
				return fmt.Sprintf("[ %s = 1 ]", val), nil
			}
			a0, err := g.genExprValue(e.Args[0])
			if err != nil {
				return "", err
			}
			g.requireRuntimeHelper("includes")
			return fmt.Sprintf("_bst_includes %s %s", recv, a0), nil
		}
	}
	if isListType {
		switch e.Method {
		case "includes":
			a0, err := g.genExprValue(e.Args[0])
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("printf '%%s\\n' %s | grep -qxF %s", recv, a0), nil
		}
	}
	val, err := g.genMethodCall(e)
	if err != nil {
		return "", err
	}
	return truthyCondition(val), nil
}

func truthyCondition(value string) string {
	return "(_bst_cond=" + value + "; [ -n \"$_bst_cond\" ] && [ \"$_bst_cond\" != 0 ])"
}

func (g *Generator) genBinaryCondition(e *ast.BinaryExpr) (string, error) {
	switch e.Op {
	case "&&":
		left, err := g.genCondition(e.Left)
		if err != nil {
			return "", err
		}
		right, err := g.genCondition(e.Right)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s && %s", left, right), nil

	case "||":
		left, err := g.genCondition(e.Left)
		if err != nil {
			return "", err
		}
		right, err := g.genCondition(e.Right)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s || %s", left, right), nil
	}
	leftStr, err := g.genExprValue(e.Left)
	if err != nil {
		return "", err
	}
	rightStr, err := g.genExprValue(e.Right)
	if err != nil {
		return "", err
	}

	leftType := e.Left.GetType()
	if leftType == nil {
		leftType = g.inferReceiverType(e.Left)
	}
	rightType := e.Right.GetType()
	if rightType == nil {
		rightType = g.inferReceiverType(e.Right)
	}

	switch e.Op {
	case "==", "===":
		return binaryStringTest(leftStr, "=", rightStr), nil
	case "!=", "!==":
		return binaryStringTest(leftStr, "!=", rightStr), nil
	case "+", "-", "*", "/", "%":
		val, err := g.genBinaryRHS(e, nil)
		if err != nil {
			return "", err
		}
		return truthyCondition(val), nil
	case ">":
		return fmt.Sprintf("awk -v _a=%s -v _b=%s 'BEGIN{OFMT=\"%%.17g\";exit !(_a > _b)}'", stripQuotes(leftStr), stripQuotes(rightStr)), nil
	case "<":
		return fmt.Sprintf("awk -v _a=%s -v _b=%s 'BEGIN{OFMT=\"%%.17g\";exit !(_a < _b)}'", stripQuotes(leftStr), stripQuotes(rightStr)), nil
	case ">=":
		return fmt.Sprintf("awk -v _a=%s -v _b=%s 'BEGIN{OFMT=\"%%.17g\";exit !(_a >= _b)}'", stripQuotes(leftStr), stripQuotes(rightStr)), nil
	case "<=":
		return fmt.Sprintf("awk -v _a=%s -v _b=%s 'BEGIN{OFMT=\"%%.17g\";exit !(_a <= _b)}'", stripQuotes(leftStr), stripQuotes(rightStr)), nil
	}
	return "", fmt.Errorf("unknown binary operator %q in condition", e.Op)
}

func binaryStringTest(left, op, right string) string {
	return fmt.Sprintf("{ _bst_left=%s; _bst_right=%s; [ -n \"${%s+x}\" ] && [ \"$_bst_left\" = \"$%s\" ] && _bst_left=; [ -n \"${%s+x}\" ] && [ \"$_bst_right\" = \"$%s\" ] && _bst_right=; [ \"$_bst_left\" %s \"$_bst_right\" ]; }", left, right, nullishSentinelVar, nullishSentinelVar, nullishSentinelVar, nullishSentinelVar, op)
}

func (g *Generator) genBuiltinCondition(e *ast.BuiltinCallExpr) (string, error) {
	switch e.Name {
	case "Besht.fs.isFile":
		if len(e.Args) != 1 {
			return "", fmt.Errorf("Besht.fs.isFile() takes 1 argument")
		}
		arg0, err := g.genExprValue(e.Args[0])
		if err != nil {
			return "", err
		}
		clean := stripQuotes(arg0)
		return fmt.Sprintf("[ -f %s ]", clean), nil
	case "Besht.fs.isDir":
		if len(e.Args) != 1 {
			return "", fmt.Errorf("Besht.fs.isDir() takes 1 argument")
		}
		arg0, err := g.genExprValue(e.Args[0])
		if err != nil {
			return "", err
		}
		clean := stripQuotes(arg0)
		return fmt.Sprintf("[ -d %s ]", clean), nil
	case "Besht.fs.isReadable":
		if len(e.Args) != 1 {
			return "", fmt.Errorf("Besht.fs.isReadable() takes 1 argument")
		}
		arg0, err := g.genExprValue(e.Args[0])
		if err != nil {
			return "", err
		}
		clean := stripQuotes(arg0)
		return fmt.Sprintf("[ -r %s ]", clean), nil
	case "Besht.fs.isWritable":
		if len(e.Args) != 1 {
			return "", fmt.Errorf("Besht.fs.isWritable() takes 1 argument")
		}
		arg0, err := g.genExprValue(e.Args[0])
		if err != nil {
			return "", err
		}
		clean := stripQuotes(arg0)
		return fmt.Sprintf("[ -w %s ]", clean), nil
	case "Besht.fs.isExecutable":
		if len(e.Args) != 1 {
			return "", fmt.Errorf("Besht.fs.isExecutable() takes 1 argument")
		}
		arg0, err := g.genExprValue(e.Args[0])
		if err != nil {
			return "", err
		}
		clean := stripQuotes(arg0)
		return fmt.Sprintf("[ -x %s ]", clean), nil
	case "Besht.strings.isEmpty":
		if len(e.Args) != 1 {
			return "", fmt.Errorf("Besht.strings.isEmpty() takes 1 argument")
		}
		arg0, err := g.genExprValue(e.Args[0])
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("[ -z %s ]", arg0), nil
	case "Besht.strings.isNonEmpty":
		if len(e.Args) != 1 {
			return "", fmt.Errorf("Besht.strings.isNonEmpty() takes 1 argument")
		}
		arg0, err := g.genExprValue(e.Args[0])
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("[ -n %s ]", arg0), nil
	case "Number.isFinite":
		return "true", nil
	case "Boolean", "Number.isInteger", "Number.isSafeInteger", "Number.isNaN", "Array.isArray", "Object.hasOwn":
		val, err := g.genBuiltinCapture(e)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("[ %s = 1 ]", val), nil
	}
	return "", fmt.Errorf("builtin %q cannot be used as condition", e.Name)
}

func escapeForDoubleQuote(s string) string {
	var b strings.Builder
	i := 0
	for i < len(s) {
		ch := s[i]
		switch {
		case ch == '`':
			b.WriteString("\\`")
			i++
		case ch == '$' && i+1 < len(s) && s[i+1] == '(':
			b.WriteString("\\$(")
			i += 2
		case ch == '$' && i+1 < len(s) && s[i+1] == '{':
			// ${...} — besht interpolation, leave intact
			b.WriteByte(ch)
			i++
		case ch == '\\' && i+1 < len(s) && s[i+1] == '$':
			// \$ — already an escaped literal dollar from the lexer, leave as-is
			b.WriteString("\\$")
			i += 2
		case ch == '$' && i+1 < len(s) && (isAlphaOrUnderscore(s[i+1]) || (s[i+1] >= '0' && s[i+1] <= '9')):
			// $VAR or $1 not inside ${} — escape so it doesn't expand in sh
			b.WriteString("\\$")
			i++
		default:
			b.WriteByte(ch)
			i++
		}
	}
	return b.String()
}

func isAlphaOrUnderscore(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_'
}

func genStringLiteral(s string) string {
	if s == "" {
		return "\"\""
	}
	escaped := escapeForDoubleQuote(s)
	escaped = strings.ReplaceAll(escaped, "\"", "\\\"")
	return fmt.Sprintf("\"%s\"", escaped)
}

func (g *Generator) genTemplateLiteral(e *ast.TemplateLit) (string, error) {
	if len(e.Exprs) == 0 || len(e.Parts) == 0 {
		return genStringLiteral(g.rewriteVarRefs(e.Value)), nil
	}
	var out strings.Builder
	for i, part := range e.Parts {
		part = escapeForDoubleQuote(g.rewriteVarRefs(part))
		part = strings.ReplaceAll(part, "\"", "\\\"")
		out.WriteString(part)
		if i < len(e.Exprs) {
			val, err := g.genExprValue(e.Exprs[i])
			if err != nil {
				return "", err
			}
			if g.isBooleanExpr(e.Exprs[i]) {
				out.WriteString(fmt.Sprintf("$(if [ %s = 1 ]; then printf true; else printf false; fi)", stripQuotes(val)))
			} else {
				out.WriteString(strInner(val))
			}
		}
	}
	return fmt.Sprintf("\"%s\"", out.String()), nil
}

func (g *Generator) rewriteVarRefs(s string) string {
	if len(g.paramMap) == 0 {
		return s
	}
	var result strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '$' && i+1 < len(s) && s[i+1] == '{' {
			j := i + 2
			for j < len(s) && s[j] != '}' {
				j++
			}
			if j < len(s) {
				ref := s[i+2 : j]
				name := ref
				suffix := ""
				if idx := strings.IndexAny(ref, ":-+=?#%"); idx >= 0 {
					name = ref[:idx]
					suffix = ref[idx:]
				}
				if mangled, ok := g.paramMap[name]; ok {
					result.WriteString("${")
					result.WriteString(mangled)
					result.WriteString(suffix)
					result.WriteString("}")
				} else {
					result.WriteString("${")
					result.WriteString(ref)
					result.WriteString("}")
				}
				i = j + 1
				continue
			}
		}
		result.WriteByte(s[i])
		i++
	}
	return result.String()
}

func mangle(name string) string {
	return name
}

func fnParamVar(fnName, paramName string) string {
	return fmt.Sprintf("_%s_%s", fnName, paramName)
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

func (g *Generator) genCmdArgs(args []ast.Expression) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("command requires at least a name")
	}
	hasSpread := false
	for _, arg := range args {
		if _, ok := arg.(*ast.SpreadExpr); ok {
			hasSpread = true
			break
		}
	}
	if hasSpread {
		var b strings.Builder
		b.WriteString("(set --\n")
		for i, arg := range args {
			if spread, ok := arg.(*ast.SpreadExpr); ok {
				val, err := g.genExprValue(spread.Expr)
				if err != nil {
					return "", err
				}
				tmpVar := fmt.Sprintf("_bst_spread_arg_%d", i)
				b.WriteString(fmt.Sprintf("%s=%s\n", tmpVar, val))
				b.WriteString("while IFS= read -r _arg; do set -- \"$@\" \"$_arg\"; done <<EOF\n")
				b.WriteString("$" + tmpVar + "\n")
				b.WriteString("EOF\n")
				continue
			}
			val, err := g.genExprValue(arg)
			if err != nil {
				return "", err
			}
			if i > 0 {
				prevLiteral := exprLiteralValue(args[i-1])
				if prevLiteral != "-e" && prevLiteral != "--" {
					warnIfFlagLikePattern(arg, val)
				}
			}
			b.WriteString(fmt.Sprintf("set -- \"$@\" %s\n", cmdArgWordForExpr(arg, val, i == 0)))
		}
		b.WriteString("\"$@\")")
		return b.String(), nil
	}
	var parts []string
	for i, arg := range args {
		val, err := g.genExprValue(arg)
		if err != nil {
			return "", err
		}
		if i > 0 {
			prevLiteral := exprLiteralValue(args[i-1])
			if prevLiteral != "-e" && prevLiteral != "--" {
				warnIfFlagLikePattern(arg, val)
			}
		}
		parts = append(parts, cmdArgWordForExpr(arg, val, i == 0))
	}
	return strings.Join(parts, " "), nil
}

func exprLiteralValue(expr ast.Expression) string {
	switch e := expr.(type) {
	case *ast.StringLit:
		return e.Value
	case *ast.RawStringLit:
		return e.Value
	}
	return ""
}

func warnIfFlagLikePattern(expr ast.Expression, val string) {
	var literal string
	switch e := expr.(type) {
	case *ast.StringLit:
		literal = e.Value
	default:
		return
	}
	if !strings.HasPrefix(literal, "-") {
		return
	}
	specialChars := `$^[]{}.+*?()\`
	for _, ch := range specialChars {
		if strings.ContainsRune(literal, ch) {
			fmt.Fprintf(os.Stderr, "besht: warning: argument %q starts with '-' and contains special characters — use r%q for a raw pattern, or add '--' / '-e' flag before it\n", literal, literal)
			return
		}
	}
}

func hasShellExpansion(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] != '$' {
			continue
		}
		if i > 0 && s[i-1] == '\\' {
			continue
		}
		if i+1 >= len(s) {
			break
		}
		next := s[i+1]
		if next == '{' || next == '(' || next == '@' || next == '*' || next == '#' ||
			next == '?' || next == '!' || next == '-' || next == '_' ||
			(next >= '0' && next <= '9') ||
			(next >= 'a' && next <= 'z') ||
			(next >= 'A' && next <= 'Z') {
			return true
		}
	}
	return false
}

func cmdArgQuote(val string) string {
	return cmdArgWord(val, false, false)
}

func cmdArgWordForExpr(expr ast.Expression, val string, commandPosition bool) string {
	_, ordinaryString := expr.(*ast.StringLit)
	return cmdArgWord(val, commandPosition, ordinaryString)
}

func cmdArgWord(val string, commandPosition bool, unquoteSingleQuoted bool) string {
	if strings.HasPrefix(val, "$(") {
		return val
	}
	isSingleQuoted := strings.HasPrefix(val, "'") && strings.HasSuffix(val, "'") && len(val) >= 2
	if isSingleQuoted {
		inner := val[1 : len(val)-1]
		if unquoteSingleQuoted && !strings.Contains(inner, "'") && shellSafeBareWord(inner, commandPosition) {
			return inner
		}
		return val
	}
	isDoubleQuoted := strings.HasPrefix(val, "\"") && strings.HasSuffix(val, "\"") && len(val) >= 2
	if isDoubleQuoted {
		inner := val[1 : len(val)-1]
		if hasShellExpansion(inner) {
			return val
		}
		if shellSafeBareWord(inner, commandPosition) {
			return inner
		}
		return shellQuote(inner)
	}
	if strings.HasPrefix(val, "$") {
		return "\"" + val + "\""
	}
	if shellSafeBareWord(val, commandPosition) {
		return val
	}
	return shellQuote(val)
}

func shellSafeBareWord(s string, commandPosition bool) bool {
	if s == "" {
		return false
	}
	if commandPosition && (strings.Contains(s, "=") || isShellReservedWord(s)) {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
			continue
		}
		switch c {
		case '@', '%', '_', '+', '=', ':', ',', '.', '/', '-':
			continue
		default:
			return false
		}
	}
	return true
}

func isShellReservedWord(s string) bool {
	switch s {
	case "!", "{", "}", "case", "do", "done", "elif", "else", "esac", "fi", "for", "if", "in", "then", "until", "while":
		return true
	default:
		return false
	}
}

const workdirPipelinePrefix = "__BESHT_WORKDIR__"

func workdirPipeline(path, pipeline string) string {
	return workdirPipelinePrefix + strconv.Itoa(len(path)) + ":" + path + pipeline
}

func splitWorkdirPipeline(pipeline string) (string, string, bool) {
	if !strings.HasPrefix(pipeline, workdirPipelinePrefix) {
		return "", "", false
	}
	rest := strings.TrimPrefix(pipeline, workdirPipelinePrefix)
	idx := strings.IndexByte(rest, ':')
	if idx < 0 {
		return "", "", false
	}
	pathLen, err := strconv.Atoi(rest[:idx])
	if err != nil || pathLen < 0 || len(rest[idx+1:]) < pathLen {
		return "", "", false
	}
	pathStart := idx + 1
	pathEnd := pathStart + pathLen
	return rest[pathStart:pathEnd], rest[pathEnd:], true
}

func mapWorkdirPipeline(pipeline string, fn func(string) string) string {
	path, inner, ok := splitWorkdirPipeline(pipeline)
	if !ok {
		return fn(pipeline)
	}
	return workdirPipeline(path, fn(inner))
}

func formatCmdForBare(pipeline, redirect string) string {
	if path, inner, ok := splitWorkdirPipeline(pipeline); ok {
		return "(" + formatWorkdirCommand(path, inner+redirect) + ")"
	}
	if redirect == "" {
		return pipeline
	}
	if !strings.Contains(pipeline, " | ") {
		return pipeline + redirect
	}
	return "{ " + pipeline + "; }" + redirect
}

func formatCmdForCapture(pipeline, redirect string) string {
	if path, inner, ok := splitWorkdirPipeline(pipeline); ok {
		return formatWorkdirCommand(path, inner+redirect)
	}
	return formatCmdForBare(pipeline, redirect)
}

func formatCmdForStderrCapture(pipeline, redirect string) string {
	if path, inner, ok := splitWorkdirPipeline(pipeline); ok {
		return formatWorkdirCommand(path, inner+" 2>&1 1>/dev/null"+redirect)
	}
	return formatCmdForBare(pipeline, "") + " 2>&1 1>/dev/null" + redirect
}

func formatWorkdirCommand(path, command string) string {
	return "_bst_workdir=" + path + "; if [ \"${_bst_workdir#-}\" != \"$_bst_workdir\" ]; then _bst_workdir=./$_bst_workdir; fi; CDPATH= cd \"$_bst_workdir\" && " + command
}

func commandSubstitution(cmd string) string {
	if strings.HasPrefix(cmd, "(") {
		return "$( " + cmd + " )"
	}
	return "$(" + cmd + ")"
}

func (g *Generator) genCmdChainExpr(expr ast.Expression) (pipeline string, redirect string, err error) {
	switch e := expr.(type) {
	case *ast.CmdExpr:
		return g.genCmdPipeline(e)
	case *ast.MethodCallExpr:
		return g.genCmdChain(e)
	case *ast.IdentExpr:
		if chain, ok := g.cmdChains[e.Name]; ok && chain != expr {
			return g.genCmdChainExpr(chain)
		}
		if g.cmdAnalysis != nil && g.cmdScope != nil {
			if id, ok := g.cmdScope.lookup(e.Name); ok {
				if ident := g.cmdAnalysis.identity(id); ident != nil && ident.FullChain != nil && ident.FullChain != expr {
					return g.genCmdChainExpr(ident.FullChain)
				}
			}
		}
	}
	return "", "", fmt.Errorf("genCmdChainExpr: unexpected expr type %T", expr)
}

func (g *Generator) genCmdPipeline(e *ast.CmdExpr) (pipeline string, redirect string, err error) {
	if spread, ok := e.Args[0].(*ast.SpreadExpr); ok {
		if len(e.Args) != 1 {
			return "", "", fmt.Errorf("command-name spread must be the only $() argument")
		}
		cmd, err := g.genSpreadCommand(spread.Expr)
		return cmd, "", err
	}
	cmd, err := g.genCmdArgs(e.Args)
	if err != nil {
		return "", "", err
	}
	return cmd, "", nil
}

func (g *Generator) genSpreadCommand(expr ast.Expression) (string, error) {
	val, err := g.genExprValue(expr)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("(_bst_spread_args=%s; set --; while IFS= read -r _arg; do set -- \"$@\" \"$_arg\"; done <<EOF\n$_bst_spread_args\nEOF\n\"$@\")", val), nil
}

func (g *Generator) genCmdChain(e *ast.MethodCallExpr) (pipeline string, redirect string, err error) {
	recv := e.Receiver

	var basePipeline string
	var baseRedirect string

	switch r := recv.(type) {
	case *ast.CmdExpr:
		basePipeline, baseRedirect, err = g.genCmdPipeline(r)
		if err != nil {
			return "", "", err
		}
	case *ast.MethodCallExpr:
		basePipeline, baseRedirect, err = g.genCmdChain(r)
		if err != nil {
			return "", "", err
		}
	case *ast.IdentExpr:
		if chain, ok := g.cmdChains[r.Name]; ok && chain != recv {
			basePipeline, baseRedirect, err = g.genCmdChainExpr(chain)
			if err != nil {
				return "", "", err
			}
			break
		}
		if g.cmdAnalysis != nil && g.cmdScope != nil {
			if id, ok := g.cmdScope.lookup(r.Name); ok {
				if ident := g.cmdAnalysis.identity(id); ident != nil && ident.FullChain != nil {
					basePipeline, baseRedirect, err = g.genCmdChainExpr(ident.FullChain)
					if err != nil {
						return "", "", err
					}
					break
				}
			}
		}
		recvStr, err := g.genExprValue(recv)
		if err != nil {
			return "", "", err
		}
		basePipeline = fmt.Sprintf("printf '%%s' %s", recvStr)
	default:
		recvStr, err := g.genExprValue(recv)
		if err != nil {
			return "", "", err
		}
		basePipeline = fmt.Sprintf("printf '%%s' %s", recvStr)
	}

	switch e.Method {
	case "pipe":
		pipeArg := e.Args[0]
		var stage string
		switch p := pipeArg.(type) {
		case *ast.CmdExpr:
			stage, _, err = g.genCmdPipeline(p)
			if err != nil {
				return "", "", err
			}
		case *ast.IdentExpr:
			stage = g.resolveVarName(p.Name)
		default:
			s, err := g.genExprValue(pipeArg)
			if err != nil {
				return "", "", err
			}
			stage = stripQuotes(s)
		}
		return mapWorkdirPipeline(basePipeline, func(inner string) string { return inner + " | " + stage }), baseRedirect, nil

	case "stdout":
		pathStr, err := g.genExprValue(e.Args[0])
		if err != nil {
			return "", "", err
		}
		pathClean := stripQuotes(pathStr)
		pathWord := redirectTarget(pathStr)
		if pathClean == "null" {
			pathWord = "/dev/null"
		}
		op := ">"
		if len(e.Args) == 2 {
			mode, _ := g.genExprValue(e.Args[1])
			if stripQuotes(mode) == "append" {
				op = ">>"
			}
		}
		return basePipeline, baseRedirect + " " + op + " " + pathWord, nil

	case "stderr":
		argStr, err := g.genExprValue(e.Args[0])
		if err != nil {
			return "", "", err
		}
		argClean := stripQuotes(argStr)
		switch argClean {
		case "&1":
			return basePipeline, baseRedirect + " 2>&1", nil
		case "capture":
			return basePipeline + " 2>&1 1>/dev/null", baseRedirect, nil
		case "null":
			return basePipeline, baseRedirect + " 2>/dev/null", nil
		default:
			return basePipeline, baseRedirect + " 2>" + redirectTarget(argStr), nil
		}

	case "workdir":
		pathStr, err := g.genExprValue(e.Args[0])
		if err != nil {
			return "", "", err
		}
		pathWord := ensureArgSafe(pathStr)
		if _, inner, ok := splitWorkdirPipeline(basePipeline); ok {
			return workdirPipeline(pathWord, inner), baseRedirect, nil
		}
		return workdirPipeline(pathWord, basePipeline), baseRedirect, nil

	case "env":
		name, err := commandEnvName(e.Args[0])
		if err != nil {
			return "", "", err
		}
		valueStr, err := g.genExprValue(e.Args[1])
		if err != nil {
			return "", "", err
		}
		if path, inner, ok := splitWorkdirPipeline(basePipeline); ok {
			return workdirPipeline(path, name+"="+valueStr+" "+inner), baseRedirect, nil
		}
		return mapWorkdirPipeline(basePipeline, func(inner string) string { return name + "=" + valueStr + " " + inner }), baseRedirect, nil

	case "readStdoutLines", "readStdout", "readStderr", "run", "exitCode", "clone":
		return basePipeline, baseRedirect, nil
	}

	return "", "", fmt.Errorf("unknown command method %q", e.Method)
}

func redirectTarget(val string) string {
	if strings.HasPrefix(val, "$(") {
		return ensureArgSafe(val)
	}
	raw := stripQuotes(val)
	if isSafeRedirectPath(raw) {
		return raw
	}
	return cmdArgQuote(val)
}

func isSafeRedirectPath(path string) bool {
	if path == "" {
		return false
	}
	for _, r := range path {
		if r == '/' || r == '.' || r == '-' || r == '_' ||
			(r >= '0' && r <= '9') ||
			(r >= 'A' && r <= 'Z') ||
			(r >= 'a' && r <= 'z') {
			continue
		}
		return false
	}
	return true
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
	if !isShellIdentifier(name) {
		return "", fmt.Errorf("invalid command env name %q", name)
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

func singleQuotedToRaw(s string) string {
	var result strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '\'' {
			i++
			for i < len(s) && s[i] != '\'' {
				result.WriteByte(s[i])
				i++
			}
			if i < len(s) {
				i++
			}
		} else if s[i] == '"' {
			i++
			for i < len(s) && s[i] != '"' {
				if s[i] == '\\' && i+1 < len(s) {
					result.WriteByte(s[i+1])
					i += 2
				} else {
					result.WriteByte(s[i])
					i++
				}
			}
			if i < len(s) {
				i++
			}
		} else {
			result.WriteByte(s[i])
			i++
		}
	}
	return result.String()
}

func ensureArgSafe(s string) string {
	if strings.HasPrefix(s, "$(") {
		return "\"" + s + "\""
	}
	return s
}

func strInner(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "\"") && strings.HasSuffix(s, "\"") && len(s) >= 2 {
		inner := s[1 : len(s)-1]
		if strings.HasPrefix(inner, "$") && !strings.HasPrefix(inner, "${") && !strings.HasPrefix(inner, "$(") {
			return "${" + inner[1:] + "}"
		}
		return inner
	}
	if strings.HasPrefix(s, "'") {
		raw := singleQuotedToRaw(s)
		return escapeForDoubleQuote(raw)
	}
	if strings.HasPrefix(s, "$(") {
		return s
	}
	if strings.HasPrefix(s, "$") && !strings.HasPrefix(s, "${") {
		return "${" + s[1:] + "}"
	}
	return s
}

func stripQuotes(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "\"") && strings.HasSuffix(s, "\"") && len(s) >= 2 {
		inner := s[1 : len(s)-1]
		if !strings.Contains(inner, "\"") {
			return inner
		}
	}
	if strings.HasPrefix(s, "'") && strings.HasSuffix(s, "'") && len(s) >= 2 {
		inner := s[1 : len(s)-1]
		if !strings.Contains(inner, "'") {
			return inner
		}
	}
	return s
}

func objectPropVar(varName, propName string) string {
	return fmt.Sprintf("_obj_%s_%s", varName, propName)
}

func arithmeticOperand(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "$((") && strings.HasSuffix(s, "))") {
		return "(" + strings.TrimSpace(s[3:len(s)-2]) + ")"
	}
	return s
}

func needsAwkCapture(s string) bool {
	return strings.Contains(s, "$(") || strings.Contains(s, "$( ")
}

func awkArg(name, expr string) string {
	expr = strings.TrimSpace(expr)
	if strings.HasPrefix(expr, "'") || strings.HasPrefix(expr, "\"") {
		return "-v " + name + "=" + expr
	}
	if needsAwkCapture(expr) {
		return "-v " + name + "=\"" + expr + "\""
	}
	if strings.HasPrefix(expr, "$") {
		return "-v " + name + "=\"" + expr + "\""
	}
	return "-v " + name + "=" + expr
}

func classPropVar(className, propName string) string {
	return fmt.Sprintf("_class_%s_%s", className, propName)
}

func classMethodName(className, methodName string) string {
	return fmt.Sprintf("%s__%s", className, methodName)
}

func methodMutatesThis(stmts []ast.Statement) bool {
	for _, stmt := range stmts {
		switch s := stmt.(type) {
		case *ast.PropertyAssignStmt:
			if s.Object == "this" {
				return true
			}
		case *ast.Block:
			if methodMutatesThis(s.Statements) {
				return true
			}
		case *ast.IfStmt:
			if methodMutatesThis(s.Then.Statements) || (s.Else != nil && methodMutatesThis(s.Else.Statements)) {
				return true
			}
			for _, ei := range s.ElseIfs {
				if methodMutatesThis(ei.Body.Statements) {
					return true
				}
			}
		case *ast.ForStmt:
			if methodMutatesThis(s.Body.Statements) {
				return true
			}
		case *ast.CStyleForStmt:
			if methodStmtMutatesThis(s.Init) || methodStmtMutatesThis(s.Update) || methodMutatesThis(s.Body.Statements) {
				return true
			}
		case *ast.WhileStmt:
			if methodMutatesThis(s.Body.Statements) {
				return true
			}
		case *ast.TryStmt:
			if methodMutatesThis(s.Body.Statements) || methodMutatesThis(s.Catch.Statements) {
				return true
			}
		case *ast.SwitchStmt:
			for _, swCase := range s.Cases {
				if methodMutatesThis(swCase.Body.Statements) {
					return true
				}
			}
		}
	}
	return false
}

func methodStmtMutatesThis(stmt ast.Statement) bool {
	if stmt == nil {
		return false
	}
	return methodMutatesThis([]ast.Statement{stmt})
}

func stmtPos(stmt ast.Statement) ast.Pos {
	switch s := stmt.(type) {
	case *ast.ImportDecl:
		return s.Pos
	case *ast.LetDecl:
		return s.Pos
	case *ast.DestructureDecl:
		return s.Pos
	case *ast.Assignment:
		return s.Pos
	case *ast.IndexAssignStmt:
		return s.Pos
	case *ast.PropertyAssignStmt:
		return s.Pos
	case *ast.FnDecl:
		return s.Pos
	case *ast.ClassDecl:
		return s.Pos
	case *ast.Block:
		return s.Pos
	case *ast.IfStmt:
		return s.Pos
	case *ast.ForStmt:
		return s.Pos
	case *ast.WhileStmt:
		return s.Pos
	case *ast.TryStmt:
		return s.Pos
	case *ast.SwitchStmt:
		return s.Pos
	case *ast.ReturnStmt:
		return s.Pos
	case *ast.ExitStmt:
		return s.Pos
	case *ast.ExprStmt:
		return s.Pos
	case *ast.CStyleForStmt:
		return s.Pos
	case *ast.DeclareStmt:
		return s.Pos
	case *ast.DeclareFnStmt:
		return s.Pos
	case *ast.BreakStmt:
		return s.Pos
	case *ast.ContinueStmt:
		return s.Pos
	}
	return ast.Pos{}
}

func sanitizeShellComment(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	return s
}
