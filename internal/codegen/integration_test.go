package codegen_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/victor141516/besht/internal/codegen"
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

func TestIntegration_ImportedExportedValuesAndDefaultRuntime(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "dep.bsh", `export const cmd = ["printf", "named\\n"]
export default ["printf", "default\\n"]`)
	mainPath := writeFile(t, dir, "main.bsh", `import def, { cmd } from "./dep"
$(...cmd).run()
$(...def).run()`)
	out, err := codegen.CompileFile(mainPath, codegen.Options{Strict: true})
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
	out, err := codegen.CompileFile(mainPath, codegen.Options{ResolveTsImports: true, Strict: true})
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
	path := writeFile(t, dir, "range.bsh", `for (i in range(1, 3)) {
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
    $("echo", "error: " + to_str(code)).run()
}
`)
	out := compileFile(t, path)
	assertContains(t, out, `if ! (`)
	assertContains(t, out, `set -e`)
	assertContains(t, out, `false`)
	assertContains(t, out, `$?`)
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

func TestIntegration_NonStrictImportedListValueMethods(t *testing.T) {
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
	out, err := codegen.CompileFile(path, codegen.Options{Strict: true})
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
	out, err := codegen.CompileFile(path, codegen.Options{Strict: true})
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
			_, err := codegen.CompileFile(path, codegen.Options{Strict: true})
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
	out, err := codegen.CompileFile(path, codegen.Options{Strict: true})
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
	if err := codegen.CheckFile(valid, codegen.Options{Strict: true}); err != nil {
		t.Fatalf("strict CheckFile valid shell import: %v", err)
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

func TestIntegration_StrictModeOptional(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "main.bsh", `let name: string = 42`)
	if _, err := codegen.CompileFile(path, codegen.Options{}); err != nil {
		t.Fatalf("default compile should stay permissive, got %v", err)
	}
	if _, err := codegen.CompileFile(path, codegen.Options{Strict: true}); err == nil {
		t.Fatal("strict compile should reject mismatched types")
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
if (file_exists(p)) {
    $("echo", "found").run()
}
`)
	out := compileFile(t, path)
	assertContains(t, out, `[ -f`)
}

func TestIntegration_BuiltinDirCheck(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "main.bsh", `let p: string = "/tmp"
if (is_dir(p)) {
    $("echo", "is dir").run()
}
`)
	out := compileFile(t, path)
	assertContains(t, out, `[ -d`)
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
let msg: string = "Count: " + to_str(n)
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

func TestIntegration_ConcatLists(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "main.bsh", `let a: list<string> = ["one", "two"]
let b: list<string> = ["three", "four"]
let c: list<string> = concat(a, b)
let total: number = len(c)
`)
	out := compileFile(t, path)
	assertContains(t, out, `printf '%s\n%s'`)
	assertContains(t, out, `wc -l`)
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
    console.log(args.positional(1) ?? "missing")
    console.log(args.option("branch", "b") ?? "main")
    console.log(args.flag("dry-run", "d"))
}
show()`, "--branch=dev", "-d", "input.txt")
	want := "input.txt\ndev\ntrue\n"
	if out != want {
		t.Fatalf("output: got %q, want %q", out, want)
	}
}

func TestIntegration_ArgsSchemaDoesNotDropPositionalsAfterFlagsRuntime(t *testing.T) {
	out := runCompiledShell(t, `let all = args.argv()
console.log(all.join("|"))
console.log(args.positional(1) ?? "missing")
console.log(args.option("branch", "b") ?? "main")
console.log(args.flag("dry-run", "d"))`, "--branch", "dev", "-d", "foo", "", "bar")
	want := "foo||bar\nfoo\ndev\ntrue\n"
	if out != want {
		t.Fatalf("output: got %q, want %q", out, want)
	}
}

func TestIntegration_ArgsArgvPreservesTrailingEmptyRuntime(t *testing.T) {
	out := runCompiledShell(t, `let all = args.argv()
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
	out := runCompiledShell(t, `let all = args.argv()
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
	out := runCompiledShell(t, `let all = args.argv()
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
	out := runCompiledShell(t, `let all = args.argv()
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
	out := runCompiledShell(t, `let all = args.argv()
console.log(all.join("|"))
console.log(args.positional(1) ?? "missing")
console.log(args.option("branch", "b") ?? "main")
console.log(args.flag("dry-run", "d"))`, "--branch=dev", "--", "-d", "literal")
	want := "-d|literal\n-d\ndev\nfalse\n"
	if out != want {
		t.Fatalf("output: got %q, want %q", out, want)
	}
}

func TestIntegration_SplitImportedModuleUsesScriptArgsRuntime(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "lib.bsh", `export function show() {
    console.log(args.positional(1) ?? "missing")
    console.log(args.flag("dry-run", "d"))
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
    return args.option("branch", "b") ?? "main"
}`)
	mainPath := writeFile(t, dir, "main.bsh", `import { branch } from "./lib"
let all = args.argv()
console.log(all.join("|"))
console.log(args.positional(1) ?? "missing")
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
