// Package ast defines the Abstract Syntax Tree node types for the besht language.
package ast

import "fmt"

// ─── Position ─────────────────────────────────────────────────────────────────

// Pos carries source location information for error reporting.
type Pos struct {
	File   string
	Line   int
	Column int
}

func (p Pos) String() string {
	return fmt.Sprintf("%s:%d:%d", p.File, p.Line, p.Column)
}

// ─── Types ────────────────────────────────────────────────────────────────────

// TypeKind enumerates the primitive types in besht.
type TypeKind int

const (
	TypeString TypeKind = iota
	TypeNumber
	TypeBoolean
	TypeStatus
	TypeVoid
	TypeList
	TypeSet
	TypeCommand // result of $() — lazy command pipeline
	TypeObject
)

// Type represents a besht type annotation.
type Type struct {
	Kind TypeKind
	Elem *Type
	Pos  Pos
}

func (t *Type) String() string {
	switch t.Kind {
	case TypeString:
		return "string"
	case TypeNumber:
		return "number"
	case TypeBoolean:
		return "boolean"
	case TypeStatus:
		return "status"
	case TypeVoid:
		return "void"
	case TypeList:
		return fmt.Sprintf("list<%s>", t.Elem)
	case TypeSet:
		return fmt.Sprintf("Set<%s>", t.Elem)
	case TypeCommand:
		return "command"
	case TypeObject:
		return "object"
	}
	return "unknown"
}

func (t *Type) Equal(other *Type) bool {
	if t == nil && other == nil {
		return true
	}
	if t == nil || other == nil {
		return false
	}
	if t.Kind != other.Kind {
		return false
	}
	if t.Kind == TypeList || t.Kind == TypeSet {
		return t.Elem.Equal(other.Elem)
	}
	return true
}

// ─── Node interface ───────────────────────────────────────────────────────────

// Node is implemented by all AST nodes.
type Node interface {
	nodePos() Pos
}

// Statement is a node that produces no value.
type Statement interface {
	Node
	stmtNode()
}

// Expression is a node that produces a typed value.
type Expression interface {
	Node
	exprNode()
	GetType() *Type // filled in by the type checker; nil before that
}

// ─── Program ──────────────────────────────────────────────────────────────────

// Program is the root node.
type Program struct {
	File       string
	Imports    []*ImportDecl
	Statements []Statement
}

// ─── Declarations / Statements ────────────────────────────────────────────────

// ImportDecl: import defaultName from "path" or import { Name, ... } from "path"
type ImportDecl struct {
	Pos         Pos
	DefaultName string
	Names       []string // imported named symbols; nil means none
	Source      string   // module path (without extension)
	AssertType  string   // import assertion type, e.g. "shell"
}

func (n *ImportDecl) nodePos() Pos { return n.Pos }
func (n *ImportDecl) stmtNode()    {}

// LetDecl: let/const name: Type = Expr
type LetDecl struct {
	Pos           Pos
	Name          string
	IsConst       bool
	Exported      bool
	DefaultExport bool
	TypeAnnot     *Type
	Value         Expression
	ResolvedTy    *Type // set by checker
}

func (n *LetDecl) nodePos() Pos { return n.Pos }
func (n *LetDecl) stmtNode()    {}

// DestructureDecl: let/const [a, b] = Expr
type DestructureDecl struct {
	Pos     Pos
	Names   []string
	IsConst bool
	Value   Expression
}

func (n *DestructureDecl) nodePos() Pos { return n.Pos }
func (n *DestructureDecl) stmtNode()    {}

// Assignment: name = Expr
type Assignment struct {
	Pos   Pos
	Name  string
	Value Expression
}

func (n *Assignment) nodePos() Pos { return n.Pos }
func (n *Assignment) stmtNode()    {}

type IndexAssignStmt struct {
	Pos   Pos
	Name  string
	Index Expression
	Value Expression
}

func (n *IndexAssignStmt) nodePos() Pos { return n.Pos }
func (n *IndexAssignStmt) stmtNode()    {}

type PropertyAssignStmt struct {
	Pos      Pos
	Object   string
	Property string
	Value    Expression
}

func (n *PropertyAssignStmt) nodePos() Pos { return n.Pos }
func (n *PropertyAssignStmt) stmtNode()    {}

// FnDecl: [export] fn name(params) [: ReturnType] { body }
type FnDecl struct {
	Pos        Pos
	Name       string
	Exported   bool
	Params     []*Param
	ReturnType *Type // nil → void
	Body       *Block
}

func (n *FnDecl) nodePos() Pos { return n.Pos }
func (n *FnDecl) stmtNode()    {}

type ClassMethod struct {
	Pos        Pos
	Name       string
	IsStatic   bool
	Params     []*Param
	ReturnType *Type
	Body       *Block
}

type ClassProperty struct {
	Pos      Pos
	Name     string
	Type     *Type
	Value    Expression
	IsStatic bool
}

type ClassDecl struct {
	Pos         Pos
	Name        string
	Exported    bool
	Properties  []ClassProperty
	Constructor *ClassMethod
	Methods     []ClassMethod
	StaticProps []ClassProperty
}

func (n *ClassDecl) nodePos() Pos { return n.Pos }
func (n *ClassDecl) stmtNode()    {}

// Param: name: Type
type Param struct {
	Pos  Pos
	Name string
	Type *Type
}

// Block: { statements... }
type Block struct {
	Pos        Pos
	Statements []Statement
}

func (n *Block) nodePos() Pos { return n.Pos }
func (n *Block) stmtNode()    {}

// IfStmt: if expr { } [else if expr { }]* [else { }]
type IfStmt struct {
	Pos       Pos
	Condition Expression
	Then      *Block
	ElseIfs   []*ElseIf
	Else      *Block
}

func (n *IfStmt) nodePos() Pos { return n.Pos }
func (n *IfStmt) stmtNode()    {}

// ElseIf: else if expr { }
type ElseIf struct {
	Pos       Pos
	Condition Expression
	Body      *Block
}

// ForStmt: for ident in iterExpr { }
type ForStmt struct {
	Pos      Pos
	VarName  string
	Iterator Expression // can be IdentExpr (list var), RangeExpr, or ShellExpr
	Body     *Block
}

func (n *ForStmt) nodePos() Pos { return n.Pos }
func (n *ForStmt) stmtNode()    {}

// WhileStmt: while expr { }
type WhileStmt struct {
	Pos       Pos
	Condition Expression
	Body      *Block
}

func (n *WhileStmt) nodePos() Pos { return n.Pos }
func (n *WhileStmt) stmtNode()    {}

// TryStmt: try { } catch (name: status) { }
type TryStmt struct {
	Pos      Pos
	Body     *Block
	CatchVar string
	Catch    *Block
}

func (n *TryStmt) nodePos() Pos { return n.Pos }
func (n *TryStmt) stmtNode()    {}

type SwitchStmt struct {
	Pos   Pos
	Value Expression
	Cases []SwitchCase
}

type SwitchCase struct {
	Pos       Pos
	IsDefault bool
	Value     Expression
	Body      *Block
}

func (n *SwitchStmt) nodePos() Pos { return n.Pos }
func (n *SwitchStmt) stmtNode()    {}

// ReturnStmt: return [expr]
type ReturnStmt struct {
	Pos   Pos
	Value Expression // nil for bare return / void functions
}

func (n *ReturnStmt) nodePos() Pos { return n.Pos }
func (n *ReturnStmt) stmtNode()    {}

// ExitStmt: exit(expr)
type ExitStmt struct {
	Pos  Pos
	Code Expression // int expression
}

func (n *ExitStmt) nodePos() Pos { return n.Pos }
func (n *ExitStmt) stmtNode()    {}

// ExprStmt wraps an expression used as a statement (e.g. bare function call).
type ExprStmt struct {
	Pos  Pos
	Expr Expression
}

func (n *ExprStmt) nodePos() Pos { return n.Pos }
func (n *ExprStmt) stmtNode()    {}

// CStyleForStmt: for (init; condition; update) { }
type CStyleForStmt struct {
	Pos       Pos
	Init      Statement // LetDecl or Assignment
	Condition Expression
	Update    Statement // Assignment (i++ desugared to i = i + 1)
	Body      *Block
}

func (n *CStyleForStmt) nodePos() Pos { return n.Pos }
func (n *CStyleForStmt) stmtNode()    {}

// DeclareStmt: declare ... — ignored by compiler, exists only for editor support
type DeclareStmt struct{ Pos Pos }

func (n *DeclareStmt) nodePos() Pos { return n.Pos }
func (n *DeclareStmt) stmtNode()    {}

type DeclareFnStmt struct {
	Pos        Pos
	Name       string
	Exported   bool
	Params     []*Param
	ReturnType *Type
}

func (n *DeclareFnStmt) nodePos() Pos { return n.Pos }
func (n *DeclareFnStmt) stmtNode()    {}

// BreakStmt: break
type BreakStmt struct{ Pos Pos }

func (n *BreakStmt) nodePos() Pos { return n.Pos }
func (n *BreakStmt) stmtNode()    {}

// ContinueStmt: continue
type ContinueStmt struct{ Pos Pos }

func (n *ContinueStmt) nodePos() Pos { return n.Pos }
func (n *ContinueStmt) stmtNode()    {}

// ─── Expressions ──────────────────────────────────────────────────────────────

// baseExpr holds the resolved type; embedded by all expression nodes.
type baseExpr struct{ resolvedType *Type }

func (e *baseExpr) GetType() *Type  { return e.resolvedType }
func (e *baseExpr) SetType(t *Type) { e.resolvedType = t }

// IntLit: 42
type IntLit struct {
	baseExpr
	Pos   Pos
	Value int64
}

func (n *IntLit) nodePos() Pos { return n.Pos }
func (n *IntLit) exprNode()    {}

// FloatLit: 3.14
type FloatLit struct {
	baseExpr
	Pos   Pos
	Value string
}

func (n *FloatLit) nodePos() Pos { return n.Pos }
func (n *FloatLit) exprNode()    {}

// StringLit: "hello"
type StringLit struct {
	baseExpr
	Pos   Pos
	Value string // raw content (may contain ${...} references)
}

func (n *StringLit) nodePos() Pos { return n.Pos }
func (n *StringLit) exprNode()    {}

// RawStringLit: r"..." — always single-quoted, no shell expansion
type RawStringLit struct {
	baseExpr
	Pos   Pos
	Value string
}

func (n *RawStringLit) nodePos() Pos { return n.Pos }
func (n *RawStringLit) exprNode()    {}

// TemplateLit: `...${expr}...` — interpolated string
type TemplateLit struct {
	baseExpr
	Pos   Pos
	Value string // raw content with ${...} intact
	Parts []string
	Exprs []Expression
}

func (n *TemplateLit) nodePos() Pos { return n.Pos }
func (n *TemplateLit) exprNode()    {}

// BoolLit: true | false
type BoolLit struct {
	baseExpr
	Pos   Pos
	Value bool
}

func (n *BoolLit) nodePos() Pos { return n.Pos }
func (n *BoolLit) exprNode()    {}

type UndefinedLit struct {
	baseExpr
	Pos Pos
}

func (n *UndefinedLit) nodePos() Pos { return n.Pos }
func (n *UndefinedLit) exprNode()    {}

type NullLit struct {
	baseExpr
	Pos Pos
}

func (n *NullLit) nodePos() Pos { return n.Pos }
func (n *NullLit) exprNode()    {}

// ListLit: ["a", "b", "c"]
type ListLit struct {
	baseExpr
	Pos      Pos
	Elements []Expression
}

func (n *ListLit) nodePos() Pos { return n.Pos }
func (n *ListLit) exprNode()    {}

type ObjectField struct {
	Pos   Pos
	Key   string
	Value Expression
}

type ObjectLit struct {
	baseExpr
	Pos    Pos
	Fields []ObjectField
}

func (n *ObjectLit) nodePos() Pos { return n.Pos }
func (n *ObjectLit) exprNode()    {}

type ArrowExpr struct {
	baseExpr
	Pos       Pos
	Params    []*Param
	Body      Expression // expression body (nil if BlockBody is set)
	BlockBody *Block     // block body (nil if expression body)
}

func (n *ArrowExpr) nodePos() Pos { return n.Pos }
func (n *ArrowExpr) exprNode()    {}

// IdentExpr: a variable reference
type IdentExpr struct {
	baseExpr
	Pos  Pos
	Name string
}

func (n *IdentExpr) nodePos() Pos { return n.Pos }
func (n *IdentExpr) exprNode()    {}

type NewExpr struct {
	baseExpr
	Pos       Pos
	ClassName string
	TypeArgs  []*Type
	Args      []Expression
}

func (n *NewExpr) nodePos() Pos { return n.Pos }
func (n *NewExpr) exprNode()    {}

type ThisExpr struct {
	baseExpr
	Pos Pos
}

func (n *ThisExpr) nodePos() Pos { return n.Pos }
func (n *ThisExpr) exprNode()    {}

// BinaryExpr: left op right
type BinaryExpr struct {
	baseExpr
	Pos   Pos
	Op    string // "==", "!=", ">", "<", ">=", "<=", "+", "-", "*", "/", "%", "&&", "||", "??"
	Left  Expression
	Right Expression
}

func (n *BinaryExpr) nodePos() Pos { return n.Pos }
func (n *BinaryExpr) exprNode()    {}

type TernaryExpr struct {
	baseExpr
	Pos       Pos
	Condition Expression
	Then      Expression
	Else      Expression
}

func (n *TernaryExpr) nodePos() Pos { return n.Pos }
func (n *TernaryExpr) exprNode()    {}

// UnaryExpr: op expr
type UnaryExpr struct {
	baseExpr
	Pos  Pos
	Op   string // "!"
	Expr Expression
}

func (n *UnaryExpr) nodePos() Pos { return n.Pos }
func (n *UnaryExpr) exprNode()    {}

// UpdateExpr: prefix ++name or --name in expression position.
type UpdateExpr struct {
	baseExpr
	Pos  Pos
	Op   string
	Name string
}

func (n *UpdateExpr) nodePos() Pos { return n.Pos }
func (n *UpdateExpr) exprNode()    {}

// PipeExpr: left | right
type PipeExpr struct {
	baseExpr
	Pos   Pos
	Left  Expression
	Right Expression
}

func (n *PipeExpr) nodePos() Pos { return n.Pos }
func (n *PipeExpr) exprNode()    {}

// CmdExpr: $("cmd", arg1, arg2, ...) — structured command invocation
type CmdExpr struct {
	baseExpr
	Pos  Pos
	Args []Expression // Args[0] is the command name, rest are arguments
}

func (n *CmdExpr) nodePos() Pos { return n.Pos }
func (n *CmdExpr) exprNode()    {}

// FnCallExpr: name(args...)
type FnCallExpr struct {
	baseExpr
	Pos  Pos
	Name string
	Args []Expression
}

func (n *FnCallExpr) nodePos() Pos { return n.Pos }
func (n *FnCallExpr) exprNode()    {}

// BuiltinCallExpr: file_exists(p), is_dir(p), len(l), etc.
type BuiltinCallExpr struct {
	baseExpr
	Pos  Pos
	Name string
	Args []Expression
}

func (n *BuiltinCallExpr) nodePos() Pos { return n.Pos }
func (n *BuiltinCallExpr) exprNode()    {}

// RangeExpr: range(start, end)  — only valid as for-loop iterator
type RangeExpr struct {
	baseExpr
	Pos   Pos
	Start Expression
	End   Expression
}

func (n *RangeExpr) nodePos() Pos { return n.Pos }
func (n *RangeExpr) exprNode()    {}

// PropagateExpr: expr?  — error propagation
type PropagateExpr struct {
	baseExpr
	Pos  Pos
	Expr Expression
}

func (n *PropagateExpr) nodePos() Pos { return n.Pos }
func (n *PropagateExpr) exprNode()    {}

// IndexExpr: expr[index]
type IndexExpr struct {
	baseExpr
	Pos      Pos
	Expr     Expression
	Index    Expression
	Optional bool
}

func (n *IndexExpr) nodePos() Pos { return n.Pos }
func (n *IndexExpr) exprNode()    {}

// MethodCallExpr: expr.method(args)
type MethodCallExpr struct {
	baseExpr
	Pos      Pos
	Receiver Expression
	Method   string
	Args     []Expression
	Optional bool
}

func (n *MethodCallExpr) nodePos() Pos { return n.Pos }
func (n *MethodCallExpr) exprNode()    {}

// PropertyExpr: expr.property (no call, e.g. arr.length)
type PropertyExpr struct {
	baseExpr
	Pos      Pos
	Receiver Expression
	Property string
	Optional bool
}

func (n *PropertyExpr) nodePos() Pos { return n.Pos }
func (n *PropertyExpr) exprNode()    {}

type SpreadExpr struct {
	baseExpr
	Pos  Pos
	Expr Expression
}

func (n *SpreadExpr) nodePos() Pos { return n.Pos }
func (n *SpreadExpr) exprNode()    {}

// AsExpr: expr as Type - TypeScript-compatible erased type assertion.
type AsExpr struct {
	baseExpr
	Pos  Pos
	Expr Expression
	Type *Type
}

func (n *AsExpr) nodePos() Pos { return n.Pos }
func (n *AsExpr) exprNode()    {}

// ─── Builtin names (compile-time functions) ───────────────────────────────────

// IsBuiltin returns true if name is a built-in compile-time function.
func IsBuiltin(name string) bool {
	switch name {
	case "file_exists", "is_dir", "is_readable", "is_writable", "is_executable",
		"is_empty", "is_set", "len", "head", "tail", "append", "contains",
		"range", "exit", "env", "to_str", "to_int", "String", "concat",
		"console.log", "console.error":
		return true
	}
	return false
}
