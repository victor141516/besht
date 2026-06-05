package parser_test

import (
	"strings"
	"testing"

	"github.com/victor141516/besht/internal/ast"
	"github.com/victor141516/besht/internal/parser"
)

func mustParse(t *testing.T, src string) *ast.Program {
	t.Helper()
	prog, err := parser.Parse(src, "test.bsh")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	return prog
}

func expectParseError(t *testing.T, src string) {
	t.Helper()
	_, err := parser.Parse(src, "test.bsh")
	if err == nil {
		t.Fatal("expected parse error, got nil")
	}
}

func expectParseErrorContains(t *testing.T, src, want string) {
	t.Helper()
	_, err := parser.Parse(src, "test.bsh")
	if err == nil {
		t.Fatal("expected parse error, got nil")
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("parse error %q does not contain %q", err.Error(), want)
	}
}

func TestParser_LetDecl(t *testing.T) {
	prog := mustParse(t, `let x: string = "hello"`)
	if len(prog.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(prog.Statements))
	}
	decl, ok := prog.Statements[0].(*ast.LetDecl)
	if !ok {
		t.Fatalf("expected *ast.LetDecl, got %T", prog.Statements[0])
	}
	if decl.Name != "x" {
		t.Errorf("name: got %q, want %q", decl.Name, "x")
	}
	if decl.TypeAnnot.Kind != ast.TypeString {
		t.Errorf("type: got %v, want string", decl.TypeAnnot.Kind)
	}
	lit, ok := decl.Value.(*ast.StringLit)
	if !ok {
		t.Fatalf("value: expected *ast.StringLit, got %T", decl.Value)
	}
	if lit.Value != "hello" {
		t.Errorf("value: got %q, want %q", lit.Value, "hello")
	}
}

func TestParser_LetDeclInt(t *testing.T) {
	prog := mustParse(t, "let count: number = 42")
	decl := prog.Statements[0].(*ast.LetDecl)
	if decl.TypeAnnot.Kind != ast.TypeNumber {
		t.Errorf("type: got %v, want int", decl.TypeAnnot.Kind)
	}
	lit := decl.Value.(*ast.IntLit)
	if lit.Value != 42 {
		t.Errorf("value: got %d, want 42", lit.Value)
	}
}

func TestParser_LetDeclBool(t *testing.T) {
	prog := mustParse(t, "let flag: boolean = true")
	decl := prog.Statements[0].(*ast.LetDecl)
	if decl.TypeAnnot.Kind != ast.TypeBoolean {
		t.Errorf("type: got %v, want bool", decl.TypeAnnot.Kind)
	}
	lit := decl.Value.(*ast.BoolLit)
	if !lit.Value {
		t.Error("bool value: got false, want true")
	}
}

func TestParser_LetDeclList(t *testing.T) {
	prog := mustParse(t, `let files: Array<string> = ["a.txt", "b.txt"]`)
	decl := prog.Statements[0].(*ast.LetDecl)
	if decl.TypeAnnot.Kind != ast.TypeList {
		t.Fatalf("type: got %v, want list", decl.TypeAnnot.Kind)
	}
	if decl.TypeAnnot.Elem.Kind != ast.TypeString {
		t.Errorf("elem type: got %v, want string", decl.TypeAnnot.Elem.Kind)
	}
	lit := decl.Value.(*ast.ListLit)
	if len(lit.Elements) != 2 {
		t.Errorf("list elements: got %d, want 2", len(lit.Elements))
	}
}

func TestParser_ObjectTypeAnnotation(t *testing.T) {
	prog := mustParse(t, `function keys(obj: object): string[] {
    return Object.keys(obj)
}`)
	fn := prog.Statements[0].(*ast.FnDecl)
	if fn.Params[0].Type.Kind != ast.TypeObject {
		t.Fatalf("param type: got %v, want object", fn.Params[0].Type)
	}
	if fn.ReturnType.Kind != ast.TypeList || fn.ReturnType.Elem.Kind != ast.TypeString {
		t.Fatalf("return type: got %v, want string[]", fn.ReturnType)
	}
}

func TestParser_JSONValueTypeAnnotation(t *testing.T) {
	prog := mustParse(t, `let data: JSONValue = JSON.parse("{}")
let name = data.name as string`)
	decl := prog.Statements[0].(*ast.LetDecl)
	if decl.TypeAnnot == nil || decl.TypeAnnot.Kind != ast.TypeJSON {
		t.Fatalf("type: got %v, want JSONValue", decl.TypeAnnot)
	}
	asExpr := prog.Statements[1].(*ast.LetDecl).Value.(*ast.AsExpr)
	if asExpr.Type == nil || asExpr.Type.Kind != ast.TypeString {
		t.Fatalf("assertion type: got %v, want string", asExpr.Type)
	}
}

func TestParser_AsTypeAssertion(t *testing.T) {
	prog := mustParse(t, `let xs = [] as string[]`)
	decl := prog.Statements[0].(*ast.LetDecl)
	asExpr, ok := decl.Value.(*ast.AsExpr)
	if !ok {
		t.Fatalf("value: expected *ast.AsExpr, got %T", decl.Value)
	}
	if asExpr.Type.Kind != ast.TypeList || asExpr.Type.Elem.Kind != ast.TypeString {
		t.Fatalf("type: got %v, want Array<string>", asExpr.Type)
	}
}

func TestParser_SetTypeArgsAndBlocklessIf(t *testing.T) {
	prog := mustParse(t, `function f(row: string[]): string {
    const seen = new Set<string>()
    if (seen.has(row[0])) return "yes"
    else seen.add(row[0])
    return "no"
}`)
	fn := prog.Statements[0].(*ast.FnDecl)
	if fn.Params[0].Type.Kind != ast.TypeList || fn.Params[0].Type.Elem.Kind != ast.TypeString {
		t.Fatalf("param type: got %v, want Array<string>", fn.Params[0].Type)
	}
	letDecl := fn.Body.Statements[0].(*ast.LetDecl)
	newExpr, ok := letDecl.Value.(*ast.NewExpr)
	if !ok {
		t.Fatalf("value: expected *ast.NewExpr, got %T", letDecl.Value)
	}
	if newExpr.ClassName != "Set" || len(newExpr.TypeArgs) != 1 || newExpr.TypeArgs[0].Kind != ast.TypeString {
		t.Fatalf("new Set type args: got %#v", newExpr.TypeArgs)
	}
	ifStmt := fn.Body.Statements[1].(*ast.IfStmt)
	if len(ifStmt.Then.Statements) != 1 || ifStmt.Else == nil || len(ifStmt.Else.Statements) != 1 {
		t.Fatalf("blockless if/else statements were not wrapped correctly")
	}
}

func TestParser_NestedArrayType(t *testing.T) {
	prog := mustParse(t, `let matrix: string[][] = [["a"]]`)
	decl := prog.Statements[0].(*ast.LetDecl)
	if decl.TypeAnnot.Kind != ast.TypeList || decl.TypeAnnot.Elem.Kind != ast.TypeList || decl.TypeAnnot.Elem.Elem.Kind != ast.TypeString {
		t.Fatalf("type: got %v, want Array<Array<string>>", decl.TypeAnnot)
	}
}

func TestParser_SetTypeAndNewTypeArgs(t *testing.T) {
	prog := mustParse(t, `const seen: Set<string> = new Set<string>()`)
	decl := prog.Statements[0].(*ast.LetDecl)
	if decl.TypeAnnot.Kind != ast.TypeSet || decl.TypeAnnot.Elem.Kind != ast.TypeString {
		t.Fatalf("type: got %v, want Set<string>", decl.TypeAnnot)
	}
	newExpr, ok := decl.Value.(*ast.NewExpr)
	if !ok {
		t.Fatalf("value: expected *ast.NewExpr, got %T", decl.Value)
	}
	if newExpr.ClassName != "Set" || len(newExpr.TypeArgs) != 1 || newExpr.TypeArgs[0].Kind != ast.TypeString {
		t.Fatalf("new expression: got %#v", newExpr)
	}
}

func TestParser_TypeAliasNestedListAndBlocklessIf(t *testing.T) {
	prog := mustParse(t, `type Factory = string[]
function run(factory: Factory): string {
    const matrix: string[][] = factory.map(e => e.split("") as string[])
    if (matrix[0].length > 0) return matrix[0][0]
    else return ""
}`)
	fn := prog.Statements[1].(*ast.FnDecl)
	if fn.Params[0].Type.Kind != ast.TypeList || fn.Params[0].Type.Elem.Kind != ast.TypeString {
		t.Fatalf("alias param type: got %v, want Array<string>", fn.Params[0].Type)
	}
	ifStmt := fn.Body.Statements[1].(*ast.IfStmt)
	if len(ifStmt.Then.Statements) != 1 || len(ifStmt.Else.Statements) != 1 {
		t.Fatalf("blockless if was not wrapped as single-statement blocks")
	}
}

func TestParser_GenericTypeAliasIsNotResolvedAsConcreteAlias(t *testing.T) {
	prog := mustParse(t, `type Box<T> = T[]
let value: Box = "x"`)
	decl := prog.Statements[1].(*ast.LetDecl)
	if decl.TypeAnnot.Kind != ast.TypeString {
		t.Fatalf("generic alias should remain unsupported/string, got %v", decl.TypeAnnot)
	}
}

func TestParser_Assignment(t *testing.T) {
	prog := mustParse(t, `let x: string = "a"
x = "b"`)
	if len(prog.Statements) != 2 {
		t.Fatalf("expected 2 statements, got %d", len(prog.Statements))
	}
	assign, ok := prog.Statements[1].(*ast.Assignment)
	if !ok {
		t.Fatalf("expected *ast.Assignment, got %T", prog.Statements[1])
	}
	if assign.Name != "x" {
		t.Errorf("name: got %q, want %q", assign.Name, "x")
	}
}

func TestParser_FnDecl(t *testing.T) {
	prog := mustParse(t, `function add(a: number, b: number): number {
    return a
}`)
	function, ok := prog.Statements[0].(*ast.FnDecl)
	if !ok {
		t.Fatalf("expected *ast.FnDecl, got %T", prog.Statements[0])
	}
	if function.Name != "add" {
		t.Errorf("name: got %q, want add", function.Name)
	}
	if function.Exported {
		t.Error("should not be exported")
	}
	if len(function.Params) != 2 {
		t.Errorf("params: got %d, want 2", len(function.Params))
	}
	if function.Params[0].Name != "a" || function.Params[0].Type.Kind != ast.TypeNumber {
		t.Errorf("param[0]: got %s: %v", function.Params[0].Name, function.Params[0].Type.Kind)
	}
	if function.ReturnType.Kind != ast.TypeNumber {
		t.Errorf("return type: got %v, want int", function.ReturnType.Kind)
	}
}

func TestParser_FnDeclExported(t *testing.T) {
	prog := mustParse(t, `export function greet(name: string) {
    $("echo", "hi")
}`)
	function := prog.Statements[0].(*ast.FnDecl)
	if !function.Exported {
		t.Error("should be exported")
	}
	if function.ReturnType != nil {
		t.Errorf("return type: got %v, want nil (void)", function.ReturnType)
	}
}

func TestParser_FnDeclVoid(t *testing.T) {
	prog := mustParse(t, `function noop() {
    $("true")
}`)
	function := prog.Statements[0].(*ast.FnDecl)
	if function.ReturnType != nil {
		t.Errorf("void function should have nil return type, got %v", function.ReturnType)
	}
}

func TestParser_IfStmt(t *testing.T) {
	prog := mustParse(t, `if (x > 0) {
    $("echo", "positive")
}`)
	stmt, ok := prog.Statements[0].(*ast.IfStmt)
	if !ok {
		t.Fatalf("expected *ast.IfStmt, got %T", prog.Statements[0])
	}
	_ = stmt.Condition.(*ast.BinaryExpr)
	if len(stmt.Then.Statements) != 1 {
		t.Errorf("then block: got %d statements", len(stmt.Then.Statements))
	}
	if stmt.Else != nil {
		t.Error("unexpected else block")
	}
}

func TestParser_IfElseStmt(t *testing.T) {
	prog := mustParse(t, `if (x > 0) {
    $("echo", "pos")
} else {
    $("echo", "neg")
}`)
	stmt := prog.Statements[0].(*ast.IfStmt)
	if stmt.Else == nil {
		t.Fatal("expected else block")
	}
}

func TestParser_IfElseIfChain(t *testing.T) {
	prog := mustParse(t, `if (x > 10) {
    $("echo", "big")
} else if (x > 0) {
    $("echo", "small")
} else {
    $("echo", "zero")
}`)
	stmt := prog.Statements[0].(*ast.IfStmt)
	if len(stmt.ElseIfs) != 1 {
		t.Errorf("else-ifs: got %d, want 1", len(stmt.ElseIfs))
	}
	if stmt.Else == nil {
		t.Error("expected final else block")
	}
}

func TestParser_IfElseIfBracketlessBodies(t *testing.T) {
	prog := mustParse(t, `if (x > 10) x = 1; else if (x > 0) x = 2;`)
	stmt := prog.Statements[0].(*ast.IfStmt)
	if len(stmt.Then.Statements) != 1 {
		t.Fatalf("then block: got %d statements, want 1", len(stmt.Then.Statements))
	}
	if len(stmt.ElseIfs) != 1 {
		t.Fatalf("else-ifs: got %d, want 1", len(stmt.ElseIfs))
	}
	if len(stmt.ElseIfs[0].Body.Statements) != 1 {
		t.Fatalf("else-if body: got %d statements, want 1", len(stmt.ElseIfs[0].Body.Statements))
	}
}

func TestParser_IfElseBracketlessBody(t *testing.T) {
	prog := mustParse(t, `if (x > 0) x = 1; else x = 2;`)
	stmt := prog.Statements[0].(*ast.IfStmt)
	if stmt.Else == nil {
		t.Fatal("expected else block")
	}
	if len(stmt.Else.Statements) != 1 {
		t.Fatalf("else block: got %d statements, want 1", len(stmt.Else.Statements))
	}
}

func TestParser_IfDanglingElseBindsNearestIf(t *testing.T) {
	prog := mustParse(t, `if (outer) if (inner) x = 1; else x = 2;`)
	outer := prog.Statements[0].(*ast.IfStmt)
	if outer.Else != nil {
		t.Fatal("outer if should not capture dangling else")
	}
	inner, ok := outer.Then.Statements[0].(*ast.IfStmt)
	if !ok {
		t.Fatalf("expected nested if, got %T", outer.Then.Statements[0])
	}
	if inner.Else == nil {
		t.Fatal("inner if should capture dangling else")
	}
}

func TestParser_ForRange(t *testing.T) {
	prog := mustParse(t, `for (i in Besht.iter.range(1, 10)) {
    $("echo", "${i}")
}`)
	stmt, ok := prog.Statements[0].(*ast.ForStmt)
	if !ok {
		t.Fatalf("expected *ast.ForStmt, got %T", prog.Statements[0])
	}
	if stmt.VarName != "i" {
		t.Errorf("var name: got %q, want i", stmt.VarName)
	}
	method, ok := stmt.Iterator.(*ast.MethodCallExpr)
	if !ok {
		t.Fatalf("iterator: expected *ast.MethodCallExpr, got %T", stmt.Iterator)
	}
	if method.Method != "range" {
		t.Errorf("method name: got %q, want range", method.Method)
	}
	receiver, ok := method.Receiver.(*ast.PropertyExpr)
	if !ok {
		t.Fatalf("receiver: expected *ast.PropertyExpr, got %T", method.Receiver)
	}
	if receiver.Property != "iter" {
		t.Errorf("receiver property: got %q, want iter", receiver.Property)
	}
}

func TestParser_ForList(t *testing.T) {
	prog := mustParse(t, `for (f in files) {
    $("echo", "${f}")
}`)
	stmt := prog.Statements[0].(*ast.ForStmt)
	ident, ok := stmt.Iterator.(*ast.IdentExpr)
	if !ok {
		t.Fatalf("iterator: expected *ast.IdentExpr, got %T", stmt.Iterator)
	}
	if ident.Name != "files" {
		t.Errorf("ident name: got %q, want files", ident.Name)
	}
}

func TestParser_ForOfList(t *testing.T) {
	prog := mustParse(t, `for (f of files) {
    $("echo", f)
}`)
	stmt := prog.Statements[0].(*ast.ForStmt)
	ident, ok := stmt.Iterator.(*ast.IdentExpr)
	if !ok {
		t.Fatalf("iterator: expected *ast.IdentExpr, got %T", stmt.Iterator)
	}
	if stmt.VarName != "f" || ident.Name != "files" {
		t.Fatalf("for...of parsed wrong loop: var=%q iter=%q", stmt.VarName, ident.Name)
	}
}

func TestParser_ForDeclaredOfList(t *testing.T) {
	prog := mustParse(t, `for (const f of files) {
    $("echo", f)
}`)
	stmt := prog.Statements[0].(*ast.ForStmt)
	ident, ok := stmt.Iterator.(*ast.IdentExpr)
	if !ok {
		t.Fatalf("iterator: expected *ast.IdentExpr, got %T", stmt.Iterator)
	}
	if stmt.VarName != "f" || ident.Name != "files" {
		t.Fatalf("for...of parsed wrong loop: var=%q iter=%q", stmt.VarName, ident.Name)
	}
}

func TestParser_CompoundAssignment(t *testing.T) {
	prog := mustParse(t, `count += 1`)
	stmt := prog.Statements[0].(*ast.Assignment)
	binary, ok := stmt.Value.(*ast.BinaryExpr)
	if !ok || binary.Op != "+" {
		t.Fatalf("expected desugared + binary assignment, got %T %#v", stmt.Value, stmt.Value)
	}
}

func TestParser_TemplateLiteralExpression(t *testing.T) {
	prog := mustParse(t, "let msg = `sum=${a + b}`")
	decl := prog.Statements[0].(*ast.LetDecl)
	tmpl := decl.Value.(*ast.TemplateLit)
	if len(tmpl.Exprs) != 1 {
		t.Fatalf("expr count: got %d", len(tmpl.Exprs))
	}
	if _, ok := tmpl.Exprs[0].(*ast.BinaryExpr); !ok {
		t.Fatalf("expected binary expr, got %T", tmpl.Exprs[0])
	}
}

func TestParser_TernaryExpr(t *testing.T) {
	prog := mustParse(t, `let bigger = x > y ? x : y`)
	decl := prog.Statements[0].(*ast.LetDecl)
	ternary, ok := decl.Value.(*ast.TernaryExpr)
	if !ok {
		t.Fatalf("expected *ast.TernaryExpr, got %T", decl.Value)
	}
	if _, ok := ternary.Condition.(*ast.BinaryExpr); !ok {
		t.Fatalf("expected binary condition, got %T", ternary.Condition)
	}
	if _, ok := ternary.Then.(*ast.IdentExpr); !ok {
		t.Fatalf("expected ident then branch, got %T", ternary.Then)
	}
	if _, ok := ternary.Else.(*ast.IdentExpr); !ok {
		t.Fatalf("expected ident else branch, got %T", ternary.Else)
	}
}

func TestParser_PropagateMethodExpr(t *testing.T) {
	prog := mustParse(t, `let content = $("cat", path).run().readStdout()?`)
	decl := prog.Statements[0].(*ast.LetDecl)
	if _, ok := decl.Value.(*ast.PropagateExpr); !ok {
		t.Fatalf("expected *ast.PropagateExpr, got %T", decl.Value)
	}
}

func TestParser_NumberConstants(t *testing.T) {
	prog := mustParse(t, `let max = Number.MAX_SAFE_INTEGER
let min = Number.MIN_SAFE_INTEGER
let eps = Number.EPSILON`)
	maxDecl := prog.Statements[0].(*ast.LetDecl)
	maxLit, ok := maxDecl.Value.(*ast.IntLit)
	if !ok {
		t.Fatalf("MAX_SAFE_INTEGER: expected *ast.IntLit, got %T", maxDecl.Value)
	}
	if maxLit.Value != 9007199254740991 {
		t.Fatalf("MAX_SAFE_INTEGER: got %d", maxLit.Value)
	}
	minDecl := prog.Statements[1].(*ast.LetDecl)
	minLit, ok := minDecl.Value.(*ast.IntLit)
	if !ok {
		t.Fatalf("MIN_SAFE_INTEGER: expected *ast.IntLit, got %T", minDecl.Value)
	}
	if minLit.Value != -9007199254740991 {
		t.Fatalf("MIN_SAFE_INTEGER: got %d", minLit.Value)
	}
	epsDecl := prog.Statements[2].(*ast.LetDecl)
	epsLit, ok := epsDecl.Value.(*ast.FloatLit)
	if !ok {
		t.Fatalf("EPSILON: expected *ast.FloatLit, got %T", epsDecl.Value)
	}
	if epsLit.Value != "2.220446049250313e-16" {
		t.Fatalf("EPSILON: got %q", epsLit.Value)
	}
}

func TestParser_NumberConstantsInTemplateInterpolation(t *testing.T) {
	prog := mustParse(t, "let msg = `max=${Number.MAX_SAFE_INTEGER} eps=${Number.EPSILON}`")
	decl := prog.Statements[0].(*ast.LetDecl)
	tmpl := decl.Value.(*ast.TemplateLit)
	if len(tmpl.Exprs) != 2 {
		t.Fatalf("template expressions: got %d, want 2", len(tmpl.Exprs))
	}
	if _, ok := tmpl.Exprs[0].(*ast.IntLit); !ok {
		t.Fatalf("first interpolation: expected *ast.IntLit, got %T", tmpl.Exprs[0])
	}
	if _, ok := tmpl.Exprs[1].(*ast.FloatLit); !ok {
		t.Fatalf("second interpolation: expected *ast.FloatLit, got %T", tmpl.Exprs[1])
	}
}

func TestParser_MathConstants(t *testing.T) {
	prog := mustParse(t, `let e = Math.E
let ln2 = Math.LN2
let ln10 = Math.LN10
let log2e = Math.LOG2E
let log10e = Math.LOG10E
let pi = Math.PI
let sqrtHalf = Math.SQRT1_2
let sqrt2 = Math.SQRT2`)
	wants := []string{
		"2.718281828459045",
		"0.6931471805599453",
		"2.302585092994046",
		"1.4426950408889634",
		"0.4342944819032518",
		"3.141592653589793",
		"0.7071067811865476",
		"1.4142135623730951",
	}
	for i, want := range wants {
		decl := prog.Statements[i].(*ast.LetDecl)
		lit, ok := decl.Value.(*ast.FloatLit)
		if !ok {
			t.Fatalf("statement %d: expected *ast.FloatLit, got %T", i, decl.Value)
		}
		if lit.Value != want {
			t.Fatalf("statement %d: got %q, want %q", i, lit.Value, want)
		}
	}
}

func TestParser_DeclareFunction(t *testing.T) {
	prog := mustParse(t, `declare function external(name: string): string`)
	if _, ok := prog.Statements[0].(*ast.DeclareFnStmt); !ok {
		t.Fatalf("expected declare function stmt, got %T", prog.Statements[0])
	}
}

func TestParser_ForShell(t *testing.T) {
	prog := mustParse(t, `for (line in $("cat", "/etc/hosts").readStdoutLines()) {
    $("echo", line)
}`)
	stmt := prog.Statements[0].(*ast.ForStmt)
	_, ok := stmt.Iterator.(*ast.MethodCallExpr)
	if !ok {
		t.Fatalf("iterator: expected *ast.MethodCallExpr, got %T", stmt.Iterator)
	}
}

func TestParser_ForBracketlessBody(t *testing.T) {
	prog := mustParse(t, `for (let i = 0; i < 3; i++) x += i`)
	stmt := prog.Statements[0].(*ast.CStyleForStmt)
	if len(stmt.Body.Statements) != 1 {
		t.Fatalf("for body: got %d statements, want 1", len(stmt.Body.Statements))
	}
}

func TestParser_WhileStmt(t *testing.T) {
	prog := mustParse(t, `while (count > 0) {
    $("echo", "${count}")
}`)
	stmt, ok := prog.Statements[0].(*ast.WhileStmt)
	if !ok {
		t.Fatalf("expected *ast.WhileStmt, got %T", prog.Statements[0])
	}
	_ = stmt.Condition.(*ast.BinaryExpr)
}

func TestParser_WhileBracketlessBody(t *testing.T) {
	prog := mustParse(t, `while (count > 0) count--`)
	stmt := prog.Statements[0].(*ast.WhileStmt)
	if len(stmt.Body.Statements) != 1 {
		t.Fatalf("while body: got %d statements, want 1", len(stmt.Body.Statements))
	}
}

func TestParser_TryCatch(t *testing.T) {
	prog := mustParse(t, `try {
    $("risky_cmd")
} catch (err: status) {
    $("echo", "failed")
}`)
	stmt, ok := prog.Statements[0].(*ast.TryStmt)
	if !ok {
		t.Fatalf("expected *ast.TryStmt, got %T", prog.Statements[0])
	}
	if stmt.CatchVar != "err" {
		t.Errorf("catch var: got %q, want err", stmt.CatchVar)
	}
}

func TestParser_ReturnValue(t *testing.T) {
	prog := mustParse(t, `function f(): number {
    return 42
}`)
	function := prog.Statements[0].(*ast.FnDecl)
	ret := function.Body.Statements[0].(*ast.ReturnStmt)
	if ret.Value == nil {
		t.Fatal("expected return value")
	}
	lit := ret.Value.(*ast.IntLit)
	if lit.Value != 42 {
		t.Errorf("return value: got %d, want 42", lit.Value)
	}
}

func TestParser_ImportDecl(t *testing.T) {
	prog := mustParse(t, `import { foo, bar } from "./lib"`)
	if len(prog.Imports) != 1 {
		t.Fatalf("imports: got %d, want 1", len(prog.Imports))
	}
	imp := prog.Imports[0]
	if imp.Source != "./lib" {
		t.Errorf("source: got %q, want ./lib", imp.Source)
	}
	if imp.AssertType != "" {
		t.Errorf("assert type: got %q, want empty", imp.AssertType)
	}
	if len(imp.Names) != 2 || imp.Names[0] != "foo" || imp.Names[1] != "bar" {
		t.Errorf("names: got %v, want [foo bar]", imp.Names)
	}
}

func TestParser_ImportDeclWithAssertion(t *testing.T) {
	prog := mustParse(t, `import { legacy } from "./legacy.sh" assert { type: "shell" }`)
	if len(prog.Imports) != 1 {
		t.Fatalf("imports: got %d, want 1", len(prog.Imports))
	}
	imp := prog.Imports[0]
	if imp.Source != "./legacy.sh" {
		t.Errorf("source: got %q, want ./legacy.sh", imp.Source)
	}
	if imp.AssertType != "shell" {
		t.Errorf("assert type: got %q, want shell", imp.AssertType)
	}
	if len(imp.Names) != 1 || imp.Names[0] != "legacy" {
		t.Errorf("names: got %v, want [legacy]", imp.Names)
	}
}

func TestParser_DefaultAndCombinedImportDecl(t *testing.T) {
	prog := mustParse(t, `import def, { foo } from "./lib"`)
	imp := prog.Imports[0]
	if imp.DefaultName != "def" {
		t.Errorf("default: got %q, want def", imp.DefaultName)
	}
	if imp.AssertType != "" {
		t.Errorf("assert type: got %q, want empty", imp.AssertType)
	}
	if len(imp.Names) != 1 || imp.Names[0] != "foo" {
		t.Errorf("names: got %v, want [foo]", imp.Names)
	}
}

func TestParser_DefaultImportCommaRequiresNamedImportList(t *testing.T) {
	_, err := parser.Parse(`import def, from "./lib"`, "test.bsh")
	if err == nil {
		t.Fatal("expected parse error, got nil")
	}
	if !strings.Contains(err.Error(), "expected named import list after comma") {
		t.Fatalf("error: got %v, want named import list message", err)
	}
}

func TestParser_ExportValues(t *testing.T) {
	prog := mustParse(t, `export const cmd = ["echo", "named"]
export default ["echo", "default"]`)
	if len(prog.Statements) != 2 {
		t.Fatalf("statements: got %d, want 2", len(prog.Statements))
	}
	named := prog.Statements[0].(*ast.LetDecl)
	if !named.Exported || named.DefaultExport || named.Name != "cmd" {
		t.Fatalf("named export: %#v", named)
	}
	def := prog.Statements[1].(*ast.LetDecl)
	if !def.Exported || !def.DefaultExport || def.Name != "default" {
		t.Fatalf("default export: %#v", def)
	}
}

func TestParser_DefaultImportDecl(t *testing.T) {
	prog := mustParse(t, `import def from "./lib"`)
	if len(prog.Imports) != 1 {
		t.Fatalf("imports: got %d, want 1", len(prog.Imports))
	}
	imp := prog.Imports[0]
	if imp.DefaultName != "def" {
		t.Fatalf("default import: got %q, want def", imp.DefaultName)
	}
	if imp.Source != "./lib" {
		t.Fatalf("source: got %q, want ./lib", imp.Source)
	}
	if imp.AssertType != "" {
		t.Fatalf("assert type: got %q, want empty", imp.AssertType)
	}
}

func TestParser_ImportDeclMalformedAssertion(t *testing.T) {
	_, err := parser.Parse(`import { legacy } from "./legacy.sh" assert { type: "text" }`, "test.bsh")
	if err == nil {
		t.Fatal("expected parse error, got nil")
	}
	if !strings.Contains(err.Error(), `expected import assertion type "shell"`) {
		t.Fatalf("error: got %v, want import assertion type message", err)
	}
}

func TestParser_ExportConstAndDefault(t *testing.T) {
	prog := mustParse(t, `export const cmd = ["echo", "named"]
export default ["echo", "default"]`)
	if len(prog.Statements) != 2 {
		t.Fatalf("statements: got %d, want 2", len(prog.Statements))
	}
	cmd := prog.Statements[0].(*ast.LetDecl)
	if !cmd.Exported || cmd.DefaultExport || cmd.Name != "cmd" || !cmd.IsConst {
		t.Fatalf("export const metadata wrong: %#v", cmd)
	}
	def := prog.Statements[1].(*ast.LetDecl)
	if !def.Exported || !def.DefaultExport || def.Name != "default" || !def.IsConst {
		t.Fatalf("export default metadata wrong: %#v", def)
	}
}

func TestParser_CmdExprStmt(t *testing.T) {
	prog := mustParse(t, `$("echo", "hello world")`)
	stmt, ok := prog.Statements[0].(*ast.ExprStmt)
	if !ok {
		t.Fatalf("expected *ast.ExprStmt, got %T", prog.Statements[0])
	}
	cmd, ok := stmt.Expr.(*ast.CmdExpr)
	if !ok {
		t.Fatalf("expected *ast.CmdExpr, got %T", stmt.Expr)
	}
	if len(cmd.Args) != 2 {
		t.Errorf("cmd args: got %d, want 2", len(cmd.Args))
	}
}

func TestParser_FnCallExpr(t *testing.T) {
	prog := mustParse(t, `greet("Alice", 3)`)
	stmt := prog.Statements[0].(*ast.ExprStmt)
	call, ok := stmt.Expr.(*ast.FnCallExpr)
	if !ok {
		t.Fatalf("expected *ast.FnCallExpr, got %T", stmt.Expr)
	}
	if call.Name != "greet" {
		t.Errorf("function name: got %q, want greet", call.Name)
	}
	if len(call.Args) != 2 {
		t.Errorf("args: got %d, want 2", len(call.Args))
	}
}

func TestParser_BinaryArithmetic(t *testing.T) {
	prog := mustParse(t, `let r: number = a + b * c`)
	decl := prog.Statements[0].(*ast.LetDecl)
	add := decl.Value.(*ast.BinaryExpr)
	if add.Op != "+" {
		t.Errorf("outer op: got %q, want +", add.Op)
	}
	mul := add.Right.(*ast.BinaryExpr)
	if mul.Op != "*" {
		t.Errorf("inner op: got %q, want *", mul.Op)
	}
}

func TestParser_UnaryNot(t *testing.T) {
	prog := mustParse(t, `if (!flag) {
    $("echo", "nope")
}`)
	stmt := prog.Statements[0].(*ast.IfStmt)
	unary, ok := stmt.Condition.(*ast.UnaryExpr)
	if !ok {
		t.Fatalf("condition: expected *ast.UnaryExpr, got %T", stmt.Condition)
	}
	if unary.Op != "!" {
		t.Errorf("op: got %q, want !", unary.Op)
	}
}

func TestParser_PipeMethod(t *testing.T) {
	prog := mustParse(t, `let r: string = $("cat", "/etc/passwd").pipe($("grep", "root"))`)
	decl := prog.Statements[0].(*ast.LetDecl)
	method, ok := decl.Value.(*ast.MethodCallExpr)
	if !ok {
		t.Fatalf("value: expected *ast.MethodCallExpr, got %T", decl.Value)
	}
	if method.Method != "pipe" {
		t.Errorf("method: got %q, want pipe", method.Method)
	}
	_ = method.Receiver.(*ast.CmdExpr)
}

func TestParser_PropagateExpr(t *testing.T) {
	prog := mustParse(t, `function f(): string {
    let x: string = $("cat", "file")?
    return x
}`)
	function := prog.Statements[0].(*ast.FnDecl)
	decl := function.Body.Statements[0].(*ast.LetDecl)
	_, ok := decl.Value.(*ast.PropagateExpr)
	if !ok {
		t.Fatalf("value: expected *ast.PropagateExpr, got %T", decl.Value)
	}
}

func TestParser_BuiltinCalls(t *testing.T) {
	tests := []struct {
		src  string
		name string
	}{
		{`let _ : boolean = Boolean(value)`, "Boolean"},
		{`let _ : number = Number.parseInt(value)`, "Number.parseInt"},
		{`let _ : number = parseInt(value)`, "parseInt"},
		{`let _ : number = parseFloat(value)`, "parseFloat"},
		{`let _ : boolean = isFinite(value)`, "isFinite"},
		{`let _ : boolean = isNaN(value)`, "isNaN"},
		{`let _ : string[] = Object.keys(value)`, "Object.keys"},
	}
	for _, tt := range tests {
		prog := mustParse(t, tt.src)
		decl := prog.Statements[0].(*ast.LetDecl)
		builtin, ok := decl.Value.(*ast.BuiltinCallExpr)
		if !ok {
			t.Errorf("%s: expected *ast.BuiltinCallExpr, got %T", tt.name, decl.Value)
			continue
		}
		if builtin.Name != tt.name {
			t.Errorf("builtin name: got %q, want %q", builtin.Name, tt.name)
		}
	}
}

func TestParser_FetchParsesAsBuiltinCall(t *testing.T) {
	prog := mustParse(t, `let body = fetch(url).text()`)
	decl := prog.Statements[0].(*ast.LetDecl)
	method, ok := decl.Value.(*ast.MethodCallExpr)
	if !ok || method.Method != "text" {
		t.Fatalf("value: expected .text() method call, got %T %#v", decl.Value, decl.Value)
	}
	builtin, ok := method.Receiver.(*ast.BuiltinCallExpr)
	if !ok {
		t.Fatalf("receiver: expected *ast.BuiltinCallExpr, got %T", method.Receiver)
	}
	if builtin.Name != "fetch" || len(builtin.Args) != 1 {
		t.Fatalf("fetch builtin shape: got %#v", builtin)
	}
}

func TestParser_BeshtFsCallShape(t *testing.T) {
	prog := mustParse(t, `let ok: boolean = Besht.fs.isFile(path)`)
	decl := prog.Statements[0].(*ast.LetDecl)
	method, ok := decl.Value.(*ast.MethodCallExpr)
	if !ok {
		t.Fatalf("value: expected *ast.MethodCallExpr, got %T", decl.Value)
	}
	if method.Method != "isFile" {
		t.Fatalf("method: got %q, want isFile", method.Method)
	}
	receiver, ok := method.Receiver.(*ast.PropertyExpr)
	if !ok {
		t.Fatalf("receiver: expected *ast.PropertyExpr, got %T", method.Receiver)
	}
	if receiver.Property != "fs" {
		t.Fatalf("receiver property: got %q, want fs", receiver.Property)
	}
	ident, ok := receiver.Receiver.(*ast.IdentExpr)
	if !ok {
		t.Fatalf("receiver receiver: expected *ast.IdentExpr, got %T", receiver.Receiver)
	}
	if ident.Name != "Besht" {
		t.Fatalf("receiver ident: got %q, want Besht", ident.Name)
	}
}

func TestParser_ErrorMissingColon(t *testing.T) {
	expectParseError(t, "let x string = 5")
}

func TestParser_ErrorMissingRBrace(t *testing.T) {
	expectParseError(t, `function f() {
    $("echo", "hi")
`)
}

func TestParser_ErrorMissingCatchParens(t *testing.T) {
	expectParseError(t, `try {
    $("x")
} catch err: status {
    $("y")
}`)
}

func TestParser_ParenGrouping(t *testing.T) {
	prog := mustParse(t, `let r: boolean = (a > 0) && (b < 10)`)
	decl := prog.Statements[0].(*ast.LetDecl)
	binary := decl.Value.(*ast.BinaryExpr)
	if binary.Op != "&&" {
		t.Errorf("op: got %q, want &&", binary.Op)
	}
}

func TestParser_AndOrPrecedence(t *testing.T) {
	prog := mustParse(t, `let r: boolean = a && b || c`)
	decl := prog.Statements[0].(*ast.LetDecl)
	or := decl.Value.(*ast.BinaryExpr)
	if or.Op != "||" {
		t.Errorf("outer: got %q, want ||", or.Op)
	}
	and := or.Left.(*ast.BinaryExpr)
	if and.Op != "&&" {
		t.Errorf("inner: got %q, want &&", and.Op)
	}
}

func TestParser_NullishPrecedence(t *testing.T) {
	prog := mustParse(t, `let r = missing ?? "fallback" + "!"`)
	decl := prog.Statements[0].(*ast.LetDecl)
	nullish := decl.Value.(*ast.BinaryExpr)
	if nullish.Op != "??" {
		t.Fatalf("outer op: got %q, want ??", nullish.Op)
	}
	add := nullish.Right.(*ast.BinaryExpr)
	if add.Op != "+" {
		t.Errorf("right op: got %q, want +", add.Op)
	}
}

func TestParser_NullishChaining(t *testing.T) {
	prog := mustParse(t, `let r = a ?? b ?? c`)
	decl := prog.Statements[0].(*ast.LetDecl)
	outer := decl.Value.(*ast.BinaryExpr)
	if outer.Op != "??" {
		t.Fatalf("outer op: got %q, want ??", outer.Op)
	}
	inner := outer.Left.(*ast.BinaryExpr)
	if inner.Op != "??" {
		t.Errorf("inner op: got %q, want ??", inner.Op)
	}
}

func TestParser_NullLiteral(t *testing.T) {
	prog := mustParse(t, `let r = null ?? "fallback"`)
	decl := prog.Statements[0].(*ast.LetDecl)
	nullish := decl.Value.(*ast.BinaryExpr)
	if _, ok := nullish.Left.(*ast.NullLit); !ok {
		t.Fatalf("left: expected *ast.NullLit, got %T", nullish.Left)
	}
}

func TestParser_EmptyList(t *testing.T) {
	prog := mustParse(t, `let l: Array<string> = []`)
	decl := prog.Statements[0].(*ast.LetDecl)
	lit := decl.Value.(*ast.ListLit)
	if len(lit.Elements) != 0 {
		t.Errorf("empty list: got %d elements", len(lit.Elements))
	}
}

func TestParser_ClassDecl(t *testing.T) {
	prog := mustParse(t, `class User {
    name: string
    age: number
    constructor(name: string, age: number) {
        this.name = name
        this.age = age
    }
    greet(): string {
        return "Hello, " + this.name
    }
    static label: string = "user"
    static make(): string {
        return User.label
    }
}`)
	cls, ok := prog.Statements[0].(*ast.ClassDecl)
	if !ok {
		t.Fatalf("expected *ast.ClassDecl, got %T", prog.Statements[0])
	}
	if cls.Name != "User" || len(cls.Properties) != 2 || cls.Constructor == nil || len(cls.Methods) != 2 || len(cls.StaticProps) != 1 {
		t.Fatalf("unexpected class parse: %#v", cls)
	}
}

func TestParser_NewAndThisExpr(t *testing.T) {
	prog := mustParse(t, `class User {
    name: string
    constructor(name: string) { this.name = name }
}
let u = new User("Alice")`)
	decl := prog.Statements[1].(*ast.LetDecl)
	if _, ok := decl.Value.(*ast.NewExpr); !ok {
		t.Fatalf("expected *ast.NewExpr, got %T", decl.Value)
	}
}

func TestParser_ClassMembersCanUseTypeKeywordNames(t *testing.T) {
	prog := mustParse(t, `class Feature {
    status: string
    constructor(status: string) {
        this.status = status
    }
    status(): string {
        return this.status
    }
}`)
	cls := prog.Statements[0].(*ast.ClassDecl)
	if cls.Properties[0].Name != "status" || cls.Methods[0].Name != "status" {
		t.Fatalf("expected status property and method, got %#v", cls)
	}
}

func TestParser_ClassAccessors(t *testing.T) {
	prog := mustParse(t, `class Circle {
    radius: number
    get area(): number { return this.radius * this.radius }
    set area(value: number) { this.radius = value }
    static get label(): string { return "circle" }
}`)
	cls := prog.Statements[0].(*ast.ClassDecl)
	if len(cls.Accessors) != 3 {
		t.Fatalf("expected 3 accessors, got %#v", cls.Accessors)
	}
	if cls.Accessors[0].Kind != ast.AccessorGet || cls.Accessors[0].Name != "area" || cls.Accessors[0].ReturnType == nil || cls.Accessors[0].ReturnType.Kind != ast.TypeNumber {
		t.Fatalf("unexpected getter: %#v", cls.Accessors[0])
	}
	if cls.Accessors[1].Kind != ast.AccessorSet || cls.Accessors[1].Name != "area" || len(cls.Accessors[1].Params) != 1 || cls.Accessors[1].Params[0].Name != "value" {
		t.Fatalf("unexpected setter: %#v", cls.Accessors[1])
	}
	if cls.Accessors[2].Kind != ast.AccessorGet || !cls.Accessors[2].IsStatic || cls.Accessors[2].Name != "label" {
		t.Fatalf("unexpected static getter: %#v", cls.Accessors[2])
	}
}

func TestParser_TypeScriptClassModifiersAndStaticRecord(t *testing.T) {
	prog := mustParse(t, `function run() {
    class Game {
        readonly matrix: string[][]
        private static Deltas: Record<string, [number, number]> = { U: [-1, 0] }
        private moveCur(delta: [number, number]) {
            const [dr, dc] = Game.Deltas["U"]
            return this.matrix[dr]?.[dc]
        }
    }
}`)
	fn := prog.Statements[0].(*ast.FnDecl)
	cls := fn.Body.Statements[0].(*ast.ClassDecl)
	if len(cls.Properties) != 1 || len(cls.StaticProps) != 1 || len(cls.Methods) != 1 {
		t.Fatalf("unexpected class parse: %#v", cls)
	}
	method := cls.Methods[0]
	if _, ok := method.Body.Statements[0].(*ast.DestructureDecl); !ok {
		t.Fatalf("expected destructure declaration, got %T", method.Body.Statements[0])
	}
}

func TestParser_OptionalChainingMetadata(t *testing.T) {
	prog := mustParse(t, `let a = obj?.prop
let b = obj?.[key]
let c = obj?.method(arg)
let d = obj?.prop?.nested
let e = obj?.items?.[i]
let f = obj.prop[index].method(arg)`)

	prop := prog.Statements[0].(*ast.LetDecl).Value.(*ast.PropertyExpr)
	if !prop.Optional || prop.Property != "prop" {
		t.Fatalf("optional property metadata lost: %#v", prop)
	}

	idx := prog.Statements[1].(*ast.LetDecl).Value.(*ast.IndexExpr)
	if !idx.Optional {
		t.Fatalf("optional index metadata lost: %#v", idx)
	}

	method := prog.Statements[2].(*ast.LetDecl).Value.(*ast.MethodCallExpr)
	if !method.Optional || method.Method != "method" {
		t.Fatalf("optional method metadata lost: %#v", method)
	}

	nested := prog.Statements[3].(*ast.LetDecl).Value.(*ast.PropertyExpr)
	innerProp, ok := nested.Receiver.(*ast.PropertyExpr)
	if !ok || !nested.Optional || !innerProp.Optional {
		t.Fatalf("nested optional property metadata lost: outer=%#v inner=%#v", nested, innerProp)
	}

	nestedIndex := prog.Statements[4].(*ast.LetDecl).Value.(*ast.IndexExpr)
	innerItems, ok := nestedIndex.Expr.(*ast.PropertyExpr)
	if !ok || !nestedIndex.Optional || !innerItems.Optional {
		t.Fatalf("nested optional index metadata lost: outer=%#v inner=%#v", nestedIndex, innerItems)
	}

	normalMethod := prog.Statements[5].(*ast.LetDecl).Value.(*ast.MethodCallExpr)
	normalIndex := normalMethod.Receiver.(*ast.IndexExpr)
	normalProp := normalIndex.Expr.(*ast.PropertyExpr)
	if normalMethod.Optional || normalIndex.Optional || normalProp.Optional {
		t.Fatalf("normal chain marked optional: method=%#v index=%#v prop=%#v", normalMethod, normalIndex, normalProp)
	}
}

func TestParser_OptionalChainingRejectsUnsupportedCalls(t *testing.T) {
	expectParseError(t, `let a = fn?.()`)
	expectParseError(t, `let b = obj.method?.()`)
}

func TestParser_ProcessAPIASTShape(t *testing.T) {
	prog := mustParse(t, `let home = process.env.HOME
process.exit(1)`)

	decl := prog.Statements[0].(*ast.LetDecl)
	home, ok := decl.Value.(*ast.PropertyExpr)
	if !ok || home.Property != "HOME" {
		t.Fatalf("expected process.env.HOME outer property, got %T %#v", decl.Value, decl.Value)
	}
	env, ok := home.Receiver.(*ast.PropertyExpr)
	if !ok || env.Property != "env" {
		t.Fatalf("expected process.env receiver property, got %T %#v", home.Receiver, home.Receiver)
	}
	process, ok := env.Receiver.(*ast.IdentExpr)
	if !ok || process.Name != "process" {
		t.Fatalf("expected process ident, got %T %#v", env.Receiver, env.Receiver)
	}

	stmt := prog.Statements[1].(*ast.ExprStmt)
	call, ok := stmt.Expr.(*ast.MethodCallExpr)
	if !ok || call.Method != "exit" || len(call.Args) != 1 {
		t.Fatalf("expected process.exit(1) method call, got %T %#v", stmt.Expr, stmt.Expr)
	}
	receiver, ok := call.Receiver.(*ast.IdentExpr)
	if !ok || receiver.Name != "process" {
		t.Fatalf("expected process receiver, got %T %#v", call.Receiver, call.Receiver)
	}
}

func TestParser_ListMapArrow(t *testing.T) {
	prog := mustParse(t, `let items = ["a"]
let mapped = items.map(x => x + "!")`)
	decl := prog.Statements[1].(*ast.LetDecl)
	call, ok := decl.Value.(*ast.MethodCallExpr)
	if !ok {
		t.Fatalf("expected method call, got %T", decl.Value)
	}
	arrow, ok := call.Args[0].(*ast.ArrowExpr)
	if !ok {
		t.Fatalf("expected arrow callback, got %T", call.Args[0])
	}
	if arrow.Params[0].Name != "x" {
		t.Fatalf("param: got %q", arrow.Params[0].Name)
	}
}

func TestParser_ListFilterTypedArrow(t *testing.T) {
	prog := mustParse(t, `let items: Array<string> = ["a"]
let picked = items.filter((x: string) => x.startsWith("a"))`)
	decl := prog.Statements[1].(*ast.LetDecl)
	call := decl.Value.(*ast.MethodCallExpr)
	arrow := call.Args[0].(*ast.ArrowExpr)
	if arrow.Params[0].Type == nil || arrow.Params[0].Type.Kind != ast.TypeString {
		t.Fatalf("expected string param type")
	}
}

func TestParser_ArrowFunctionValueTypes(t *testing.T) {
	prog := mustParse(t, `let cb: (x: string) => string = (x: string): string => x + "!"`)
	decl := prog.Statements[0].(*ast.LetDecl)
	if decl.TypeAnnot == nil || decl.TypeAnnot.Kind != ast.TypeFunction {
		t.Fatalf("expected function type annotation, got %#v", decl.TypeAnnot)
	}
	if len(decl.TypeAnnot.Params) != 1 || decl.TypeAnnot.Params[0].Kind != ast.TypeString {
		t.Fatalf("expected one string function param, got %#v", decl.TypeAnnot.Params)
	}
	if decl.TypeAnnot.Return == nil || decl.TypeAnnot.Return.Kind != ast.TypeString {
		t.Fatalf("expected string function return, got %#v", decl.TypeAnnot.Return)
	}
	arrow := decl.Value.(*ast.ArrowExpr)
	if arrow.ReturnType == nil || arrow.ReturnType.Kind != ast.TypeString {
		t.Fatalf("expected arrow return type, got %#v", arrow.ReturnType)
	}
}

func TestParser_ArrayFromAndPrefixUpdateInBlockMap(t *testing.T) {
	prog := mustParse(t, `let count = 0
let mapped = Array.from({ length: 3 }).map((_, i) => {
    if (++count % 2 !== 0) return i
    return count
})`)
	decl := prog.Statements[1].(*ast.LetDecl)
	call := decl.Value.(*ast.MethodCallExpr)
	if call.Method != "map" {
		t.Fatalf("method: got %q", call.Method)
	}
	from, ok := call.Receiver.(*ast.BuiltinCallExpr)
	if !ok || from.Name != "Array.from" {
		t.Fatalf("receiver: got %T %#v", call.Receiver, call.Receiver)
	}
	arrow := call.Args[0].(*ast.ArrowExpr)
	if len(arrow.Params) != 2 || arrow.BlockBody == nil {
		t.Fatalf("expected two-param block arrow, got %#v", arrow)
	}
	ifStmt := arrow.BlockBody.Statements[0].(*ast.IfStmt)
	mod := ifStmt.Condition.(*ast.BinaryExpr).Left.(*ast.BinaryExpr)
	if _, ok := mod.Left.(*ast.UpdateExpr); !ok {
		t.Fatalf("expected prefix update in condition, got %T", mod.Left)
	}
}

func TestParser_ArrayOfParsesAsBuiltinCall(t *testing.T) {
	prog := mustParse(t, `let values = Array.of("a", "b", "c")`)
	decl := prog.Statements[0].(*ast.LetDecl)
	builtin, ok := decl.Value.(*ast.BuiltinCallExpr)
	if !ok {
		t.Fatalf("value: expected *ast.BuiltinCallExpr, got %T", decl.Value)
	}
	if builtin.Name != "Array.of" {
		t.Fatalf("builtin name: got %q, want Array.of", builtin.Name)
	}
	if len(builtin.Args) != 3 {
		t.Fatalf("args: got %d, want 3", len(builtin.Args))
	}
}

func TestParser_RPrefixedStringUnsupported(t *testing.T) {
	expectParseError(t, `let pattern = r"^foo"`)
}

func TestParser_StringRawTaggedTemplateUnsupported(t *testing.T) {
	expectParseError(t, "let path = String.raw`C:\\temp\\file.txt`")
}

func TestParser_ArrayIsArrayParsesAsBuiltinCall(t *testing.T) {
	prog := mustParse(t, `let ok = Array.isArray(["a", "b"])`)
	decl := prog.Statements[0].(*ast.LetDecl)
	builtin, ok := decl.Value.(*ast.BuiltinCallExpr)
	if !ok {
		t.Fatalf("value: expected *ast.BuiltinCallExpr, got %T", decl.Value)
	}
	if builtin.Name != "Array.isArray" {
		t.Fatalf("builtin name: got %q, want Array.isArray", builtin.Name)
	}
	if len(builtin.Args) != 1 {
		t.Fatalf("args: got %d, want 1", len(builtin.Args))
	}
}

func TestParser_BooleanParsesAsBuiltinCall(t *testing.T) {
	prog := mustParse(t, `let ok = Boolean("x")`)
	decl := prog.Statements[0].(*ast.LetDecl)
	builtin, ok := decl.Value.(*ast.BuiltinCallExpr)
	if !ok {
		t.Fatalf("value: expected *ast.BuiltinCallExpr, got %T", decl.Value)
	}
	if builtin.Name != "Boolean" {
		t.Fatalf("builtin name: got %q, want Boolean", builtin.Name)
	}
	if len(builtin.Args) != 1 {
		t.Fatalf("args: got %d, want 1", len(builtin.Args))
	}
}

func TestParser_StringParsesAsBuiltinCall(t *testing.T) {
	prog := mustParse(t, `let text = String(42)`)
	decl := prog.Statements[0].(*ast.LetDecl)
	builtin, ok := decl.Value.(*ast.BuiltinCallExpr)
	if !ok {
		t.Fatalf("value: expected *ast.BuiltinCallExpr, got %T", decl.Value)
	}
	if builtin.Name != "String" {
		t.Fatalf("builtin name: got %q, want String", builtin.Name)
	}
	if len(builtin.Args) != 1 {
		t.Fatalf("args: got %d, want 1", len(builtin.Args))
	}
}

func TestParser_StringStaticMethodsRejectClearly(t *testing.T) {
	expectParseErrorContains(t, "let text = String.raw(\"x\")", "String.raw is not supported; only String(value) is supported")
	expectParseErrorContains(t, "let text = String.raw`x`", "String.raw tagged templates are not supported")
}

func TestParser_ObjectStaticMethodsParseAsBuiltinCalls(t *testing.T) {
	tests := []struct {
		name     string
		src      string
		wantName string
		wantArgs int
	}{
		{"keys", `let keys = Object.keys(user)`, "Object.keys", 1},
		{"values", `let values = Object.values(user)`, "Object.values", 1},
		{"entries", `let entries = Object.entries(user)`, "Object.entries", 1},
		{"hasOwn", `let ok = Object.hasOwn(user, "name")`, "Object.hasOwn", 2},
		{"is", `let ok = Object.is(value, other)`, "Object.is", 2},
		{"assign", `let merged = Object.assign(user, defaults, overrides)`, "Object.assign", 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prog := mustParse(t, tt.src)
			decl := prog.Statements[0].(*ast.LetDecl)
			builtin, ok := decl.Value.(*ast.BuiltinCallExpr)
			if !ok {
				t.Fatalf("value: expected *ast.BuiltinCallExpr, got %T", decl.Value)
			}
			if builtin.Name != tt.wantName {
				t.Fatalf("builtin name: got %q, want %s", builtin.Name, tt.wantName)
			}
			if len(builtin.Args) != tt.wantArgs {
				t.Fatalf("args: got %d, want %d", len(builtin.Args), tt.wantArgs)
			}
		})
	}
}

func TestParser_ObjectLiteralSpread(t *testing.T) {
	prog := mustParse(t, `let merged = { ...defaults, active: true, ...overrides }`)
	decl := prog.Statements[0].(*ast.LetDecl)
	obj, ok := decl.Value.(*ast.ObjectLit)
	if !ok {
		t.Fatalf("value: expected *ast.ObjectLit, got %T", decl.Value)
	}
	if len(obj.Fields) != 3 {
		t.Fatalf("fields: got %d, want 3", len(obj.Fields))
	}
	if obj.Fields[0].Spread == nil {
		t.Fatalf("first field: expected spread")
	}
	if obj.Fields[1].Key != "active" || obj.Fields[1].Value == nil {
		t.Fatalf("second field: got key=%q value=%T, want active field", obj.Fields[1].Key, obj.Fields[1].Value)
	}
	if obj.Fields[2].Spread == nil {
		t.Fatalf("third field: expected spread")
	}
}

func TestParser_BracketlessIfElseBodies(t *testing.T) {
	prog := mustParse(t, `if (m1 === "F") dmgTo2 += 2;
else if (m1 === "A" && m2 !== "B") dmgTo2 += 1;
else dmgTo1 += 1;`)
	stmt := prog.Statements[0].(*ast.IfStmt)
	if len(stmt.Then.Statements) != 1 {
		t.Fatalf("then statements: got %d", len(stmt.Then.Statements))
	}
	if len(stmt.ElseIfs) != 1 || len(stmt.ElseIfs[0].Body.Statements) != 1 {
		t.Fatalf("expected one else-if with one statement, got %#v", stmt.ElseIfs)
	}
	if stmt.Else == nil || len(stmt.Else.Statements) != 1 {
		t.Fatalf("expected one-statement else block, got %#v", stmt.Else)
	}
}

func TestParser_BracketlessDanglingElseBindsInnerIf(t *testing.T) {
	prog := mustParse(t, `if (a) if (b) x += 1; else y += 1;`)
	outer := prog.Statements[0].(*ast.IfStmt)
	if outer.Else != nil {
		t.Fatalf("outer if should not consume dangling else: %#v", outer.Else)
	}
	inner, ok := outer.Then.Statements[0].(*ast.IfStmt)
	if !ok {
		t.Fatalf("expected inner if, got %T", outer.Then.Statements[0])
	}
	if inner.Else == nil || len(inner.Else.Statements) != 1 {
		t.Fatalf("expected dangling else on inner if, got %#v", inner.Else)
	}
}

func TestParser_BracketlessLoopBodies(t *testing.T) {
	prog := mustParse(t, `while (i < 10) i++
for (let i = 0; i < 3; i++) total += i
for (item in items) break`)
	whileStmt := prog.Statements[0].(*ast.WhileStmt)
	if len(whileStmt.Body.Statements) != 1 {
		t.Fatalf("while body statements: got %d", len(whileStmt.Body.Statements))
	}
	cstyle := prog.Statements[1].(*ast.CStyleForStmt)
	if len(cstyle.Body.Statements) != 1 {
		t.Fatalf("c-style for body statements: got %d", len(cstyle.Body.Statements))
	}
	forIn := prog.Statements[2].(*ast.ForStmt)
	if len(forIn.Body.Statements) != 1 {
		t.Fatalf("for-in body statements: got %d", len(forIn.Body.Statements))
	}
}

func TestParser_BracedControlBodiesStillParse(t *testing.T) {
	prog := mustParse(t, `if (x) { y = 1 } else { y = 2 }
while (x) { x-- }
for (let i = 0; i < 3; i++) { total += i }`)
	ifStmt := prog.Statements[0].(*ast.IfStmt)
	if len(ifStmt.Then.Statements) != 1 || ifStmt.Else == nil || len(ifStmt.Else.Statements) != 1 {
		t.Fatalf("unexpected braced if parse: %#v", ifStmt)
	}
	whileStmt := prog.Statements[1].(*ast.WhileStmt)
	if len(whileStmt.Body.Statements) != 1 {
		t.Fatalf("unexpected braced while parse: %#v", whileStmt.Body)
	}
	cstyle := prog.Statements[2].(*ast.CStyleForStmt)
	if len(cstyle.Body.Statements) != 1 {
		t.Fatalf("unexpected braced for parse: %#v", cstyle.Body)
	}
}
