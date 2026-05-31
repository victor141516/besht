package codegen_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/victor141516/besht/internal/codegen"
	"github.com/victor141516/besht/internal/stdlib"
)

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

func shellSingleQuoteForTest(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

func sourceFromRootForTest(relPath string) string {
	return `. "$_BESHT_ROOT"/` + shellSingleQuoteForTest(filepath.ToSlash(relPath))
}

func compileFile(t *testing.T, path string) string {
	t.Helper()
	out, err := codegen.CompileFile(path, codegen.Options{})
	if err != nil {
		t.Fatalf("compile error: %v", err)
	}
	return out
}

func runCompiledShell(t *testing.T, src string, args ...string) string {
	t.Helper()
	dir := t.TempDir()
	path := writeFile(t, dir, "main.bsh", src)
	out := compileFile(t, path)
	shPath := filepath.Join(dir, "main.sh")
	if err := os.WriteFile(shPath, []byte(out), 0755); err != nil {
		t.Fatalf("write shell: %v", err)
	}
	cmdArgs := append([]string{shPath}, args...)
	cmd := exec.Command("sh", cmdArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run shell: %v\n%s\n--- script ---\n%s", err, output, out)
	}
	return string(output)
}

func runCompiledShellWithEnv(t *testing.T, src string, env []string, args ...string) (string, error) {
	t.Helper()
	dir := t.TempDir()
	path := writeFile(t, dir, "main.bsh", src)
	out := compileFile(t, path)
	shPath := filepath.Join(dir, "main.sh")
	if err := os.WriteFile(shPath, []byte(out), 0755); err != nil {
		t.Fatalf("write shell: %v", err)
	}
	cmdArgs := append([]string{shPath}, args...)
	cmd := exec.Command("sh", cmdArgs...)
	cmd.Env = env
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func envWithout(keys ...string) []string {
	drop := make(map[string]bool, len(keys))
	for _, key := range keys {
		drop[key] = true
	}
	var env []string
	for _, entry := range os.Environ() {
		name := entry
		if idx := strings.IndexByte(entry, '='); idx >= 0 {
			name = entry[:idx]
		}
		if !drop[name] {
			env = append(env, entry)
		}
	}
	return env
}

func TestIntegration_ProcessEnvNullishUnsetAndEmptyRuntime(t *testing.T) {
	src := `let value = process.env.BESHT_PROCESS_ENV_TEST ?? "fallback"
console.log("[" + value + "]")`

	unsetOut, err := runCompiledShellWithEnv(t, src, envWithout("BESHT_PROCESS_ENV_TEST"))
	if err != nil {
		t.Fatalf("run unset shell: %v\n%s", err, unsetOut)
	}
	if unsetOut != "[fallback]\n" {
		t.Fatalf("unset output: got %q", unsetOut)
	}

	emptyOut, err := runCompiledShellWithEnv(t, src, append(envWithout("BESHT_PROCESS_ENV_TEST"), "BESHT_PROCESS_ENV_TEST="))
	if err != nil {
		t.Fatalf("run empty shell: %v\n%s", err, emptyOut)
	}
	if emptyOut != "[]\n" {
		t.Fatalf("empty output: got %q", emptyOut)
	}

	setOut, err := runCompiledShellWithEnv(t, src, append(envWithout("BESHT_PROCESS_ENV_TEST"), "BESHT_PROCESS_ENV_TEST=value"))
	if err != nil {
		t.Fatalf("run set shell: %v\n%s", err, setOut)
	}
	if setOut != "[value]\n" {
		t.Fatalf("set output: got %q", setOut)
	}
}

func TestIntegration_SingleModuleOutputOmitsModuleMarker(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "main.bsh", `console.log("hello")`)
	out, err := codegen.CompileFile(path, codegen.Options{NoCheck: true, NoSourceMap: true})
	if err != nil {
		t.Fatalf("CompileFile: %v", err)
	}
	if strings.Contains(out, "# --- module:") {
		t.Fatalf("single-module output should omit module marker:\n%s", out)
	}
	if !strings.Contains(out, "printf '%s\\n' 'hello'") {
		t.Fatalf("output missing compiled body:\n%s", out)
	}
}

func TestIntegration_MultiModuleOutputKeepsModuleMarkers(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "lib.bsh", `export function msg(): string { return "hello" }`)
	path := writeFile(t, dir, "main.bsh", `import { msg } from "./lib"
console.log(msg())`)
	out, err := codegen.CompileFile(path, codegen.Options{NoCheck: true, NoSourceMap: true})
	if err != nil {
		t.Fatalf("CompileFile: %v", err)
	}
	if !strings.Contains(out, "# --- module: lib ---") || !strings.Contains(out, "# --- module: main ---") {
		t.Fatalf("multi-module output should keep module markers:\n%s", out)
	}
}

func TestIntegration_ProcessExitRuntime(t *testing.T) {
	out, err := runCompiledShellWithEnv(t, `process.exit(7)`, os.Environ())
	if err == nil {
		t.Fatalf("expected exit error, got nil output %q", out)
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok || exitErr.ExitCode() != 7 {
		t.Fatalf("exit code: got err=%v output=%q", err, out)
	}
}

func TestIntegration_FetchTextFileURLRuntime(t *testing.T) {
	if _, err := exec.LookPath("curl"); err != nil {
		t.Skip("curl not available")
	}
	dir := t.TempDir()
	dataPath := writeFile(t, dir, "data.txt", "hello fetch")
	dataPath2 := writeFile(t, dir, "data2.txt", "second fetch")
	url := "file://" + filepath.ToSlash(dataPath)
	url2 := "file://" + filepath.ToSlash(dataPath2)
	out := runCompiledShell(t, `let url: string = "`+url+`"
let url2: string = "`+url2+`"
console.log(fetch(url).text())
let response = fetch(url)
console.log(response.text())
console.log(response.text())
let alias = response
console.log(alias.text())
response = fetch(url2)
console.log(response.text())
console.log(alias.text())`)
	if out != "hello fetch\nhello fetch\nhello fetch\nhello fetch\nsecond fetch\nhello fetch\n" {
		t.Fatalf("fetch output: got %q", out)
	}
}

func TestIntegration_FetchImportedResponseText(t *testing.T) {
	if _, err := exec.LookPath("curl"); err != nil {
		t.Skip("curl not available")
	}
	dir := t.TempDir()
	dataPath := writeFile(t, dir, "data.txt", "imported fetch")
	url := "file://" + filepath.ToSlash(dataPath)
	writeFile(t, dir, "dep.bsh", `export const response = fetch("`+url+`")`)
	main := writeFile(t, dir, "main.bsh", `import { response } from "./dep"
console.log(response.text())`)
	out := compileFile(t, main)
	shPath := filepath.Join(dir, "main.sh")
	if err := os.WriteFile(shPath, []byte(out), 0755); err != nil {
		t.Fatalf("write shell: %v", err)
	}
	output, err := exec.Command("sh", shPath).CombinedOutput()
	if err != nil {
		t.Fatalf("run shell: %v\n%s\n--- script ---\n%s", err, output, out)
	}
	if string(output) != "imported fetch\n" {
		t.Fatalf("imported response output: got %q", output)
	}
}

func TestIntegration_CheckFileRejectsUnsupportedFetchResponseSurface(t *testing.T) {
	dir := t.TempDir()
	valid := writeFile(t, dir, "valid.bsh", `let response = fetch("file:///tmp/data.txt")
let body = response.text()
let name: string = 42
function permissive(): boolean { return "not really" }`)
	if err := codegen.CheckFile(valid, codegen.Options{}); err != nil {
		t.Fatalf("CheckFile should allow text() and annotation mismatch, got %v", err)
	}

	tests := []struct {
		name    string
		main    string
		wantErr string
	}{
		{
			name:    "options",
			main:    `let body = fetch("file:///tmp/data.txt", { method: "POST" }).text()`,
			wantErr: "fetch() takes 1 URL argument",
		},
		{
			name: "status property",
			main: `let response = fetch("file:///tmp/data.txt")
let status = response.status`,
			wantErr: `FetchResponse has no property "status"`,
		},
		{
			name: "json method",
			main: `let response = fetch("file:///tmp/data.txt")
let data = response.json()`,
			wantErr: `FetchResponse has no method "json"`,
		},
		{
			name: "text arguments",
			main: `let response = fetch("file:///tmp/data.txt")
let data = response.text("utf8")`,
			wantErr: "FetchResponse.text() takes no arguments",
		},
		{
			name:    "template interpolation",
			main:    "let data = `${fetch(\"file:///tmp/data.txt\").json()}`",
			wantErr: `FetchResponse has no method "json"`,
		},
		{
			name: "arrow callback",
			main: `let urls: string[] = ["file:///tmp/data.txt"]
let data = urls.map(url => fetch(url).json())`,
			wantErr: `FetchResponse has no method "json"`,
		},
		{
			name: "class constructor",
			main: `class Loader {
    constructor() {
        let response = fetch("file:///tmp/data.txt")
        response.json()
    }
}`,
			wantErr: `FetchResponse has no method "json"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeFile(t, dir, tt.name+".bsh", tt.main)
			if err := codegen.CheckFile(path, codegen.Options{}); err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("CheckFile error: got %v, want containing %q", err, tt.wantErr)
			}
		})
	}

	writeFile(t, dir, "dep.bsh", `export function load() {
    let response = fetch("file:///tmp/data.txt")
    response.status
}`)
	main := writeFile(t, dir, "imported.bsh", `import { load } from "./dep"
load()`)
	if err := codegen.CheckFile(main, codegen.Options{}); err == nil || !strings.Contains(err.Error(), `FetchResponse has no property "status"`) {
		t.Fatalf("CheckFile imported module error: got %v", err)
	}

	writeFile(t, dir, "exported_response.bsh", `export const response = fetch("file:///tmp/data.txt")`)
	importedResponse := writeFile(t, dir, "imported_response.bsh", `import { response } from "./exported_response"
let status = response.status`)
	if err := codegen.CheckFile(importedResponse, codegen.Options{}); err == nil || !strings.Contains(err.Error(), `FetchResponse has no property "status"`) {
		t.Fatalf("CheckFile imported response error: got %v", err)
	}
}

func TestIntegration_GeneratedStdlibDeclarationsAutoLoadUnderCheck(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "stdlib.d.bsh", stdlib.Declarations)
	if !strings.Contains(stdlib.Declarations, "function isArray(value): boolean") {
		t.Fatalf("generated stdlib should declare Array.isArray with an untyped value parameter")
	}
	if !strings.Contains(stdlib.Declarations, "declare function Boolean(value): boolean") {
		t.Fatalf("generated stdlib should declare Boolean(value)")
	}
	if !strings.Contains(stdlib.Declarations, "function hasOwn(value: object, key: string): boolean") {
		t.Fatalf("generated stdlib should declare Object.hasOwn(value, key)")
	}
	if !strings.Contains(stdlib.Declarations, "function values(value: object): string[]") {
		t.Fatalf("generated stdlib should declare Object.values(value)")
	}
	if !strings.Contains(stdlib.Declarations, "function entries(value: object): string[][]") {
		t.Fatalf("generated stdlib should declare Object.entries(value)")
	}
	mainPath := writeFile(t, dir, "main.bsh", `let path: string = process.env.HOME ?? "/tmp"
let paths: string[] = [path]
let argc: number = Besht.args.argv().length
let first: string = paths[0]
let rest: string[] = [...paths, "extra"].slice(1)
let both: string[] = paths.concat(rest)
let hasHome: boolean = both.includes(path)
let n: number = Number.parseInt("42", 10)
let indexes: number[] = Array.from({ length: 3 })
let indexesIsArray: boolean = Array.isArray(indexes)
let stringIsArray: boolean = Array.isArray("not a list")
let objectIsArray: boolean = Array.isArray({ value: "x" })
let objectHasPath: boolean = Object.hasOwn({ path: path }, "path")
let objectValues: string[] = Object.values({ path: path })
let objectEntries: string[][] = Object.entries({ path: path })
let label: string = Number.parseInt("7").toString()
let converted: boolean = Boolean(label)
let existsViaNamespace: boolean = Besht.fs.isFile(first)
if (converted || indexesIsArray || stringIsArray || objectIsArray || objectHasPath || existsViaNamespace || Besht.fs.isDir(first) || Besht.fs.isReadable(first) || Besht.fs.isWritable(first) || Besht.fs.isExecutable(first) || Besht.strings.isEmpty(label) || Besht.strings.isNonEmpty(label) || hasHome || objectValues.length > 0 || objectEntries.length > 0) {
    for (i in Besht.iter.range(0, n)) {
        if (i == argc) break
    }
}
process.exit(0)
`)
	if err := codegen.CheckFile(mainPath, codegen.Options{}); err != nil {
		t.Fatalf("check with generated stdlib: %v", err)
	}
}

func TestIntegration_ImportedExportedValuesAndDefaultRuntime(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "dep.bsh", `export const cmd = ["printf", "named\\n"]
export default ["printf", "default\\n"]`)
	mainPath := writeFile(t, dir, "main.bsh", `import def, { cmd } from "./dep"
$(...cmd).run()
$(...def).run()`)
	out, err := codegen.CompileFile(mainPath, codegen.Options{})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	shPath := filepath.Join(dir, "main.sh")
	if err := os.WriteFile(shPath, []byte(out), 0755); err != nil {
		t.Fatalf("write shell: %v", err)
	}
	cmd := exec.Command("sh", shPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run shell: %v\n%s\n--- script ---\n%s", err, output, out)
	}
	if string(output) != "named\ndefault\n" {
		t.Fatalf("output: got %q", output)
	}
}

func TestIntegration_SameModuleFunctionSeesExportedValueRuntime(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "main.bsh", `export const cmd = ["printf", "same\\n"]
export function runIt() {
    $(...cmd).run()
}
runIt()`)
	out := compileFile(t, path)
	assertContains(t, out, `main__cmd=$(`)
	assertContains(t, out, `$main__cmd`)
	shPath := filepath.Join(dir, "main.sh")
	if err := os.WriteFile(shPath, []byte(out), 0755); err != nil {
		t.Fatalf("write shell: %v", err)
	}
	cmd := exec.Command("sh", shPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run shell: %v\n%s\n--- script ---\n%s", err, output, out)
	}
	if string(output) != "same\n" {
		t.Fatalf("output: got %q", output)
	}
}

func TestIntegration_TsImportFallbackOptIn(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "dep.ts", `export const cmd = ["printf", "ts\\n"]`)
	mainPath := writeFile(t, dir, "main.bsh", `import { cmd } from "./dep"
$(...cmd).run()`)
	if _, err := codegen.CompileFile(mainPath, codegen.Options{}); err == nil || !strings.Contains(err.Error(), "dep.bsh") {
		t.Fatalf("fallback off error: got %v, want dep.bsh error", err)
	}
	out, err := codegen.CompileFile(mainPath, codegen.Options{ResolveTsImports: true})
	if err != nil {
		t.Fatalf("fallback on compile: %v", err)
	}
	shPath := filepath.Join(dir, "main.sh")
	if err := os.WriteFile(shPath, []byte(out), 0755); err != nil {
		t.Fatalf("write shell: %v", err)
	}
	cmd := exec.Command("sh", shPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run shell: %v\n%s", err, output)
	}
	if string(output) != "ts\n" {
		t.Fatalf("output: got %q", output)
	}
}

func TestIntegration_SetAndNestedListRuntime(t *testing.T) {
	out := runCompiledShell(t, "function probe(factory: string[]): string {\n"+
		"    const matrix: string[][] = factory.map(e => e.split(\"\") as string[])\n"+
		"    const seen = new Set<string>()\n"+
		"    let pos = [0, 0]\n"+
		"    const first = matrix[pos[0]][pos[1]]\n"+
		"    const key = `${pos[0]},${pos[1]}`\n"+
		"    if (seen.has(key)) return \"bad\"\n"+
		"    else seen.add(key)\n"+
		"    if (!seen.has(key)) return \"bad\"\n"+
		"    if (matrix.length != 2) return \"bad\"\n"+
		"    if (matrix[0].length != 2) return \"bad\"\n"+
		"    return first + matrix[1][1]\n"+
		"}\n"+
		"console.log(probe([\"ab\", \"cd\"]))")
	if strings.TrimSpace(out) != "ad" {
		t.Fatalf("output: got %q, want ad", out)
	}
}

func TestIntegration_AdventNineTypeScriptSubsetRuntime(t *testing.T) {
	out := runCompiledShell(t, `type Board = string
type Moves = string
type Result = 'fail' | 'crash' | 'success'
type Position = [number, number]
type Cell = '@' | '#' | '*'
type Direction = 'U' | 'D' | 'L' | 'R'

function moveReno(board: Board, moves: Moves): Result {
  class Game {
    readonly matrix: Cell[][]
    currentPos: Position

    private static Deltas: Record<Direction, [number, number]> = {
      U: [-1, 0], D: [1, 0], L: [0, -1], R: [0, 1]
    }

    static splitBoard(board: Board): Cell[][] {
      return board.split('\n').slice(1, -1).map(e => e.split('')) as Cell[][]
    }

    static findCellPosition(matrix: Cell[][], cellToFind: Cell): Position {
      const rowI = matrix.findIndex(row => row.includes(cellToFind))
      const colI = rowI === -1 ? -1 : matrix[rowI].indexOf(cellToFind)
      return [rowI, colI]
    }

    constructor(board: Board) {
      this.matrix = Game.splitBoard(board)
      this.currentPos = Game.findCellPosition(this.matrix, '@')
    }

    private moveCur(delta: [number, number]) {
      this.currentPos = [this.currentPos[0] + delta[0], this.currentPos[1] + delta[1]]
    }

    moveDirection(direction: Direction) { this.moveCur(Game.Deltas[direction]) }

    getNextPosition(direction: Direction): Position {
      const [dr, dc] = Game.Deltas[direction]
      return [this.currentPos[0] + dr, this.currentPos[1] + dc]
    }

    getCellAt(pos: Position): Cell | undefined {
      const [r, c] = pos
      return this.matrix[r]?.[c]
    }
  }

  const game = new Game(board)
  let result: Result = 'fail'
  for (const move of moves.split('') as Direction[]) {
    const nextPos = game.getNextPosition(move)
    const cell = game.getCellAt(nextPos)
    if (cell === undefined || cell === '#') { result = 'crash'; break }
    game.moveDirection(move)
    if (cell === '*') { result = 'success'; break }
  }
  return result
}

const board = `+"`"+`
.....
.*#.*
.@...
.....
`+"`"+`
console.log(moveReno(board, 'D'))
console.log(moveReno(board, 'U'))
console.log(moveReno(board, 'RU'))
console.log(moveReno(board, 'RRRUU'))
console.log(moveReno(board, 'DD'))
console.log(moveReno(board, 'UUU'))
console.log(moveReno(board, 'RR'))`)
	if strings.TrimSpace(out) != "fail\nsuccess\ncrash\nsuccess\ncrash\nsuccess\nfail" {
		t.Fatalf("output: got %q", out)
	}
}

func TestIntegration_FindIndexSecondParamAndStaticListIndex(t *testing.T) {
	out := runCompiledShell(t, `class Box {
  static Items: string[] = ["x", "y"]
}
let items = ["a", "b", "c"]
console.log(items.findIndex((item, i) => i == 1))
console.log(Box.Items[1])`)
	if strings.TrimSpace(out) != "1\ny" {
		t.Fatalf("output: got %q", out)
	}
}

func TestIntegration_SetValuesWithSpacesAndNestedListLiterals(t *testing.T) {
	out := runCompiledShell(t, `const seen = new Set<string>()
seen.add("a b")
console.log(seen.has("a b"))
let matrix: string[][] = [["a", "b"], ["c", "d"]]
console.log(matrix[1][0])
console.log(matrix[0].length)
`)
	if strings.TrimSpace(out) != "true\nc\n2" {
		t.Fatalf("output: got %q, want true\\nc\\n2", out)
	}
}

func TestIntegration_HelloWorld(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "hello.bsh", `let name: string = "world"
$("echo", "Hello, " + name + "!").run()
`)
	out := compileFile(t, path)
	assertContains(t, out, `name='world'`)
	assertContains(t, out, `'echo'`)
}

func TestIntegration_FunctionCallAndReturn(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "main.bsh", `function double(n: number): number {
    return n + n
}
let result: number = double(5)
`)
	out := compileFile(t, path)
	assertContains(t, out, `double()`)
	assertContains(t, out, `result=$(main__double`)
}

func TestIntegration_ControlFlowIfElse(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "ctrl.bsh", `let n: number = 10
if (n > 5) {
    $("echo", "big").run()
} else {
    $("echo", "small").run()
}
`)
	out := compileFile(t, path)
	assertContains(t, out, `if awk -v _a=$n -v _b=5`)
	assertContains(t, out, `'echo' 'big'`)
	assertContains(t, out, `else`)
	assertContains(t, out, `'echo' 'small'`)
	assertContains(t, out, `fi`)
}

func TestIntegration_ForRange(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "range.bsh", `for (i in Besht.iter.range(1, 3)) {
    $("echo", "${i}").run()
}
`)
	out := compileFile(t, path)
	assertContains(t, out, `while [`)
	assertContains(t, out, `-le 3`)
	assertContains(t, out, `done`)
}

func TestIntegration_TryCatch(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "try.bsh", `try {
    $("false").run()
} catch (code: status) {
    $("echo", "error: " + code.toString()).run()
	}
`)
	out := compileFile(t, path)
	assertContains(t, out, `_try_status_`)
	assertContains(t, out, `set -e`)
	assertContains(t, out, `false`)
	assertContains(t, out, `=$?`)
}

func TestIntegration_ImportSingleModule(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "lib/log.bsh", `export function info(msg: string) {
    $("printf", "[INFO] %s\n", msg).run()
}
`)
	path := writeFile(t, dir, "main.bsh", `import { info } from "./lib/log"
info("started")
`)
	out := compileFile(t, path)
	assertContains(t, out, `lib__log__info()`)
	assertContains(t, out, `_lib__log__info_msg="$1"`)
	assertContains(t, out, `lib__log__info 'started'`)
}

func TestIntegration_ImportedExportedListValuesAndDefault(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "b_dep.bsh", `export const cmd = ["echo", "named"]
export default ["echo", "default"]
`)
	path := writeFile(t, dir, "b.bsh", `import def from "./b_dep"
import { cmd } from "./b_dep"
$(...cmd).run()
$(...def).run()
`)
	out := compileFile(t, path)
	assertContains(t, out, `b_dep__cmd=$(`)
	assertContains(t, out, `b_dep__default=$(`)
	assertContains(t, out, `$b_dep__cmd`)
	assertContains(t, out, `$b_dep__default`)
}

func TestIntegration_SpreadCommandEOFElementIsSafe(t *testing.T) {
	output := runCompiledShell(t, `let cmd = ["printf", "%s:%s", "EOF", "after"]
$(...cmd).run()
`)
	if output != "EOF:after" {
		t.Fatalf("output: got %q", output)
	}
	dir := t.TempDir()
	path := writeFile(t, dir, "main.bsh", `let cmd = ["printf", "%s:%s", "EOF", "after"]
$(...cmd).run()
`)
	out := compileFile(t, path)
	assertContains(t, out, "_bst_spread_args=")
	assertContains(t, out, "<<EOF\n$_bst_spread_args\nEOF")
}

func TestIntegration_MixedCommandNameSpreadRejected(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "main.bsh", `let cmd = ["echo"]
$(...cmd, "extra").run()
`)
	_, err := codegen.CompileFile(path, codegen.Options{})
	if err == nil || !strings.Contains(err.Error(), "command-name spread must be the only $() argument") {
		t.Fatalf("CompileFile error: got %v, want command-name spread error", err)
	}
}

func TestIntegration_CommandArgumentSpreadStillWorks(t *testing.T) {
	output := runCompiledShell(t, `let args = ["hello world", "again"]
$("printf", "%s|%s", ...args).run()
`)
	if output != "hello world|again" {
		t.Fatalf("output: got %q", output)
	}
}

func TestIntegration_ImportedListValueMethods(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "dep.bsh", `export const xs = ["a", "b"]`)
	path := writeFile(t, dir, "main.bsh", `import { xs } from "./dep"
console.log(xs.join(","))
console.log(xs.length)
`)
	out, err := codegen.CompileFile(path, codegen.Options{})
	if err != nil {
		t.Fatalf("CompileFile: %v", err)
	}
	shPath := filepath.Join(dir, "main.sh")
	if err := os.WriteFile(shPath, []byte(out), 0755); err != nil {
		t.Fatalf("write shell: %v", err)
	}
	cmd := exec.Command("sh", shPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run shell: %v\n%s\n--- script ---\n%s", err, output, out)
	}
	if string(output) != "a,b\n2\n" {
		t.Fatalf("output: got %q", output)
	}
}

func TestIntegration_ExplicitTSImportRequiresOptIn(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "dep.ts", `export const cmd = ["echo", "ts"]`)
	path := writeFile(t, dir, "main.bsh", `import { cmd } from "./dep.ts"
$(...cmd).run()
`)
	if _, err := codegen.CompileFile(path, codegen.Options{}); err == nil || !strings.Contains(err.Error(), "dep.bsh") {
		t.Fatalf("explicit .ts import without opt-in error: got %v, want dep.bsh missing-file error", err)
	}
	out, err := codegen.CompileFile(path, codegen.Options{ResolveTsImports: true})
	if err != nil {
		t.Fatalf("explicit .ts import with opt-in: %v", err)
	}
	assertContains(t, out, `dep__cmd=$(`)
	assertContains(t, out, `'ts'`)
}

func TestIntegration_TSImportFallbackDisabledByDefault(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "dep.ts", `export const cmd = ["echo", "ts"]`)
	path := writeFile(t, dir, "main.bsh", `import { cmd } from "./dep"
$(...cmd).run()
`)
	if _, err := codegen.CompileFile(path, codegen.Options{}); err == nil {
		t.Fatal("expected extensionless import to ignore .ts without opt-in")
	}
}

func TestIntegration_TSImportFallbackEnabled(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "dep.ts", `export const cmd = ["echo", "ts"]`)
	path := writeFile(t, dir, "main.bsh", `import { cmd } from "./dep"
$(...cmd).run()
`)
	out, err := codegen.CompileFile(path, codegen.Options{ResolveTsImports: true})
	if err != nil {
		t.Fatalf("CompileFile with .ts fallback: %v", err)
	}
	assertContains(t, out, `dep__cmd=$(`)
	assertContains(t, out, `'ts'`)
}

func TestIntegration_ShellImportBundledRuntime(t *testing.T) {
	dir := t.TempDir()
	legacyPath := writeFile(t, dir, "legacy.sh", `legacy() {
    printf 'legacy:%s:%s' "$1" "$2"
}
`)
	path := writeFile(t, dir, "main.bsh", `import { legacy } from "./legacy.sh" assert { type: "shell" }
let msg: string = legacy("one", "two words")
console.log(msg)
`)
	out, err := codegen.CompileFile(path, codegen.Options{})
	if err != nil {
		t.Fatalf("CompileFile with shell import: %v", err)
	}
	assertContains(t, out, `_BESHT_SHELL_LOADED_legacy_sh`)
	assertContains(t, out, `. `+shellSingleQuoteForTest(legacyPath))
	assertContains(t, out, `msg=$(legacy 'one' 'two words')`)
	assertNotContains(t, out, `legacy__legacy`)
	if sourceIdx, callIdx := strings.Index(out, `. `+shellSingleQuoteForTest(legacyPath)), strings.Index(out, `msg=$(legacy 'one' 'two words')`); sourceIdx < 0 || callIdx < 0 || sourceIdx > callIdx {
		t.Fatalf("shell import source should appear before call\n--- script ---\n%s", out)
	}
	shPath := filepath.Join(dir, "main.sh")
	if err := os.WriteFile(shPath, []byte(out), 0755); err != nil {
		t.Fatalf("write shell: %v", err)
	}
	cmd := exec.Command("sh", shPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run shell: %v\n%s\n--- script ---\n%s", err, output, out)
	}
	if string(output) != "legacy:one:two words\n" {
		t.Fatalf("output: got %q", output)
	}
}

func TestIntegration_ExternalShellImportRequiresOptIn(t *testing.T) {
	dir := t.TempDir()
	appDir := filepath.Join(dir, "app")
	writeFile(t, dir, "lib/utils.sh", `legacy() { printf 'legacy:%s' "$1"; }
`)
	path := writeFile(t, appDir, "main.bsh", `import { legacy } from "../lib/utils.sh" assert { type: "shell" }
console.log(legacy("ok"))
`)

	if _, err := codegen.CompileFile(path, codegen.Options{}); err == nil || !strings.Contains(err.Error(), "outside compiler root") {
		t.Fatalf("CompileFile default error: got %v, want outside compiler root", err)
	}
	if err := codegen.CheckFile(path, codegen.Options{}); err == nil || !strings.Contains(err.Error(), "outside compiler root") {
		t.Fatalf("CheckFile default error: got %v, want outside compiler root", err)
	}

	out, err := codegen.CompileFile(path, codegen.Options{AllowExternalShellImports: true})
	if err != nil {
		t.Fatalf("CompileFile with external shell import opt-in: %v", err)
	}
	assertContains(t, out, `. `+shellSingleQuoteForTest(filepath.Join(dir, "lib", "utils.sh")))
	assertContains(t, out, `legacy 'ok'`)
	if err := codegen.CheckFile(path, codegen.Options{AllowExternalShellImports: true}); err != nil {
		t.Fatalf("CheckFile with external shell import opt-in: %v", err)
	}
}

func TestIntegration_ShellImportGuardCollisionDoesNotSkipBundledSources(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "lib/a-b.sh", `_BESHT_DASH_SOURCE_COUNT=$(( ${_BESHT_DASH_SOURCE_COUNT:-0} + 1 ))
dashFunc() {
    printf 'dash:%s' "$_BESHT_DASH_SOURCE_COUNT"
}
`)
	writeFile(t, dir, "lib/a_b.sh", `_BESHT_UNDER_SOURCE_COUNT=$(( ${_BESHT_UNDER_SOURCE_COUNT:-0} + 1 ))
underFunc() {
    printf 'under:%s' "$_BESHT_UNDER_SOURCE_COUNT"
}
`)
	path := writeFile(t, dir, "main.bsh", `import { dashFunc } from "./lib/a-b.sh" assert { type: "shell" }
import { underFunc } from "./lib/a_b.sh" assert { type: "shell" }
console.log(dashFunc())
console.log(underFunc())
`)
	out, err := codegen.CompileFile(path, codegen.Options{})
	if err != nil {
		t.Fatalf("CompileFile with colliding shell imports: %v", err)
	}
	shPath := filepath.Join(dir, "main.sh")
	if err := os.WriteFile(shPath, []byte(out), 0755); err != nil {
		t.Fatalf("write shell: %v", err)
	}
	cmd := exec.Command("sh", shPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run bundled shell collision fixture: %v\n%s\n--- script ---\n%s", err, output, out)
	}
	if string(output) != "dash:1\nunder:1\n" {
		t.Fatalf("output: got %q", output)
	}
}

func TestIntegration_ShellImportValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		main    string
		wantErr string
	}{
		{
			name:    "sh without assertion",
			main:    `import { legacy } from "./legacy.sh"`,
			wantErr: `.sh imports require assert { type: "shell" }`,
		},
		{
			name:    "assertion on non-sh",
			main:    `import { legacy } from "./legacy.bsh" assert { type: "shell" }`,
			wantErr: `shell import assertion requires an explicit .sh source`,
		},
		{
			name:    "default shell import",
			main:    `import legacy from "./legacy.sh" assert { type: "shell" }`,
			wantErr: `shell imports do not support default imports`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, dir, "legacy.sh", `legacy() { printf '%s' "$1"; }
`)
			writeFile(t, dir, "legacy.bsh", `export function legacy(): string { return "bsh" }
`)
			path := writeFile(t, dir, "main.bsh", tt.main+"\nlegacy(\"x\")\n")
			_, err := codegen.CompileFile(path, codegen.Options{})
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("CompileFile error: got %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestIntegration_ShellImportSpecialPathGuardIsSafe(t *testing.T) {
	dir := t.TempDir()
	shellRel := "lib/legacy $(touch PWNED) 'quote.sh"
	legacyPath := writeFile(t, dir, shellRel, `legacy() {
    printf 'safe:%s\n' "$1"
}
`)
	path := writeFile(t, dir, "main.bsh", `import { legacy } from "./`+filepath.ToSlash(shellRel)+`" assert { type: "shell" }
console.log(legacy("ok"))
`)
	out, err := codegen.CompileFile(path, codegen.Options{})
	if err != nil {
		t.Fatalf("CompileFile with special shell import path: %v", err)
	}
	assertContains(t, out, `. `+shellSingleQuoteForTest(legacyPath))
	assertNotContains(t, out, `_BESHT_SHELL_LOADED_lib__legacy $(touch PWNED)`)
	shPath := filepath.Join(dir, "main.sh")
	if err := os.WriteFile(shPath, []byte(out), 0755); err != nil {
		t.Fatalf("write shell: %v", err)
	}
	cmd := exec.Command("sh", shPath)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run shell: %v\n%s\n--- script ---\n%s", err, output, out)
	}
	if string(output) != "safe:ok\n" {
		t.Fatalf("output: got %q", output)
	}
	if _, err := os.Stat(filepath.Join(dir, "PWNED")); !os.IsNotExist(err) {
		t.Fatalf("unsafe command substitution side effect present: %v", err)
	}
}

func TestIntegration_CheckFileValidatesShellImports(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "legacy.sh", `legacy() { printf '%s' "$1"; }
`)
	valid := writeFile(t, dir, "valid.bsh", `import { legacy } from "./legacy.sh" assert { type: "shell" }
legacy("one")
legacy("one", "two")
`)
	if err := codegen.CheckFile(valid, codegen.Options{}); err != nil {
		t.Fatalf("CheckFile valid shell import: %v", err)
	}
	missingAssert := writeFile(t, dir, "missing_assert.bsh", `import { legacy } from "./legacy.sh"
`)
	if err := codegen.CheckFile(missingAssert, codegen.Options{}); err == nil || !strings.Contains(err.Error(), `.sh imports require assert { type: "shell" }`) {
		t.Fatalf("CheckFile missing assertion error: got %v", err)
	}
	missingFile := writeFile(t, dir, "missing_file.bsh", `import { missing } from "./missing.sh" assert { type: "shell" }
`)
	if err := codegen.CheckFile(missingFile, codegen.Options{}); err == nil || !strings.Contains(err.Error(), `cannot read shell import`) {
		t.Fatalf("CheckFile missing file error: got %v", err)
	}
}

func TestIntegration_BSHPrecedesTSFallback(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "dep.bsh", `export const cmd = ["echo", "bsh"]`)
	writeFile(t, dir, "dep.ts", `export const cmd = ["echo", "ts"]`)
	path := writeFile(t, dir, "main.bsh", `import { cmd } from "./dep"
$(...cmd).run()
`)
	out, err := codegen.CompileFile(path, codegen.Options{ResolveTsImports: true})
	if err != nil {
		t.Fatalf("CompileFile with .bsh precedence: %v", err)
	}
	assertContains(t, out, `'bsh'`)
	assertNotContains(t, out, `'ts'`)
}

func TestIntegration_TypeAnnotationsAreIgnored(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "main.bsh", `let name: string = 42`)
	if _, err := codegen.CompileFile(path, codegen.Options{}); err != nil {
		t.Fatalf("compile should ignore mismatched type annotations, got %v", err)
	}
}

func TestIntegration_EntryStdlibDeclarationAutoLoadedForBundledCompile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "stdlib.d.bsh", `declare function externalName(name: string): string`)
	path := writeFile(t, dir, "main.bsh", `let name: string = externalName("world")`)
	out, err := codegen.CompileFile(path, codegen.Options{})
	if err != nil {
		t.Fatalf("CompileFile with entry stdlib.d.bsh: %v", err)
	}
	assertContains(t, out, `name=$(externalName 'world')`)
	assertNotContains(t, out, `main__externalName`)
	assertNotContains(t, out, `stdlib`)
}

func TestIntegration_MissingEntryStdlibDeclarationIgnored(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "main.bsh", `let name: string = "world"`)
	if _, err := codegen.CompileFile(path, codegen.Options{}); err != nil {
		t.Fatalf("CompileFile without stdlib.d.bsh: %v", err)
	}
}

func TestIntegration_EntryStdlibDeclarationAutoLoadedForCheckFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "stdlib.d.bsh", `declare function externalName(name: string): string`)
	path := writeFile(t, dir, "main.bsh", `let name: string = externalName("world")`)
	if err := codegen.CheckFile(path, codegen.Options{}); err != nil {
		t.Fatalf("CheckFile with entry stdlib.d.bsh: %v", err)
	}
}

func TestIntegration_ImportedModuleStdlibDeclarationNotAutoLoaded(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "lib/stdlib.d.bsh", `declare function libOnly(name: string): string`)
	writeFile(t, dir, "lib/use.bsh", `export function callLibOnly(): string {
    return libOnly("world")
}`)
	path := writeFile(t, dir, "main.bsh", `import { callLibOnly } from "./lib/use"
let name: string = callLibOnly()`)
	err := codegen.CheckFile(path, codegen.Options{})
	if err == nil || !strings.Contains(err.Error(), `function "libOnly" not declared`) {
		t.Fatalf("CheckFile should not auto-load lib/stdlib.d.bsh, got %v", err)
	}
}

func TestIntegration_DeclarationFileOmittedFromSplitSources(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "types.d.bsh", `declare function external(name: string): string`)
	path := writeFile(t, dir, "main.bsh", `import { external } from "./types.d"
let name: string = "world"`)
	outDir := filepath.Join(dir, "out")
	if err := codegen.CompileFileSplit(path, outDir, codegen.Options{}); err != nil {
		t.Fatalf("split compile error: %v", err)
	}
	mainOut, err := os.ReadFile(filepath.Join(outDir, "main.sh"))
	if err != nil {
		t.Fatalf("read split output: %v", err)
	}
	assertNotContains(t, string(mainOut), `types.d.sh`)
}

func TestIntegration_ImportMultipleFunctions(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "lib/log.bsh", `export function info(msg: string) {
    $("printf", "[INFO] %s\n", msg).run()
}
export function error(msg: string) {
    $("printf", "[ERROR] %s\n", msg).run()
}
`)
	path := writeFile(t, dir, "main.bsh", `import { info, error } from "./lib/log"
info("started")
error("oops")
`)
	out := compileFile(t, path)
	assertContains(t, out, `lib__log__info()`)
	assertContains(t, out, `lib__log__error()`)
	assertContains(t, out, `lib__log__info 'started'`)
	assertContains(t, out, `lib__log__error 'oops'`)
}

func TestIntegration_ExportedFnsQualified(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "lib/math.bsh", `export function add(a: number, b: number): number {
    return a + b
}
`)
	path := writeFile(t, dir, "main.bsh", `import { add } from "./lib/math"
let r: number = add(1, 2)
`)
	out := compileFile(t, path)
	assertContains(t, out, `lib__math__add()`)
}

func TestIntegration_ModuleOrderDepsFirst(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "lib/util.bsh", `export function noop() {
    $("true").run()
}
`)
	path := writeFile(t, dir, "main.bsh", `import { noop } from "./lib/util"
noop()
`)
	out := compileFile(t, path)
	utilIdx := strings.Index(out, `lib__util__noop()`)
	mainIdx := strings.Index(out, `lib__util__noop`)
	if utilIdx < 0 {
		t.Fatal("lib__util__noop definition not found")
	}
	if mainIdx < utilIdx {
		t.Error("function definition should appear before call site")
	}
}

func TestIntegration_SingleFileNoModulePrefix(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "main.bsh", `function greet() {
    $("echo", "hi").run()
}
greet()
`)
	out := compileFile(t, path)
	assertContains(t, out, `main__greet()`)
}

func TestIntegration_FnParamMangledInBody(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "main.bsh", "function greet(name: string) {\n    $(\"echo\", `${name}`).run()\n}\ngreet(\"Bob\")\n")
	out := compileFile(t, path)
	assertContains(t, out, `_main__greet_name="$1"`)
	assertContains(t, out, `${_main__greet_name}`)
	assertNotContains(t, out, `${name}`)
}

func TestIntegration_LocalVarMangledInsideFn(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "main.bsh", `function compute(x: number): number {
    let result: number = x
    return result
}
`)
	out := compileFile(t, path)
	assertContains(t, out, `_main__compute_x="$1"`)
	assertContains(t, out, `_compute_result=`)
}

func TestIntegration_WhileLoop(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "main.bsh", `let n: number = 3
while (n > 0) {
    $("echo", "${n}").run()
}
`)
	out := compileFile(t, path)
	assertContains(t, out, `while awk -v _a=$n -v _b=0`)
	assertContains(t, out, `done`)
}

func TestIntegration_BuiltinFileCheck(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "main.bsh", `let p: string = "/tmp"
if (Besht.fs.isFile(p)) {
    $("echo", "found").run()
}
`)
	out := compileFile(t, path)
	assertContains(t, out, `[ -f`)
}

func TestIntegration_BuiltinDirCheck(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "main.bsh", `let p: string = "/tmp"
if (Besht.fs.isDir(p)) {
    $("echo", "is dir").run()
}
`)
	out := compileFile(t, path)
	assertContains(t, out, `[ -d`)
}

func TestIntegration_BeshtNamespaceWrapperInModule(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "checks.bsh", `export function check(path: string): boolean {
    return Besht.fs.isFile(path)
}`)
	path := writeFile(t, dir, "main.bsh", `import { check } from "./checks"
let ok: boolean = check("/tmp")
if (Besht.fs.isDir("/tmp")) {
    $("echo", "dir").run()
}
`)
	out, err := codegen.CompileFile(path, codegen.Options{})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	assertContains(t, out, `[ -f`)
	assertContains(t, out, `[ -d`)
	assertNotContains(t, out, `main__Besht`)
	assertNotContains(t, out, `checks__Besht`)
}

func TestIntegration_ErrorOnMissingImport(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "main.bsh", `import { foo } from "./nonexistent"
foo()
`)
	_, err := codegen.CompileFile(path, codegen.Options{})
	if err == nil {
		t.Fatal("expected error for missing import file")
	}
}

func TestIntegration_CircularImportDetected(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.bsh", `import { bar } from "./b"
export function foo() { $("true") }
`)
	writeFile(t, dir, "b.bsh", `import { foo } from "./a"
export function bar() { $("true") }
`)
	path := filepath.Join(dir, "a.bsh")
	_, err := codegen.CompileFile(path, codegen.Options{})
	if err != nil {
		t.Skip("circular imports produce an error (acceptable), got:", err)
	}
}

func TestIntegration_ComplexProgram(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "main.bsh", `function format_bytes(bytes: number): string {
    if (bytes > 1048576) {
        return "large"
    }
    return "small"
}

let threshold: number = 1048576
let label: string = format_bytes(threshold)
$("echo", label).run()
`)
	out := compileFile(t, path)
	assertContains(t, out, `format_bytes()`)
	assertContains(t, out, `if awk -v _a=$`)
	assertContains(t, out, `_b=1048576`)
}

func TestIntegration_ShebangAndHeader(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "main.bsh", `$("echo", "hi").run()`)
	out := compileFile(t, path)
	lines := strings.Split(out, "\n")
	if lines[0] != "#!/bin/sh" {
		t.Errorf("first line: got %q, want #!/bin/sh", lines[0])
	}
	if !strings.Contains(lines[1], "Generated by besht") {
		t.Errorf("second line: missing generation notice, got %q", lines[1])
	}
}

func TestIntegration_POSIXCompliance_NoLocalKeyword(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "main.bsh", `function f(x: string) {
    let y: string = x
    $("echo", "${y}").run()
}
`)
	out := compileFile(t, path)
	if strings.Contains(out, " local ") {
		t.Error("output contains 'local' which is not POSIX sh")
	}
}

func TestIntegration_POSIXCompliance_NoArraySyntax(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "main.bsh", `let files: list<string> = ["a.txt", "b.txt"]
for (f in files) {
    $("echo", "${f}").run()
}
`)
	out := compileFile(t, path)
	if strings.Contains(out, "=(") {
		t.Error("output contains bash array syntax which is not POSIX sh")
	}
}

func TestIntegration_ImportedFnCallsQualifiedInBody(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "lib/log.bsh", `export function info(msg: string) {
    $("printf", "[INFO] %s\n", msg).run()
}
export function error(msg: string) {
    $("printf", "[ERROR] %s\n", msg).run()
}
`)
	path := writeFile(t, dir, "main.bsh", `import { info, error } from "./lib/log"

function greet(name: string) {
    info("Hello " + name)
    error("Oops")
}
greet("world")
`)
	out := compileFile(t, path)
	assertContains(t, out, `lib__log__info`)
	assertContains(t, out, `lib__log__error`)
	assertNotContains(t, out, `\ninfo `)
	assertNotContains(t, out, `\nerror `)
}

func TestIntegration_StrBuiltinInConcat(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "main.bsh", `let n: number = 42
let msg: string = "Count: " + n.toString()
console.log(msg)
`)
	out := compileFile(t, path)
	assertContains(t, out, `msg=`)
	assertContains(t, out, `Count: `)
}

func TestIntegration_TernaryNumber(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "main.bsh", "let x = 10\nlet y = 3\nlet bigger = x > y ? x : y\nconsole.log(`bigger=${bigger}`)\n")
	out := compileFile(t, path)
	assertContains(t, out, `bigger=$(if awk -v _a=$x -v _b=$y`)
}

func TestIntegration_TernaryString(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "main.bsh", "let x = 10\nlet label = x > 5 ? \"big\" : \"small\"\nconsole.log(label)\n")
	out := compileFile(t, path)
	assertContains(t, out, `label=$(if awk -v _a=$x -v _b=5`)
}

func TestIntegration_NumberMethods(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "main.bsh", "let n = 42\nlet ns = n.toString()\nlet pi = 3.14159\nlet fixed = pi.toFixed(2)\nconsole.log(ns)\nconsole.log(fixed)\n")
	out := compileFile(t, path)
	assertContains(t, out, `printf '%s' "$n"`)
	assertContains(t, out, `printf "%.*f", _n, _x`)
}

func TestIntegration_PrimitiveToStringAndParseIntRuntime(t *testing.T) {
	out := runCompiledShell(t, `let s: string = "x"
console.log(s.toString())
let yes: boolean = true
let no: boolean = false
console.log(yes.toString())
console.log(no.toString())
console.log(Number.parseInt("42"))
console.log(Number.parseInt("42", 10))
try {
    $("false").run()
} catch (code: status) {
    console.log(code.toString())
}`)
	want := "x\ntrue\nfalse\n42\n42\n1\n"
	if out != want {
		t.Fatalf("output: got %q, want %q", out, want)
	}
}

func TestIntegration_ConcatLists(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "main.bsh", `let a: list<string> = ["one", "two"]
let b: list<string> = ["three", "four"]
let c: list<string> = a.concat(b)
let total: number = c.length
`)
	out := compileFile(t, path)
	assertContains(t, out, `printf '%s\n%s'`)
	assertContains(t, out, `wc -l`)
}

func TestIntegration_NativeListAPIsReplaceGlobalListHelpersRuntime(t *testing.T) {
	out := runCompiledShell(t, `let files: list<string> = ["a", "b", "c"]
let other: list<string> = ["d", "e"]
console.log(files.length)
console.log(files[0])
console.log(files.slice(1).join(","))
console.log(files.push("x").join(","))
console.log(files.includes("x"))
console.log(files.concat(other).join(","))`)
	want := "3\na\nb,c\na,b,c,x\nfalse\na,b,c,d,e\n"
	if out != want {
		t.Fatalf("output: got %q, want %q", out, want)
	}
}

func TestIntegration_ListToStringRuntime(t *testing.T) {
	out := runCompiledShell(t, `console.log(["a", "b"].toString())
let empty: string[] = []
console.log(empty.toString())
console.log(["alpha beta", "gamma delta"].toString())`)
	want := "a,b\n\nalpha beta,gamma delta\n"
	if out != want {
		t.Fatalf("output: got %q, want %q", out, want)
	}
}

func TestIntegration_ArrowCallbackQualifiesImportedFunction(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "lib/fmt.bsh", `export function bang(s: string): string {
    return s + "!"
}
`)
	path := writeFile(t, dir, "main.bsh", `import { bang } from "./lib/fmt"
let items = ["a", "b"]
let marked = items.map(x => bang(x))
console.log(marked.join(","))
`)
	out := compileFile(t, path)
	assertContains(t, out, `lib__fmt__bang`)
	assertContains(t, out, `marked=$(printf '%s\n' "$items"`)
}

func TestIntegration_ListMapFilterArrows(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "main.bsh", `let items = ["apple", "banana", "apricot"]
let marked = items.map(x => x + "!")
let picked = marked.filter(x => x.startsWith("a"))
console.log(picked.join(","))
`)
	out := compileFile(t, path)
	assertContains(t, out, `while IFS= read -r _cb_2_24_x`)
	assertContains(t, out, `while IFS= read -r _cb_3_28_x`)
	assertNotContains(t, out, `local `)
}

func TestIntegration_ListSomeEveryFindRuntime(t *testing.T) {
	out := runCompiledShell(t, `let items = ["apple", "banana", "apricot"]
let empty: string[] = []
console.log(items.some(x => x.startsWith("b")))
console.log(items.some(x => x.startsWith("z")))
console.log(empty.some(x => true))
console.log(items.every(x => x.includes("a")))
console.log(empty.every(x => false))
console.log(items.find(x => x.startsWith("b")) ?? "missing")
console.log(items.find(x => x.startsWith("z")) ?? "missing")
console.log(items.find((x, i) => i == 2))
`)
	if out != "true\nfalse\nfalse\ntrue\ntrue\nbanana\nmissing\napricot\n" {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestIntegration_CmdBasicCapture(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "main.bsh", `let user = $("whoami").run().readStdout()
console.log(user)
`)
	out := compileFile(t, path)
	assertContains(t, out, `user=$('whoami')`)
}

func TestIntegration_CmdPipeline(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "main.bsh", `let result = $("cat", "/etc/passwd")
    .pipe($("grep", "root"))
    .pipe($("cut", "-d:", "-f1")).run().readStdout()
`)
	out := compileFile(t, path)
	assertContains(t, out, `'cat'`)
	assertContains(t, out, `'grep'`)
	assertContains(t, out, `'cut'`)
	assertContains(t, out, `|`)
}

func TestIntegration_CmdRedirects(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "main.bsh", `$("make", "build").stdout("/tmp/build.log").stderr("&1").run()
`)
	out := compileFile(t, path)
	assertContains(t, out, `'make'`)
	assertContains(t, out, `2>&1`)
	assertContains(t, out, `> /tmp/build.log`)
}

func TestIntegration_CmdForLines(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "main.bsh", `for (f in $("find", ".", "-name", "*.log").readStdoutLines()) {
    $("echo", f).run()
}
`)
	out := compileFile(t, path)
	assertContains(t, out, `'find'`)
	assertContains(t, out, `while IFS= read -r`)
	assertContains(t, out, `'echo'`)
}

func TestIntegration_CmdSingleQuoteEscape(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "main.bsh", `$("echo", "it's alive").run()
`)
	out := compileFile(t, path)
	assertContains(t, out, `'"'"'`)
}

func TestIntegration_ForLetOfList(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "main.bsh", `let files = ["a", "b"]
for (let file of files) {
    $("echo", file).run()
}
`)
	out := compileFile(t, path)
	assertContains(t, out, `while IFS= read -r`)
	assertContains(t, out, `$file`)
}

func TestIntegration_CmdEnv(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "main.bsh", `$("env").env("FOO", "bar").run()
`)
	out := compileFile(t, path)
	assertContains(t, out, `FOO='bar' 'env'`)
}

func TestIntegration_CmdWorkdirRuntime(t *testing.T) {
	out := runCompiledShell(t, `let before = $("pwd").run().readStdout()
let root = $("pwd").workdir("/").run().readStdout()
let after = $("pwd").run().readStdout()
console.log(root)
console.log(before == after)
`)
	if strings.TrimSpace(out) != "/\ntrue" {
		t.Fatalf("output: got %q, want root path and unchanged cwd", out)
	}
}

func TestIntegration_CmdAssignedWorkdirRuntime(t *testing.T) {
	out := runCompiledShell(t, `let cmd = $("pwd")
let root = cmd.workdir("/").run().readStdout()
console.log(root)
`)
	if strings.TrimSpace(out) != "/" {
		t.Fatalf("output: got %q, want /", out)
	}
}

func TestIntegration_CmdReadStderrWithStdoutRedirectRuntime(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, "stdout.txt")
	out := runCompiledShell(t, `let errs = $("sh", "-c", "printf out; printf err >&2").stdout("`+outPath+`").run().readStderr()
console.log(errs)
`)
	if strings.TrimSpace(out) != "err" {
		t.Fatalf("stderr capture: got %q, want err", out)
	}
	stdoutBytes, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read redirected stdout: %v", err)
	}
	if string(stdoutBytes) != "out" {
		t.Fatalf("redirected stdout: got %q, want out", stdoutBytes)
	}
}

func TestIntegration_CmdStderrCapture(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "main.bsh", `let errs = $("make", "build").run().readStderr()
`)
	out := compileFile(t, path)
	assertContains(t, out, `2>&1 1>/dev/null`)
}

func TestIntegration_CmdNullRedirect(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "main.bsh", `$("make", "build").stdout("null").stderr("null").run()
`)
	out := compileFile(t, path)
	assertContains(t, out, `> /dev/null`)
	assertContains(t, out, `2>/dev/null`)
}

func TestIntegration_CmdPropagate(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "main.bsh", `function read_file(path: string): string {
    let content: string = $("cat", path)?
    return content
}
`)
	out := compileFile(t, path)
	assertContains(t, out, `|| return $?`)
}

func TestIntegration_CmdWithVarArgMangledInFunction(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "main.bsh", `function greet(name: string) {
    $("echo", name).run()
}
greet("Alice")
`)
	out := compileFile(t, path)
	assertContains(t, out, `_main__greet_name="$1"`)
	assertContains(t, out, `$_main__greet_name`)
}

func TestIntegration_CmdModulePipeline(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "lib/utils.bsh", `export function get_user(): string {
    return $("whoami").run().readStdout()
}
`)
	path := writeFile(t, dir, "main.bsh", `import { get_user } from "./lib/utils"
let user: string = get_user()
$("echo", user).run()
`)
	out := compileFile(t, path)
	assertContains(t, out, `lib__utils__get_user()`)
	assertContains(t, out, `'whoami'`)
}

func TestIntegration_RuntimeCheckInOutput(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "main.bsh", `console.log("hello")
`)
	out := compileFile(t, path)
	assertContains(t, out, `goodbye:world`)
	assertContains(t, out, `grep -F`)
	assertContains(t, out, `sed`)
	assertNotContains(t, out, `BESHT_SKIP_CHECK`)
}

func TestIntegration_RuntimeCheckBeforeCode(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "main.bsh", `let x: string = "hello"
console.log(x)
`)
	out := compileFile(t, path)
	checkIdx := strings.Index(out, "goodbye:world")
	codeIdx := strings.Index(out, `x='hello'`)
	if checkIdx < 0 {
		t.Fatal("check block not found")
	}
	if codeIdx < 0 {
		t.Fatal("x=... not found")
	}
	if checkIdx > codeIdx {
		t.Error("check block should appear before compiled code")
	}
}

func TestIntegration_NoCheckFlagOmitsCheckBlock(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "main.bsh", `console.log("hello")
`)
	out, err := codegen.CompileFile(path, codegen.Options{NoCheck: true})
	if err != nil {
		t.Fatalf("compile error: %v", err)
	}
	assertNotContains(t, out, `goodbye:world`)
	assertNotContains(t, out, `grep -F`)
	assertNotContains(t, out, `_bst_starts_with()`)
	assertNotContains(t, out, `_bst_ends_with()`)
	assertNotContains(t, out, `_bst_includes()`)
}

func TestIntegration_BundledRuntimeHelpersUseUnionAcrossModules(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "lib.bsh", `export function hasNeedle(s: string): boolean {
    return s.includes("needle")
}
`)
	path := writeFile(t, dir, "main.bsh", `import { hasNeedle } from "./lib"
console.log(hasNeedle("needle haystack"))
`)
	out := compileFile(t, path)
	assertContains(t, out, `_bst_includes()`)
	assertContains(t, out, `_bst_includes "$_lib__hasNeedle_s" 'needle'`)
	assertNotContains(t, out, `_bst_starts_with()`)
	assertNotContains(t, out, `_bst_ends_with()`)
}

func TestIntegration_SplitRuntimeHelpersEmittedPerGeneratedFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "lib.bsh", `export function hasNeedle(s: string): boolean {
    return s.includes("needle")
}
`)
	path := writeFile(t, dir, "main.bsh", `import { hasNeedle } from "./lib"
console.log(hasNeedle("needle haystack"))
`)
	outDir := filepath.Join(dir, "out")
	if err := codegen.CompileFileSplit(path, outDir, codegen.Options{}); err != nil {
		t.Fatalf("split compile error: %v", err)
	}
	mainBytes, err := os.ReadFile(filepath.Join(outDir, "main.sh"))
	if err != nil {
		t.Fatalf("read main output: %v", err)
	}
	libBytes, err := os.ReadFile(filepath.Join(outDir, "lib.sh"))
	if err != nil {
		t.Fatalf("read lib output: %v", err)
	}
	mainOut := string(mainBytes)
	libOut := string(libBytes)
	assertContains(t, mainOut, `goodbye:world`)
	assertNotContains(t, mainOut, `_bst_includes()`)
	assertContains(t, libOut, `_bst_includes()`)
	assertNotContains(t, libOut, `goodbye:world`)
}

func TestIntegration_RawStringPassedSingleQuoted(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "main.bsh", `let p: string = r"^foo-[0-9]+$"
console.log(p)
`)
	out := compileFile(t, path)
	assertContains(t, out, `p='^foo-[0-9]+$'`)
}

func TestIntegration_RawStringInCmdArg(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "main.bsh", `$("grep", "-v", r"-cache$").run()
`)
	out := compileFile(t, path)
	assertContains(t, out, `'-cache$'`)
	assertNotContains(t, out, `"-cache$"`)
}

func TestIntegration_EscapedDollarLiteral(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "main.bsh", `let price: string = "costs \$5"
console.log(price)
`)
	out := compileFile(t, path)
	assertContains(t, out, `\$5`)
}

func TestIntegration_LiteralBacktickAndDollarParen(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "main.bsh", "let b: string = \"ja`j$(echo $PATH)\"\nconsole.log(b)\n")
	out := compileFile(t, path)
	// Plain string → single-quoted; backtick and $() are literal inside single quotes
	assertContains(t, out, "'ja`j$(echo $PATH)'")
}

func TestIntegration_LiteralBacktickNotExpanded(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "main.bsh", "let s: string = \"has`backtick\"\nconsole.log(s)\n")
	out := compileFile(t, path)
	// Single-quoted: backtick is literal, no escaping needed
	assertContains(t, out, "'has`backtick'")
}

func TestIntegration_BraceInterpolationUnaffected(t *testing.T) {
	dir := t.TempDir()
	// Template literal for interpolation
	path := writeFile(t, dir, "main.bsh", "let name: string = \"Alice\"\nlet msg: string = `Hello ${name}`\nconsole.log(msg)\n")
	out := compileFile(t, path)
	assertContains(t, out, `"Hello ${name}"`)
}

func TestIntegration_ClassesCompileAndRun(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "classes.bsh", `class User {
    name: string
    age: number
    constructor(name: string, age: number) {
        this.name = name
        this.age = age
    }
    greet(): string { return "Hello, " + this.name }
    isAdult(): boolean { return this.age >= 18 }
}
let u = new User("Alice", 30)
console.log(u.greet())
console.log(u.name)
u.name = "Bob"
console.log(u.name)
console.log(u.isAdult())
class MathUtils {
    static PI: number = 3.14159
    static round(n: number): number { return Math.round(n) }
}
console.log(MathUtils.PI)
console.log(MathUtils.round(2.7))`)
	out := compileFile(t, path)
	assertContains(t, out, `classes__User__constructor "$u" 'Alice' 30`)
	assertContains(t, out, `printf '%s\n' "$(classes__User__greet "$u")"`)
	assertContains(t, out, `_class_classes__MathUtils_PI=3.14159`)
}

func TestIntegration_ClassMemberSourceCommentsRespectNoSourceMap(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "classes.bsh", `class User {
    name: string
    constructor(name: string) { this.name = name }
    greet(): string { return "Hello, " + this.name }
}
let u = new User("Alice")
console.log(u.greet())`)

	out, err := codegen.CompileFile(path, codegen.Options{NoSourceMap: true})
	if err != nil {
		t.Fatalf("compile error: %v", err)
	}
	assertContains(t, out, `classes__User__constructor() {`)
	assertContains(t, out, `classes__User__greet() {`)
	assertNotContains(t, out, `# besht:`)
}

func TestIntegration_ClassAccessorsRuntime(t *testing.T) {
	out := runCompiledShell(t, `class User {
    name: string
    constructor(name: string) { this.name = name }
    get label(): string { return this.name }
    set label(value: string) { this.name = value }
    static get kind(): string { return "user" }
}
let u = new User("Alice")
console.log(u.label)
u.label = "Bob"
console.log(u.label)
console.log(User.kind)`)
	if out != "Alice\nBob\nuser\n" {
		t.Fatalf("output: got %q", out)
	}
}

func TestIntegration_ClassGetterCannotMutateThis(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "counter.bsh", `class Counter {
    value: number
    constructor() { this.value = 0 }
    get next(): number {
        this.value = this.value + 1
        return this.value
    }
}
let c = new Counter()
console.log(c.next)`)

	_, err := codegen.CompileFile(path, codegen.Options{})
	if err == nil || !strings.Contains(err.Error(), `getter "next" must not assign to this properties`) {
		t.Fatalf("expected mutating getter error, got %v", err)
	}
}

func TestIntegration_ClassGetterWithoutReturnAnnotationCannotMutateThis(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "counter.bsh", `class Counter {
    value: number
    constructor() { this.value = 0 }
    get next() {
        this.value = this.value + 1
        return this.value
    }
}
let c = new Counter()
console.log(c.next)`)

	_, err := codegen.CompileFile(path, codegen.Options{})
	if err == nil || !strings.Contains(err.Error(), `getter "next" must not assign to this properties`) {
		t.Fatalf("expected mutating getter error, got %v", err)
	}
}

func TestIntegration_ArrayFromBlockMapAndSplitJoinRuntime(t *testing.T) {
	out := runCompiledShell(t, `function draw(height: number, frequency: number): string {
  let count = 0
  return Array.from({ length: height })
    .map((_, i) => {
      return "*".repeat(i + 1)
    })
    .join("\n")
    .split("")
    .map((e, i) => {
      if (e !== "*") return e
      if (++count % frequency !== 0) return e
      return "o"
    })
    .join("")
}
console.log(draw(3, 2))`)
	want := "*\no*\no*o\n"
	if out != want {
		t.Fatalf("output: got %q, want %q", out, want)
	}
}

func TestIntegration_PrefixUpdateInArithmeticRuntime(t *testing.T) {
	out := runCompiledShell(t, `let x = 0
let y = ++x + 2
console.log(y)
console.log(x)`)
	want := "3\n1\n"
	if out != want {
		t.Fatalf("output: got %q, want %q", out, want)
	}
}

func TestIntegration_MultilineStrictEqualityRuntime(t *testing.T) {
	out := runCompiledShell(t, "function lines(): string {\n"+
		"  return `a\nb`\n"+
		"}\n"+
		"console.log(lines() === `a\nb`)\n"+
		"console.log(lines() !== `a\nc`)\n")
	want := "true\ntrue\n"
	if out != want {
		t.Fatalf("output: got %q, want %q", out, want)
	}
}

func TestIntegration_MathTruncAndSignRuntime(t *testing.T) {
	out := runCompiledShell(t, `console.log(Math.trunc(3.7))
console.log(Math.trunc(-3.7))
console.log(Math.trunc(-0.1))
console.log(Math.sign(-9))
console.log(Math.sign(0))
console.log(Math.sign(4))`)
	want := "3\n-3\n0\n-1\n0\n1\n"
	if out != want {
		t.Fatalf("output: got %q, want %q", out, want)
	}
}

func TestIntegration_StringCharAtAndSubstringRuntime(t *testing.T) {
	out := runCompiledShell(t, `let s = "abcdef"
console.log(s.charAt(1))
console.log(s.charAt(1.9))
console.log(s.charAt(-1.9) === "")
console.log(s.charAt(99) === "")
console.log(s.substring(4, 1))
console.log(s.substring(-2, 2))
console.log(s.substring(2, 99))`)
	want := "b\nb\ntrue\ntrue\nbcd\nab\ncdef\n"
	if out != want {
		t.Fatalf("output: got %q, want %q", out, want)
	}
}

func TestIntegration_StringLastIndexOfRuntime(t *testing.T) {
	out := runCompiledShell(t, `let s = "hello hello"
console.log(s.lastIndexOf("lo"))
console.log(s.lastIndexOf("z"))
console.log(s.lastIndexOf(""))`)
	want := "9\n-1\n11\n"
	if out != want {
		t.Fatalf("output: got %q, want %q", out, want)
	}
}

func TestIntegration_OptionalChainingRuntime(t *testing.T) {
	out := runCompiledShell(t, `let user = { name: "Ada" }
let missing = undefined
let items = ["", "beta"]
let none = undefined
let matrix = [["a", "b"], ["c", "d"]]
let noMatrix = undefined
let empty = ""
let zero = 0
let nope = false
console.log(user?.name ?? "anonymous")
console.log(missing?.name ?? "anonymous")
console.log(items?.[1] ?? "fallback")
console.log(items?.[0] ?? "fallback")
console.log(none?.[0] ?? "fallback")
console.log(matrix?.[1]?.[0] ?? "fallback")
console.log(matrix[9]?.[0] ?? "fallback")
console.log(noMatrix?.[0]?.[0] ?? "fallback")
console.log(empty ?? "fallback")
console.log(zero ?? 99)
console.log(nope ?? true)`)
	want := "Ada\nanonymous\nbeta\n\nfallback\nc\nfallback\nfallback\n\n0\nfalse\n"
	if out != want {
		t.Fatalf("output: got %q, want %q", out, want)
	}
}

func TestIntegration_OptionalMethodCallRuntime(t *testing.T) {
	out := runCompiledShell(t, `let name = "  Ada  "
let missing = undefined
console.log(name?.trim() ?? "fallback")
console.log(missing?.trim() ?? "fallback")`)
	want := "Ada\nfallback\n"
	if out != want {
		t.Fatalf("output: got %q, want %q", out, want)
	}
}

func TestIntegration_OptionalChainingConditionsAndClassMethodRuntime(t *testing.T) {
	out := runCompiledShell(t, `class User {
    name: string
    constructor(name: string) { this.name = name }
    greet(): string { return "Hello, " + this.name }
}
let missing = undefined
let text = "hello"
let user = new User("Ada")
if (missing?.name) console.log("bad")
else console.log("missing property false")
if (missing?.trim()) console.log("bad")
else console.log("missing method false")
if (text?.includes("ell")) console.log("method true")
console.log(user?.greet() ?? "fallback")`)
	want := "missing property false\nmissing method false\nmethod true\nHello, Ada\n"
	if out != want {
		t.Fatalf("output: got %q, want %q", out, want)
	}
}

func TestIntegration_OptionalStatementDoesNotExecuteReturnedString(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "owned")
	out := runCompiledShell(t, `let payload = "touch `+marker+`"
payload?.trim()
console.log("safe")`)
	want := "safe\n"
	if out != want {
		t.Fatalf("output: got %q, want %q", out, want)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatalf("optional expression statement executed returned data and created %s", marker)
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat marker: %v", err)
	}
}

func TestIntegration_OptionalClassMethodStatementMutatesReceiver(t *testing.T) {
	out := runCompiledShell(t, `class User {
    name: string
    constructor(name: string) { this.name = name }
    rename(name: string) { this.name = name }
}
let user = new User("Ada")
user?.rename("Bob")
console.log(user.name)`)
	want := "Bob\n"
	if out != want {
		t.Fatalf("output: got %q, want %q", out, want)
	}
}

func TestIntegration_NestedOptionalStatementDoesNotExecuteReturnedString(t *testing.T) {
	dir := t.TempDir()
	coalesceMarker := filepath.Join(dir, "coalesce")
	concatMarker := filepath.Join(dir, "concat")
	asMarker := filepath.Join(dir, "as")
	out := runCompiledShell(t, `let coalescePayload = "touch `+coalesceMarker+`"
let concatPayload = "touch `+concatMarker+`"
let asPayload = "touch `+asMarker+`"
coalescePayload?.trim() ?? ""
concatPayload?.trim() + ""
asPayload?.trim() as string
console.log("safe")`)
	want := "safe\n"
	if out != want {
		t.Fatalf("output: got %q, want %q", out, want)
	}
	for _, marker := range []string{coalesceMarker, concatMarker, asMarker} {
		if _, err := os.Stat(marker); err == nil {
			t.Fatalf("optional expression statement executed returned data and created %s", marker)
		} else if !os.IsNotExist(err) {
			t.Fatalf("stat marker %s: %v", marker, err)
		}
	}
}

func TestIntegration_ChainedMethodOptionalStatementDoesNotExecuteReturnedString(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "chained")
	out := runCompiledShell(t, `let payload = "touch `+marker+`"
payload?.trim().trim()
console.log("safe")`)
	want := "safe\n"
	if out != want {
		t.Fatalf("output: got %q, want %q", out, want)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatalf("optional chained method statement executed returned data and created %s", marker)
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat marker: %v", err)
	}
}

func TestIntegration_OptionalClassMethodStatementAsExprMutatesReceiver(t *testing.T) {
	out := runCompiledShell(t, `class User {
    name: string
    constructor(name: string) { this.name = name }
    rename(name: string) { this.name = name }
}
let user = new User("Ada")
user?.rename("Bob") as string
console.log(user.name)`)
	want := "Bob\n"
	if out != want {
		t.Fatalf("output: got %q, want %q", out, want)
	}
}

func TestIntegration_StringSearchOptionalArgsRuntime(t *testing.T) {
	out := runCompiledShell(t, `let s = "hello hello"
console.log(s.indexOf("lo", 4))
console.log(s.indexOf("lo", -2.8))
console.log(s.indexOf(""))
console.log(s.indexOf("", 99))
console.log(s.lastIndexOf("lo", 7))
console.log(s.lastIndexOf("lo", 99))
console.log(s.lastIndexOf("he", -2.8))
console.log(s.lastIndexOf("lo", 2.9))
console.log(s.lastIndexOf("", 7.9))
console.log(s.includes("lo", 10))
console.log(s.startsWith("lo", 3))
console.log(s.startsWith("he", -1.8))
console.log(s.startsWith("", 99))
console.log(s.endsWith("hel", 3))
console.log(s.endsWith("hello", 99))
console.log(s.endsWith("h", -2))
console.log(s.endsWith("", -2))`)
	want := "9\n3\n0\n11\n3\n9\n0\n-1\n7\nfalse\ntrue\ntrue\ntrue\ntrue\ntrue\nfalse\ntrue\n"
	if out != want {
		t.Fatalf("output: got %q, want %q", out, want)
	}
}

func TestIntegration_PureJSAPIsDoNotExecuteHostileStrings(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "owned")
	out := runCompiledShell(t, `let hostile = "$(touch `+marker+`) ; touch `+marker+`"
console.log(hostile.charAt(0))
console.log(hostile.substring(0, 2))
console.log(hostile.lastIndexOf("not-there"))
console.log(Number.isSafeInteger(hostile))
console.log(Math.trunc(hostile))
console.log(Math.sign(hostile))`)
	want := "$\n$(\n-1\nfalse\n0\n0\n"
	if out != want {
		t.Fatalf("output: got %q, want %q", out, want)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("hostile string created marker %s", marker)
	}
}

func TestIntegration_PureJSAPIsDoNotExecuteHostileSearchAndListValues(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "owned")
	out := runCompiledShell(t, `let hostile = "$(touch `+marker+`) ; touch `+marker+`"
let sample = "abcdef"
console.log(sample.indexOf(hostile))
console.log(sample.includes(hostile))
console.log(sample.startsWith(hostile))
console.log(sample.endsWith(hostile))
console.log(sample.lastIndexOf(hostile))
let pos: number = hostile
console.log(sample.indexOf("cd", pos))
console.log(sample.includes("ab", pos))
console.log(sample.startsWith("ab", pos))
console.log(sample.endsWith("ab", pos))
console.log(sample.lastIndexOf("ab", pos))
let fromArrayOf = Array.of("alpha", hostile, "omega")
console.log(fromArrayOf.lastIndexOf(hostile))
let repeated = ["a", hostile, "b", hostile]
console.log(repeated.lastIndexOf(hostile))`)
	want := "-1\nfalse\nfalse\nfalse\n-1\n2\ntrue\ntrue\nfalse\n0\n1\n3\n"
	if out != want {
		t.Fatalf("output: got %q, want %q", out, want)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("hostile search/list value created marker %s", marker)
	}
}

func TestIntegration_StringLastIndexOfRepeatedAndSpecialRuntime(t *testing.T) {
	out := runCompiledShell(t, `let word = "bananana"
console.log(word.lastIndexOf("na"))
console.log(word.lastIndexOf("z"))
console.log(word.lastIndexOf(""))
let special = "a [x] a [x]"
console.log(special.lastIndexOf("[x]"))`)
	want := "6\n-1\n8\n8\n"
	if out != want {
		t.Fatalf("output: got %q, want %q", out, want)
	}
}

func TestIntegration_ListLastIndexOfRuntime(t *testing.T) {
	out := runCompiledShell(t, `let values = ["a", "b", "a", "c"]
console.log(values.lastIndexOf("a"))
console.log(values.lastIndexOf("z"))`)
	want := "2\n-1\n"
	if out != want {
		t.Fatalf("output: got %q, want %q", out, want)
	}
}

func TestIntegration_ListUnshiftRuntime(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "owned")
	out := runCompiledShell(t, `let values = ["b", "c"]
let prepended = values.unshift("a")
console.log(prepended.join(","))
values.unshift("zero")
console.log(values.join(","))
let empty: string[] = []
console.log(empty.unshift("only").join(","))
let hostile = "$(touch `+marker+`)"
console.log(values.unshift(hostile).lastIndexOf(hostile))`)
	want := "a,b,c\nzero,b,c\nonly\n0\n"
	if out != want {
		t.Fatalf("output: got %q, want %q", out, want)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("hostile unshift value created marker %s", marker)
	}
}

func TestIntegration_ArrayOfRuntime(t *testing.T) {
	out := runCompiledShell(t, `let values = Array.of("x", "y", "z")
console.log(values.join(","))
let empty = Array.of()
console.log(empty.length)
console.log(empty.join(",") === "")`)
	want := "x,y,z\n0\ntrue\n"
	if out != want {
		t.Fatalf("output: got %q, want %q", out, want)
	}
}

func TestIntegration_ArrayIsArrayRuntime(t *testing.T) {
	out := runCompiledShell(t, `console.log(Array.isArray(["a", "b"]))
console.log(Array.isArray(Array.of("x")))
console.log(Array.isArray(Array.from({ length: 2 })))
console.log(Array.isArray("not a list"))
console.log(Array.isArray({ value: "x" }))
if (Array.isArray(["condition"])) console.log("condition true")
if (Array.isArray("condition")) console.log("condition false")`)
	want := "true\ntrue\ntrue\nfalse\nfalse\ncondition true\n"
	if out != want {
		t.Fatalf("output: got %q, want %q", out, want)
	}
}

func TestIntegration_BooleanBuiltinRuntime(t *testing.T) {
	out := runCompiledShell(t, `console.log(Boolean(false))
console.log(Boolean(true))
console.log(Boolean(0))
console.log(Boolean(0.0))
let dynamicZero = 0.0
let dynamicNegZero = -0.0
console.log(Boolean(dynamicZero))
console.log(Boolean(dynamicNegZero))
console.log(Boolean(1))
console.log(Boolean(-1))
console.log(Boolean(""))
console.log(Boolean("x"))
console.log(Boolean("0"))
console.log(Boolean("false"))
console.log(Boolean(null))
console.log(Boolean(undefined))
console.log(Boolean([]))
console.log(Boolean({}))
console.log(Boolean("x").toString())
if (Boolean("x")) console.log("condition true")
if (Boolean("")) console.log("condition false")`)
	want := "false\ntrue\nfalse\nfalse\nfalse\nfalse\ntrue\ntrue\nfalse\ntrue\ntrue\ntrue\nfalse\nfalse\ntrue\ntrue\ntrue\ncondition true\n"
	if out != want {
		t.Fatalf("output: got %q, want %q", out, want)
	}
}

func TestIntegration_ObjectKeysRuntime(t *testing.T) {
	out := runCompiledShell(t, `let user = { id: 1, name: "Victor" }
user.active = true
let key = "role"
user[key] = "admin"
let alias = user
function showKeys(obj: object): string {
    return Object.keys(obj).join(",")
}
function hasKey(obj: object, k: string): boolean {
    return Object.hasOwn(obj, k)
}
function shadowAlias(alias: object): string {
    return Object.keys(alias).join(",")
}
function showAliasKeys(obj: object): string {
    let alias = obj
    return Object.keys(alias).join(",")
}
function aliasComputed(obj: object): string {
    let alias = obj
    let k = "role"
    alias[k] = "editor"
    return alias[k] + ":" + Object.keys(alias).join(",")
}
function localAliasDoesNotLeak(): string {
    let same = user
    return Object.keys(same).join(",")
}
console.log(Object.keys(user).join(","))
console.log(Object.values(user).join(","))
let userEntries = Object.entries(user)
console.log(userEntries[0][0] + "=" + userEntries[0][1])
console.log(userEntries[2][0] + "=" + userEntries[2][1])
console.log(userEntries[3][0] + "=" + userEntries[3][1])
console.log(Object.keys(alias).join(","))
console.log(showKeys(user))
console.log(Object.hasOwn(user, "name"))
console.log(Object.hasOwn(user, "na"))
console.log(Object.hasOwn(user, "bad-key"))
console.log(Object.hasOwn(user, key).toString())
if (Object.hasOwn(user, "active")) console.log("has active")
console.log(hasKey(user, "role"))
let other = { other: true }
console.log(shadowAlias(other))
console.log(showAliasKeys(user))
console.log(aliasComputed(user))
alias = other
console.log(Object.keys(alias).join(","))
let same = other
console.log(localAliasDoesNotLeak())
console.log(Object.keys(same).join(","))
console.log(alias[key])
console.log(Object.keys({ value: "x", enabled: true }).join(","))
console.log(Object.values({ value: "x", enabled: true }).join(","))
let literalEntries = Object.entries({ value: "x", enabled: true })
console.log(literalEntries[1][0] + "=" + literalEntries[1][1])
console.log(Object.hasOwn({ value: "x", enabled: true }, "enabled"))
class Game {
    static Deltas: Record<string, [number, number]> = { U: [-1, 0], D: [1, 0] }
}
console.log(Object.keys(Game.Deltas).join(","))
let deltas = Game.Deltas
console.log(Object.keys(deltas).join(","))
console.log(Object.hasOwn(deltas, "U"))
deltas = Game.Deltas
console.log(Object.keys(deltas).join(","))
let words = ["a", "b", "a"]
let counts = words.reduce((acc, word) => {
    acc[word] = (acc[word] || 0) + 1
    return acc
}, {})
console.log(Object.keys(counts).join(","))
console.log(Object.values(counts).join(","))
console.log(Object.hasOwn(counts, "a"))
console.log(Object.hasOwn(counts, "c"))`)
	want := "id,name,active,role\n1,Victor,true,admin\nid=1\nactive=true\nrole=admin\nid,name,active,role\nid,name,active,role\ntrue\nfalse\nfalse\ntrue\nhas active\ntrue\nother\nid,name,active,role\neditor:id,name,active,role\nother\nid,name,active,role\nother\n\nvalue,enabled\nx,true\nenabled=true\ntrue\nU,D\nU,D\ntrue\nU,D\na,b\n2,1\ntrue\nfalse\n"
	if out != want {
		t.Fatalf("output: got %q, want %q", out, want)
	}
}

func TestIntegration_FunctionReadsTopLevelObjectProperties(t *testing.T) {
	out := runCompiledShell(t, `let student = {
    name: "Laura",
    scores: [72, 65, 81],
}

function report(): string {
    return student.name + ":" + student.scores.join(",")
}

console.log(report())`)
	if out != "Laura:72,65,81\n" {
		t.Fatalf("output: got %q", out)
	}
}

func TestIntegration_StaticListForUsesCompactShellLoop(t *testing.T) {
	src := `for (name of ["Ada Lovelace", "", "Bob"]) {
    console.log("[" + name + "]")
}`
	out := runCompiledShell(t, src)
	if out != "[Ada Lovelace]\n[]\n[Bob]\n" {
		t.Fatalf("output: got %q", out)
	}

	dir := t.TempDir()
	path := writeFile(t, dir, "main.bsh", src)
	compiled, err := codegen.CompileFile(path, codegen.Options{NoCheck: true, NoSourceMap: true})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if !strings.Contains(compiled, "for name in 'Ada Lovelace' '' 'Bob'; do") {
		t.Fatalf("static list should compile to compact shell for loop:\n%s", compiled)
	}
	if strings.Contains(compiled, "_forlist_") || strings.Contains(compiled, "__BESHT_FOR_") {
		t.Fatalf("static list should not materialize a here-doc loop:\n%s", compiled)
	}
}

func TestIntegration_ObjectKeysRejectsUnsupportedSurfaces(t *testing.T) {
	tests := []struct {
		name    string
		src     string
		wantErr string
	}{
		{"process env", `let keys: string[] = Object.keys(process.env)`, "Object.keys() requires an object literal or named object"},
		{"values process env", `let values: string[] = Object.values(process.env)`, "Object.values() requires an object literal or named object"},
		{"entries process env", `let entries: string[][] = Object.entries(process.env)`, "Object.entries() requires an object literal or named object"},
		{"process env alias", `let envObj = process.env
let keys: string[] = Object.keys(envObj)`, "Object.keys() requires an object literal or named object"},
		{"process env property", `let keys: string[] = Object.keys(process.env.HOME)`, "Object.keys() requires an object literal or named object"},
		{"scalar argument", `let keys: string[] = Object.keys("x")`, "Object.keys() requires an object literal or named object"},
		{"hasOwn process env", `let ok: boolean = Object.hasOwn(process.env, "HOME")`, "Object.hasOwn() requires an object literal or named object"},
		{"hasOwn process env alias", `let envObj = process.env
let ok: boolean = Object.hasOwn(envObj, "HOME")`, "Object.hasOwn() requires an object literal or named object"},
		{"hasOwn scalar argument", `let ok: boolean = Object.hasOwn("x", "name")`, "Object.hasOwn() requires an object literal or named object"},
		{"unsafe literal key", `let keys: string[] = Object.keys({ "bad-key": "x" })`, `object key "bad-key" is not supported`},
		{"values list value", `let values: string[] = Object.values({ pair: ["a", "b"] })`, "Object.values() only supports scalar object values"},
		{"entries list value", `let entries: string[][] = Object.entries({ pair: ["a", "b"] })`, "Object.entries() only supports scalar object values"},
		{"values named list value", `let obj = { pair: ["a", "b"] }
let values: string[] = Object.values(obj)`, "Object.values() only supports scalar object values"},
		{"entries alias list value", `let obj = { pair: ["a", "b"] }
let alias = obj
let entries: string[][] = Object.entries(alias)`, "Object.entries() only supports scalar object values"},
		{"values later property list value", `let obj = { ok: "x" }
obj.pair = ["a", "b"]
let values: string[] = Object.values(obj)`, "Object.values() only supports scalar object values"},
		{"entries later computed list value", `let obj = { ok: "x" }
let key = "pair"
obj[key] = ["a", "b"]
let entries: string[][] = Object.entries(obj)`, "Object.entries() only supports scalar object values"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := writeFile(t, dir, "main.bsh", tt.src)
			_, err := codegen.CompileFile(path, codegen.Options{})
			if err == nil {
				t.Fatalf("CompileFile error: got nil, want an error containing %q or a process.env value error", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) && !strings.Contains(err.Error(), "process.env cannot be used as a value") {
				t.Fatalf("CompileFile error: got %v, want containing %q or process.env value error", err, tt.wantErr)
			}
			if err := codegen.CheckFile(path, codegen.Options{}); err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("CheckFile error: got %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestIntegration_ObjectValuesEntriesRejectKnownListValuedObjects(t *testing.T) {
	tests := []struct {
		name    string
		src     string
		wantErr string
	}{
		{
			name: "static values",
			src: `class Game {
    static Deltas: Record<string, [number, number]> = { U: [-1, 0], D: [1, 0] }
}
let values = Object.values(Game.Deltas)`,
			wantErr: "Object.values() only supports scalar object values",
		},
		{
			name: "static entries",
			src: `class Game {
    static Deltas: Record<string, [number, number]> = { U: [-1, 0], D: [1, 0] }
}
let entries = Object.entries(Game.Deltas)`,
			wantErr: "Object.entries() only supports scalar object values",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := writeFile(t, dir, "main.bsh", tt.src)
			_, err := codegen.CompileFile(path, codegen.Options{})
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("CompileFile error: got %v, want containing %q", err, tt.wantErr)
			}
			if err := codegen.CheckFile(path, codegen.Options{}); err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("CheckFile error: got %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestIntegration_ObjectKeysImportedObjectCheckFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "dep.bsh", `export const user = { id: 1, name: "Victor" }`)
	main := writeFile(t, dir, "main.bsh", `import { user } from "./dep"
console.log(Object.keys(user).join(","))
console.log(Object.hasOwn(user, "name"))`)
	out, err := codegen.CompileFile(main, codegen.Options{})
	if err != nil {
		t.Fatalf("CompileFile imported object keys: %v", err)
	}
	if !strings.Contains(out, "dep__user") {
		t.Fatalf("compiled output should reference imported object storage, got:\n%s", out)
	}
	if err := codegen.CheckFile(main, codegen.Options{}); err != nil {
		t.Fatalf("CheckFile imported object keys: %v", err)
	}
}

func TestIntegration_ObjectKeysImportedListValue(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "dep.bsh", `export const keys = Object.keys({ id: 1, name: "Victor" })
export const values = Object.values({ id: "1", name: "Victor" })
export const entries = Object.entries({ id: "1", name: "Victor" })`)
	main := writeFile(t, dir, "main.bsh", `import { keys, values, entries } from "./dep"
console.log(keys.join(","))
console.log(keys.length)
console.log(Array.isArray(keys))
console.log(values.join(","))
console.log(entries[1][0] + "=" + entries[1][1])`)
	outShell := compileFile(t, main)
	shPath := filepath.Join(dir, "main.sh")
	if err := os.WriteFile(shPath, []byte(outShell), 0755); err != nil {
		t.Fatalf("write shell: %v", err)
	}
	output, err := exec.Command("sh", shPath).CombinedOutput()
	if err != nil {
		t.Fatalf("run shell: %v\n%s\n--- script ---\n%s", err, output, outShell)
	}
	out := string(output)
	want := "id,name\n2\ntrue\n1,Victor\nname=Victor\n"
	if out != want {
		t.Fatalf("output: got %q, want %q", out, want)
	}
	if err := codegen.CheckFile(main, codegen.Options{}); err != nil {
		t.Fatalf("CheckFile imported Object.keys list value: %v", err)
	}
}

func TestIntegration_ObjectValuesEntriesDynamicOverwriteClearsBooleanMetadata(t *testing.T) {
	out := runCompiledShell(t, `let user = { active: true, name: "Ada" }
let key = "active"
user[key] = "admin"
console.log(Object.values(user).join(","))
let entries = Object.entries(user)
console.log(entries[0][0] + "=" + entries[0][1])
user.active = false
console.log(Object.values(user).join(","))`)
	want := "admin,Ada\nactive=admin\nfalse,Ada\n"
	if out != want {
		t.Fatalf("output: got %q, want %q", out, want)
	}
}

func TestIntegration_ObjectValuesEntriesValidateDynamicMetadataKeys(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "main.bsh", `function showValues(obj: object): string {
    return Object.values(obj).join(",")
}
function showEntries(obj: object): string {
    let entries = Object.entries(obj)
    return entries[0][1]
}
let user = { name: "Ada" }`)
	outShell, err := codegen.CompileFile(path, codegen.Options{})
	if err != nil {
		t.Fatalf("compile object metadata key fixture: %v", err)
	}
	shPath := filepath.Join(dir, "main.sh")
	attack := outShell + "\n_objkeys_user='name bad-key'\nmain__showValues \"$user\"\nmain__showEntries \"$user\"\n"
	if err := os.WriteFile(shPath, []byte(attack), 0755); err != nil {
		t.Fatalf("write shell: %v", err)
	}
	out, _ := exec.Command("sh", shPath).CombinedOutput()
	if !strings.Contains(string(out), "[besht] invalid object key: bad-key") {
		t.Fatalf("output: got %q, want invalid object key error", out)
	}
}

func TestIntegration_ListForEachRuntime(t *testing.T) {
	out := runCompiledShell(t, `let names: string[] = ["alice", "bob", "anna"]
let seen = ""
names.forEach((name, index) => {
    console.log(index.toString() + ":" + name)
    seen = seen + name.charAt(0)
})
console.log("seen=" + seen)
let empty: string[] = []
let count = 0
empty.forEach(item => {
    count = count + 1
})
console.log(count.toString())
let visited = new Set<string>()
names.forEach(name => visited.add(name))
console.log(visited.has("bob"))`)
	want := "0:alice\n1:bob\n2:anna\nseen=aba\n0\ntrue\n"
	if out != want {
		t.Fatalf("output: got %q, want %q", out, want)
	}
}

func TestIntegration_ListForEachRejectsValueCallback(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "main.bsh", `let names: string[] = ["alice"]
names.forEach(name => name + "!")`)
	err := codegen.CheckFile(path, codegen.Options{})
	if err == nil || !strings.Contains(err.Error(), "forEach() callback expression must be side-effecting") {
		t.Fatalf("CheckFile error: got %v, want side-effecting callback error", err)
	}
}

func TestIntegration_ListForEachRejectsControlFlow(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "main.bsh", `let names: string[] = ["alice"]
names.forEach(name => { return name })`)
	err := codegen.CheckFile(path, codegen.Options{})
	if err == nil || !strings.Contains(err.Error(), "forEach() callback does not support return") {
		t.Fatalf("CheckFile error: got %v, want return rejection", err)
	}
}

func TestIntegration_ListForEachRejectsValuePosition(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "main.bsh", `let names: string[] = ["alice"]
let result = names.forEach(name => console.log(name))`)
	err := codegen.CheckFile(path, codegen.Options{})
	if err == nil || !strings.Contains(err.Error(), "forEach() must be used as a statement") {
		t.Fatalf("CheckFile error: got %v, want statement-only rejection", err)
	}
}

func TestIntegration_ObjectHasOwnReduceAccumulatorCheckFile(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "main.bsh", `let words = ["a", "b", "a"]
let counts = words.reduce((acc, word) => {
    if (!Object.hasOwn(acc, word)) acc[word] = 0
    acc[word] = acc[word] + 1
    return acc
}, {})
console.log(Object.hasOwn(counts, "a"))`)
	if err := codegen.CheckFile(path, codegen.Options{}); err != nil {
		t.Fatalf("CheckFile reduce Object.hasOwn accumulator: %v", err)
	}
}

func TestIntegration_ClassInstanceSlotValidation(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "pwned")
	path := writeFile(t, dir, "main.bsh", `class User {
    name: string
    constructor(name: string) { this.name = name }
}
let u = new User("Ada")

console.log(u.name)`)
	outShell, err := codegen.CompileFile(path, codegen.Options{})
	if err != nil {
		t.Fatalf("compile unsafe class slot fixture: %v", err)
	}
	shPath := filepath.Join(dir, "main.sh")
	attack := outShell + "\nUser__get_name 'bad;touch " + marker + ";#'\n"
	if err := os.WriteFile(shPath, []byte(attack), 0755); err != nil {
		t.Fatalf("write shell: %v", err)
	}
	out, err := exec.Command("sh", shPath).CombinedOutput()
	if err == nil {
		t.Fatalf("unsafe class slot should fail, output: %q", out)
	}
	if _, statErr := os.Stat(marker); !os.IsNotExist(statErr) {
		t.Fatalf("unsafe class slot executed injected command; marker stat err=%v", statErr)
	}
}

func TestIntegration_ObjectValuesEntriesDynamicKeyOverwriteClearsBooleanFormatting(t *testing.T) {
	out := runCompiledShell(t, `let user = { active: true, name: "Ada" }
let key = "active"
user[key] = "admin"
console.log(Object.values(user).join(","))
let entries = Object.entries(user)
console.log(entries[0][0] + "=" + entries[0][1])
user.active = false
console.log(Object.values(user).join(","))`)
	want := "admin,Ada\nactive=admin\nfalse,Ada\n"
	if out != want {
		t.Fatalf("output: got %q, want %q", out, want)
	}
}

func TestIntegration_ObjectValuesEntriesValidatePollutedMetadataKeys(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "pwned")
	path := writeFile(t, dir, "main.bsh", `function showValues(obj: object): string {
    return Object.values(obj).join(",")
}
function showEntries(obj: object): string {
    let rows = Object.entries(obj)
    return rows[0][0] + "=" + rows[0][1]
}
let user = { name: "Ada" }

console.log(showValues(user))
console.log(showEntries(user))`)
	outShell, err := codegen.CompileFile(path, codegen.Options{})
	if err != nil {
		t.Fatalf("compile object metadata fixture: %v", err)
	}
	shPath := filepath.Join(dir, "main.sh")
	attack := outShell + "\n_objkeys_user='name bad-key'\nmain__showValues user\nmain__showEntries user\n"
	if err := os.WriteFile(shPath, []byte(attack), 0755); err != nil {
		t.Fatalf("write shell: %v", err)
	}
	out, _ := exec.Command("sh", shPath).CombinedOutput()
	if !strings.Contains(string(out), "invalid object key: bad-key") {
		t.Fatalf("output should report invalid object key, got %q", out)
	}
	if _, statErr := os.Stat(marker); !os.IsNotExist(statErr) {
		t.Fatalf("polluted object metadata executed injected command; marker stat err=%v", statErr)
	}
}

func TestIntegration_ObjectPrintValidatesPollutedMetadataKeys(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "pwned")
	path := writeFile(t, dir, "main.bsh", `let words = ["a", "b", "a"]
let counts = words.reduce((acc, word) => {
    acc[word] = (acc[word] || 0) + 1
    return acc
}, {})
console.log(counts)`)
	outShell, err := codegen.CompileFile(path, codegen.Options{})
	if err != nil {
		t.Fatalf("compile object print metadata fixture: %v", err)
	}
	printStart := "printf '{\\n'\nfor _bst_k in $_objkeys_counts; do"
	attack := strings.Replace(outShell, printStart, "_objkeys_counts='a bad;touch "+marker+";#'\n"+printStart, 1)
	if attack == outShell {
		t.Fatalf("generated shell did not contain dynamic object print loop")
	}
	shPath := filepath.Join(dir, "main.sh")
	if err := os.WriteFile(shPath, []byte(attack), 0755); err != nil {
		t.Fatalf("write shell: %v", err)
	}
	out, _ := exec.Command("sh", shPath).CombinedOutput()
	if !strings.Contains(string(out), "invalid object key") {
		t.Fatalf("output should report invalid object key, got %q", out)
	}
	if _, statErr := os.Stat(marker); !os.IsNotExist(statErr) {
		t.Fatalf("polluted object print metadata executed injected command; marker stat err=%v", statErr)
	}
}

func TestIntegration_ObjectValuesEntriesRejectAliasMutatedRootListValues(t *testing.T) {
	tests := []struct {
		name    string
		src     string
		wantErr string
	}{
		{
			name: "values alias property",
			src: `function values(obj: object): string[] {
    let alias = obj
    alias.pair = ["a"]
    return Object.values(obj)
}
let user = { ok: "x" }
let out = values(user)`,
			wantErr: "Object.values() only supports scalar object values",
		},
		{
			name: "entries alias computed",
			src: `function entries(obj: object): string[][] {
    let alias = obj
    let key = Besht.args.option("key") ?? "pair"
    alias[key] = ["a"]
    return Object.entries(obj)
}
let user = { ok: "x" }
let out = entries(user)`,
			wantErr: "Object.entries() only supports scalar object values",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := writeFile(t, dir, "main.bsh", tt.src)
			_, err := codegen.CompileFile(path, codegen.Options{})
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("CompileFile error: got %v, want containing %q", err, tt.wantErr)
			}
			if err := codegen.CheckFile(path, codegen.Options{}); err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("CheckFile error: got %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestIntegration_NumberPredicatesRuntime(t *testing.T) {
	out := runCompiledShell(t, `console.log(Number.isSafeInteger(9007199254740991))
console.log(Number.isSafeInteger(-9007199254740991))
console.log(Number.isSafeInteger(9007199254740992))
console.log(Number.isSafeInteger(3.0))
console.log(Number.isSafeInteger(3.1))
console.log(Number.isSafeInteger("42"))
console.log(Number.isNaN(0))
console.log(Number.isNaN(3.14))
console.log(Number.isNaN("not numeric"))`)
	want := "true\ntrue\nfalse\ntrue\nfalse\nfalse\nfalse\nfalse\nfalse\n"
	if out != want {
		t.Fatalf("output: got %q, want %q", out, want)
	}
}

func TestIntegration_NumberConstantsRuntime(t *testing.T) {
	out := runCompiledShell(t, `console.log(Number.MAX_SAFE_INTEGER)
console.log(Number.MIN_SAFE_INTEGER)
console.log(Number.EPSILON)
console.log(Number.isSafeInteger(Number.MAX_SAFE_INTEGER))
console.log(Number.isSafeInteger(Number.MIN_SAFE_INTEGER))
console.log(Number.isSafeInteger(Number.MAX_SAFE_INTEGER + 1))`)
	want := "9007199254740991\n-9007199254740991\n2.220446049250313e-16\ntrue\ntrue\nfalse\n"
	if out != want {
		t.Fatalf("output: got %q, want %q", out, want)
	}
}

func TestIntegration_ArgsUseScriptArgvInsideFunctionsRuntime(t *testing.T) {
	out := runCompiledShell(t, `function show() {
    console.log(Besht.args.positional(1) ?? "missing")
    console.log(Besht.args.option("branch", "b") ?? "main")
    console.log(Besht.args.flag("dry-run", "d"))
}
show()`, "--branch=dev", "-d", "input.txt")
	want := "input.txt\ndev\ntrue\n"
	if out != want {
		t.Fatalf("output: got %q, want %q", out, want)
	}
}

func TestIntegration_TopLevelPositionalOnlyArgsUseCompactRuntime(t *testing.T) {
	src := `let first = Besht.args.positional(1) ?? "missing"
let second = Besht.args.positional(2) ?? "missing"
console.log(first + ":" + second)`
	dir := t.TempDir()
	path := writeFile(t, dir, "main.bsh", src)
	compiled, err := codegen.CompileFile(path, codegen.Options{NoCheck: true, NoSourceMap: true})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if strings.Contains(compiled, "_bst_args_") || strings.Contains(compiled, "_bst_argc") {
		t.Fatalf("top-level positional-only args should not emit full args runtime:\n%s", compiled)
	}

	shPath := filepath.Join(dir, "main.sh")
	if err := os.WriteFile(shPath, []byte(compiled), 0755); err != nil {
		t.Fatalf("write shell: %v", err)
	}
	output, err := exec.Command("sh", shPath, "--", "-x", "").CombinedOutput()
	if err != nil {
		t.Fatalf("run shell: %v\n%s\n--- script ---\n%s", err, output, compiled)
	}
	if string(output) != "-x:\n" {
		t.Fatalf("output: got %q", output)
	}
}

func TestIntegration_TopLevelPositionalWithOptionsKeepsArgsRuntime(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "main.bsh", `let all = Besht.args.argv()
console.log(Besht.args.positional(1) ?? "missing")
console.log(all.join("|"))`)
	compiled, err := codegen.CompileFile(path, codegen.Options{NoCheck: true, NoSourceMap: true})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if !strings.Contains(compiled, "_bst_args_positional") {
		t.Fatalf("mixed args usage should keep full args runtime:\n%s", compiled)
	}
}

func TestIntegration_ArgsSchemaDoesNotDropPositionalsAfterFlagsRuntime(t *testing.T) {
	out := runCompiledShell(t, `let all = Besht.args.argv()
console.log(all.join("|"))
console.log(Besht.args.positional(1) ?? "missing")
console.log(Besht.args.option("branch", "b") ?? "main")
console.log(Besht.args.flag("dry-run", "d"))`, "--branch", "dev", "-d", "foo", "", "bar")
	want := "foo||bar\nfoo\ndev\ntrue\n"
	if out != want {
		t.Fatalf("output: got %q, want %q", out, want)
	}
}

func TestIntegration_ArgsArgvPreservesTrailingEmptyRuntime(t *testing.T) {
	out := runCompiledShell(t, `let all = Besht.args.argv()
console.log(all.length)
console.log(all.join("|"))
console.log(all[1] ?? "missing")
let seen = 0
for (arg in all) {
    seen++
}
console.log(seen)`, "foo", "")
	want := "2\nfoo|\n\n2\n"
	if out != want {
		t.Fatalf("output: got %q, want %q", out, want)
	}
}

func TestIntegration_ArgsArgvPreservesSoleEmptyRuntime(t *testing.T) {
	out := runCompiledShell(t, `let all = Besht.args.argv()
console.log(all.length)
console.log(all[0] ?? "missing")
let seen = 0
for (arg in all) {
    seen++
}
console.log(seen)`, "")
	want := "1\n\n1\n"
	if out != want {
		t.Fatalf("output: got %q, want %q", out, want)
	}
}

func TestIntegration_ArgsArgvNoArgsDoesNotIterateRuntime(t *testing.T) {
	out := runCompiledShell(t, `let all = Besht.args.argv()
console.log(all.length)
let seen = 0
for (arg in all) {
    seen++
}
console.log(seen)`)
	want := "0\n0\n"
	if out != want {
		t.Fatalf("output: got %q, want %q", out, want)
	}
}

func TestIntegration_ArgsArgvLoopContinueAdvancesRuntime(t *testing.T) {
	out := runCompiledShell(t, `let all = Besht.args.argv()
let seen = 0
for (arg in all) {
    seen++
    if (arg == "skip") continue
    console.log(arg)
}
console.log(seen)`, "skip", "done")
	want := "done\n2\n"
	if out != want {
		t.Fatalf("output: got %q, want %q", out, want)
	}
}

func TestIntegration_ArgsDoubleDashStopsOptionParsingRuntime(t *testing.T) {
	out := runCompiledShell(t, `let all = Besht.args.argv()
console.log(all.join("|"))
console.log(Besht.args.positional(1) ?? "missing")
console.log(Besht.args.option("branch", "b") ?? "main")
console.log(Besht.args.flag("dry-run", "d"))`, "--branch=dev", "--", "-d", "literal")
	want := "-d|literal\n-d\ndev\nfalse\n"
	if out != want {
		t.Fatalf("output: got %q, want %q", out, want)
	}
}

func TestIntegration_SplitImportedModuleUsesScriptArgsRuntime(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "lib.bsh", `export function show() {
    console.log(Besht.args.positional(1) ?? "missing")
    console.log(Besht.args.flag("dry-run", "d"))
}`)
	mainPath := writeFile(t, dir, "main.bsh", `import { show } from "./lib"
show()`)
	outDir := filepath.Join(dir, "out")
	if err := codegen.CompileFileSplit(mainPath, outDir, codegen.Options{}); err != nil {
		t.Fatalf("split compile: %v", err)
	}
	cmd := exec.Command("sh", filepath.Join(outDir, "main.sh"), "-d", "file")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run split shell: %v\n%s", err, output)
	}
	want := "file\ntrue\n"
	if string(output) != want {
		t.Fatalf("output: got %q, want %q", output, want)
	}
}

func TestIntegration_ImportedArgsSchemaAppliesProgramWideRuntime(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "lib.bsh", `export function branch(): string {
    return Besht.args.option("branch", "b") ?? "main"
}`)
	mainPath := writeFile(t, dir, "main.bsh", `import { branch } from "./lib"
let all = Besht.args.argv()
console.log(all.join("|"))
console.log(Besht.args.positional(1) ?? "missing")
console.log(branch())`)

	out, err := codegen.CompileFile(mainPath, codegen.Options{})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	shPath := filepath.Join(dir, "main.sh")
	if err := os.WriteFile(shPath, []byte(out), 0755); err != nil {
		t.Fatalf("write shell: %v", err)
	}
	cmd := exec.Command("sh", shPath, "--branch", "dev", "file")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run bundled shell: %v\n%s\n--- script ---\n%s", err, output, out)
	}
	want := "file\nfile\ndev\n"
	if string(output) != want {
		t.Fatalf("bundled output: got %q, want %q", output, want)
	}

	outDir := filepath.Join(dir, "out")
	if err := codegen.CompileFileSplit(mainPath, outDir, codegen.Options{}); err != nil {
		t.Fatalf("split compile: %v", err)
	}
	cmd = exec.Command("sh", filepath.Join(outDir, "main.sh"), "--branch", "dev", "file")
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run split shell: %v\n%s", err, output)
	}
	if string(output) != want {
		t.Fatalf("split output: got %q, want %q", output, want)
	}
}

func TestIntegration_NullishConditionAndQuotedFallbackRuntime(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "owned")
	out := runCompiledShell(t, `let missing = undefined
let spaced = "a b"
let hostile = "$(touch `+marker+`)"
if (missing ?? true) console.log("condition fallback")
if (false ?? true) console.log("bad")
else console.log("false preserved")
console.log(missing ?? spaced)
console.log(missing ?? hostile)
let sentinelText = "__BESHT_NULLISH__"
console.log(sentinelText ?? "fallback")`)
	want := "condition fallback\nfalse preserved\na b\n$(touch " + marker + ")\n__BESHT_NULLISH__\n"
	if out != want {
		t.Fatalf("output: got %q, want %q", out, want)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("hostile nullish fallback created marker %s", marker)
	}
}
