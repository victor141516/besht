package codegen

import (
	"testing"

	"github.com/victor141516/besht/internal/ast"
)

func TestMethodMutatesThisChecksCStyleForInitAndUpdate(t *testing.T) {
	mutatingAssign := &ast.PropertyAssignStmt{
		Object:   "this",
		Property: "value",
		Value:    &ast.IntLit{Value: 1},
	}

	if !methodMutatesThis([]ast.Statement{&ast.CStyleForStmt{Init: mutatingAssign, Body: &ast.Block{}}}) {
		t.Fatal("expected C-style for init mutation to be detected")
	}
	if !methodMutatesThis([]ast.Statement{&ast.CStyleForStmt{Update: mutatingAssign, Body: &ast.Block{}}}) {
		t.Fatal("expected C-style for update mutation to be detected")
	}
}
