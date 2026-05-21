package codegen_test

import (
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
