package lexer

import "fmt"

type TokenType int

const (
	// Literals
	TokInt         TokenType = iota
	TokFloat                 // 3.14 — decimal literal
	TokString                // "..." or '...' — plain literal, no interpolation
	TokRawString             // r"..." — always single-quoted, no interpolation (alias for TokString now)
	TokTemplateLit           // `...${expr}...` — template literal with interpolation
	TokIdent                 // identifier

	// Keywords
	TokLet
	TokFunction
	TokExport
	TokImport
	TokFrom
	TokReturn
	TokIf
	TokElse
	TokFor
	TokIn
	TokOf
	TokWhile
	TokSwitch
	TokCase
	TokDefault
	TokTry
	TokCatch
	TokType
	TokInterface
	TokTrue
	TokFalse
	TokBreak
	TokContinue
	TokConst
	TokDeclare
	TokClass
	TokNew
	TokThis
	TokStatic

	// Types
	TokTypeString
	TokTypeNumber
	TokTypeBoolean
	TokTypeStatus
	TokTypeList  // kept for backward compat
	TokTypeArray // Array<T>

	// Punctuation
	TokLBrace           // {
	TokRBrace           // }
	TokLParen           // (
	TokRParen           // )
	TokLBracket         // [
	TokRBracket         // ]
	TokLAngle           // <
	TokRAngle           // > (also used as comparison)
	TokComma            // ,
	TokColon            // :
	TokSemicolon        // ;
	TokDot              // .
	TokQuestion         // ?
	TokQuestionQuestion // ??
	TokPipe             // |
	TokEllipsis

	// Operators
	TokAssign // =
	TokArrow  // =>
	TokPlusAssign
	TokMinusAssign
	TokStarAssign
	TokEq // ==
	TokStrictEq
	TokNeq // !=
	TokStrictNeq
	TokLt         // < (overloaded with TokLAngle for generics context)
	TokLte        // <=
	TokGte        // >=
	TokPlusPlus   // ++
	TokMinusMinus // --
	TokPlus       // +
	TokMinus      // -
	TokStar       // *
	TokSlash      // /
	TokPercent    // %
	TokAnd        // &&
	TokOr         // ||
	TokBang       // !

	// Special
	TokDollar // $ before ( for command expressions
	TokEOF
)

var keywords = map[string]TokenType{
	"let":       TokLet,
	"function":  TokFunction,
	"export":    TokExport,
	"import":    TokImport,
	"from":      TokFrom,
	"return":    TokReturn,
	"if":        TokIf,
	"else":      TokElse,
	"for":       TokFor,
	"in":        TokIn,
	"of":        TokOf,
	"while":     TokWhile,
	"switch":    TokSwitch,
	"case":      TokCase,
	"default":   TokDefault,
	"try":       TokTry,
	"catch":     TokCatch,
	"type":      TokType,
	"interface": TokInterface,
	"true":      TokTrue,
	"false":     TokFalse,
	"break":     TokBreak,
	"continue":  TokContinue,
	"const":     TokConst,
	"declare":   TokDeclare,
	"class":     TokClass,
	"new":       TokNew,
	"this":      TokThis,
	"static":    TokStatic,
	"string":    TokTypeString,
	"number":    TokTypeNumber,
	"boolean":   TokTypeBoolean,
	"status":    TokTypeStatus,
	"list":      TokTypeList,
	"Array":     TokTypeArray,
}

type Token struct {
	Type    TokenType
	Literal string
	Line    int
	Column  int
	File    string
}

func (t Token) String() string {
	return fmt.Sprintf("Token(%d, %q, %s:%d:%d)", t.Type, t.Literal, t.File, t.Line, t.Column)
}
