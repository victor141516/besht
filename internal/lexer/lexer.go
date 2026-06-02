package lexer

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

type LexError struct {
	File    string
	Line    int
	Column  int
	Message string
}

func (e *LexError) Error() string {
	return fmt.Sprintf("%s:%d:%d: %s", e.File, e.Line, e.Column, e.Message)
}

type Lexer struct {
	src    []rune
	pos    int
	line   int
	col    int
	file   string
	tokens []Token
}

func New(src, file string) *Lexer {
	return &Lexer{
		src:  []rune(src),
		pos:  0,
		line: 1,
		col:  1,
		file: file,
	}
}

func (l *Lexer) Tokenize() ([]Token, error) {
	for {
		tok, err := l.nextToken()
		if err != nil {
			return nil, err
		}
		l.tokens = append(l.tokens, tok)
		if tok.Type == TokEOF {
			break
		}
	}
	return l.tokens, nil
}

func (l *Lexer) peek() rune {
	if l.pos >= len(l.src) {
		return 0
	}
	return l.src[l.pos]
}

func (l *Lexer) peekAt(offset int) rune {
	i := l.pos + offset
	if i >= len(l.src) {
		return 0
	}
	return l.src[i]
}

func (l *Lexer) advance() rune {
	if l.pos >= len(l.src) {
		return 0
	}
	ch := l.src[l.pos]
	l.pos++
	if ch == '\n' {
		l.line++
		l.col = 1
	} else {
		l.col++
	}
	return ch
}

func (l *Lexer) skipWhitespace() {
	for l.pos < len(l.src) && unicode.IsSpace(l.peek()) {
		l.advance()
	}
}

func (l *Lexer) skipLineComment() {
	for l.pos < len(l.src) && l.peek() != '\n' {
		l.advance()
	}
}

func (l *Lexer) skipBlockComment() error {
	startLine, startCol := l.line, l.col
	l.advance() // consume /
	l.advance() // consume *
	for l.pos < len(l.src) {
		if l.peek() == '*' && l.peekAt(1) == '/' {
			l.advance()
			l.advance()
			return nil
		}
		l.advance()
	}
	return &LexError{File: l.file, Line: startLine, Column: startCol, Message: "unterminated block comment"}
}

func (l *Lexer) tok(tt TokenType, lit string, line, col int) Token {
	return Token{Type: tt, Literal: lit, Line: line, Column: col, File: l.file}
}

func (l *Lexer) nextToken() (Token, error) {
	for {
		l.skipWhitespace()
		if l.pos >= len(l.src) {
			return l.tok(TokEOF, "", l.line, l.col), nil
		}

		ch := l.peek()
		line, col := l.line, l.col

		if ch == '/' && l.peekAt(1) == '/' {
			l.skipLineComment()
			continue
		}
		if ch == '/' && l.peekAt(1) == '*' {
			if err := l.skipBlockComment(); err != nil {
				return Token{}, err
			}
			continue
		}

		switch {
		case ch == '"':
			return l.lexDoubleQuotedString(line, col)
		case ch == '\'':
			return l.lexSingleQuotedString(line, col)
		case ch == '`':
			if l.hasStringRawTagBefore() {
				return l.lexRawTemplateLit(line, col)
			}
			return l.lexTemplateLit(line, col)
		case unicode.IsDigit(ch):
			return l.lexInt(line, col)
		case unicode.IsLetter(ch) || ch == '_':
			return l.lexIdent(line, col)
		default:
			return l.lexSymbol(line, col)
		}
	}
}

func (l *Lexer) lexDoubleQuotedString(line, col int) (Token, error) {
	l.advance() // consume opening "
	var sb strings.Builder
	for l.pos < len(l.src) {
		ch := l.peek()
		if ch == '"' {
			l.advance()
			return l.tok(TokString, sb.String(), line, col), nil
		}
		if ch == '\\' {
			if err := l.writeEscape(&sb, '"', true); err != nil {
				return Token{}, err
			}
			continue
		}
		sb.WriteRune(l.advance())
	}
	return Token{}, &LexError{File: l.file, Line: line, Column: col, Message: "unterminated string literal"}
}

func (l *Lexer) lexSingleQuotedString(line, col int) (Token, error) {
	l.advance() // consume opening '
	var sb strings.Builder
	for l.pos < len(l.src) {
		ch := l.peek()
		if ch == '\'' {
			l.advance()
			return l.tok(TokString, sb.String(), line, col), nil
		}
		if ch == '\\' {
			if err := l.writeEscape(&sb, '\'', true); err != nil {
				return Token{}, err
			}
			continue
		}
		sb.WriteRune(l.advance())
	}
	return Token{}, &LexError{File: l.file, Line: line, Column: col, Message: "unterminated string literal"}
}

func (l *Lexer) lexTemplateLit(line, col int) (Token, error) {
	l.advance() // consume opening `
	var sb strings.Builder
	for l.pos < len(l.src) {
		ch := l.peek()
		if ch == '`' {
			l.advance()
			return l.tok(TokTemplateLit, sb.String(), line, col), nil
		}
		if ch == '\\' {
			if l.peekAt(1) == '$' {
				l.advance()
				l.advance()
				sb.WriteString(`\$`)
				continue
			}
			if err := l.writeEscape(&sb, '`', true); err != nil {
				return Token{}, err
			}
			continue
		}
		if ch == '$' && l.peekAt(1) == '{' {
			// ${...} interpolation — pass through verbatim, tracking brace depth
			sb.WriteRune(l.advance()) // $
			sb.WriteRune(l.advance()) // {
			depth := 1
			for l.pos < len(l.src) && depth > 0 {
				c := l.advance()
				if c == '{' {
					depth++
				} else if c == '}' {
					depth--
				}
				sb.WriteRune(c)
			}
			continue
		}
		sb.WriteRune(l.advance())
	}
	return Token{}, &LexError{File: l.file, Line: line, Column: col, Message: "unterminated template literal"}
}

func (l *Lexer) hasStringRawTagBefore() bool {
	i := l.pos - 1
	for i >= 0 && unicode.IsSpace(l.src[i]) {
		i--
	}
	needle := []rune("String.raw")
	if i+1 < len(needle) {
		return false
	}
	start := i + 1 - len(needle)
	return string(l.src[start:i+1]) == string(needle)
}

func (l *Lexer) lexRawTemplateLit(line, col int) (Token, error) {
	l.advance()
	var sb strings.Builder
	for l.pos < len(l.src) {
		ch := l.peek()
		if ch == '`' {
			l.advance()
			return l.tok(TokTemplateLit, sb.String(), line, col), nil
		}
		sb.WriteRune(l.advance())
	}
	return Token{}, &LexError{File: l.file, Line: line, Column: col, Message: "unterminated template literal"}
}

func (l *Lexer) lexInt(line, col int) (Token, error) {
	var sb strings.Builder
	for l.pos < len(l.src) && unicode.IsDigit(l.peek()) {
		sb.WriteRune(l.advance())
	}
	if l.pos < len(l.src) && l.peek() == '.' && l.pos+1 < len(l.src) && unicode.IsDigit(rune(l.src[l.pos+1])) {
		sb.WriteRune(l.advance()) // consume '.'
		for l.pos < len(l.src) && unicode.IsDigit(l.peek()) {
			sb.WriteRune(l.advance())
		}
		return l.tok(TokFloat, sb.String(), line, col), nil
	}
	return l.tok(TokInt, sb.String(), line, col), nil
}

func (l *Lexer) writeEscape(sb *strings.Builder, quote rune, decodeUnicode bool) error {
	l.advance()
	escaped := l.advance()
	switch escaped {
	case '\n':
		return nil
	case '\r':
		if l.peek() == '\n' {
			l.advance()
		}
		return nil
	case 'b':
		sb.WriteByte('\b')
	case 'f':
		sb.WriteByte('\f')
	case 'n':
		sb.WriteByte('\n')
	case 't':
		sb.WriteByte('\t')
	case 'v':
		sb.WriteByte('\v')
	case 'r':
		sb.WriteByte('\r')
	case '\\':
		sb.WriteByte('\\')
	case '"':
		sb.WriteByte('"')
	case '\'':
		sb.WriteByte('\'')
	case '`':
		sb.WriteByte('`')
	case 'x':
		if !decodeUnicode || l.pos+2 > len(l.src) {
			sb.WriteByte('x')
			return nil
		}
		hex := string(l.src[l.pos : l.pos+2])
		v, err := strconv.ParseInt(hex, 16, 8)
		if err != nil {
			sb.WriteByte('x')
			return nil
		}
		for i := 0; i < 2; i++ {
			l.advance()
		}
		sb.WriteByte(byte(v))
	case 'u':
		if !decodeUnicode || l.pos+4 > len(l.src) {
			sb.WriteByte('u')
			return nil
		}
		hex := string(l.src[l.pos : l.pos+4])
		v, err := strconv.ParseInt(hex, 16, 32)
		if err != nil {
			sb.WriteByte('u')
			return nil
		}
		for i := 0; i < 4; i++ {
			l.advance()
		}
		sb.WriteRune(rune(v))
	default:
		if escaped == quote {
			sb.WriteRune(escaped)
		} else {
			sb.WriteRune(escaped)
		}
	}
	return nil
}

func (l *Lexer) lexIdent(line, col int) (Token, error) {
	var sb strings.Builder
	for l.pos < len(l.src) && (unicode.IsLetter(l.peek()) || unicode.IsDigit(l.peek()) || l.peek() == '_') {
		sb.WriteRune(l.advance())
	}
	word := sb.String()

	if word == "r" && l.peek() == '"' {
		return l.lexRawString(line, col)
	}

	if tt, ok := keywords[word]; ok {
		return l.tok(tt, word, line, col), nil
	}
	return l.tok(TokIdent, word, line, col), nil
}

func (l *Lexer) lexRawString(line, col int) (Token, error) {
	l.advance() // consume opening "
	var sb strings.Builder
	for l.pos < len(l.src) {
		ch := l.peek()
		if ch == '"' {
			l.advance()
			return l.tok(TokRawString, sb.String(), line, col), nil
		}
		if ch == '\\' {
			l.advance()
			escaped := l.advance()
			switch escaped {
			case '"':
				sb.WriteByte('"')
			case '\\':
				sb.WriteByte('\\')
			default:
				sb.WriteByte('\\')
				sb.WriteRune(escaped)
			}
			continue
		}
		sb.WriteRune(l.advance())
	}
	return Token{}, &LexError{File: l.file, Line: line, Column: col, Message: "unterminated raw string literal"}
}

func (l *Lexer) lexSymbol(line, col int) (Token, error) {
	ch := l.advance()
	switch ch {
	case '{':
		return l.tok(TokLBrace, "{", line, col), nil
	case '}':
		return l.tok(TokRBrace, "}", line, col), nil
	case '(':
		return l.tok(TokLParen, "(", line, col), nil
	case ')':
		return l.tok(TokRParen, ")", line, col), nil
	case '[':
		return l.tok(TokLBracket, "[", line, col), nil
	case ']':
		return l.tok(TokRBracket, "]", line, col), nil
	case ',':
		return l.tok(TokComma, ",", line, col), nil
	case ':':
		return l.tok(TokColon, ":", line, col), nil
	case ';':
		return l.tok(TokSemicolon, ";", line, col), nil
	case '.':
		if l.peek() == '.' && l.peekAt(1) == '.' {
			l.advance()
			l.advance()
			return l.tok(TokEllipsis, "...", line, col), nil
		}
		return l.tok(TokDot, ".", line, col), nil
	case '?':
		if l.peek() == '?' {
			l.advance()
			return l.tok(TokQuestionQuestion, "??", line, col), nil
		}
		return l.tok(TokQuestion, "?", line, col), nil
	case '$':
		return l.tok(TokDollar, "$", line, col), nil
	case '|':
		if l.peek() == '|' {
			l.advance()
			return l.tok(TokOr, "||", line, col), nil
		}
		return l.tok(TokPipe, "|", line, col), nil
	case '&':
		if l.peek() == '&' {
			l.advance()
			return l.tok(TokAnd, "&&", line, col), nil
		}
		return Token{}, &LexError{File: l.file, Line: line, Column: col, Message: "unexpected '&', did you mean '&&'?"}
	case '=':
		if l.peek() == '>' {
			l.advance()
			return l.tok(TokArrow, "=>", line, col), nil
		}
		if l.peek() == '=' {
			l.advance()
			if l.peek() == '=' {
				l.advance()
				return l.tok(TokStrictEq, "===", line, col), nil
			}
			return l.tok(TokEq, "==", line, col), nil
		}
		return l.tok(TokAssign, "=", line, col), nil
	case '!':
		if l.peek() == '=' {
			l.advance()
			if l.peek() == '=' {
				l.advance()
				return l.tok(TokStrictNeq, "!==", line, col), nil
			}
			return l.tok(TokNeq, "!=", line, col), nil
		}
		return l.tok(TokBang, "!", line, col), nil
	case '<':
		if l.peek() == '=' {
			l.advance()
			return l.tok(TokLte, "<=", line, col), nil
		}
		return l.tok(TokLt, "<", line, col), nil
	case '>':
		if l.peek() == '=' {
			l.advance()
			return l.tok(TokGte, ">=", line, col), nil
		}
		return l.tok(TokRAngle, ">", line, col), nil
	case '+':
		if l.peek() == '+' {
			l.advance()
			return l.tok(TokPlusPlus, "++", line, col), nil
		}
		if l.peek() == '=' {
			l.advance()
			return l.tok(TokPlusAssign, "+=", line, col), nil
		}
		return l.tok(TokPlus, "+", line, col), nil
	case '-':
		if l.peek() == '-' {
			l.advance()
			return l.tok(TokMinusMinus, "--", line, col), nil
		}
		if l.peek() == '=' {
			l.advance()
			return l.tok(TokMinusAssign, "-=", line, col), nil
		}
		return l.tok(TokMinus, "-", line, col), nil
	case '*':
		if l.peek() == '=' {
			l.advance()
			return l.tok(TokStarAssign, "*=", line, col), nil
		}
		return l.tok(TokStar, "*", line, col), nil
	case '/':
		return l.tok(TokSlash, "/", line, col), nil
	case '%':
		return l.tok(TokPercent, "%", line, col), nil
	}
	return Token{}, &LexError{File: l.file, Line: line, Column: col, Message: fmt.Sprintf("unexpected character %q", ch)}
}
