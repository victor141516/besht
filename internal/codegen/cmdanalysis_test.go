package codegen_test

import (
	"strings"
	"testing"

	"github.com/victor141516/besht/internal/codegen"
	"github.com/victor141516/besht/internal/parser"
)

func TestCmdAnalysis_CloneSourceDoesNotWarn(t *testing.T) {
	prog, err := parser.Parse(`let base = $("echo", "cloned")
let c1 = base.clone()
let c2 = base.clone()
c1.run()
c2.run()
console.log(c1.readStdout())
console.log(c2.readStdout())
`, "test.bsh")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	analysis := codegen.AnalyzeProgram(prog.Statements)
	if len(analysis.Warnings) != 0 {
		t.Fatalf("expected no warnings, got %d: %#v", len(analysis.Warnings), analysis.Warnings)
	}
}

func TestCmdAnalysis_WorkdirChainTracksIdentity(t *testing.T) {
	prog, err := parser.Parse(`let cmd = $("pwd").workdir("/")
cmd.run()
console.log(cmd.readStdout())
`, "test.bsh")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	analysis := codegen.AnalyzeProgram(prog.Statements)
	if len(analysis.Warnings) != 0 {
		t.Fatalf("expected no warnings, got %d: %#v", len(analysis.Warnings), analysis.Warnings)
	}
	if len(analysis.Errors) != 0 {
		t.Fatalf("expected no errors, got %d: %#v", len(analysis.Errors), analysis.Errors)
	}
}

func TestCmdAnalysis_ArrowCallbackTracksOuterCommandRun(t *testing.T) {
	prog, err := parser.Parse(`let names = ["a", "b"]
let cmd = $("echo", "x")
names.forEach(name => cmd.run())
cmd.run()
`, "test.bsh")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	analysis := codegen.AnalyzeProgram(prog.Statements)
	if len(analysis.Errors) != 1 || !strings.Contains(analysis.Errors[0].Message, "command already run") {
		t.Fatalf("expected command already run error, got %#v", analysis.Errors)
	}
}

func TestCmdAnalysis_ArrowCallbackTracksCommandLiteralRun(t *testing.T) {
	prog, err := parser.Parse(`let names = ["a", "b"]
names.forEach(name => $("echo", name).run())
`, "test.bsh")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	analysis := codegen.AnalyzeProgram(prog.Statements)
	if len(analysis.Errors) != 0 {
		t.Fatalf("expected no errors, got %#v", analysis.Errors)
	}
	if len(analysis.Warnings) != 0 {
		t.Fatalf("expected no warnings, got %#v", analysis.Warnings)
	}
	if len(analysis.Identities) != 1 {
		t.Fatalf("expected one command identity, got %d", len(analysis.Identities))
	}
}
