package lexer_test

import (
	"testing"

	"github.com/victor141516/besht/internal/lexer"
)

func tokenize(t *testing.T, src string) []lexer.Token {
	t.Helper()
	l := lexer.New(src, "test.bsh")
	toks, err := l.Tokenize()
	if err != nil {
		t.Fatalf("unexpected lex error: %v", err)
	}
	// strip trailing EOF
	if len(toks) > 0 && toks[len(toks)-1].Type == lexer.TokEOF {
		toks = toks[:len(toks)-1]
	}
	return toks
}

func expectTypes(t *testing.T, toks []lexer.Token, want ...lexer.TokenType) {
	t.Helper()
	if len(toks) != len(want) {
		t.Fatalf("token count: got %d, want %d\ntokens: %v", len(toks), len(want), toks)
	}
	for i, tok := range toks {
		if tok.Type != want[i] {
			t.Errorf("token[%d]: got type %d (%q), want %d", i, tok.Type, tok.Literal, want[i])
		}
	}
}

func TestLexer_Keywords(t *testing.T) {
	toks := tokenize(t, "let function export import from return if else for in of while try catch true false class new this static")
	expectTypes(t, toks,
		lexer.TokLet, lexer.TokFunction, lexer.TokExport, lexer.TokImport, lexer.TokFrom,
		lexer.TokReturn, lexer.TokIf, lexer.TokElse, lexer.TokFor, lexer.TokIn, lexer.TokOf,
		lexer.TokWhile, lexer.TokTry, lexer.TokCatch, lexer.TokTrue, lexer.TokFalse,
		lexer.TokClass, lexer.TokNew, lexer.TokThis, lexer.TokStatic,
	)
}

func TestLexer_TypeKeywords(t *testing.T) {
	toks := tokenize(t, "string number boolean status list")
	expectTypes(t, toks,
		lexer.TokTypeString, lexer.TokTypeNumber, lexer.TokTypeBoolean,
		lexer.TokTypeStatus, lexer.TokTypeList,
	)
}

func TestLexer_Integers(t *testing.T) {
	toks := tokenize(t, "0 42 1000")
	expectTypes(t, toks, lexer.TokInt, lexer.TokInt, lexer.TokInt)
	if toks[1].Literal != "42" {
		t.Errorf("int literal: got %q, want %q", toks[1].Literal, "42")
	}
}

func TestLexer_StringLiteral(t *testing.T) {
	toks := tokenize(t, `"hello world"`)
	expectTypes(t, toks, lexer.TokString)
	if toks[0].Literal != "hello world" {
		t.Errorf("string literal: got %q, want %q", toks[0].Literal, "hello world")
	}
}

func TestLexer_StringEscapes(t *testing.T) {
	toks := tokenize(t, `"line1\nline2\ttabbed"`)
	expectTypes(t, toks, lexer.TokString)
	if toks[0].Literal != "line1\nline2\ttabbed" {
		t.Errorf("escaped string: got %q", toks[0].Literal)
	}
}

func TestLexer_StringWithInterpolation(t *testing.T) {
	toks := tokenize(t, `"Hello ${name}!"`)
	expectTypes(t, toks, lexer.TokString)
	if toks[0].Literal != "Hello ${name}!" {
		t.Errorf("interpolated string: got %q", toks[0].Literal)
	}
}

func TestLexer_Identifiers(t *testing.T) {
	toks := tokenize(t, "foo bar_baz _private myVar123")
	expectTypes(t, toks, lexer.TokIdent, lexer.TokIdent, lexer.TokIdent, lexer.TokIdent)
	if toks[0].Literal != "foo" {
		t.Errorf("ident: got %q", toks[0].Literal)
	}
}

func TestLexer_Operators(t *testing.T) {
	toks := tokenize(t, "= += -= *= == != < <= > >= + - * / % && || !")
	expectTypes(t, toks,
		lexer.TokAssign, lexer.TokPlusAssign, lexer.TokMinusAssign, lexer.TokStarAssign, lexer.TokEq, lexer.TokNeq,
		lexer.TokLt, lexer.TokLte, lexer.TokRAngle, lexer.TokGte,
		lexer.TokPlus, lexer.TokMinus, lexer.TokStar, lexer.TokSlash, lexer.TokPercent,
		lexer.TokAnd, lexer.TokOr, lexer.TokBang,
	)
}

func TestLexer_Arrow(t *testing.T) {
	toks := tokenize(t, `x => x`)
	expectTypes(t, toks, lexer.TokIdent, lexer.TokArrow, lexer.TokIdent)
}

func TestLexer_NullishCoalescing(t *testing.T) {
	toks := tokenize(t, `a ?? b ? c : d`)
	expectTypes(t, toks,
		lexer.TokIdent, lexer.TokQuestionQuestion, lexer.TokIdent,
		lexer.TokQuestion, lexer.TokIdent, lexer.TokColon, lexer.TokIdent,
	)
}

func TestLexer_Ellipsis(t *testing.T) {
	toks := tokenize(t, `$("echo", ...args)`)
	expectTypes(t, toks,
		lexer.TokDollar, lexer.TokLParen, lexer.TokString, lexer.TokComma, lexer.TokEllipsis, lexer.TokIdent, lexer.TokRParen,
	)
}

func TestLexer_Punctuation(t *testing.T) {
	toks := tokenize(t, "{ } ( ) [ ] , : | ?")
	expectTypes(t, toks,
		lexer.TokLBrace, lexer.TokRBrace,
		lexer.TokLParen, lexer.TokRParen,
		lexer.TokLBracket, lexer.TokRBracket,
		lexer.TokComma, lexer.TokColon,
		lexer.TokPipe, lexer.TokQuestion,
	)
}

func TestLexer_DollarParen(t *testing.T) {
	toks := tokenize(t, `$("echo", "hello")`)
	expectTypes(t, toks,
		lexer.TokDollar, lexer.TokLParen,
		lexer.TokString, lexer.TokComma, lexer.TokString,
		lexer.TokRParen,
	)
}

func TestLexer_DollarParenChained(t *testing.T) {
	toks := tokenize(t, `$("git", "log").readStdoutLines()`)
	expectTypes(t, toks,
		lexer.TokDollar, lexer.TokLParen,
		lexer.TokString, lexer.TokComma, lexer.TokString,
		lexer.TokRParen, lexer.TokDot, lexer.TokIdent, lexer.TokLParen, lexer.TokRParen,
	)
}

func TestLexer_DollarParenNoArgs(t *testing.T) {
	toks := tokenize(t, `$("whoami")`)
	expectTypes(t, toks,
		lexer.TokDollar, lexer.TokLParen,
		lexer.TokString,
		lexer.TokRParen,
	)
}

func TestLexer_LineCommentStripped(t *testing.T) {
	toks := tokenize(t, "let // this is a comment\nfunction")
	expectTypes(t, toks, lexer.TokLet, lexer.TokFunction)
}

func TestLexer_BlockCommentStripped(t *testing.T) {
	toks := tokenize(t, "let /* block comment */ function")
	expectTypes(t, toks, lexer.TokLet, lexer.TokFunction)
}

func TestLexer_PositionTracking(t *testing.T) {
	l := lexer.New("let x", "test.bsh")
	toks, _ := l.Tokenize()
	if toks[0].Line != 1 || toks[0].Column != 1 {
		t.Errorf("first token position: got %d:%d, want 1:1", toks[0].Line, toks[0].Column)
	}
	if toks[1].Column != 5 {
		t.Errorf("second token column: got %d, want 5", toks[1].Column)
	}
}

func TestLexer_MultilinePositionTracking(t *testing.T) {
	l := lexer.New("let\nfn", "test.bsh")
	toks, _ := l.Tokenize()
	if toks[1].Line != 2 {
		t.Errorf("second line token: got line %d, want 2", toks[1].Line)
	}
}

func TestLexer_LetDeclaration(t *testing.T) {
	toks := tokenize(t, `let x: string = "hello"`)
	expectTypes(t, toks,
		lexer.TokLet, lexer.TokIdent, lexer.TokColon,
		lexer.TokTypeString, lexer.TokAssign, lexer.TokString,
	)
}

func TestLexer_FnDeclaration(t *testing.T) {
	toks := tokenize(t, "function add(a: number, b: number): number")
	expectTypes(t, toks,
		lexer.TokFunction, lexer.TokIdent, lexer.TokLParen,
		lexer.TokIdent, lexer.TokColon, lexer.TokTypeNumber, lexer.TokComma,
		lexer.TokIdent, lexer.TokColon, lexer.TokTypeNumber,
		lexer.TokRParen, lexer.TokColon, lexer.TokTypeNumber,
	)
}

func TestLexer_ListType(t *testing.T) {
	toks := tokenize(t, "list<string>")
	expectTypes(t, toks, lexer.TokTypeList, lexer.TokLt, lexer.TokTypeString, lexer.TokRAngle)
}

func TestLexer_TrueFalse(t *testing.T) {
	toks := tokenize(t, "true false")
	expectTypes(t, toks, lexer.TokTrue, lexer.TokFalse)
}

func TestLexer_ImportStatement(t *testing.T) {
	toks := tokenize(t, `import { foo, bar } from "./lib"`)
	expectTypes(t, toks,
		lexer.TokImport, lexer.TokLBrace,
		lexer.TokIdent, lexer.TokComma, lexer.TokIdent,
		lexer.TokRBrace, lexer.TokFrom, lexer.TokString,
	)
}

func TestLexer_ErrorUnterminatedString(t *testing.T) {
	l := lexer.New(`"unterminated`, "test.bsh")
	_, err := l.Tokenize()
	if err == nil {
		t.Fatal("expected error for unterminated string, got nil")
	}
}

func TestLexer_ErrorUnterminatedString2(t *testing.T) {
	l := lexer.New(`$("echo`, "test.bsh")
	_, err := l.Tokenize()
	if err == nil {
		t.Fatal("expected error for unterminated string inside $(), got nil")
	}
}

func TestLexer_ErrorUnterminatedBlockComment(t *testing.T) {
	l := lexer.New("/* unterminated", "test.bsh")
	_, err := l.Tokenize()
	if err == nil {
		t.Fatal("expected error for unterminated block comment, got nil")
	}
}

func TestLexer_ErrorUnexpectedAmpersand(t *testing.T) {
	l := lexer.New("a & b", "test.bsh")
	_, err := l.Tokenize()
	if err == nil {
		t.Fatal("expected error for single '&', got nil")
	}
}

func TestLexer_ErrorUnexpectedChar(t *testing.T) {
	l := lexer.New("@", "test.bsh")
	_, err := l.Tokenize()
	if err == nil {
		t.Fatal("expected error for '@', got nil")
	}
}

func TestLexer_WhitespaceIgnored(t *testing.T) {
	toks := tokenize(t, "  \t  let  \n  function  ")
	expectTypes(t, toks, lexer.TokLet, lexer.TokFunction)
}

func TestLexer_EOF(t *testing.T) {
	l := lexer.New("", "test.bsh")
	toks, err := l.Tokenize()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(toks) != 1 || toks[0].Type != lexer.TokEOF {
		t.Errorf("empty input should produce only EOF, got %v", toks)
	}
}

func TestLexer_RawStringLiteral(t *testing.T) {
	toks := tokenize(t, `r"hello world"`)
	expectTypes(t, toks, lexer.TokRawString)
	if toks[0].Literal != "hello world" {
		t.Errorf("raw string: got %q, want %q", toks[0].Literal, "hello world")
	}
}

func TestLexer_RawStringWithDollar(t *testing.T) {
	toks := tokenize(t, `r"-cache$"`)
	expectTypes(t, toks, lexer.TokRawString)
	if toks[0].Literal != "-cache$" {
		t.Errorf("raw string dollar: got %q", toks[0].Literal)
	}
}

func TestLexer_RawStringWithRegex(t *testing.T) {
	toks := tokenize(t, `r"^foo-[0-9]+$"`)
	expectTypes(t, toks, lexer.TokRawString)
	if toks[0].Literal != "^foo-[0-9]+$" {
		t.Errorf("raw string regex: got %q", toks[0].Literal)
	}
}

func TestLexer_RawStringEscapedQuote(t *testing.T) {
	toks := tokenize(t, `r"say \"hi\""`)
	expectTypes(t, toks, lexer.TokRawString)
	if toks[0].Literal != `say "hi"` {
		t.Errorf("raw string escaped quote: got %q", toks[0].Literal)
	}
}

func TestLexer_EscapedDollarInString(t *testing.T) {
	toks := tokenize(t, `"price is \$5"`)
	expectTypes(t, toks, lexer.TokString)
	if toks[0].Literal != `price is \$5` {
		t.Errorf("escaped dollar: got %q", toks[0].Literal)
	}
}

func TestLexer_RawStringUnterminatedError(t *testing.T) {
	l := lexer.New(`r"unterminated`, "test.bsh")
	_, err := l.Tokenize()
	if err == nil {
		t.Fatal("expected error for unterminated raw string, got nil")
	}
}
