package codegen_test

import (
	"strings"
	"testing"

	"github.com/victor141516/besht/internal/checker"
	"github.com/victor141516/besht/internal/codegen"
	"github.com/victor141516/besht/internal/parser"
)

func compile(t *testing.T, src string) string {
	t.Helper()
	prog, err := parser.Parse(src, "test.bsh")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	chk := checker.New()
	if err := chk.Check(prog); err != nil {
		t.Fatalf("type error: %v", err)
	}
	out, err := codegen.Generate(prog)
	if err != nil {
		t.Fatalf("codegen error: %v", err)
	}
	return out
}

func compileError(t *testing.T, src string) error {
	t.Helper()
	prog, err := parser.Parse(src, "test.bsh")
	if err != nil {
		return err
	}
	chk := checker.New()
	if err := chk.Check(prog); err != nil {
		return err
	}
	_, err = codegen.Generate(prog)
	return err
}

func assertContains(t *testing.T, got, want string) {
	t.Helper()
	if !strings.Contains(got, want) {
		t.Errorf("output missing %q\n\nfull output:\n%s", want, got)
	}
}

func assertNotContains(t *testing.T, got, want string) {
	t.Helper()
	if strings.Contains(got, want) {
		t.Errorf("output should not contain %q\n\nfull output:\n%s", want, got)
	}
}

func TestCodegen_ShebangHeader(t *testing.T) {
	out := compile(t, `$("true").run()`)
	if !strings.HasPrefix(out, "#!/bin/sh\n") {
		t.Errorf("missing shebang, got: %q", out[:min(len(out), 30)])
	}
}

func TestCodegen_LetStringDecl(t *testing.T) {
	out := compile(t, `let name: string = "Alice"`)
	assertContains(t, out, `name='Alice'`)
}

func TestCodegen_LetIntDecl(t *testing.T) {
	out := compile(t, `let count: number = 42`)
	assertContains(t, out, `count=42`)
}

func TestCodegen_LetBoolTrue(t *testing.T) {
	out := compile(t, `let flag: boolean = true`)
	assertContains(t, out, `flag=1`)
}

func TestCodegen_LetBoolFalse(t *testing.T) {
	out := compile(t, `let flag: boolean = false`)
	assertContains(t, out, `flag=0`)
}

func TestCodegen_LetShellCapture(t *testing.T) {
	out := compile(t, `let user = $("whoami").run().readStdout()`)
	assertContains(t, out, `user=$(whoami)`)
}

func TestCodegen_LetListLiteral(t *testing.T) {
	out := compile(t, `let files: list<string> = ["a.txt", "b.txt"]`)
	assertContains(t, out, "files='a.txt\nb.txt'")
	assertNotContains(t, out, `$( { printf '%s\n'`)
}

func TestCodegen_StaticListLiteralEscapesSingleQuotes(t *testing.T) {
	out := compile(t, `let words: list<string> = ["can't", "stop"]`)
	assertContains(t, out, `words='can'"'"'t`)
	assertNotContains(t, out, `$( { printf '%s\n'`)
}

func TestCodegen_Assignment(t *testing.T) {
	out := compile(t, `let x: string = "a"
x = "b"`)
	assertContains(t, out, `x='b'`)
}

func TestCodegen_ShellStmt(t *testing.T) {
	out := compile(t, `$("echo", "hello").run()`)
	assertContains(t, out, `echo hello`)
}

func TestCodegen_FnDeclVoid(t *testing.T) {
	out := compile(t, `function greet(name: string) {
    $("echo", "hi").run()
}`)
	assertContains(t, out, `greet()`)
	assertContains(t, out, `_greet_name="$1"`)
	assertContains(t, out, `echo hi`)
}

func TestCodegen_FnDeclWithReturn(t *testing.T) {
	out := compile(t, `function double(n: number): number {
    return n + n
}`)
	assertContains(t, out, `double()`)
	assertContains(t, out, `_double_n="$1"`)
	assertContains(t, out, `printf '%s'`)
	assertContains(t, out, `return 0`)
}

func TestCodegen_FnCallVoid(t *testing.T) {
	out := compile(t, `function greet(name: string) {
    $("echo", "${name}").run()
}
greet("Alice")`)
	assertContains(t, out, `greet 'Alice'`)
}

func TestCodegen_FnCallCapture(t *testing.T) {
	out := compile(t, `function shout(msg: string): string {
    return msg
}
let r: string = shout("hello")`)
	assertContains(t, out, `r=$(shout 'hello')`)
}

func TestCodegen_IfSimple(t *testing.T) {
	out := compile(t, `let n: number = 5
if (n > 0) {
    $("echo", "pos").run()
}`)
	assertContains(t, out, `awk -v _a=$n -v _b=0`)
	assertContains(t, out, `echo pos`)
	assertContains(t, out, `fi`)
}

func TestCodegen_IfElse(t *testing.T) {
	out := compile(t, `let n: number = 5
if (n > 0) {
    $("echo", "pos").run()
} else {
    $("echo", "neg").run()
}`)
	assertContains(t, out, `awk -v _a=$n -v _b=0`)
	assertContains(t, out, `else`)
	assertContains(t, out, `fi`)
}

func TestCodegen_IfElseIf(t *testing.T) {
	out := compile(t, `let n: number = 5
if (n > 10) {
    $("echo", "big").run()
} else if (n > 0) {
    $("echo", "small").run()
} else {
    $("echo", "zero").run()
}`)
	assertContains(t, out, `awk -v _a=$n -v _b=10`)
	assertContains(t, out, `awk -v _a=$n -v _b=0`)
	assertContains(t, out, `else`)
	assertContains(t, out, `fi`)
}

func TestCodegen_IfBoolCondition(t *testing.T) {
	out := compile(t, `let flag: boolean = true
if (flag) {
    $("echo", "yes").run()
}`)
	assertContains(t, out, `[ "$flag" = 1 ]`)
}

func TestCodegen_IfNegation(t *testing.T) {
	out := compile(t, `let b: boolean = true
if (!b) {
    $("echo", "no").run()
}`)
	assertContains(t, out, `! [`)
}

func TestCodegen_IfAndCondition(t *testing.T) {
	out := compile(t, `let a: number = 1
let b: number = 2
if (a > 0 && b > 0) {
    $("echo", "both").run()
}`)
	assertContains(t, out, `&&`)
}

func TestCodegen_IfOrCondition(t *testing.T) {
	out := compile(t, `let a: number = 1
let b: number = 2
if (a > 0 || b > 0) {
    $("echo", "either").run()
}`)
	assertContains(t, out, `||`)
}

func TestCodegen_IfStringEquality(t *testing.T) {
	out := compile(t, `let s: string = "hello"
let t: string = "world"
if (s == t) {
    $("echo", "same").run()
}`)
	assertContains(t, out, `_bst_left="$s"`)
	assertContains(t, out, `_bst_right="$t"`)
	assertContains(t, out, `[ "$_bst_left" = "$_bst_right" ]`)
}

func TestCodegen_IfIntEquality(t *testing.T) {
	out := compile(t, `let a: number = 1
let b: number = 1
if (a == b) {
    $("echo", "equal").run()
}`)
	assertContains(t, out, `_bst_left="$a"`)
	assertContains(t, out, `_bst_right="$b"`)
	assertContains(t, out, `[ "$_bst_left" = "$_bst_right" ]`)
}

func TestCodegen_StaticComparisons(t *testing.T) {
	out := compile(t, `let same = "a" === "a"
let diff = "a" !== "b"
let less = 2 < 3
let atLeast = 3 >= 3
if ("a" === "a") {
    $("echo", "same").run()
}`)
	assertContains(t, out, `same=1`)
	assertContains(t, out, `diff=1`)
	assertContains(t, out, `less=1`)
	assertContains(t, out, `atLeast=1`)
	assertContains(t, out, `if true; then`)
	assertNotContains(t, out, `_bst_left='a'`)
	assertNotContains(t, out, `awk -v _a=2 -v _b=3`)
}

func TestCodegen_StaticComparisonsKeepDynamicFallback(t *testing.T) {
	out := compile(t, `let a = "a"
let same = a === "a"
let n = 2
let less = n < 3`)
	assertContains(t, out, `_bst_left="$a"`)
	assertContains(t, out, `_bst_right='a'`)
	assertContains(t, out, `awk -v _a=$n -v _b=3`)
}

func TestCodegen_WhileLoop(t *testing.T) {
	out := compile(t, `let n: number = 5
while (n > 0) {
    $("echo", "${n}").run()
}`)
	assertContains(t, out, `while awk -v _a=$n -v _b=0`)
	assertContains(t, out, `done`)
}

func TestCodegen_ForRange(t *testing.T) {
	out := compile(t, `for (i in Besht.iter.range(1, 10)) {
    $("echo", "${i}").run()
}`)
	assertContains(t, out, `while [`)
	assertContains(t, out, `-le 10`)
	assertContains(t, out, `done`)
}

func TestCodegen_ForList(t *testing.T) {
	out := compile(t, `let files: list<string> = ["a", "b"]
for (f in files) {
    $("echo", "${f}").run()
}`)
	assertContains(t, out, `for f in 'a' 'b'; do`)
	assertContains(t, out, `done`)
}

func TestCodegen_ForOfList(t *testing.T) {
	out := compile(t, `let files: list<string> = ["a", "b"]
for (f of files) {
    $("echo", f).run()
}`)
	assertContains(t, out, `for f in 'a' 'b'; do`)
}

func TestCodegen_ForLetOfList(t *testing.T) {
	out := compile(t, `let files: list<string> = ["a", "b"]
for (let f of files) {
    $("echo", f).run()
}`)
	assertContains(t, out, `for f in 'a' 'b'; do`)
	assertNotContains(t, out, `_forlist_`)
}

func TestCodegen_ForStaticListVariableInvalidatedAfterAssignment(t *testing.T) {
	out := compile(t, `let files: list<string> = ["a", "b"]
files = files.push("c")
for (let f of files) {
    $("echo", f).run()
}`)
	assertContains(t, out, `while IFS= read -r`)
}

func TestCodegen_CompoundAssignment(t *testing.T) {
	out := compile(t, `let count: number = 1
count += 2`)
	assertContains(t, out, `count=$(( $count + 2 ))`)
}

func TestCodegen_StaticArithmetic(t *testing.T) {
	out := compile(t, `let sum = 2 + 3
let diff = 10 - 4
let product = 6 * 7
let quotient = 5 / 2
let remainder = 7 % 4
let nested = (2 + 3) * 4
let negated = -3`)
	assertContains(t, out, `sum=5`)
	assertContains(t, out, `diff=6`)
	assertContains(t, out, `product=42`)
	assertContains(t, out, `quotient=2.5`)
	assertContains(t, out, `remainder=3`)
	assertContains(t, out, `nested=20`)
	assertContains(t, out, `negated=-3`)
	assertNotContains(t, out, `$(( 2 + 3 ))`)
	assertNotContains(t, out, `awk -v _a=5 -v _b=2`)
}

func TestCodegen_StaticArithmeticKeepsDynamicFallback(t *testing.T) {
	out := compile(t, `let a = 2
let sum = a + 3
let divZero = 1 / 0`)
	assertContains(t, out, `sum=$(( $a + 3 ))`)
	assertContains(t, out, `divZero=$(awk -v _a=1 -v _b=0`)
}

func TestCodegen_TemplateLiteralExpression(t *testing.T) {
	out := compile(t, "let a: number = 1\nlet b: number = 2\nlet msg: string = `sum=${a + b}`")
	assertContains(t, out, `sum=`)
}

func TestCodegen_TernaryNumber(t *testing.T) {
	out := compile(t, `let x: number = 10
let y: number = 3
let bigger: number = x > y ? x : y`)
	assertContains(t, out, `awk -v _a=$x -v _b=$y`)
}

func TestCodegen_TernaryString(t *testing.T) {
	out := compile(t, `let x: number = 10
let label: string = x > 5 ? "big" : "small"`)
	assertContains(t, out, `label=$(if awk -v _a=$x -v _b=5`)
}

func TestCodegen_StaticBooleanTernary(t *testing.T) {
	out := compile(t, `let label = true ? "yes" : "no"
let negated = !true ? "bad" : "good"
let both = true && false ? "bad" : "ok"`)
	assertContains(t, out, `label='yes'`)
	assertContains(t, out, `negated='good'`)
	assertContains(t, out, `both='ok'`)
	assertNotContains(t, out, `$(if true; then`)
	assertNotContains(t, out, `$(if ! true; then`)
	assertNotContains(t, out, `printf '%s' 'bad'`)
}

func TestCodegen_StaticBooleanIf(t *testing.T) {
	out := compile(t, `if (true) {
    console.log("yes")
} else {
    console.log("no")
}
if (false) {
    console.log("bad")
} else {
    console.log("fallback")
}`)
	assertContains(t, out, `printf '%s\n' 'yes'`)
	assertContains(t, out, `printf '%s\n' 'fallback'`)
	assertNotContains(t, out, `if true; then`)
	assertNotContains(t, out, `if false; then`)
	assertNotContains(t, out, `printf '%s\n' 'no'`)
	assertNotContains(t, out, `printf '%s\n' 'bad'`)
}

func TestCodegen_NumberToString(t *testing.T) {
	out := compile(t, `let n: number = 42
let s: string = n.toString()`)
	assertContains(t, out, `printf '%s' "$n"`)
}

func TestCodegen_PrimitiveToString(t *testing.T) {
	out := compile(t, `let s: string = "x"
let ss: string = s.toString()
let t: boolean = true
let ts: string = t.toString()
let f: boolean = false
let fs: string = f.toString()
try {
    $("false").run()
} catch (code: status) {
    let cs: string = code.toString()
}`)
	assertContains(t, out, `ss=$(printf '%s' "$s")`)
	assertContains(t, out, `ts=$(if [ $t = 1 ]; then printf true; else printf false; fi)`)
	assertContains(t, out, `fs=$(if [ $f = 1 ]; then printf true; else printf false; fi)`)
	assertContains(t, out, `cs=$(printf '%s' "$code")`)
}

func TestCodegen_NumberToFixed(t *testing.T) {
	out := compile(t, `let pi: number = 3.14159
let s: string = pi.toFixed(2)`)
	assertContains(t, out, `awk -v _x="$pi" -v _n=2 'BEGIN{OFMT="%.17g";printf "%.*f", _n, _x}'`)
}

func TestCodegen_StaticNumberMethods(t *testing.T) {
	out := compile(t, `let s: string = (42).toString()
let fixed: string = (3.14159).toFixed(2)
let whole: string = (3.9).toFixed()`)
	assertContains(t, out, `s='42'`)
	assertContains(t, out, `fixed='3.14'`)
	assertContains(t, out, `whole='4'`)
	assertNotContains(t, out, `awk`)
}

func TestCodegen_StaticPrimitiveToStringBindings(t *testing.T) {
	out := compile(t, `let a = true.toString()
let b = false.toString()
let c = ("x" === "x").toString()
let d = Boolean("").toString()
let e = (2 + 3).toString()`)
	assertContains(t, out, `a='true'`)
	assertContains(t, out, `b='false'`)
	assertContains(t, out, `c='true'`)
	assertContains(t, out, `d='false'`)
	assertContains(t, out, `e='5'`)
	assertNotContains(t, out, `if [ 1 = 1 ]; then printf true`)
	assertNotContains(t, out, `printf '%s' 5`)
}

func TestCodegen_StaticToStringConcatFragments(t *testing.T) {
	out := compile(t, `let a = "count=" + (2 + 3).toString()
let b = "flag=" + true.toString()
let c = `+"`same=${(\"x\" === \"x\").toString()}`"+`
console.log("value=" + false.toString())`)
	assertContains(t, out, `a="count=5"`)
	assertContains(t, out, `b="flag=true"`)
	assertContains(t, out, `c="same=true"`)
	assertContains(t, out, `printf '%s\n' "value=false"`)
	assertNotContains(t, out, `count=$(printf '%s' 5)`)
	assertNotContains(t, out, `flag=$(if [ 1 = 1 ]`)
}

func TestCodegen_StaticToStringConcatKeepsDynamicFallback(t *testing.T) {
	out := compile(t, `function show(n: number, ok: boolean) {
    let a = "n=" + n.toString()
    let b = `+"`ok=${ok.toString()}`"+`
}`)
	assertContains(t, out, `a="n=$(printf '%s' "$_show_n")"`)
	assertContains(t, out, `b="ok=$(if [ $_show_ok = 1 ]; then printf true; else printf false; fi)"`)
}

func TestCodegen_SpreadCommandArgs(t *testing.T) {
	out := compile(t, `let args: list<string> = ["a b", "c"]
$("echo", ...args).run()`)
	assertContains(t, out, `set --`)
	assertContains(t, out, `while IFS= read -r`)
}

func TestCodegen_SourceComments(t *testing.T) {
	out := compile(t, `let name: string = "Alice"`)
	assertContains(t, out, `# besht:test.bsh:1:1`)
}

func TestCodegen_ForShell(t *testing.T) {
	out := compile(t, `for (line in $("cat", "/etc/hosts").readStdoutLines()) {
    $("echo", "${line}").run()
}`)
	assertContains(t, out, `cat`)
	assertContains(t, out, `while IFS= read -r`)
	assertContains(t, out, `done`)
}

func TestCodegen_TryCatch(t *testing.T) {
	out := compile(t, `try {
    $("risky_cmd").run()
} catch (err: status) {
    $("echo", "failed").run()
}`)
	assertContains(t, out, `_try_status_`)
	assertContains(t, out, `set -e`)
	assertContains(t, out, `risky_cmd`)
	assertContains(t, out, `=$?`)
	assertContains(t, out, `echo failed`)
	assertContains(t, out, `fi`)
}

func TestCodegen_TryCatchVarAvailable(t *testing.T) {
	out := compile(t, `try {
    $("risky_cmd").run()
} catch (code: status) {
    $("echo", "exit: " + code).run()
}`)
	assertContains(t, out, `code="$_try_status_`)
}

func TestCodegen_BuiltinFileExists(t *testing.T) {
	out := compile(t, `let p: string = "/tmp/x"
if (Besht.fs.isFile(p)) {
    $("echo", "exists").run()
}`)
	assertContains(t, out, `[ -f`)
}

func TestCodegen_BuiltinIsDir(t *testing.T) {
	out := compile(t, `let p: string = "/tmp"
if (Besht.fs.isDir(p)) {
    $("echo", "dir").run()
}`)
	assertContains(t, out, `[ -d`)
}

func TestCodegen_BuiltinIsReadable(t *testing.T) {
	out := compile(t, `let p: string = "/tmp"
if (Besht.fs.isReadable(p)) {
    $("echo", "ok").run()
}`)
	assertContains(t, out, `[ -r`)
}

func TestCodegen_BuiltinIsWritable(t *testing.T) {
	out := compile(t, `let p: string = "/tmp"
if (Besht.fs.isWritable(p)) {
    $("echo", "ok").run()
}`)
	assertContains(t, out, `[ -w`)
}

func TestCodegen_BuiltinIsExecutable(t *testing.T) {
	out := compile(t, `let p: string = "/usr/bin/env"
if (Besht.fs.isExecutable(p)) {
    $("echo", "ok").run()
}`)
	assertContains(t, out, `[ -x`)
}

func TestCodegen_BuiltinIsEmpty(t *testing.T) {
	out := compile(t, `let s: string = ""
if (Besht.strings.isEmpty(s)) {
    $("echo", "empty").run()
}`)
	assertContains(t, out, `[ -z`)
}

func TestCodegen_BuiltinIsSet(t *testing.T) {
	out := compile(t, `let s: string = "x"
if (Besht.strings.isNonEmpty(s)) {
    $("echo", "set").run()
}`)
	assertContains(t, out, `[ -n`)
}

func TestCodegen_BeshtNamespaceWrappers(t *testing.T) {
	tests := []struct {
		group  string
		method string
		arg    string
		op     string
	}{
		{"fs", "isFile", "p", "-f"},
		{"fs", "isDir", "p", "-d"},
		{"fs", "isReadable", "p", "-r"},
		{"fs", "isWritable", "p", "-w"},
		{"fs", "isExecutable", "p", "-x"},
		{"strings", "isEmpty", "s", "-z"},
		{"strings", "isNonEmpty", "s", "-n"},
	}
	for _, tt := range tests {
		out := compile(t, `let p: string = "/tmp"
let s: string = ""
if (Besht.`+tt.group+`.`+tt.method+`(`+tt.arg+`)) {
    $("echo", "ok").run()
}`)
		assertContains(t, out, `[ `+tt.op)
		assertNotContains(t, out, `_bst_starts_with()`)
		assertNotContains(t, out, `_bst_ends_with()`)
		assertNotContains(t, out, `_bst_includes()`)
	}
}

func TestCodegen_BeshtNamespaceValuePosition(t *testing.T) {
	out := compile(t, `let p: string = "/tmp"
let ok: boolean = Besht.fs.isFile(p)
if (ok) {
    $("echo", "ok").run()
}`)
	assertContains(t, out, `ok=$(if [ -f $p ]; then printf 1; else printf 0; fi)`)
	assertContains(t, out, `[ "$ok" = 1 ]`)
}

func TestCodegen_BuiltinLen(t *testing.T) {
	out := compile(t, `let files: list<string> = ["a"]
let n: number = files.length`)
	assertContains(t, out, `wc -l`)
}

func TestCodegen_BuiltinHead(t *testing.T) {
	out := compile(t, `let files: list<string> = ["a", "b"]
let first: string = files[0]`)
	assertContains(t, out, `first='a'`)
	assertNotContains(t, out, `sed -n "$(( 0 + 1 ))p"`)
}

func TestCodegen_BuiltinTail(t *testing.T) {
	out := compile(t, `let files: list<string> = ["a", "b"]
let rest: list<string> = files.slice(1)`)
	assertContains(t, out, `tail -n +$(( 1 + 1 ))`)
}

func TestCodegen_BuiltinAppend(t *testing.T) {
	out := compile(t, `let files: list<string> = ["a"]
files = files.push("b")`)
	assertContains(t, out, `printf`)
}

func TestCodegen_BuiltinContainsCondition(t *testing.T) {
	out := compile(t, `let files: list<string> = ["a"]
if (files.includes("a")) {
    $("echo", "found").run()
}`)
	assertContains(t, out, `grep -qxF`)
}

func TestCodegen_ExitZero(t *testing.T) {
	out := compile(t, `process.exit(0)`)
	assertContains(t, out, `exit 0`)
}

func TestCodegen_ExitCode(t *testing.T) {
	out := compile(t, `let code: number = 1
process.exit(code)`)
	assertContains(t, out, `exit`)
	assertContains(t, out, `code`)
}

func TestCodegen_ArithmeticAdd(t *testing.T) {
	out := compile(t, `let a: number = 3
let b: number = 4
let c: number = a + b`)
	assertContains(t, out, `$(( $a + $b ))`)
}

func TestCodegen_ArithmeticSub(t *testing.T) {
	out := compile(t, `let a: number = 10
let b: number = 3
let c: number = a - b`)
	assertContains(t, out, `$(( $a - $b ))`)
}

func TestCodegen_ArithmeticMul(t *testing.T) {
	out := compile(t, `let a: number = 3
let b: number = 4
let c: number = a * b`)
	assertContains(t, out, `$(( $a * $b ))`)
}

func TestCodegen_ArithmeticDiv(t *testing.T) {
	out := compile(t, `let a: number = 10
let b: number = 2
let c: number = a / b`)
	assertContains(t, out, `awk -v _a=`)
}

func TestCodegen_ArithmeticMod(t *testing.T) {
	out := compile(t, `let a: number = 10
let b: number = 3
let c: number = a % b`)
	assertContains(t, out, `$(( $a % $b ))`)
}

func TestCodegen_PipeMethod(t *testing.T) {
	out := compile(t, `let r = $("whoami").pipe($("tr", "-d", "\n")).run().readStdout()`)
	assertContains(t, out, `whoami`)
	assertContains(t, out, `|`)
	assertContains(t, out, `tr`)
}

func TestCodegen_PropagateExprInFunction(t *testing.T) {
	out := compile(t, `function f(): string {
    let x: string = $("cat", "file")?
    return x
}`)
	assertContains(t, out, `|| return $?`)
}

func TestCodegen_FnParamNameMangledInBody(t *testing.T) {
	out := compile(t, "function greet(name: string) {\n    $(\"echo\", `${name}`).run()\n}")
	assertContains(t, out, `_greet_name="$1"`)
	assertContains(t, out, `${_greet_name}`)
}

func TestCodegen_FnLocalVarMangled(t *testing.T) {
	out := compile(t, `function f(x: number): number {
    let result: number = x
    return result
}`)
	assertContains(t, out, `_f_x="$1"`)
	assertContains(t, out, `_f_result=`)
}

func TestCodegen_FnReturnStringViaPrintf(t *testing.T) {
	out := compile(t, `function greet(name: string): string {
    return "Hello"
}`)
	assertContains(t, out, `printf '%s'`)
}

func TestCodegen_ComparisonLt(t *testing.T) {
	out := compile(t, `let n: number = 5
if (n < 10) {
    $("echo", "small").run()
}`)
	assertContains(t, out, `exit !(_a < _b)`)
}

func TestCodegen_ComparisonGte(t *testing.T) {
	out := compile(t, `let n: number = 5
if (n >= 5) {
    $("echo", "ok").run()
}`)
	assertContains(t, out, `exit !(_a >= _b)`)
}

func TestCodegen_ComparisonLte(t *testing.T) {
	out := compile(t, `let n: number = 5
if (n <= 10) {
    $("echo", "ok").run()
}`)
	assertContains(t, out, `exit !(_a <= _b)`)
}

func TestCodegen_ComparisonNeqInt(t *testing.T) {
	out := compile(t, `let a: number = 1
let b: number = 2
if (a != b) {
    $("echo", "diff").run()
}`)
	assertContains(t, out, `_bst_left="$a"`)
	assertContains(t, out, `_bst_right="$b"`)
	assertContains(t, out, `[ "$_bst_left" != "$_bst_right" ]`)
}

func TestCodegen_CmdWithVarArg(t *testing.T) {
	out := compile(t, `let path: string = "/tmp"
$("ls", path).run()`)
	assertContains(t, out, `ls`)
	assertContains(t, out, `$path`)
}

func TestCodegen_IntLiteralInShellArith(t *testing.T) {
	out := compile(t, `let n: number = 5
let m: number = n + 1`)
	assertContains(t, out, `$(( $n + 1 ))`)
}

func TestCodegen_StringConcatMethod(t *testing.T) {
	out := compile(t, `let a: string = "hello"
let b: string = " world"
let c: string = a + b`)
	assertContains(t, out, `"${a}${b}"`)
}

func TestCodegen_TemplateLitConcatWithVar(t *testing.T) {
	out := compile(t, "let name: string = \"Alice\"\nlet greeting: string = `Hello, ` + name")
	assertContains(t, out, `"Hello, ${name}"`)
}

func TestCodegen_BreakInLoop(t *testing.T) {
	out := compile(t, `let n: number = 5
while (n > 0) {
    break
}`)
	assertContains(t, out, `break`)
}

func TestCodegen_ContinueInLoop(t *testing.T) {
	out := compile(t, `let files: list<string> = ["a"]
for (f in files) {
    continue
}`)
	assertContains(t, out, `continue`)
}

func TestCodegen_ProcessEnvUsesUnsetOnlyNullishSentinel(t *testing.T) {
	out := compile(t, `let home = process.env.HOME ?? "fallback"`)
	assertContains(t, out, `${HOME+x}`)
	assertContains(t, out, `_BESHT_NULLISH_SENTINEL`)
	assertNotContains(t, out, `${HOME:-`)
}

func TestCodegen_ProcessEnvConditionIsNullishAware(t *testing.T) {
	out := compile(t, `if (process.env.HOME) {
    console.log("set")
}`)
	assertContains(t, out, `${HOME+x}`)
	assertContains(t, out, `[ "$_bst_cond" != "$_BESHT_NULLISH_SENTINEL" ]`)
}

func TestCodegen_ProcessExit(t *testing.T) {
	out := compile(t, `process.exit()
process.exit(7)`)
	assertContains(t, out, `exit 0`)
	assertContains(t, out, `exit 7`)
}

func TestCodegen_ProcessEnvDirect(t *testing.T) {
	out := compile(t, `let home: string = process.env.HOME`)
	assertContains(t, out, `${HOME+x}`)
}

func TestCodegen_ProcessEnvWithDefault(t *testing.T) {
	out := compile(t, `let port: string = process.env.PORT ?? "8080"`)
	assertContains(t, out, `${PORT+x}`)
	assertContains(t, out, `8080`)
}

func TestCodegen_FetchDirectText(t *testing.T) {
	out := compile(t, `let url: string = "file:///tmp/data.txt"
let body: string = fetch(url).text()`)
	assertContains(t, out, `body=$(curl -sS -- "$url")`)
	assertNotContains(t, out, `local `)
	assertNotContains(t, out, `[[`)
}

func TestCodegen_FetchAssignedResponseRunsOnceAndReusesBody(t *testing.T) {
	out := compile(t, `let url: string = "file:///tmp/data.txt"
let response = fetch(url)
let first: string = response.text()
let second: string = response.text()`)
	assertContains(t, out, `response='response'`)
	assertContains(t, out, `_obj_response_body=$(curl -sS -- "$url")`)
	assertContains(t, out, `first="$_obj_response_body"`)
	assertContains(t, out, `second="$_obj_response_body"`)
	if strings.Count(out, `curl -sS --`) != 1 {
		t.Fatalf("assigned fetch should run curl once, output:\n%s", out)
	}
}

func TestCodegen_FetchResponseAliasCopiesBody(t *testing.T) {
	out := compile(t, `let url: string = "file:///tmp/data.txt"
let response = fetch(url)
let alias = response
let fromAlias: string = alias.text()`)
	assertContains(t, out, `_obj_alias_body="$_obj_response_body"`)
	assertContains(t, out, `fromAlias="$_obj_alias_body"`)
}

func TestCodegen_FetchResponseReassignmentRefreshesBody(t *testing.T) {
	out := compile(t, `let one: string = "file:///tmp/one.txt"
let two: string = "file:///tmp/two.txt"
let response = fetch(one)
response = fetch(two)
let body: string = response.text()`)
	assertContains(t, out, `_obj_response_body=$(curl -sS -- "$one")`)
	assertContains(t, out, `_obj_response_body=$(curl -sS -- "$two")`)
	assertContains(t, out, `body="$_obj_response_body"`)
}

func TestCodegen_FetchQuotesCommandSubstitutionURL(t *testing.T) {
	out := compile(t, `function makeUrl(): string { return "file:///tmp/data file.txt" }
let body: string = fetch(makeUrl()).text()`)
	assertContains(t, out, `curl -sS -- "$(makeUrl)"`)
}

func TestCodegen_PrintBuiltin(t *testing.T) {
	out := compile(t, `let msg: string = "hello"
console.log(msg)`)
	assertContains(t, out, `printf '%s\n'`)
}

func TestCodegen_StaticBooleanConsoleArgs(t *testing.T) {
	out := compile(t, `console.log(Boolean(""))
console.log(Boolean("x"))
console.log(!false)
console.error(true && false)`)
	assertContains(t, out, `printf '%s\n' false`)
	assertContains(t, out, `printf '%s\n' true`)
	assertContains(t, out, `printf '%s\n' false >&2`)
	assertNotContains(t, out, `$(if [ 0 = 1 ]; then printf true; else printf false; fi)`)
	assertNotContains(t, out, `$(if [ 1 = 1 ]; then printf true; else printf false; fi)`)
}

func TestCodegen_IndexExpr(t *testing.T) {
	out := compile(t, `let files: list<string> = ["a", "b"]
let first: string = files[0]`)
	assertContains(t, out, `first='a'`)
	assertNotContains(t, out, `sed -n "$(( 0 + 1 ))p"`)
}

func TestCodegen_IndexExprVariable(t *testing.T) {
	out := compile(t, `let files: list<string> = ["a", "b"]
let i: number = 1
let item: string = files[i]`)
	assertContains(t, out, `$(( $i + 1 ))`)
}

func TestCodegen_StaticListLiteralIndexExpr(t *testing.T) {
	out := compile(t, `let first: string = ["a", "b"][0]
let fromFactory: string = Array.of("x", "y")[1]
let fromSplit: string = "a,b".split(",")[1]`)
	assertContains(t, out, `first='a'`)
	assertContains(t, out, `fromFactory='y'`)
	assertContains(t, out, `fromSplit='b'`)
	assertNotContains(t, out, `sed -n "$(( 0 + 1 ))p"`)
	assertNotContains(t, out, `sed -n "$(( 1 + 1 ))p"`)
}

func TestCodegen_StaticListOutOfRangeIndexKeepsRuntimePath(t *testing.T) {
	out := compile(t, `let missing: string = ["a", "b"][3]`)
	assertContains(t, out, `sed -n "$(( 3 + 1 ))p"`)
}

func TestCodegen_StaticListIndexSkipsControlFlowAssignedVars(t *testing.T) {
	out := compile(t, `let currentPos = [0, 0]
while (true) {
    let row = currentPos[0]
    currentPos = [row + 1, currentPos[1]]
    break
}`)
	assertContains(t, out, `row=$(printf '%s\n' "$currentPos" | sed -n "$(( 0 + 1 ))p")`)
	assertNotContains(t, out, `row='0'`)
}

func TestCodegen_ConstDecl(t *testing.T) {
	out := compile(t, `const threshold: number = 90`)
	assertContains(t, out, `threshold=90`)
}

func TestCodegen_NoCommentsInOutput(t *testing.T) {
	out := compile(t, `let x: string = "hello"`)
	lines := strings.Split(out, "\n")
	for _, line := range lines[2:] {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			trimmed := strings.TrimSpace(line)
			if !strings.HasPrefix(trimmed, "# besht:") {
				t.Errorf("unexpected comment in output: %q", line)
			}
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func TestCodegen_NumberToStringConversion(t *testing.T) {
	out := compile(t, `let n: number = 42
let s: string = n.toString()`)
	assertContains(t, out, `s=`)
	assertContains(t, out, `n`)
}

func TestCodegen_IntBuiltin(t *testing.T) {
	out := compile(t, `let s: string = "42"
let n: number = Number.parseInt(s)`)
	assertContains(t, out, `$(( $s + 0 ))`)
}

func TestCodegen_NumberParseIntOneAndTwoArgs(t *testing.T) {
	out := compile(t, `let a: number = Number.parseInt("42")
let b: number = Number.parseInt("42", 10)`)
	assertContains(t, out, `a=42`)
	assertContains(t, out, `b=42`)
	assertNotContains(t, out, `$(( 42 + 0 ))`)
}

func TestCodegen_NumberParseIntStaticPrefix(t *testing.T) {
	out := compile(t, `let a: number = Number.parseInt("2a", 10)
let b: number = Number.parseInt("ff", 16)
let c: number = Number.parseInt("-10px", 10)`)
	assertContains(t, out, `a=2`)
	assertContains(t, out, `b=255`)
	assertContains(t, out, `c=-10`)
	assertNotContains(t, out, `$(( 2a + 0 ))`)
}

func TestCodegen_ListConcatMethod(t *testing.T) {
	out := compile(t, `let a: list<string> = ["x"]
let b: list<string> = ["y"]
let c: list<string> = a.concat(b)`)
	assertContains(t, out, `printf '%s\n%s'`)
}

func TestCodegen_StringTrim(t *testing.T) {
	out := compile(t, `let s: string = "  hi  "
let t: string = s.trim()`)
	assertContains(t, out, `sed 's/^[[:space:]]`)
}

func TestCodegen_StringToUpperCase(t *testing.T) {
	out := compile(t, `let s: string = "hello"
let u: string = s.toUpperCase()`)
	assertContains(t, out, `tr '[:lower:]' '[:upper:]'`)
}

func TestCodegen_StringToLowerCase(t *testing.T) {
	out := compile(t, `let s: string = "HELLO"
let l: string = s.toLowerCase()`)
	assertContains(t, out, `tr '[:upper:]' '[:lower:]'`)
}

func TestCodegen_StringSplit(t *testing.T) {
	out := compile(t, `let s: string = "a,b,c"
let parts: list<string> = s.split(",")`)
	assertContains(t, out, `tr ',' '\n'`)
}

func TestCodegen_StringSplitEmptySeparator(t *testing.T) {
	out := compile(t, `let s: string = "abc"
let parts: list<string> = s.split("")`)
	assertContains(t, out, `for(i=1;i<=length($0);i++) print substr($0,i,1)`)
}

func TestCodegen_StaticStringSplit(t *testing.T) {
	out := compile(t, `let parts = "a,b,c".split(",")
let chars = "abc".split("")
let count = "a,b,c".split(",").length`)
	assertContains(t, out, "parts='a\nb\nc'")
	assertContains(t, out, "chars='a\nb\nc'")
	assertContains(t, out, `count=3`)
	assertNotContains(t, out, `tr ',' '\n'`)
	assertNotContains(t, out, `for(i=1;i<=length($0);i++)`)
}

func TestCodegen_StaticStringSplitForLoop(t *testing.T) {
	out := compile(t, `for (part of "a,b,c".split(",")) {
    console.log(part)
}`)
	assertContains(t, out, `for part in 'a' 'b' 'c'; do`)
	assertNotContains(t, out, `tr ',' '\n'`)
}

func TestCodegen_SetHasAdd(t *testing.T) {
	out := compile(t, `const seen = new Set<string>()
seen.add("a")
if (seen.has("a")) { console.log("yes") }`)
	assertContains(t, out, `seen=""`)
	assertContains(t, out, `awk '!seen[$0]++'`)
	assertContains(t, out, `grep -qxF -- 'a'`)
	assertNotContains(t, out, `local `)
	assertNotContains(t, out, `[[`)
}

func TestCodegen_NestedListMapAndIndex(t *testing.T) {
	out := compile(t, `type Factory = string[]
function first(factory: Factory): string {
    const matrix: string[][] = factory.map(e => e.split("") as string[])
    return matrix[0][1]
}`)
	assertContains(t, out, `printf "\037"`)
	assertContains(t, out, `tr '\037' '\n'`)
}

func TestCodegen_StringRuntimeHelpersOmittedWhenUnused(t *testing.T) {
	out := compile(t, `class MathUtils {
    static PI: number = 3.14159
    static round(n: number): number { return Math.round(n) }
}
console.log(MathUtils.PI)
console.log(MathUtils.round(2.7))`)
	assertNotContains(t, out, `_bst_starts_with()`)
	assertNotContains(t, out, `_bst_ends_with()`)
	assertNotContains(t, out, `_bst_includes()`)
}

func TestCodegen_OneArgStringRuntimeHelpersEmittedWhenUsed(t *testing.T) {
	out := compile(t, `let s: string = "hello"
let has = s.includes("ell")
let sw = s.startsWith("he")
let ew = s.endsWith("lo")`)
	assertContains(t, out, `_bst_includes()`)
	assertContains(t, out, `_bst_includes "$s" 'ell'`)
	assertContains(t, out, `_bst_starts_with()`)
	assertContains(t, out, `_bst_starts_with "$s" 'he'`)
	assertContains(t, out, `_bst_ends_with()`)
	assertContains(t, out, `_bst_ends_with "$s" 'lo'`)
}

func TestCodegen_StringStartsWithCondition(t *testing.T) {
	out := compile(t, `let s: string = "hello"
if (s.startsWith("hel")) {
    $("echo", "yes").run()
}`)
	assertContains(t, out, `_bst_starts_with`)
	assertContains(t, out, `'hel'`)
}

func TestCodegen_StringEndsWithCondition(t *testing.T) {
	out := compile(t, `let s: string = "hello"
if (s.endsWith("llo")) {
    $("echo", "yes").run()
}`)
	assertContains(t, out, `_bst_ends_with`)
	assertContains(t, out, `'llo'`)
}

func TestCodegen_StringReplace(t *testing.T) {
	out := compile(t, `let s: string = "hello world"
let r: string = s.replace("world", "besht")`)
	assertContains(t, out, `sed "s/world/besht/"`)

}

func TestCodegen_StringReplaceAll(t *testing.T) {
	out := compile(t, `let s: string = "aaa"
let r: string = s.replaceAll("a", "b")`)
	assertContains(t, out, `sed "s/a/b/g"`)

}

func TestCodegen_StaticStringTransforms(t *testing.T) {
	out := compile(t, `let trimmed: string = "  hi  ".trim()
let upper: string = "hello".toUpperCase()
let lower: string = "HELLO".toLowerCase()
let sliced: string = "hello".slice(1, 4)
let sub: string = "hello".substring(4, 1)
let repeated: string = "ha".repeat(3)
let padded: string = "hi".padStart(5, "0")
let ended: string = "hi".padEnd(5, ".")`)
	assertContains(t, out, `trimmed='hi'`)
	assertContains(t, out, `upper='HELLO'`)
	assertContains(t, out, `lower='hello'`)
	assertContains(t, out, `sliced='ell'`)
	assertContains(t, out, `sub='ell'`)
	assertContains(t, out, `repeated='hahaha'`)
	assertContains(t, out, `padded='000hi'`)
	assertContains(t, out, `ended='hi...'`)
	assertNotContains(t, out, `sed 's/^[[:space:]]`)
	assertNotContains(t, out, `tr '[:lower:]'`)
	assertNotContains(t, out, `cut -c`)
	assertNotContains(t, out, `awk`)
}

func TestCodegen_StringLength(t *testing.T) {
	out := compile(t, `let s: string = "hello"
let n: number = s.length`)
	assertContains(t, out, `n=5`)
	assertNotContains(t, out, `wc -c`)
}

func TestCodegen_StringLengthFallsBackAfterControlFlowAssignment(t *testing.T) {
	out := compile(t, `let s: string = "hello"
while (true) {
    let n: number = s.length
    s = "hello!"
    break
}`)
	assertContains(t, out, `wc -c`)
	assertNotContains(t, out, `n=5`)
}

func TestCodegen_StaticStringLiteralLength(t *testing.T) {
	out := compile(t, `let n: number = "hello".length`)
	assertContains(t, out, `n=5`)
	assertNotContains(t, out, `wc -c`)
}

func TestCodegen_ListPush(t *testing.T) {
	out := compile(t, `let l: list<string> = ["a"]
let l2: list<string> = l.push("b")`)
	assertContains(t, out, `printf '%s\n%s'`)
}

func TestCodegen_ListJoin(t *testing.T) {
	out := compile(t, `let l: list<string> = ["a", "b", "c"]
let s: string = l.join(", ")`)
	assertContains(t, out, `NR>1{printf s}`)
}

func TestCodegen_StaticListLiteralJoin(t *testing.T) {
	out := compile(t, `let s: string = ["a", "b", "c"].join(",")`)
	assertContains(t, out, `s='a,b,c'`)
	assertNotContains(t, out, `awk -v s=','`)
	assertNotContains(t, out, `printf '%s\n' 'a'`)
}

func TestCodegen_StaticListLiteralToString(t *testing.T) {
	out := compile(t, `let s: string = ["a", "b", "c"].toString()`)
	assertContains(t, out, `s='a,b,c'`)
	assertNotContains(t, out, `awk -v s=','`)
}

func TestCodegen_ListToString(t *testing.T) {
	out := compile(t, `let l: list<string> = ["a", "b", "c"]
let s: string = l.toString()`)
	assertContains(t, out, `awk -v s=','`)
	assertContains(t, out, `NR>1{printf s}`)
	assertNotContains(t, out, `_bst_includes()`)
	assertNotContains(t, out, `local `)
	assertNotContains(t, out, `[[`)
}

func TestCodegen_ListLength(t *testing.T) {
	out := compile(t, `let l: list<string> = ["a", "b"]
let n: number = l.length`)
	assertContains(t, out, `wc -l`)
}

func TestCodegen_StaticListLiteralLength(t *testing.T) {
	out := compile(t, `let n: number = ["a", "b", "c"].length`)
	assertContains(t, out, `n=3`)
	assertNotContains(t, out, `wc -l`)
}

func TestCodegen_ListReverse(t *testing.T) {
	out := compile(t, `let l: list<string> = ["c", "b", "a"]
let r: list<string> = l.reverse()`)
	assertContains(t, out, `tail -r`)
}

func TestCodegen_ListConcat(t *testing.T) {
	out := compile(t, `let a: list<string> = ["x"]
let b: list<string> = ["y"]
let c: list<string> = a.concat(b)`)
	assertContains(t, out, `printf '%s\n%s'`)
}

func TestCodegen_ListSlice(t *testing.T) {
	out := compile(t, `let l: list<string> = ["a", "b", "c", "d"]
let s: list<string> = l.slice(1, 3)`)
	assertContains(t, out, `awk -v _s=1 -v _e=3`)
}

func TestCodegen_ListIncludesCondition(t *testing.T) {
	out := compile(t, `let l: list<string> = ["a", "b"]
if (l.includes("a")) { $("echo", "yes") }`)
	assertContains(t, out, `grep -qxF`)
	assertNotContains(t, out, `_bst_includes()`)
}

func TestCodegen_StaticListLiteralIncludes(t *testing.T) {
	out := compile(t, `let found: boolean = ["a", "b"].includes("b")`)
	assertContains(t, out, `found=1`)
	assertNotContains(t, out, `grep -qxF`)
}

func TestCodegen_StaticListLiteralIndexOf(t *testing.T) {
	out := compile(t, `let first: number = ["a", "b", "a"].indexOf("a")
let last: number = ["a", "b", "a"].lastIndexOf("a")
let missing: number = ["a", "b", "a"].indexOf("z")`)
	assertContains(t, out, `first=0`)
	assertContains(t, out, `last=2`)
	assertContains(t, out, `missing=-1`)
	assertNotContains(t, out, `awk -v _needle`)
}

func TestCodegen_NativeListAPIsReplaceGlobalListHelpers(t *testing.T) {
	out := compile(t, `let files: list<string> = ["a", "b", "c"]
let other: list<string> = ["d"]
let count: number = files.length
let first: string = files[0]
let rest: list<string> = files.slice(1)
let appended: list<string> = files.push("x")
if (files.includes("x")) { $("echo", "found").run() }
let combined: list<string> = files.concat(other)`)
	assertContains(t, out, `wc -l`)
	assertContains(t, out, `first='a'`)
	assertContains(t, out, `tail -n +$(( 1 + 1 ))`)
	assertContains(t, out, `printf '%s\n%s'`)
	assertContains(t, out, `grep -qxF`)
	assertNotContains(t, out, `head -n1`)
	assertNotContains(t, out, `tail -n +2`)
	assertNotContains(t, out, `_bst_includes()`)
}

func TestCodegen_ListMapArrow(t *testing.T) {
	out := compile(t, `let items = ["a", "b"]
let mapped = items.map(x => x + "!")`)
	assertContains(t, out, `while IFS= read -r _cb_2_24_x`)
	assertContains(t, out, `printf '%s\n' "${_cb_2_24_x}!"`)
	assertNotContains(t, out, `local `)
}

func TestCodegen_ListFilterArrow(t *testing.T) {
	out := compile(t, `let items = ["a", "b"]
let picked = items.filter(x => x.startsWith("a"))`)
	assertContains(t, out, `_bst_starts_with "$_cb_2_27_x" 'a'`)
	assertContains(t, out, `printf '%s\n' "$_cb_2_27_x"`)
}

func TestCodegen_ListSomeEveryFindArrows(t *testing.T) {
	out := compile(t, `let items = ["a", "b"]
let hasB = items.some((x, i) => x == "b" && i == 1)
let allNamed = items.every(x => x != "")
let found = items.find(x => x == "b")`)
	assertContains(t, out, `_listpred_2_23_result=0`)
	assertContains(t, out, `_listpred_3_28_result=1`)
	assertContains(t, out, `break`)
	assertContains(t, out, `_listfind_4_24_result=$_BESHT_NULLISH_SENTINEL`)
	assertContains(t, out, `_BESHT_NULLISH_SENTINEL`)
	assertNotContains(t, out, `local `)
	assertNotContains(t, out, `[[`)
}

func TestCodegen_ListReduceExpressionBodyCanBeJoined(t *testing.T) {
	out := compile(t, `let nums = [0, 2]
let lines = nums.reduce((acc, n) => ([...acc, "#".repeat(n)]), [] as string[]).join("\n")`)
	assertContains(t, out, `lines_reduce=`)
	assertContains(t, out, `if [ -n "$lines_reduce" ]; then printf '%s\n' "$lines_reduce"; fi`)
	assertContains(t, out, `awk -v _s='#'`)
	assertContains(t, out, `NR>1{printf "\n"}`)
}

func TestCodegen_CallbackReceiversAreQuoted(t *testing.T) {
	out := compile(t, `let items = ["red apple", "green pear"]
let mapped = items.map(x => x + "!")
let picked = items.filter(x => x.includes(" "))
let joined = items.reduce((acc, x) => [...acc, x], [] as string[]).join("|")`)
	assertContains(t, out, `printf '%s\n' "$items" | while IFS= read -r _cb_2_24_x`)
	assertContains(t, out, `printf '%s\n' "$items" | while IFS= read -r _cb_3_27_x`)
	assertContains(t, out, `$(printf '%s\n' "$items")`)
}

func TestCodegen_ComputedObjectKeysAreValidated(t *testing.T) {
	out := compile(t, `let counts = {}
let key = "apple"
counts[key] = 1
console.log(counts[key])`)
	assertContains(t, out, `if [ -z "$_objk_3_1" ]`)
	assertContains(t, out, `eval "_obj_counts_${_objk_3_1}=\"\$_objv_3_1\""`)
	assertContains(t, out, `if [ -z "$_objk_4_19" ]`)
}

func TestCodegen_ConsoleLogList(t *testing.T) {
	out := compile(t, `let nums = [0, 1]
console.log(nums)`)
	assertContains(t, out, `printf '[ '`)
	assertContains(t, out, `awk 'BEGIN{first=1}`)
	assertContains(t, out, `printf ' ]\n'`)
}

func TestCodegen_SetHasAndAdd(t *testing.T) {
	out := compile(t, `const seen = new Set<string>()
seen.add("a")
let found: boolean = seen.has("a")`)
	assertContains(t, out, `seen=""`)
	assertContains(t, out, `awk '!seen[$0]++'`)
	assertContains(t, out, `grep -qxF -- 'a'`)
	assertNotContains(t, out, `local `)
	assertNotContains(t, out, `[[`)
}

func TestCodegen_SetHasDashPrefixedValueUsesGrepDelimiter(t *testing.T) {
	out := compile(t, `const seen = new Set<string>()
seen.add("-f/etc/passwd")
if (seen.has("-f/etc/passwd")) { console.log("yes") }`)
	assertContains(t, out, `grep -qxF -- '-f/etc/passwd'`)
}

func TestCodegen_SetAddExpressionRejected(t *testing.T) {
	prog, err := parser.Parse(`const seen = new Set<string>()
let x = seen.add("a")`, "test.bsh")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	_, err = codegen.Generate(prog)
	if err == nil || !strings.Contains(err.Error(), "Set.add() is only supported as a statement") {
		t.Fatalf("expected Set.add expression error, got %v", err)
	}
}

func TestCodegen_NestedListMapAndIndexing(t *testing.T) {
	out := compile(t, "let factory: string[] = [\"ab\", \"cd\"]\n"+
		"let matrix: string[][] = factory.map(e => e.split(\"\") as string[])\n"+
		"let row: string[] = matrix[1]\n"+
		"let cell: string = matrix[1][0]\n"+
		"let width: number = matrix[0].length\n"+
		"let key: string = `${matrix[0][0]},${matrix[1][1]}`")
	assertContains(t, out, `awk 'NR>1{printf "\037"}{printf "%s",$0}'`)
	assertContains(t, out, `tr '\037' '\n'`)
	assertContains(t, out, `sed -n "$(( 0 + 1 ))p"`)
	assertNotContains(t, out, `local `)
	assertNotContains(t, out, `[[`)
}

func TestCodegen_NestedListLiteralRowsArePacked(t *testing.T) {
	out := compile(t, `let matrix: string[][] = [["a", "b"], ["c", "d"]]
let cell: string = matrix[1][0]
let width: number = matrix[0].length`)
	assertContains(t, out, `printf "\037"`)
	assertContains(t, out, `tr '\037' '\n'`)
}

func TestCodegen_ClassBasic(t *testing.T) {
	out := compile(t, `class User {
    name: string
    age: number

    constructor(name: string, age: number) {
        this.name = name
        this.age = age
    }

    greet(): string {
        return "Hello, " + this.name
    }

    isAdult(): boolean {
        return this.age >= 18
    }
}

let u = new User("Alice", 30)
console.log(u.greet())
console.log(u.name)
u.name = "Bob"
console.log(u.name)
console.log(u.isAdult())`)
	assertContains(t, out, `User__constructor()`)
	assertContains(t, out, `u='u'`)
	assertContains(t, out, `User__constructor "$u" 'Alice' 30`)
	assertContains(t, out, `User__greet "$u"`)
	assertContains(t, out, `_obj_u_name='Bob'`)
}

func TestCodegen_ClassStatic(t *testing.T) {
	out := compile(t, `class MathUtils {
    static PI: number = 3.14159
    static round(n: number): number {
        return Math.round(n)
    }
}

console.log(MathUtils.PI)
console.log(MathUtils.round(2.7))`)
	assertContains(t, out, `_class_MathUtils_PI=3.14159`)
	assertContains(t, out, `MathUtils__round()`)
	assertContains(t, out, `MathUtils__round 2.7`)
}

// ── $() codegen tests ─────────────────────────────────────────────────────────

func TestCodegen_CmdBasicCapture(t *testing.T) {
	out := compile(t, `let u = $("whoami").run().readStdout()`)
	assertContains(t, out, `u=$(whoami)`)
}

func TestCodegen_InlineRunReadStdoutExpressionUsesCommandSubstitution(t *testing.T) {
	out := compile(t, `console.log($("printf", "inline").run().readStdout())`)
	assertContains(t, out, `printf '%s\n' "$(printf inline)"`)
	assertNotContains(t, out, `$_cmd`)
}

func TestCodegen_NamedInlineRunReadStdoutExpressionKeepsCapture(t *testing.T) {
	out := compile(t, `let cmd = $("printf", "named")
console.log(cmd.run().readStdout())
console.log(cmd.readStdout())`)
	assertContains(t, out, `cmd=$(printf named)`)
	assertContains(t, out, `printf '%s\n' "$cmd"`)
}

func TestCodegen_InlineRunExitCodeLetUsesExitVar(t *testing.T) {
	out := compile(t, `let code = $("false").run().exitCode()
console.log(code)`)
	assertContains(t, out, `false`)
	assertContains(t, out, `code_exit=$?`)
	assertContains(t, out, `code="$code_exit"`)
	assertContains(t, out, `printf '%s\n' "$code"`)
}

func TestCodegen_CmdMultiArgCapture(t *testing.T) {
	out := compile(t, `let b = $("git", "rev-parse", "--abbrev-ref", "HEAD").run().readStdout()`)
	assertContains(t, out, `git`)
	assertContains(t, out, `rev-parse`)
	assertContains(t, out, `--abbrev-ref`)
	assertContains(t, out, `HEAD`)
}

func TestCodegen_CmdSafeLiteralArgsUseBareWords(t *testing.T) {
	out := compile(t, `$("git", "status", "--short").run()
$("find", ".", "-name", "*.go").run()
$("sed", r"s/foo/bar/").run()
$("if", "value").run()`)
	assertContains(t, out, `git status --short`)
	assertContains(t, out, `find . -name '*.go'`)
	assertContains(t, out, `sed 's/foo/bar/'`)
	assertContains(t, out, `'if' value`)
	assertNotContains(t, out, `'git'`)
	assertNotContains(t, out, `'--short'`)
}

func TestCodegen_CmdVarArgPassedRaw(t *testing.T) {
	out := compile(t, `let path = "/tmp"
let r = $("ls", path).run().readStdout()`)
	assertContains(t, out, `$path`)
	assertNotContains(t, out, `'$path'`)
}

func TestCodegen_CmdRunSideEffect(t *testing.T) {
	out := compile(t, `$("chmod", "+x", "script.sh").run()`)
	assertContains(t, out, `chmod`)
	assertContains(t, out, `+x`)
	assertContains(t, out, `script.sh`)
	assertNotContains(t, out, `$(chmod`)
}

func TestCodegen_CmdPipe(t *testing.T) {
	out := compile(t, `let r = $("cat", "/etc/passwd").pipe($("grep", "root")).run().readStdout()`)
	assertContains(t, out, `cat`)
	assertContains(t, out, `|`)
	assertContains(t, out, `grep`)
}

func TestCodegen_CmdPipeChain(t *testing.T) {
	out := compile(t, `let r = $("cat", "/etc/passwd")
    .pipe($("grep", "root"))
    .pipe($("cut", "-d:", "-f1")).run().readStdout()`)
	assertContains(t, out, `cat`)
	assertContains(t, out, `grep`)
	assertContains(t, out, `cut`)
	n := len(strings.Split(out, "|"))
	if n < 3 {
		t.Errorf("expected 2 pipes in output, got %d", n-1)
	}
}

func TestCodegen_CmdStdoutRedirect(t *testing.T) {
	out := compile(t, `$("make", "build").stdout("/tmp/build.log").run()`)
	assertContains(t, out, `make build > /tmp/build.log`)
	assertContains(t, out, `> /tmp/build.log`)
	assertNotContains(t, out, `{ make build; } > /tmp/build.log`)
}

func TestCodegen_CmdStdoutAppend(t *testing.T) {
	out := compile(t, `$("echo", "line").stdout("/tmp/out.txt", "append").run()`)
	assertContains(t, out, `>> /tmp/out.txt`)
}

func TestCodegen_CmdStdoutNull(t *testing.T) {
	out := compile(t, `$("make", "build").stdout("null").run()`)
	assertContains(t, out, `> /dev/null`)
}

func TestCodegen_CmdStderrNull(t *testing.T) {
	out := compile(t, `$("make", "build").stderr("null").run()`)
	assertContains(t, out, `2>/dev/null`)
}

func TestCodegen_CmdStderrAnd1(t *testing.T) {
	out := compile(t, `$("make", "build").stderr("&1").run()`)
	assertContains(t, out, `2>&1`)
}

func TestCodegen_CmdStderrCapture(t *testing.T) {
	out := compile(t, `let errs = $("make").run().readStderr()`)
	assertContains(t, out, `2>&1 1>/dev/null`)
}

func TestCodegen_CmdStderrFile(t *testing.T) {
	out := compile(t, `$("make").stderr("/tmp/errs.log").run()`)
	assertContains(t, out, `2>/tmp/errs.log`)
}

func TestCodegen_CmdLines(t *testing.T) {
	out := compile(t, `let lsCmd = $("ls", "/tmp")
lsCmd.run()
let ls = lsCmd.readStdoutLines()`)
	assertContains(t, out, `ls`)
	assertContains(t, out, `/tmp`)
}

func TestCodegen_CmdText(t *testing.T) {
	out := compile(t, `let s = $("whoami").run().readStdout()`)
	assertContains(t, out, `whoami`)
}

func TestCodegen_CmdInFor(t *testing.T) {
	out := compile(t, `for (f in $("find", ".", "-name", "*.log").readStdoutLines()) {
    $("echo", f).run()
}`)
	assertContains(t, out, `find`)
	assertContains(t, out, `while IFS= read -r`)
	assertContains(t, out, `echo`)
}

func TestCodegen_CmdEnv(t *testing.T) {
	out := compile(t, `$("env").env("FOO", "bar").run()`)
	assertContains(t, out, `FOO='bar' env`)
}

func TestCodegen_CmdEnvWithVariable(t *testing.T) {
	out := compile(t, `let value: string = "bar"
$("env").env("FOO", value).run()`)
	assertContains(t, out, `FOO="$value" env`)
}

func TestCodegen_CmdEnvCapture(t *testing.T) {
	out := compile(t, `let scoped = $("sh", "-c", 'printf %s "$FOO"').env("FOO", "bar").run().readStdout()`)
	assertContains(t, out, `scoped=$(FOO='bar' sh`)
}

func TestCodegen_CmdEnvPipe(t *testing.T) {
	out := compile(t, `$("printenv", "FOO").env("FOO", "bar").pipe($("cat")).run()`)
	assertContains(t, out, `FOO='bar' printenv`)
	assertContains(t, out, `| cat`)
}

func TestCodegen_CmdWorkdir(t *testing.T) {
	out := compile(t, `$("ls").workdir("/").run()`)
	assertContains(t, out, `_bst_workdir='/'`)
	assertContains(t, out, `CDPATH= cd "$_bst_workdir" && ls`)
}

func TestCodegen_CmdWorkdirWithVariable(t *testing.T) {
	out := compile(t, `let dir: string = "/tmp"
$("pwd").workdir(dir).run()`)
	assertContains(t, out, `_bst_workdir="$dir"`)
	assertContains(t, out, `CDPATH= cd "$_bst_workdir" && pwd`)
}

func TestCodegen_CmdWorkdirCapture(t *testing.T) {
	out := compile(t, `let root = $("pwd").workdir("/").run().readStdout()`)
	assertContains(t, out, `root=$(_bst_workdir='/'`)
	assertContains(t, out, `CDPATH= cd "$_bst_workdir" && pwd)`)
}

func TestCodegen_CmdWorkdirPipe(t *testing.T) {
	out := compile(t, `$("printf", "hello").pipe($("grep", "hello")).workdir("/").run()`)
	assertContains(t, out, `CDPATH= cd "$_bst_workdir" && printf hello | grep hello)`)
}

func TestCodegen_CmdWorkdirRedirect(t *testing.T) {
	out := compile(t, `$("printf", "hello").stdout("out.txt").workdir("/").run()`)
	assertContains(t, out, `CDPATH= cd "$_bst_workdir" && printf hello > out.txt)`)
}

func TestCodegen_CmdWorkdirThenRedirect(t *testing.T) {
	out := compile(t, `$("printf", "hello").workdir("/").stdout("out.txt").run()`)
	assertContains(t, out, `CDPATH= cd "$_bst_workdir" && printf hello > out.txt)`)
}

func TestCodegen_CmdWorkdirThenEnv(t *testing.T) {
	out := compile(t, `$("printenv", "FOO").workdir("/").env("FOO", "bar").run()`)
	assertContains(t, out, `CDPATH= cd "$_bst_workdir" && FOO='bar' printenv FOO)`)
}

func TestCodegen_CmdWorkdirWithoutRunStaysLazy(t *testing.T) {
	out := compile(t, `$("ls").workdir("/")`)
	assertNotContains(t, out, `(cd '/' && ls)`)
}

func TestCodegen_CmdEnvWithoutRunStaysLazy(t *testing.T) {
	out := compile(t, `$("env").env("FOO", "bar")`)
	assertNotContains(t, out, `FOO='bar' env`)
}

func TestCodegen_CmdEnvRejectsInvalidName(t *testing.T) {
	prog, err := parser.Parse(`$("env").env("FOO;touch", "bar").run()`, "test.bsh")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	_, err = codegen.Generate(prog)
	if err == nil || !strings.Contains(err.Error(), `invalid command env name`) {
		t.Fatalf("expected invalid env name error, got %v", err)
	}
}

func TestCodegen_CmdPipeIntoStdout(t *testing.T) {
	out := compile(t, `$("cat", "input.txt").run()
    .pipe($("grep", "ERROR"))
    .stdout("/tmp/errors.log")
    `)
	assertContains(t, out, `{ cat input.txt | grep ERROR; } > /tmp/errors.log`)
	assertContains(t, out, `|`)
	assertContains(t, out, `grep`)
	assertContains(t, out, `> /tmp/errors.log`)
}

func TestCodegen_CmdStderrAndStdout(t *testing.T) {
	out := compile(t, `$("make", "all").stderr("&1").stdout("/tmp/build.log").run()`)
	assertContains(t, out, `make all 2>&1 > /tmp/build.log`)
	assertContains(t, out, `2>&1`)
	assertContains(t, out, `> /tmp/build.log`)
	assertNotContains(t, out, `{ make all; } 2>&1 > /tmp/build.log`)
}

func TestCodegen_CmdSingleQuoteInArg(t *testing.T) {
	out := compile(t, `$("echo", "it's alive").run()`)
	assertContains(t, out, `'"'"'`)
}

func TestCodegen_CmdVarStringConcat(t *testing.T) {
	out := compile(t, `let name: string = "world"
$("echo", "Hello " + name).run()`)
	assertContains(t, out, `echo`)
}

func TestCodegen_CmdAssignedToInt(t *testing.T) {
	out := compile(t, `let n = $("wc", "-l", "file.txt").run().readStdout()`)
	assertContains(t, out, `wc`)
	assertContains(t, out, `-l`)
}

func TestCodegen_CmdAssignedReceiverWorkdirPreserved(t *testing.T) {
	out := compile(t, `let cmd = $("pwd")
let root = cmd.workdir("/").run().readStdout()`)
	assertContains(t, out, `cmd=$(_bst_workdir='/'`)
	assertContains(t, out, `root="$cmd"`)
	assertNotContains(t, out, `cmd=$(pwd)`)
}

func TestCodegen_CmdIdentifierRootedLazyChainBinding(t *testing.T) {
	out := compile(t, `let base = $("pwd")
let rooted = base.workdir("/")
rooted.run()`)
	assertContains(t, out, `_bst_workdir='/'`)
	assertContains(t, out, `CDPATH= cd "$_bst_workdir" && pwd`)
}

func TestCodegen_CmdIdentifierRootedLazyChainCapture(t *testing.T) {
	out := compile(t, `let base = $("pwd")
let rooted = base.workdir("/")
let root = rooted.run().readStdout()`)
	assertContains(t, out, `base=$(_bst_workdir='/'`)
	assertContains(t, out, `root="$base"`)
}

func TestCodegen_CmdAssignedChainRunPreserved(t *testing.T) {
	out := compile(t, `let cmd = $("pwd").workdir("/")
cmd.run()`)
	assertContains(t, out, `_bst_workdir='/'`)
	assertContains(t, out, `CDPATH= cd "$_bst_workdir" && pwd`)
}

func TestCodegen_CmdReadStderrWithStdoutRedirectPreservesRedirect(t *testing.T) {
	out := compile(t, `let errs = $("sh", "-c", "printf out; printf err >&2").stdout("out.txt").run().readStderr()`)
	assertContains(t, out, `2>&1 1>/dev/null > out.txt`)
	assertNotContains(t, out, `> out.txt 2>&1 1>/dev/null`)
}

func TestCodegen_CmdRedirectHostileTargetsQuoted(t *testing.T) {
	out := compile(t, `$("make").stdout("out; touch /tmp/pwn").stderr("err; touch /tmp/pwn").run()`)
	assertContains(t, out, `> 'out; touch /tmp/pwn'`)
	assertContains(t, out, `2>'err; touch /tmp/pwn'`)
	assertNotContains(t, out, `> out; touch /tmp/pwn`)
	assertNotContains(t, out, `2>err; touch /tmp/pwn`)
}

func TestCodegen_CmdWorkdirFunctionPathIsQuotedCommandSubstitution(t *testing.T) {
	out := compile(t, `function getDir(): string { return "/" }
$("pwd").workdir(getDir()).run()`)
	assertContains(t, out, `_bst_workdir="$(getDir)"`)
	assertContains(t, out, `CDPATH= cd "$_bst_workdir" && pwd`)
}

func TestCodegen_CmdNestedWorkdirLastWinsAndNoMarkerLeaks(t *testing.T) {
	out := compile(t, `$("pwd").workdir("/tmp").workdir("/").run()`)
	assertContains(t, out, `_bst_workdir='/'`)
	assertContains(t, out, `CDPATH= cd "$_bst_workdir" && pwd`)
	assertNotContains(t, out, `__BESHT_WORKDIR__`)
	assertNotContains(t, out, `_bst_workdir='/tmp'`)
}

func TestCodegen_CmdWorkdirNewlinePathDoesNotLeakMarker(t *testing.T) {
	out := compile(t, `$("pwd").workdir("/tmp/line\nbreak").run()`)
	assertContains(t, out, `pwd`)
	assertNotContains(t, out, `__BESHT_WORKDIR__`)
}

func TestCodegen_CmdWorkdirDisablesCDPATHAndNormalizesHyphen(t *testing.T) {
	out := compile(t, `$("pwd").workdir("-dash").run()`)
	assertContains(t, out, `CDPATH= cd "$_bst_workdir"`)
	assertContains(t, out, `if [ "${_bst_workdir#-}" != "$_bst_workdir" ]`)
}

// ── String method codegen tests ────────────────────────────────────────────────

func TestCodegen_StringPadStart(t *testing.T) {
	out := compile(t, `let s: string = "hi"
let p: string = s.padStart(10, "-")`)
	assertContains(t, out, `printf`)
}

func TestCodegen_StringPadEnd(t *testing.T) {
	out := compile(t, `let s: string = "hi"
let p: string = s.padEnd(10, ".")`)
	assertContains(t, out, `printf`)
}

func TestCodegen_StringAt(t *testing.T) {
	out := compile(t, `let s: string = "hello"
let c: string = s.at(0)`)
	assertContains(t, out, `cut -c`)
}

func TestCodegen_StringSlice(t *testing.T) {
	out := compile(t, `let s: string = "hello world"
let sub: string = s.slice(0, 5)`)
	assertContains(t, out, `cut -c`)
}

func TestCodegen_StringConcatMethodNew(t *testing.T) {
	out := compile(t, `let a: string = "hello"
let b: string = " world"
let c: string = a.concat(b)`)
	assertContains(t, out, `"${a}${b}"`)
}

func TestCodegen_StringIndexOf(t *testing.T) {
	out := compile(t, `let s: string = "hello world"
let i: number = s.indexOf("world")`)
	assertContains(t, out, `awk`)
}

func TestCodegen_StringSearchOptionalArgsUseAwkArgs(t *testing.T) {
	out := compile(t, `let s: string = "hello hello"
let i: number = s.indexOf("lo", 4)
let j: number = s.lastIndexOf("lo", 7)
let has: boolean = s.includes("lo", 4)
let sw: boolean = s.startsWith("lo", 3)
let ew: boolean = s.endsWith("hel", 3)`)
	assertContains(t, out, `awk`)
	assertContains(t, out, `_pos`)
	assertContains(t, out, `_needle`)
	assertContains(t, out, `int(_pos)`)
	assertContains(t, out, `int(_len)`)
}

func TestCodegen_StaticStringSearchMethods(t *testing.T) {
	out := compile(t, `let has: boolean = "hello".includes("ell")
let sw: boolean = "hello".startsWith("he")
let ew: boolean = "hello".endsWith("lo")
let first: number = "hello".indexOf("l")
let last: number = "hello".lastIndexOf("l")
let pos: number = "hello hello".indexOf("lo", 4)`)
	assertContains(t, out, `has=1`)
	assertContains(t, out, `sw=1`)
	assertContains(t, out, `ew=1`)
	assertContains(t, out, `first=2`)
	assertContains(t, out, `last=3`)
	assertContains(t, out, `pos=9`)
	assertNotContains(t, out, `_bst_includes`)
	assertNotContains(t, out, `_bst_starts_with`)
	assertNotContains(t, out, `_bst_ends_with`)
	assertNotContains(t, out, `awk`)
}

func TestCodegen_StaticStringCharAt(t *testing.T) {
	out := compile(t, `let c: string = "hello".charAt(1)
let missing: string = "hello".charAt(99)`)
	assertContains(t, out, `c='e'`)
	assertContains(t, out, `missing=''`)
	assertNotContains(t, out, `awk`)
}

func TestCodegen_StringLastIndexOf(t *testing.T) {
	out := compile(t, `let s: string = "banana"
let i: number = s.lastIndexOf("na")`)
	assertContains(t, out, `awk`)
	assertContains(t, out, `length(_needle)`)
	assertContains(t, out, `substr(_s,i+1,n)==_needle`)
}

func TestCodegen_StringLastIndexOfSafeAwkArgs(t *testing.T) {
	out := compile(t, `let s: string = "hello hello"
let i: number = s.lastIndexOf("lo")`)
	assertContains(t, out, `found=-1`)
	assertContains(t, out, `_needle`)
}

func TestCodegen_MathTrunc(t *testing.T) {
	out := compile(t, `let x = -3.7
let n: number = Math.trunc(x)`)
	assertContains(t, out, `awk`)
	assertContains(t, out, `int(`)
}

func TestCodegen_MathSign(t *testing.T) {
	out := compile(t, `let x = -3
let n: number = Math.sign(x)`)
	assertContains(t, out, `awk`)
	assertContains(t, out, `-1`)
	assertContains(t, out, `1`)
}

func TestCodegen_StaticMathMethods(t *testing.T) {
	out := compile(t, `let minValue = Math.min(4, 2)
let maxValue = Math.max(4, 2)
let rounded = Math.round(2.7)
let floored = Math.floor(2.7)
let ceiled = Math.ceil(2.1)
let truncated = Math.trunc(-3.7)
let signValue = Math.sign(-3)
let absValue = Math.abs(-3)
let powValue = Math.pow(2, 3)
let sqrtValue = Math.sqrt(9)`)
	assertContains(t, out, `minValue=2`)
	assertContains(t, out, `maxValue=4`)
	assertContains(t, out, `rounded=3`)
	assertContains(t, out, `floored=2`)
	assertContains(t, out, `ceiled=3`)
	assertContains(t, out, `truncated=-3`)
	assertContains(t, out, `signValue=-1`)
	assertContains(t, out, `absValue=3`)
	assertContains(t, out, `powValue=8`)
	assertContains(t, out, `sqrtValue=3`)
	assertNotContains(t, out, `awk`)
}

func TestCodegen_FloatTrackingClearedOnIntegerReassignment(t *testing.T) {
	out := compile(t, `let r = Math.round(2.7)
r = 4
let sum = r + 2`)
	assertContains(t, out, `sum=$(( $r + 2 ))`)
	assertNotContains(t, out, `awk -v _a=$r -v _b=2`)
}

func TestCodegen_FloatTrackingSetOnFloatReassignment(t *testing.T) {
	out := compile(t, `let r = 1
r = Math.sqrt(4)
let sum = r + 2`)
	assertContains(t, out, `sum=$(awk -v _a=$r -v _b=2`)
}

func TestCodegen_StringCharAt(t *testing.T) {
	out := compile(t, `let s: string = "hello"
let c: string = s.charAt(1)`)
	assertContains(t, out, `awk`)
	assertContains(t, out, `substr`)
}

func TestCodegen_StringSubstring(t *testing.T) {
	out := compile(t, `let s: string = "hello"
let sub: string = s.substring(1, 4)`)
	assertContains(t, out, `awk`)
	assertContains(t, out, `substr`)
}

func TestCodegen_ListLastIndexOf(t *testing.T) {
	out := compile(t, `let l: string[] = ["a", "b", "a"]
let i: number = l.lastIndexOf("a")`)
	assertContains(t, out, `awk`)
	assertContains(t, out, `-1`)
}

func TestCodegen_ListUnshift(t *testing.T) {
	out := compile(t, `let l: string[] = ["b", "c"]
let next: string[] = l.unshift("a")`)
	assertContains(t, out, `printf '%s\n%s'`)
	assertContains(t, out, `a`)
}

func TestCodegen_ArrayOf(t *testing.T) {
	out := compile(t, `let l: string[] = Array.of("a", "b", "c")`)
	assertContains(t, out, "l='a\nb\nc'")
	assertNotContains(t, out, `printf '%s\n%s'`)
}

func TestCodegen_StaticArrayFactories(t *testing.T) {
	out := compile(t, `let ofValues = Array.of("a", "b", "c")
let indexes = Array.from({ length: 3 })
for (value of Array.of("x", "y")) {
    console.log(value)
}
for (i of Array.from({ length: 3 })) {
    console.log(i)
}`)
	assertContains(t, out, "ofValues='a\nb\nc'")
	assertContains(t, out, "indexes='0\n1\n2'")
	assertContains(t, out, `for value in 'x' 'y'; do`)
	assertContains(t, out, `for i in '0' '1' '2'; do`)
	assertNotContains(t, out, `_arrfrom_`)
}

func TestCodegen_ArrayIsArray(t *testing.T) {
	out := compile(t, `let isList: boolean = Array.isArray(["a", "b"])
let madeList: boolean = Array.isArray(Array.of("a"))
let notList: boolean = Array.isArray("a")`)
	assertContains(t, out, `isList=1`)
	assertContains(t, out, `madeList=1`)
	assertContains(t, out, `notList=0`)
}

func TestCodegen_BooleanBuiltin(t *testing.T) {
	out := compile(t, `let emptyString: boolean = Boolean("")
let zero: boolean = Boolean(0)
let nonZero: boolean = Boolean(2)
let textZero: boolean = Boolean("0")
let listValue: boolean = Boolean([])`)
	assertContains(t, out, `emptyString=0`)
	assertContains(t, out, `zero=0`)
	assertContains(t, out, `nonZero=1`)
	assertContains(t, out, `textZero=1`)
	assertContains(t, out, `listValue=1`)
}

func TestCodegen_ObjectKeys(t *testing.T) {
	out := compile(t, `let user = { id: 1, name: "Victor" }
user.active = true
let key = "role"
user[key] = "admin"
let keys: string[] = Object.keys(user)
let values: string[] = Object.values(user)
let entries: string[][] = Object.entries(user)
let literalKeys: string[] = Object.keys({ value: "x", enabled: true })
let literalValues: string[] = Object.values({ value: "x", enabled: true })
let literalEntries: string[][] = Object.entries({ value: "x", enabled: true })
let hasName: boolean = Object.hasOwn(user, "name")
let hasLiteral: boolean = Object.hasOwn({ value: "x", enabled: true }, "enabled")`)
	assertContains(t, out, `_objkeys_user='id name'`)
	assertContains(t, out, `case " $_objkeys_user " in *" active "*)`)
	assertContains(t, out, `printf '%s\n' $_objkeys_user`)
	assertContains(t, out, `values=$(for _bst_obj_key in $_objkeys_user`)
	assertContains(t, out, `entries=$(for _bst_obj_key in $_objkeys_user`)
	assertContains(t, out, `if [ "$_bst_obj_key" = 'active' ]`)
	assertContains(t, out, `printf '%s\037%s\n' "$_bst_obj_key" "$_bst_obj_value"`)
	assertContains(t, out, "literalKeys='value\nenabled'")
	assertContains(t, out, "literalValues='x\ntrue'")
	assertContains(t, out, "literalEntries='value\037x\nenabled\037true'")
	assertContains(t, out, `grep -qxF -- "$_bst_obj_key"`)
	assertContains(t, out, `hasName=$(_bst_obj_key='name'`)
	assertContains(t, out, `hasLiteral=1`)
	assertNotContains(t, out, `_objkeys__objlit_`)
	assertNotContains(t, out, `_bst_object_keys`)
}

func TestCodegen_BooleanPropertyCondition(t *testing.T) {
	out := compile(t, `let user = { name: "Ada", active: true }
if (user.active) {
    console.log("active")
}`)
	assertContains(t, out, `if [ "$_obj_user_active" = 1 ]; then`)
	assertContains(t, out, `printf '%s\n' 'active'`)
	assertNotContains(t, out, `_bst_cond="$_obj_user_active"`)
}

func TestCodegen_StringPropertyConditionKeepsTruthyFallback(t *testing.T) {
	out := compile(t, `let user = { name: "Ada", active: true }
if (user.name) {
    console.log("named")
}`)
	assertContains(t, out, `if (_bst_cond="$_obj_user_name"; [ -n "$_bst_cond" ] && [ "$_bst_cond" != 0 ]); then`)
	assertNotContains(t, out, `if [ "$_obj_user_name" = 1 ]; then`)
}

func TestCodegen_StaticObjectLiteralAPIs(t *testing.T) {
	out := compile(t, `let keys = Object.keys({ name: "Ada", active: true })
let values = Object.values({ name: "Ada", active: true })
let entries = Object.entries({ name: "Ada", active: true })
let yes = Object.hasOwn({ name: "Ada", active: true }, "active")
let no = Object.hasOwn({ name: "Ada", active: true }, "missing")
for (key of Object.keys({ name: "Ada", active: true })) {
    console.log(key)
}`)
	assertContains(t, out, "keys='name\nactive'")
	assertContains(t, out, "values='Ada\ntrue'")
	assertContains(t, out, "entries='name\037Ada\nactive\037true'")
	assertContains(t, out, `yes=1`)
	assertContains(t, out, `no=0`)
	assertContains(t, out, `for key in 'name' 'active'; do`)
	assertNotContains(t, out, `_objkeys__objlit_`)
}

func TestCodegen_ListForEach(t *testing.T) {
	out := compile(t, `let names: string[] = ["alice", "bob"]
names.forEach((name, index) => console.log(index.toString() + ":" + name))`)
	assertContains(t, out, `_foreach_`)
	assertContains(t, out, `while IFS= read -r _cb_`)
	assertContains(t, out, `done <<__BESHT_FOREACH_`)
	assertNotContains(t, out, `| while IFS= read -r _cb_`)
}

func TestCodegen_ListForEachLiteralReceiver(t *testing.T) {
	out := compile(t, `let names = ["alice", "bob"]
names.map(name => name).forEach(name => console.log(name))`)
	assertContains(t, out, `_foreach_`)
	assertContains(t, out, `while IFS= read -r _cb_`)
}

func TestCodegen_NumberIsSafeInteger(t *testing.T) {
	out := compile(t, `let ok: boolean = Number.isSafeInteger(9007199254740991)`)
	assertContains(t, out, `awk`)
	assertContains(t, out, `9007199254740991`)
}

func TestCodegen_NumberIsNaN(t *testing.T) {
	out := compile(t, `let ok: boolean = Number.isNaN(0)`)
	assertContains(t, out, `ok=0`)
}

func TestCodegen_NumberConstants(t *testing.T) {
	out := compile(t, `let max: number = Number.MAX_SAFE_INTEGER
let min: number = Number.MIN_SAFE_INTEGER
let eps: number = Number.EPSILON`)
	assertContains(t, out, `max=9007199254740991`)
	assertContains(t, out, `min=-9007199254740991`)
	assertContains(t, out, `eps=2.220446049250313e-16`)
}

// ── Condition path codegen tests ───────────────────────────────────────────────

func TestCodegen_CmdInCondition(t *testing.T) {
	out := compile(t, `if ($("test", "-f", "/etc/hosts")) {
    $("echo", "exists").run()
}`)
	assertContains(t, out, `test`)
	assertContains(t, out, `-f`)
}

func TestCodegen_StringIncludesInCondition(t *testing.T) {
	out := compile(t, `let s: string = "hello world"
if (s.includes("world")) {
    $("echo", "found").run()
}`)
	assertContains(t, out, `case`)
}

func TestCodegen_StringStartsWithInCondition(t *testing.T) {
	out := compile(t, `let s: string = "hello"
if (s.startsWith("hel")) {
    $("echo", "yes").run()
}`)
	assertContains(t, out, `_bst_starts_with`)
	assertContains(t, out, `'hel'`)
}

func TestCodegen_StringEndsWithInCondition(t *testing.T) {
	out := compile(t, `let s: string = "hello"
if (s.endsWith("llo")) {
    $("echo", "yes").run()
}`)
	assertContains(t, out, `_bst_ends_with`)
	assertContains(t, out, `'llo'`)
}

func TestCodegen_ListIncludesInCondition(t *testing.T) {
	out := compile(t, `let l: list<string> = ["a", "b"]
if (l.includes("a")) {
    $("echo", "yes").run()
}`)
	assertContains(t, out, `grep -qxF`)
	assertNotContains(t, out, `_bst_includes()`)
}

// ── Integration tests for $() ──────────────────────────────────────────────────

func TestCodegen_RawStringSingleQuoted(t *testing.T) {
	out := compile(t, `let p: string = r"^foo-[0-9]+$"`)
	assertContains(t, out, `p='^foo-[0-9]+$'`)
}

func TestCodegen_RawStringDollarSign(t *testing.T) {
	out := compile(t, `let f: string = r"-cache$"`)
	assertContains(t, out, `f='-cache$'`)
}

func TestCodegen_RawStringInCmdArg(t *testing.T) {
	out := compile(t, `$("grep", "-v", r"-cache$").run()`)
	assertContains(t, out, `'-cache$'`)
	assertNotContains(t, out, `"-cache$"`)
}

func TestCodegen_RawStringRegexInSed(t *testing.T) {
	out := compile(t, `let r = $("echo", "foo").pipe($("sed", r"s/foo/bar/")).run().readStdout()`)
	assertContains(t, out, `'s/foo/bar/'`)
}

func TestCodegen_EscapedDollarInString(t *testing.T) {
	out := compile(t, `let p: string = "total is \$42"`)
	assertContains(t, out, `\$42`)
}

func TestCodegen_EscapedDollarNotTreatedAsVar(t *testing.T) {
	out := compile(t, `let s: string = "ends with \$"`)
	assertNotContains(t, out, `"$"`)
}

func TestCodegen_TemplateLitInterpolatesVar(t *testing.T) {
	out := compile(t, "let name: string = \"world\"\nlet msg: string = `Hello ${name}!`")
	assertContains(t, out, `"Hello ${name}!"`)
}

func TestCodegen_PlainStringLiteralNotInterpolated(t *testing.T) {
	out := compile(t, `let name: string = "world"
let msg: string = "Hello ${name}!"`)
	assertContains(t, out, `'Hello ${name}!'`)
	assertNotContains(t, out, `"Hello ${name}!"`)
}

func TestCodegen_LiteralDollarSingleQuoted(t *testing.T) {
	out := compile(t, `$("echo", r"cost is $5").run()`)
	assertContains(t, out, `'cost is $5'`)
	assertNotContains(t, out, `"cost is $5"`)
}

func TestCodegen_ClassDeclAndMembers(t *testing.T) {
	out := compile(t, `class User {
    name: string
    constructor(name: string) { this.name = name }
    greet(): string { return "Hello, " + this.name }
}
let u = new User("Alice")
console.log(u.greet())
u.name = "Bob"
console.log(u.name)`)
	assertContains(t, out, `User__get_name()`)
	assertContains(t, out, `User__set_name()`)
	assertContains(t, out, `u='u'`)
	assertContains(t, out, `User__constructor "$u" 'Alice'`)
	assertContains(t, out, `$(User__greet "$u")`)
	assertContains(t, out, `_obj_u_name='Bob'`)
}

func TestCodegen_ClassMemberSourceComments(t *testing.T) {
	out := compile(t, `class User {
    name: string
    constructor(name: string) { this.name = name }
    greet(): string { return "Hello, " + this.name }
    static describe(): string { return "User" }
}`)

	assertContains(t, out, "# besht:test.bsh:3:5\nUser__constructor() {")
	assertContains(t, out, "# besht:test.bsh:4:5\nUser__greet() {")
	assertContains(t, out, "# besht:test.bsh:5:12\nUser__describe() {")
	assertNotContains(t, out, "# besht:test.bsh:2:5\nUser__get_name()")
	assertNotContains(t, out, "# besht:test.bsh:2:5\nUser__set_name()")
}

func TestCodegen_ClassSyntheticFunctionsDoNotGetSourceComments(t *testing.T) {
	out := compile(t, `class AccessorOnly {
    value: string
}
class EmptyDefault {
}`)

	assertContains(t, out, "AccessorOnly__get_value() {")
	assertContains(t, out, "AccessorOnly__set_value() {")
	assertContains(t, out, "EmptyDefault__constructor() {")
	assertNotContains(t, out, "# besht:test.bsh:1:1\nAccessorOnly__get_value()")
	assertNotContains(t, out, "# besht:test.bsh:2:5\nAccessorOnly__get_value()")
	assertNotContains(t, out, "# besht:test.bsh:1:1\nAccessorOnly__set_value()")
	assertNotContains(t, out, "# besht:test.bsh:4:1\nEmptyDefault__constructor()")
}

func TestCodegen_ClassAccessors(t *testing.T) {
	out := compile(t, `class User {
    name: string
    constructor(name: string) { this.name = name }
    get label(): string { return this.name }
    set label(value: string) { this.name = value }
    static get kind(): string { return "user" }
}
let u = new User("Alice")
let before: string = u.label
u.label = "Bob"
let after: string = u.label
let kind: string = User.kind`)

	assertContains(t, out, `User__get_label() {`)
	assertContains(t, out, `User__set_label() {`)
	assertContains(t, out, `before=$(User__get_label "$u")`)
	assertContains(t, out, `User__set_label "$u" 'Bob'`)
	assertContains(t, out, `after=$(User__get_label "$u")`)
	assertContains(t, out, `kind=$(User__get_kind)`)
	assertNotContains(t, out, `_obj_u_label=`)
}

func TestCodegen_StaticClassMembers(t *testing.T) {
	out := compile(t, `class MathUtils {
    static PI: number = 3.14159
    static round(n: number): number { return Math.round(n) }
}
console.log(MathUtils.PI)
console.log(MathUtils.round(2.7))`)
	assertContains(t, out, `_class_MathUtils_PI=3.14159`)
	assertContains(t, out, `MathUtils__round()`)
	assertContains(t, out, `$(MathUtils__round 2.7)`)
}

func TestCodegen_TypeScriptClassStaticRecordAndForOfState(t *testing.T) {
	out := compile(t, `function run(moves: string): string {
    class Game {
        private static Deltas: Record<string, [number, number]> = { U: [-1, 0] }
        static find(rows: string[]): number { return rows.findIndex(row => row.includes("@")) }
        getNextPosition(direction: string): [number, number] {
            const [dr, dc] = Game.Deltas[direction]
            return [dr, dc]
        }
    }
    let result = "fail"
    let game = new Game()
    for (const move of moves.split("") as string[]) {
        const next = game.getNextPosition(move)
        result = "success"
        break
    }
    return result
}`)
	assertContains(t, out, `_obj__class_Game_Deltas_U=`)
	assertContains(t, out, `_destructure_6_13=`)
	assertContains(t, out, `_findidx_4_69=0`)
	assertContains(t, out, `done <<__BESHT_FOR_12_5`)
	assertContains(t, out, `__BESHT_FOR_12_5`)
	assertNotContains(t, out, `| while IFS= read -r _run_move`)
}

func TestCodegen_ArrayFromTwoParamBlockMapAndPrefixUpdate(t *testing.T) {
	out := compile(t, `let count = 0
let mapped = Array.from({ length: 3 }).map((_, i) => {
    if (++count % 2 !== 0) return i
    return count
})`)
	assertNotContains(t, out, `_arrfrom_2_14=0`)
	assertContains(t, out, "0\n1\n2")
	assertContains(t, out, `_cb_2_45_i=$_cb_2_44_index`)
	assertContains(t, out, `(count = count + 1)`)
	assertContains(t, out, `continue`)
	assertNotContains(t, out, `local `)
}

func TestCodegen_SplitJoinPreservesNewlineSentinel(t *testing.T) {
	out := compile(t, `let text = "a\nb"
let same = text.split("").map(ch => ch).join("")`)
	assertContains(t, out, `__BESHT_NL__`)
	assertContains(t, out, `printf "\n"`)
}

func TestCodegen_MapCallbackRejectsExpressionStatement(t *testing.T) {
	err := compileError(t, `function payload(): string { return "sh -c id" }
let values = ["x"].map(x => {
    payload()
    return x
})`)
	if err == nil {
		t.Fatal("expected codegen error")
	}
	if !strings.Contains(err.Error(), "unsupported expression statement in map callback") {
		t.Fatalf("error: got %q", err)
	}
}
