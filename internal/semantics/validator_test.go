package semantics_test

import (
	"strings"
	"testing"

	"github.com/victor141516/besht/internal/parser"
	"github.com/victor141516/besht/internal/semantics"
)

func check(t *testing.T, src string) error {
	t.Helper()
	prog, err := parser.Parse(src, "test.bsh")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	validator := semantics.New()
	return validator.Validate(prog)
}

func mustCheck(t *testing.T, src string) {
	t.Helper()
	if err := check(t, src); err != nil {
		t.Fatalf("unexpected semantic error: %v", err)
	}
}

func expectError(t *testing.T, src, fragment string) {
	t.Helper()
	err := check(t, src)
	if err == nil {
		t.Fatal("expected semantic error, got nil")
	}
	if fragment != "" && !strings.Contains(err.Error(), fragment) {
		t.Errorf("error %q does not contain %q", err.Error(), fragment)
	}
}

func TestValidator_DefaultModeIsPermissive(t *testing.T) {
	prog, err := parser.Parse(`let x: string = 42`, "test.bsh")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if err := semantics.New().Validate(prog); err != nil {
		t.Fatalf("semantic validator should ignore type annotations, got %v", err)
	}
}

func TestValidator_LetDeclValid(t *testing.T) {
	mustCheck(t, `let x: string = "hello"`)
	mustCheck(t, `let n: number = 42`)
	mustCheck(t, `let b: boolean = true`)
	mustCheck(t, `let l: Array<string> = ["a", "b"]`)
}

func TestValidator_OptionalChaining(t *testing.T) {
	mustCheck(t, `let user = { name: "Ada" }
let a: string = user?.name
let name: string = user?.name ?? "anonymous"
let items: string[] = ["a", "b"]
let b: string = items?.[0]
let item: string = items?.[1] ?? "fallback"
let c: string = undefined?.name
let d: string = undefined?.[0]
let e: string = " Ada "?.trim()`)
}

func TestValidator_OptionalChainingChecksSubexpressions(t *testing.T) {
	expectError(t, `let items: string[] = ["a"]
let value = items?.[missing]`, `variable "missing" not declared`)
}

func TestValidator_OptionalChainingIgnoresTypeShapes(t *testing.T) {
	mustCheck(t, `let items: string[] = ["a"]
let value = items?.bogus`)
	mustCheck(t, `let value = undefined?.["bad"]`)
}

func TestValidator_NullishCoalescing(t *testing.T) {
	mustCheck(t, `let value: string = undefined ?? "fallback"`)
	mustCheck(t, `let value: string = null ?? "fallback"`)
	mustCheck(t, `let value: number = undefined ?? 7`)
}

func TestValidator_ProcessEnvAndExit(t *testing.T) {
	mustCheck(t, `let home: string = process.env.HOME`)
	mustCheck(t, `let home: string = process.env.HOME ?? "fallback"`)
	mustCheck(t, `let envObj = process.env`)
	mustCheck(t, `process.exit()`)
	mustCheck(t, `process.exit(7)`)
	mustCheck(t, `try {
    $("false").run()
} catch (code: status) {
    process.exit(code)
}`)
}

func TestValidator_ProcessRejectsInvalidAPI(t *testing.T) {
	expectError(t, `process.foo()`, `process has no method "foo"`)
	expectError(t, `let cwd = process.cwd`, `process has no property "cwd"`)
	expectError(t, `process.exit(1, 2)`, `process.exit() takes 0 or 1 argument`)
}

func TestValidator_ArgsHelpers(t *testing.T) {
	mustCheck(t, `let all: string[] = Besht.args.argv()`)
	mustCheck(t, `let first: string = Besht.args.positional(1) ?? "fallback"`)
	mustCheck(t, `let branch: string = Besht.args.option("branch", "b") ?? "main"`)
	mustCheck(t, `let dryRun: boolean = Besht.args.flag("dry-run", "d")`)
}

func TestValidator_ArgsLongNameRequired(t *testing.T) {
	expectError(t, `let bad = Besht.args.option()`, "Besht.args.option() takes 1 or 2 arguments")
	expectError(t, `let bad = Besht.args.flag()`, "Besht.args.flag() takes 1 or 2 arguments")
}

func TestValidator_FnDeclValid(t *testing.T) {
	mustCheck(t, `function add(a: number, b: number): number {
    return 1
}`)
}

func TestValidator_FnCallValid(t *testing.T) {
	mustCheck(t, `function greet(name: string) {
    $("echo", "hi")
}
greet("Alice")`)
}

func TestValidator_FnParamScope(t *testing.T) {
	mustCheck(t, `function f(x: number): number {
    return x
}`)
}

func TestValidator_IfConditionBool(t *testing.T) {
	mustCheck(t, `let b: boolean = true
if (b) {
    $("echo", "x")
}`)
}

func TestValidator_IfConditionComparison(t *testing.T) {
	mustCheck(t, `let n: number = 5
if (n > 0) {
    $("echo", "pos")
}`)
}

func TestValidator_ForRangeValid(t *testing.T) {
	mustCheck(t, `for (i in Besht.iter.range(1, 10)) {
    $("echo", "${i}")
}`)
}

func TestValidator_ForListValid(t *testing.T) {
	mustCheck(t, `let files: Array<string> = ["a", "b"]
for (f in files) {
    $("echo", "${f}")
}`)
}

func TestValidator_ForShellValid(t *testing.T) {
	mustCheck(t, `for (line in $("cat", "/etc/hosts").readStdoutLines()) {
    $("echo", "${line}")
}`)
}

func TestValidator_TryCatchValid(t *testing.T) {
	mustCheck(t, `try {
    $("risky_cmd")
} catch (err: status) {
    $("echo", "failed")
}`)
}

func TestValidator_StringConcatValid(t *testing.T) {
	mustCheck(t, `let a: string = "hello"
let b: string = " world"
let c: string = a + b`)
}

func TestValidator_StringConcatWithLiteral(t *testing.T) {
	mustCheck(t, `let name: string = "Alice"
let greeting: string = "Hello, " + name + "!"`)
}

func TestValidator_BinaryArithmeticValid(t *testing.T) {
	mustCheck(t, `let a: number = 3
let b: number = 4
let c: number = a + b`)
}

func TestValidator_EqValidForStrings(t *testing.T) {
	mustCheck(t, `let a: string = "x"
let b: string = "y"
if (a == b) {
    $("true")
}`)
}

func TestValidator_UnaryNotValidBool(t *testing.T) {
	mustCheck(t, `let b: boolean = true
if (!b) {
    $("echo", "x")
}`)
}

func TestValidator_BuiltinFileExists(t *testing.T) {
	mustCheck(t, `let p: string = "/tmp/x"
if (Besht.fs.isFile(p)) {
    $("echo", "exists")
}`)
}

func TestValidator_BuiltinIsDir(t *testing.T) {
	mustCheck(t, `let p: string = "/tmp"
if (Besht.fs.isDir(p)) {
    $("echo", "dir")
}`)
}

func TestValidator_BeshtNamespaceWrappers(t *testing.T) {
	mustCheck(t, `let p: string = "/tmp"
let s: string = "x"
let a: boolean = Besht.fs.isFile(p)
let b: boolean = Besht.fs.isDir(p)
let c: boolean = Besht.fs.isReadable(p)
let d: boolean = Besht.fs.isWritable(p)
let e: boolean = Besht.fs.isExecutable(p)
let f: boolean = Besht.strings.isEmpty(s)
let g: boolean = Besht.strings.isNonEmpty(s)
if (Besht.fs.isFile(p) && Besht.strings.isNonEmpty(s)) {
    $("echo", "ok")
}`)
}

func TestValidator_BeshtNamespaceErrors(t *testing.T) {
	expectError(t, `let p: string = "/tmp"
let ok: boolean = Besht.fs.unknown(p)`, `Besht.fs has no method "unknown"`)
	expectError(t, `let ok: boolean = Besht.fs.isFile()`, `Besht.fs.isFile() takes 1 argument`)
	expectError(t, `let ok: boolean = Besht.fs.isFile("/tmp", "extra")`, `Besht.fs.isFile() takes 1 argument`)
}

func TestValidator_BuiltinLen(t *testing.T) {
	mustCheck(t, `let files: Array<string> = ["a"]
let n: number = files.length`)
}

func TestValidator_BuiltinHead(t *testing.T) {
	mustCheck(t, `let files: Array<string> = ["a", "b"]
let first: string = files[0]`)
}

func TestValidator_BuiltinTail(t *testing.T) {
	mustCheck(t, `let files: Array<string> = ["a", "b"]
let rest: Array<string> = files.slice(1)`)
}

func TestValidator_BuiltinAppend(t *testing.T) {
	mustCheck(t, `let files: Array<string> = ["a"]
files = files.push("b")`)
}

func TestValidator_BuiltinContains(t *testing.T) {
	mustCheck(t, `let files: Array<string> = ["a"]
if (files.includes("a")) {
    $("echo", "found")
}`)
}

func TestValidator_BuiltinRangeValid(t *testing.T) {
	mustCheck(t, `for (i in Besht.iter.range(1, 5)) {
    $("echo", "${i}")
}`)
}

func TestValidator_ExitNoArg(t *testing.T) {
	mustCheck(t, `process.exit(0)`)
}

func TestValidator_ExitWithCode(t *testing.T) {
	mustCheck(t, `let code: number = 1
process.exit(code)`)
}

func TestValidator_ShellExprReturnsString(t *testing.T) {
	mustCheck(t, `let s: string = $("whoami")`)
}

func TestValidator_ShellExprDefinedVar(t *testing.T) {
	mustCheck(t, `let name: string = "world"
let s: string = $("echo", "${name}")`)
}

func TestValidator_ShellExprPositionalParams(t *testing.T) {
	mustCheck(t, `let s: string = $("echo", "hello")`)
}

func TestValidator_ShellExprSpecialVars(t *testing.T) {
	mustCheck(t, `let s: string = $("echo", "hello")`)
}

func TestValidator_PipeExpr(t *testing.T) {
	mustCheck(t, `let r: string = $("whoami").pipe($("tr", "-d", "\n"))`)
}

func TestValidator_NestedFunctions(t *testing.T) {
	mustCheck(t, `function outer(x: number): number {
    return x
}
function wrapper(y: number): number {
    return outer(y)
}`)
}

func TestValidator_TemplateLitKnownVar(t *testing.T) {
	mustCheck(t, "let name: string = \"Alice\"\nlet msg: string = `Hello, ${name}!`")
}

func TestValidator_StringLiteralNoInterpolation(t *testing.T) {
	mustCheck(t, `let msg: string = "Hello, ${literally_anything}!"`)
}

func TestValidator_ListLiteralConsistentTypes(t *testing.T) {
	mustCheck(t, `let l: Array<string> = ["a", "b", "c"]`)
}

func TestValidator_SetAndNestedLists(t *testing.T) {
	mustCheck(t, "function f(factory: string[]): string {\n"+
		"    const matrix: string[][] = factory.map(e => e.split(\"\") as string[])\n"+
		"    const seen = new Set<string>()\n"+
		"    let pos = [0, 0]\n"+
		"    const current = matrix[pos[0]][pos[1]]\n"+
		"    if (pos[1] > matrix[0].length - 1) return \"broken\"\n"+
		"    const key = `${pos[0]},${pos[1]}`\n"+
		"    if (seen.has(key)) return \"loop\"\n"+
		"    else seen.add(key)\n"+
		"    return current\n"+
		"}")
}

func TestValidator_PropagateExpr(t *testing.T) {
	mustCheck(t, `function f(): string {
    let x: string = $("cat", "file")?
    return x
}`)
}

func TestValidator_TernaryExpr(t *testing.T) {
	mustCheck(t, `let x: number = 10
let y: number = 3
let bigger: number = x > y ? x : y`)
}

func TestValidator_TernaryExprStringBranches(t *testing.T) {
	mustCheck(t, `let x: number = 10
let label: string = x > 5 ? "big" : "small"`)
}

func TestValidator_NumberToStringMethod(t *testing.T) {
	mustCheck(t, `let n: number = 42
let s: string = n.toString()`)
}

func TestValidator_PrimitiveToStringMethods(t *testing.T) {
	mustCheck(t, `let s: string = "x"
let out: string = s.toString()`)
	mustCheck(t, `let b: boolean = true
let out: string = b.toString()`)
	mustCheck(t, `try {
    $("false").run()
} catch (code: status) {
    let out: string = code.toString()
}`)
}

func TestValidator_PrimitiveToStringRejectsArguments(t *testing.T) {
	expectError(t, `let s: string = "x"
let out: string = s.toString("bad")`, "toString() takes no arguments")
	expectError(t, `let n: number = 42
let out: string = n.toString("bad")`, "toString() takes no arguments")
	expectError(t, `let b: boolean = true
let out: string = b.toString("bad")`, "toString() takes no arguments")
	expectError(t, `try {
    $("false").run()
} catch (code: status) {
    let out: string = code.toString("bad")
}`, "toString() takes no arguments")
}

func TestValidator_NumberParseIntOneAndTwoArgs(t *testing.T) {
	mustCheck(t, `let n: number = Number.parseInt("42")`)
	mustCheck(t, `let n: number = Number.parseInt("42", 10)`)
}

func TestValidator_NumberToFixedMethod(t *testing.T) {
	mustCheck(t, `let pi: number = 3.14159
let s: string = pi.toFixed(2)`)
}

func TestValidator_BreakInLoop(t *testing.T) {
	mustCheck(t, `let n: number = 5
while (n > 0) {
    break
    n = n - 1
}`)
}

func TestValidator_ContinueInLoop(t *testing.T) {
	mustCheck(t, `let files: Array<string> = ["a", "b"]
for (f in files) {
    continue
}`)
}

func TestValidator_ConstDecl(t *testing.T) {
	mustCheck(t, `const threshold: number = 90`)
}

func TestValidator_ProcessEnvDirect(t *testing.T) {
	mustCheck(t, `let home: string = process.env.HOME`)
}

func TestValidator_ProcessEnvWithDefault(t *testing.T) {
	mustCheck(t, `let port: string = process.env.PORT ?? "8080"`)
}

func TestValidator_FetchTextSlice(t *testing.T) {
	mustCheck(t, `let url: string = "file:///tmp/data.txt"
let body: string = fetch(url).text()
let response = fetch(url)
let body2: string = response.text()`)
}

func TestValidator_FetchRejectsDeferredSurface(t *testing.T) {
	expectError(t, `let body = fetch("file:///tmp/data.txt", { method: "POST" }).text()`, "fetch() takes 1 URL argument")
	expectError(t, `let response = fetch("file:///tmp/data.txt")
let body = response.text("utf8")`, "FetchResponse.text() takes no arguments")
	expectError(t, `let response = fetch("file:///tmp/data.txt")
let data = response.json()`, `FetchResponse has no method "json"`)
	expectError(t, `let response = fetch("file:///tmp/data.txt")
let ok = response.ok`, `FetchResponse has no property "ok"`)
	expectError(t, `let response = fetch("file:///tmp/data.txt")
let status = response.status`, `FetchResponse has no property "status"`)
	expectError(t, `let response = fetch("file:///tmp/data.txt")
let headers = response.headers`, `FetchResponse has no property "headers"`)
	expectError(t, `let response = fetch("file:///tmp/data.txt")
let body = response.body`, `FetchResponse has no property "body"`)
}

func TestValidator_PrintBuiltin(t *testing.T) {
	mustCheck(t, `let msg: string = "hello"
console.log(msg)`)
}

func TestValidator_IndexExpr(t *testing.T) {
	mustCheck(t, `let files: Array<string> = ["a", "b", "c"]
let first: string = files[0]`)
}

func TestValidator_IndexExprVariable(t *testing.T) {
	mustCheck(t, `let files: Array<string> = ["a", "b", "c"]
let i: number = 1
let item: string = files[i]`)
}

func TestValidator_SetAndNestedListIndex(t *testing.T) {
	mustCheck(t, `type Factory = string[]
function run(factory: Factory): string {
    const matrix: string[][] = factory.map(e => e.split("") as string[])
    const seen = new Set<string>()
    const row: string[] = matrix[0]
    const cell: string = matrix[0][0]
    const width: number = row.length
    if (seen.has(cell)) return cell
    else seen.add(cell)
    return cell
}`)
}

func TestValidator_SetAddValuePositionIsNotTypeChecked(t *testing.T) {
	mustCheck(t, `const seen = new Set<string>()
let x: Set<string> = seen.add("a")`)
}

func TestValidator_SetConstructorRejectsRuntimeArgs(t *testing.T) {
	expectError(t, `const seen = new Set<string>("a")`, "Set constructor takes no runtime arguments")
}

func TestValidator_ToStringMethod(t *testing.T) {
	mustCheck(t, `let n: number = 42
let s: string = n.toString()`)
}

func TestValidator_BooleanToStringMethod(t *testing.T) {
	mustCheck(t, `let b: boolean = true
let s: string = b.toString()`)
}

func TestValidator_IntBuiltin(t *testing.T) {
	mustCheck(t, `let s: string = "42"
let n: number = Number.parseInt(s)`)
}

func TestValidator_ListConcatMethod(t *testing.T) {
	mustCheck(t, `let a: Array<string> = ["x"]
let b: Array<string> = ["y"]
let c: Array<string> = a.concat(b)`)
}

func TestValidator_ListUnshift(t *testing.T) {
	mustCheck(t, `let items: string[] = ["b", "c"]
let next: string[] = items.unshift("a")
items.unshift("z")`)
}

func TestValidator_ListUnshiftRejectsBadArity(t *testing.T) {
	expectError(t, `let items: string[] = ["b", "c"]
let next: string[] = items.unshift()`, "unshift() takes 1 argument")
}

func TestValidator_StringTrim(t *testing.T) {
	mustCheck(t, `let s: string = "  hello  "
let t: string = s.trim()`)
}

func TestValidator_StringToUpperCase(t *testing.T) {
	mustCheck(t, `let s: string = "hello"
let u: string = s.toUpperCase()`)
}

func TestValidator_StringToLowerCase(t *testing.T) {
	mustCheck(t, `let s: string = "HELLO"
let l: string = s.toLowerCase()`)
}

func TestValidator_StringSplit(t *testing.T) {
	mustCheck(t, `let s: string = "a,b,c"
let parts: Array<string> = s.split(",")`)
}

func TestValidator_StringIncludes(t *testing.T) {
	mustCheck(t, `let s: string = "hello world"
if (s.includes("world")) { $("echo", "yes") }`)
}

func TestValidator_StringSearchOptionalPositionArgs(t *testing.T) {
	mustCheck(t, `let s: string = "hello hello"
let i: number = s.indexOf("lo", 4)
let last: number = s.lastIndexOf("lo", 7)
let has: boolean = s.includes("lo", 4)
let starts: boolean = s.startsWith("lo", 3)
let ends: boolean = s.endsWith("hel", 3)`)
}

func TestValidator_StringStartsWith(t *testing.T) {
	mustCheck(t, `let s: string = "hello"
if (s.startsWith("hel")) { $("echo", "yes") }`)
}

func TestValidator_StringEndsWith(t *testing.T) {
	mustCheck(t, `let s: string = "hello"
if (s.endsWith("llo")) { $("echo", "yes") }`)
}

func TestValidator_StringReplace(t *testing.T) {
	mustCheck(t, `let s: string = "hello world"
let r: string = s.replace("world", "besht")`)
}

func TestValidator_StringReplaceAll(t *testing.T) {
	mustCheck(t, `let s: string = "aaa"
let r: string = s.replaceAll("a", "b")`)
}

func TestValidator_StringLength(t *testing.T) {
	mustCheck(t, `let s: string = "hello"
let n: number = s.length`)
}

func TestValidator_ListPush(t *testing.T) {
	mustCheck(t, `let l: Array<string> = ["a"]
let l2: Array<string> = l.push("b")`)
}

func TestValidator_ListPop(t *testing.T) {
	mustCheck(t, `let l: Array<string> = ["a", "b"]
let l2: Array<string> = l.pop()`)
}

func TestValidator_ListJoin(t *testing.T) {
	mustCheck(t, `let l: Array<string> = ["a", "b", "c"]
let s: string = l.join(", ")`)
}

func TestValidator_ListToString(t *testing.T) {
	mustCheck(t, `let list: Array<string> = ["a", "b", "c"]
let s: string = list.toString()`)
}

func TestValidator_ListToStringRejectsArguments(t *testing.T) {
	expectError(t, `let list: Array<string> = ["a", "b"]
let s: string = list.toString(",")`, "toString() takes no arguments")
}

func TestValidator_ListAt(t *testing.T) {
	mustCheck(t, `let l: Array<string> = ["a", "b"]
let first: string = l.at(0)
let last: string = l.at(-1)`)
}

func TestValidator_ListAtRejectsWrongArity(t *testing.T) {
	expectError(t, `let list: Array<string> = ["a", "b"]
let item: string = list.at()`, "at() takes 1 argument")
}

func TestValidator_ListIncludes(t *testing.T) {
	mustCheck(t, `let l: Array<string> = ["a", "b"]
if (l.includes("a")) { $("echo", "yes") }`)
}

func TestValidator_ListConcat(t *testing.T) {
	mustCheck(t, `let a: Array<string> = ["x"]
let b: Array<string> = ["y"]
let c: Array<string> = a.concat(b)`)
}

func TestValidator_ListReverse(t *testing.T) {
	mustCheck(t, `let l: Array<string> = ["c", "b", "a"]
let r: Array<string> = l.reverse()`)
}

func TestValidator_ListSlice(t *testing.T) {
	mustCheck(t, `let l: Array<string> = ["a", "b", "c", "d"]
let s: Array<string> = l.slice(1, 3)`)
}

func TestValidator_ListLength(t *testing.T) {
	mustCheck(t, `let l: Array<string> = ["a", "b", "c"]
let n: number = l.length`)
}

func TestValidator_NativeListAPIsReplaceGlobalListHelpers(t *testing.T) {
	mustCheck(t, `let files: Array<string> = ["a", "b", "c"]
let other: Array<string> = ["d"]
let count: number = files.length
let first: string = files[0]
let rest: Array<string> = files.slice(1)
let appended: Array<string> = files.push("x")
let hasX: boolean = files.includes("x")
let combined: Array<string> = files.concat(other)`)
}

func TestValidator_CmdExprBasic(t *testing.T) {
	mustCheck(t, `let u: string = $("whoami")`)
}

func TestValidator_CmdExprMultiArg(t *testing.T) {
	mustCheck(t, `let b: string = $("git", "rev-parse", "--abbrev-ref", "HEAD")`)
}

func TestValidator_CmdExprRejectsMixedCommandNameSpread(t *testing.T) {
	expectError(t, `let cmd: Array<string> = ["echo"]
$(...cmd, "extra")`, "command-name spread must be the only $() argument")
}

func TestValidator_CmdExprVarArg(t *testing.T) {
	mustCheck(t, `let path: string = "/tmp"
let out: string = $("ls", path)`)
}

func TestValidator_CmdExprAssignToList(t *testing.T) {
	mustCheck(t, `let lines: Array<string> = $("find", ".", "-name", "*.log").readStdoutLines()`)
}

func TestValidator_CmdExprRun(t *testing.T) {
	mustCheck(t, `$("chmod", "+x", "script.sh")`)
}

func TestValidator_CmdExprPipe(t *testing.T) {
	mustCheck(t, `let r: string = $("cat", "/etc/passwd").pipe($("grep", "root"))`)
}

func TestValidator_CmdExprPipeChain(t *testing.T) {
	mustCheck(t, `let r: string = $("cat", "/etc/passwd")
    .pipe($("grep", "root"))
    .pipe($("cut", "-d:", "-f1"))`)
}

func TestValidator_CmdExprStdoutRedirect(t *testing.T) {
	mustCheck(t, `$("make", "build").stdout("/tmp/build.log")`)
}

func TestValidator_CmdExprStdoutAppend(t *testing.T) {
	mustCheck(t, `$("echo", "line").stdout("/tmp/out.txt", "append")`)
}

func TestValidator_CmdExprStderrNull(t *testing.T) {
	mustCheck(t, `$("make", "build").stderr("null")`)
}

func TestValidator_CmdExprStderrAnd1(t *testing.T) {
	mustCheck(t, `$("make", "build").stderr("&1")`)
}

func TestValidator_CmdExprWorkdir(t *testing.T) {
	mustCheck(t, `$("ls").workdir("/")`)
}

func TestValidator_CmdExprWorkdirRejectsArity(t *testing.T) {
	expectError(t, `$("ls").workdir()`, "workdir() takes 1 argument")
	expectError(t, `$("ls").workdir("/", "/tmp")`, "workdir() takes 1 argument")
}

func TestValidator_CmdExprStderrCapture(t *testing.T) {
	mustCheck(t, `let errs: string = $("make").readStderr()`)
}

func TestValidator_CmdExprLines(t *testing.T) {
	mustCheck(t, `let ls: Array<string> = $("ls", "/tmp").readStdoutLines()`)
}

func TestValidator_CmdExprText(t *testing.T) {
	mustCheck(t, `let s: string = $("whoami").readStdout()`)
}

func TestValidator_CmdExprInFor(t *testing.T) {
	mustCheck(t, `for (f in $("find", ".", "-name", "*.log").readStdoutLines()) {
    $("echo", f)
}`)
}

func TestValidator_CmdExprInTryCatch(t *testing.T) {
	mustCheck(t, `try {
    $("rsync", "-az", "src", "dest")
} catch (code: status) {
    console.log("failed: " + code.toString())
}`)
}

func TestValidator_CmdExprPropagate(t *testing.T) {
	mustCheck(t, `function f(): string {
    let c: string = $("cat", "file")?
    return c
}`)
}

func TestValidator_CmdExprEmptyError(t *testing.T) {
	prog, parseErr := parser.Parse(`let x: string = $()`, "test.bsh")
	if parseErr != nil {
		return
	}
	validator := semantics.New()
	err := validator.Validate(prog)
	if err == nil {
		t.Fatal("expected error for empty $(), got nil")
	}
}

func TestValidator_CmdExprStringConcat(t *testing.T) {
	mustCheck(t, `let name: string = "world"
$("echo", "Hello " + name)`)
}

func TestValidator_CStyleForBareAssign(t *testing.T) {
	mustCheck(t, `for (i = 0; i < 3; i++) {
    $("echo", "x")
}`)
}

func TestValidator_CStyleForHelloStyle(t *testing.T) {
	mustCheck(t, `function greet(name: string, times: number): string {
    let result: string = ""
    for (i = 0; i < times; i++) {
        result = `+"`"+`${result}Hello, ${name}!`+"`"+`
    }
    return result
}`)
}

func TestValidator_MathMethods(t *testing.T) {
	mustCheck(t, `let a: number = 3
let b: number = 4
let mn: number = Math.min(a, b)
let mx: number = Math.max(a, b)
let r: number = Math.round(3.7)
let fl: number = Math.floor(3.9)
let cl: number = Math.ceil(3.1)
let ab: number = Math.abs(-5)
let pw: number = Math.pow(2, 8)
let sq: number = Math.sqrt(16)`)
}

func TestValidator_FirstPureJSCompatibleAPIBatchValid(t *testing.T) {
	tests := []struct {
		name string
		src  string
	}{
		{"math trunc", `let n: number = Math.trunc(-3.7)`},
		{"math sign", `let n: number = Math.sign(-3)`},
		{"string charAt", `let s: string = "hello"
let first: string = s.charAt(0)`},
		{"string substring", `let s: string = "hello"
let sub: string = s.substring(1, 4)`},
		{"string lastIndexOf", `let s: string = "hello hello"
let last: number = s.lastIndexOf("lo")`},
		{"list lastIndexOf", `let words: string[] = ["a", "b", "a"]
let last: number = words.lastIndexOf("a")`},
		{"Array.of", `let made: string[] = Array.of("x", "y", "z")`},
		{"Array.from string", `let chars: string[] = Array.from("abc")`},
		{"Array.isArray list", `let ok: boolean = Array.isArray(["x", "y"])`},
		{"Array.isArray non-list", `let ok: boolean = Array.isArray("x")`},
		{"Object.keys object", `let user = { id: 1, name: "Victor" }
let keys: string[] = Object.keys(user)`},
		{"Object.keys literal", `let keys: string[] = Object.keys({ id: 1, name: "Victor" })`},
		{"Object.values object", `let user = { id: 1, name: "Victor" }
let values: string[] = Object.values(user)`},
		{"Object.values literal", `let values: string[] = Object.values({ id: 1, name: "Victor" })`},
		{"Object.entries object", `let user = { id: 1, name: "Victor" }
let entries: string[][] = Object.entries(user)`},
		{"Object.entries literal", `let entries: string[][] = Object.entries({ id: 1, name: "Victor" })`},
		{"Object.fromEntries", `let user: object = Object.fromEntries([["id", "1"], ["name", "Victor"]])`},
		{"Object.hasOwn object", `let user = { id: 1, name: "Victor" }
let ok: boolean = Object.hasOwn(user, "name")`},
		{"Object.hasOwn literal", `let ok: boolean = Object.hasOwn({ id: 1, name: "Victor" }, "name")`},
		{"object spread", `let defaults = { name: "Ada", active: false }
let overrides = { active: true }
let updated: object = { ...defaults, role: "admin", ...overrides }`},
		{"Object.assign object", `let user = { id: 1, name: "Victor" }
let updated: object = Object.assign(user, { active: true })`},
		{"Object.assign literal target", `let updated: object = Object.assign({}, { id: 1, name: "Victor" })`},
		{"Object.assign multi-source", `let updated: object = Object.assign({}, { id: 1 }, { name: "Victor" })`},
		{"Object.assign object return", `function copy(obj: object): object {
    return Object.assign({}, obj)
}
let user = { id: 1 }
let keys: string[] = Object.keys(copy(user))`},
		{"Boolean value", `let ok: boolean = Boolean("x")`},
		{"JSON.stringify object", `let user = { id: 1, name: "Victor", active: true }
let json: string = JSON.stringify(user)`},
		{"JSON.stringify list", `let json: string = JSON.stringify(["a", "b"])`},
		{"JSON.parse", `let data = JSON.parse("{}")
let user = data.user
let json: string = JSON.stringify(user)`},
		{"Boolean nullable", `let ok: boolean = Boolean(undefined)`},
		{"Number.isSafeInteger", `let safe: boolean = Number.isSafeInteger(42)`},
		{"Number.isSafeInteger string predicate", `let safe: boolean = Number.isSafeInteger("1")`},
		{"Number.isNaN", `let nan: boolean = Number.isNaN(0)`},
		{"Number.MAX_SAFE_INTEGER", `let max: number = Number.MAX_SAFE_INTEGER`},
		{"Number.MIN_SAFE_INTEGER", `let min: number = Number.MIN_SAFE_INTEGER`},
		{"Number.EPSILON", `let eps: number = Number.EPSILON`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mustCheck(t, tt.src)
		})
	}
}

func TestValidator_FirstPureJSCompatibleAPIBatchRejectsBadCalls(t *testing.T) {
	tests := []struct {
		name     string
		src      string
		fragment string
	}{
		{"math sign arity", `let n: number = Math.sign()`, "Math.sign() takes 1 argument"},
		{"charAt arity", `let s: string = "abc"
let c: string = s.charAt()`, "charAt() takes 1 argument"},
		{"substring arity", `let s: string = "abc"
let sub: string = s.substring()`, "substring() takes 1 or 2 arguments"},
		{"string lastIndexOf arity", `let s: string = "abc"
let i: number = s.lastIndexOf()`, "lastIndexOf() takes 1 or 2 arguments"},
		{"list lastIndexOf arity", `let l: string[] = ["a"]
let i: number = l.lastIndexOf()`, "lastIndexOf() takes 1 argument"},
		{"string lastIndexOf arity", `let s: string = "abc"
let i: number = s.lastIndexOf("a", 1, 2)`, "lastIndexOf() takes 1 or 2 arguments"},
		{"safe integer arity", `let ok: boolean = Number.isSafeInteger()`, "Number.isSafeInteger() takes 1 argument"},
		{"isNaN arity", `let ok: boolean = Number.isNaN()`, "Number.isNaN() takes 1 argument"},
		{"isArray arity", `let ok: boolean = Array.isArray()`, "Array.isArray() takes 1 argument"},
		{"isArray extra arg", `let ok: boolean = Array.isArray([], [])`, "Array.isArray() takes 1 argument"},
		{"Object.keys arity", `let keys: string[] = Object.keys()`, "Object.keys() takes 1 argument"},
		{"Object.keys type", `let keys: string[] = Object.keys("x")`, "Object.keys() requires an object literal or named object"},
		{"Object.keys process.env alias", `let envObj = process.env
let keys: string[] = Object.keys(envObj)`, "Object.keys() requires an object literal or named object"},
		{"Object.values arity", `let values: string[] = Object.values()`, "Object.values() takes 1 argument"},
		{"Object.values type", `let values: string[] = Object.values("x")`, "Object.values() requires an object literal or named object"},
		{"Object.values process.env alias", `let envObj = process.env
let values: string[] = Object.values(envObj)`, "Object.values() requires an object literal or named object"},
		{"Object.entries arity", `let entries: string[][] = Object.entries()`, "Object.entries() takes 1 argument"},
		{"Object.entries type", `let entries: string[][] = Object.entries("x")`, "Object.entries() requires an object literal or named object"},
		{"Object.entries process.env alias", `let envObj = process.env
let entries: string[][] = Object.entries(envObj)`, "Object.entries() requires an object literal or named object"},
		{"Object.fromEntries arity", `let user: object = Object.fromEntries()`, "Object.fromEntries() takes 1 argument"},
		{"Object.hasOwn arity", `let ok: boolean = Object.hasOwn({ a: 1 })`, "Object.hasOwn() takes 2 arguments"},
		{"Object.hasOwn type", `let ok: boolean = Object.hasOwn("x", "name")`, "Object.hasOwn() requires an object literal or named object"},
		{"Object.hasOwn process.env alias", `let envObj = process.env
let ok: boolean = Object.hasOwn(envObj, "HOME")`, "Object.hasOwn() requires an object literal or named object"},
		{"Object.assign arity", `let merged = Object.assign()`, "Object.assign() takes at least 1 argument"},
		{"Object.assign type", `let merged = Object.assign("x", { name: "Ada" })`, "Object.assign() requires an object literal or named object"},
		{"Object.assign source type", `let merged = Object.assign({}, "x")`, "Object.assign() requires an object literal or named object"},
		{"Object.assign process.env", `let merged = Object.assign(process.env, { HOME: "x" })`, "Object.assign() requires an object literal or named object"},
		{"Object.assign nested value", `let merged = Object.assign({}, { values: ["a"] })`, "Object.assign() only supports scalar object values"},
		{"object spread type", `let merged = { ..."x" }`, "object spread requires an object literal or named object"},
		{"object spread process.env", `let envObj = process.env
let merged = { ...envObj }`, "object spread requires an object literal or named object"},
		{"object spread nested value", `let source = { values: ["a"] }
let merged = { ...source }`, "object spread only supports scalar object values"},
		{"Object.assign const target", `const user = { name: "Ada" }
let merged = Object.assign(user, { active: true })`, `cannot assign to const "user"`},
		{"Object.assign class instance", `class User {}
let user = new User()
let merged = Object.assign(user, { name: "Ada" })`, "Object.assign() requires an object literal or named object"},
		{"Object.assign this", `class User {
    setName() {
        Object.assign(this, { name: "Ada" })
    }
}`, "Object.assign() requires an object literal or named object"},
		{"JSON.stringify arity", `let json: string = JSON.stringify()`, "JSON.stringify() takes 1 argument"},
		{"JSON.parse arity", `let json = JSON.parse()`, "JSON.parse() takes 1 argument"},
		{"JSON.stringify unsupported", `let cmd = $("echo", "x")
let json: string = JSON.stringify(cmd)`, "JSON.stringify() cannot encode command"},
		{"JSON.stringify nested object value", `let json: string = JSON.stringify({ values: ["a"] })`, "JSON.stringify() only supports scalar object values"},
		{"Boolean arity zero", `let ok: boolean = Boolean()`, "Boolean() takes 1 argument"},
		{"Boolean arity two", `let ok: boolean = Boolean("x", "y")`, "Boolean() takes 1 argument"},
		{"forEach arity", `let l: string[] = ["a"]
l.forEach()`, "forEach() takes 1 arrow callback"},
		{"forEach non-arrow", `let l: string[] = ["a"]
l.forEach("noop")`, "forEach() callback must be an arrow expression"},
		{"forEach return", `let l: string[] = ["a"]
l.forEach(x => { return x })`, "forEach() callback does not support return"},
		{"forEach break", `let l: string[] = ["a"]
l.forEach(x => { break })`, "forEach() callback does not support break"},
		{"forEach pure expression", `let l: string[] = ["a"]
l.forEach(x => x + "!")`, "forEach() callback expression must be side-effecting"},
		{"forEach value position", `let l: string[] = ["a"]
let result = l.forEach(x => console.log(x))`, "forEach() must be used as a statement"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expectError(t, tt.src, tt.fragment)
		})
	}
}

func TestValidator_MathMethodIgnoresArgumentTypes(t *testing.T) {
	mustCheck(t, `let s: string = "x"
let n: number = Math.abs(s)`)
}

func TestValidator_ListForEach(t *testing.T) {
	mustCheck(t, `let l: Array<string> = ["a", "b"]
l.forEach((item, index) => console.log(index.toString() + ":" + item))
l.forEach(item => {
    console.log(item)
})`)
}

func TestValidator_CmdExprEnv(t *testing.T) {
	mustCheck(t, `$("env").env("FOO", "bar")`)
}

func TestValidator_CmdExprEnvRejectsDynamicName(t *testing.T) {
	expectError(t, `let name: string = "FOO"
$("env").env(name, "bar")`, "command env name must be a string literal")
}

func TestValidator_CmdExprEnvRejectsInvalidName(t *testing.T) {
	invalid := []string{"FOO=bar", "FOO;touch", "FOO BAR", "$(FOO)", "`FOO`", "1FOO"}
	for _, name := range invalid {
		expectError(t, `$("env").env("`+name+`", "bar")`, "invalid command env name")
	}
}

func TestValidator_ArrowFunctionValues(t *testing.T) {
	mustCheck(t, `let cb = (x: string): string => x`)
	mustCheck(t, `let cb = (x: string) => x + "!"
console.log(cb("a"))`)
	mustCheck(t, `let cb: (x: string) => string = x => x + "!"
let items: string[] = ["a"]
let mapped = items.map(cb)`)
}

func TestValidator_ListMapArrow(t *testing.T) {
	mustCheck(t, `let items: Array<string> = ["a", "b"]
let mapped: Array<string> = items.map(x => x + "!")`)
}

func TestValidator_ListFilterArrow(t *testing.T) {
	mustCheck(t, `let items: Array<string> = ["a", "b"]
let picked: Array<string> = items.filter((x: string) => x.startsWith("a"))`)
}

func TestValidator_ListFilterAcceptsTruthyCallback(t *testing.T) {
	mustCheck(t, `let items: number[] = [1, 2]
let picked: number[] = items.filter(x => x % 2)`)
}

func TestValidator_ListPredicateCallbacks(t *testing.T) {
	mustCheck(t, `let items: string[] = ["alpha", "beta"]
let hasAlpha: boolean = items.some((item: string) => item.startsWith("a"))
let allIndexed: boolean = items.every((item, i: number) => i >= 0)
let found: string = items.find((item, i) => i == 1)`)
}

func TestValidator_ListPredicateCallbacksRejectInvalidArgs(t *testing.T) {
	tests := []struct {
		name     string
		src      string
		fragment string
	}{
		{"some non-arrow", `let items: string[] = ["a"]
let ok: boolean = items.some("a")`, "array callback must be an arrow expression or stored callback"},
		{"every callback arity", `let items: string[] = ["a"]
let ok: boolean = items.every((a, b, c) => true)`, "arrow callbacks take 1 or 2 parameters"},
		{"find wrong arity", `let items: string[] = ["a"]
let hit: string = items.find()`, "find() takes 1 arrow callback"},
		{"some block body", `let items: string[] = ["a"]
let ok: boolean = items.some(x => { return true })`, "some() predicate callback must be expression-bodied"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expectError(t, tt.src, tt.fragment)
		})
	}
}

func TestValidator_ListReduceAsListThenJoin(t *testing.T) {
	mustCheck(t, `let nums: number[] = [1, 2]
let lines: string = nums.reduce((acc, n) => [...acc, "x"], [] as string[]).join("\n")`)
}

func TestValidator_ListReduceBlockReturn(t *testing.T) {
	mustCheck(t, `let nums: number[] = [1, 2]
let product: number = nums.reduce((acc, n) => {
    return acc * n
}, 1)`)
}

func TestValidator_ListReduceObjectBlockReturn(t *testing.T) {
	mustCheck(t, `let words: string[] = ["apple"]
let counts = words.reduce((acc, word) => {
    acc[word] = (acc[word] || 0) + 1
    return acc
}, {})`)
}

func TestValidator_LogicalOrValueType(t *testing.T) {
	mustCheck(t, `let counts = {}
let word = "apple"
counts[word] = (counts[word] || 0) + 1`)
}

func TestValidator_ClassDeclValid(t *testing.T) {
	mustCheck(t, `class User {
    name: string
    constructor(name: string) {
        this.name = name
    }
    greet(): string {
        return "Hello, " + this.name
    }
    static label: string = "user"
}`)
}

func TestValidator_ClassAccessorsValid(t *testing.T) {
	mustCheck(t, `class User {
	name: string
	constructor(name: string) { this.name = name }
	get label(): string { return this.name }
	set label(value: string) { this.name = value }
}`)
	mustCheck(t, `class Metrics {
    static get count(): number { return 1 }
}
let count: number = Metrics.count`)
}

func TestValidator_ClassAccessorValidation(t *testing.T) {
	expectError(t, `class Circle {
    get area(value: number): number { return value }
}`, `getter "area" must not take parameters`)
	expectError(t, `class Circle {
    set area() { }
}`, `setter "area" must take exactly one parameter`)
	expectError(t, `class Circle {
    set area(value: number): number { return value }
}`, `setter "area" must not declare a return type`)
	expectError(t, `class Circle {
    area: number
    get area(): number { return 1 }
}`, `conflicts with field`)
	expectError(t, `class Circle {
    get area(): number { return 1 }
    get_area(): number { return 1 }
}`, `conflicts with accessor`)
	expectError(t, `class Counter {
    value: number
    get next(): number {
        this.value = this.value + 1
        return this.value
    }
}`, `getter "next" must not assign to this properties`)
	expectError(t, `class Counter {
    value: number
    next(): number {
        this.value = this.value + 1
        return this.value
    }
}`, `class method "next" returns a value and cannot assign to this properties`)
}

func TestValidator_TypeScriptClassModifiersFindIndexAndDestructure(t *testing.T) {
	mustCheck(t, `function run(board: string): number {
    class Game {
        readonly matrix: string[][]
        private static Deltas: Record<string, [number, number]> = { U: [-1, 0] }
        static find(matrix: string[][]): number {
            return matrix.findIndex(row => row.includes("@"))
        }
        getCellAt(matrix: string[][], pos: [number, number]): string | undefined {
            const [r, c] = pos
            return matrix[r]?.[c]
        }
    }
    const [dr, dc] = Game.Deltas["U"]
    return dr + dc
}`)
}

func TestValidator_ArrayFromMapIndexAndBlockReturn(t *testing.T) {
	mustCheck(t, `let height: number = 3
let count: number = 0
let rows: string[] = Array.from({ length: height }).map((_, i: number) => {
    if (++count % 2 !== 0) return "x"
    return "y" + i.toString()
})`)
}

func TestValidator_ArrayFromLengthShorthand(t *testing.T) {
	mustCheck(t, `let length: number = 2
let values: number[] = Array.from({ length })`)
}
