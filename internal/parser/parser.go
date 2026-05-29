package parser

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/victor141516/besht/internal/ast"
	"github.com/victor141516/besht/internal/lexer"
)

type ParseError struct {
	File    string
	Line    int
	Column  int
	Message string
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("%s:%d:%d: %s", e.File, e.Line, e.Column, e.Message)
}

type Parser struct {
	tokens      []lexer.Token
	pos         int
	file        string
	typeAliases map[string]*ast.Type
}

func (p *Parser) peekN(offset int) lexer.Token {
	idx := p.pos + offset
	if idx >= len(p.tokens) {
		return lexer.Token{Type: lexer.TokEOF}
	}
	return p.tokens[idx]
}

func New(tokens []lexer.Token, file string) *Parser {
	return &Parser{tokens: tokens, pos: 0, file: file, typeAliases: make(map[string]*ast.Type)}
}

func Parse(src, file string) (*ast.Program, error) {
	l := lexer.New(src, file)
	tokens, err := l.Tokenize()
	if err != nil {
		return nil, err
	}
	p := New(tokens, file)
	return p.parseProgram()
}

func (p *Parser) peek() lexer.Token {
	if p.pos >= len(p.tokens) {
		return lexer.Token{Type: lexer.TokEOF}
	}
	return p.tokens[p.pos]
}

func (p *Parser) peekType() lexer.TokenType {
	return p.peek().Type
}

func (p *Parser) advance() lexer.Token {
	tok := p.peek()
	if p.pos < len(p.tokens) {
		p.pos++
	}
	return tok
}

func (p *Parser) expect(tt lexer.TokenType) (lexer.Token, error) {
	tok := p.peek()
	if tok.Type != tt {
		return lexer.Token{}, p.errorf(tok, "expected token %d, got %q", tt, tok.Literal)
	}
	return p.advance(), nil
}

func isIdentifierName(tt lexer.TokenType) bool {
	switch tt {
	case lexer.TokIdent, lexer.TokTypeString, lexer.TokTypeNumber, lexer.TokTypeBoolean, lexer.TokTypeStatus, lexer.TokTypeList, lexer.TokTypeArray, lexer.TokFrom:
		return true
	}
	return false
}

func (p *Parser) expectIdentifierName() (lexer.Token, error) {
	tok := p.peek()
	if !isIdentifierName(tok.Type) {
		return lexer.Token{}, p.errorf(tok, "expected identifier, got %q", tok.Literal)
	}
	return p.advance(), nil
}

func (p *Parser) errorf(tok lexer.Token, format string, args ...interface{}) error {
	return &ParseError{
		File:    tok.File,
		Line:    tok.Line,
		Column:  tok.Column,
		Message: fmt.Sprintf(format, args...),
	}
}

func (p *Parser) skipSemicolons() {
	for p.peekType() == lexer.TokSemicolon {
		p.advance()
	}
}

func (p *Parser) pos2ast(tok lexer.Token) ast.Pos {
	return ast.Pos{File: tok.File, Line: tok.Line, Column: tok.Column}
}

func (p *Parser) parseProgram() (*ast.Program, error) {
	prog := &ast.Program{File: p.file}
	for p.peekType() != lexer.TokEOF {
		stmt, err := p.parseStatement()
		if err != nil {
			return nil, err
		}
		if imp, ok := stmt.(*ast.ImportDecl); ok {
			prog.Imports = append(prog.Imports, imp)
		} else {
			prog.Statements = append(prog.Statements, stmt)
		}
		p.skipSemicolons()
	}
	return prog, nil
}

func (p *Parser) parseStatement() (ast.Statement, error) {
	tok := p.peek()
	switch tok.Type {
	case lexer.TokImport:
		return p.parseImport()
	case lexer.TokLet, lexer.TokConst:
		return p.parseLetDecl()
	case lexer.TokFunction:
		return p.parseFnDecl()
	case lexer.TokClass:
		return p.parseClassDecl(false)
	case lexer.TokExport:
		return p.parseExport()
	case lexer.TokReturn:
		return p.parseReturn()
	case lexer.TokIf:
		return p.parseIf()
	case lexer.TokFor:
		return p.parseFor()
	case lexer.TokWhile:
		return p.parseWhile()
	case lexer.TokSwitch:
		return p.parseSwitch()
	case lexer.TokTry:
		return p.parseTry()
	case lexer.TokDeclare:
		return p.parseDeclare(false)
	case lexer.TokType:
		return p.parseTypeDecl()
	case lexer.TokInterface:
		return p.parseInterfaceDecl()
	case lexer.TokBreak:
		p.advance()
		return &ast.BreakStmt{Pos: p.pos2ast(tok)}, nil
	case lexer.TokContinue:
		p.advance()
		return &ast.ContinueStmt{Pos: p.pos2ast(tok)}, nil
	case lexer.TokRawString, lexer.TokTemplateLit, lexer.TokDollar, lexer.TokNew, lexer.TokPlusPlus, lexer.TokMinusMinus:
		expr, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		return &ast.ExprStmt{Pos: p.pos2ast(tok), Expr: expr}, nil
	case lexer.TokIdent:
		return p.parseIdentStmt()
	case lexer.TokThis:
		return p.parseThisStmt()
	}
	return nil, p.errorf(tok, "unexpected token %q in statement position", tok.Literal)
}

func (p *Parser) parseClassDecl(exported bool) (*ast.ClassDecl, error) {
	tok := p.peek()
	pos := p.pos2ast(tok)
	if exported {
		p.advance()
	}
	if _, err := p.expect(lexer.TokClass); err != nil {
		return nil, err
	}
	nameTok, err := p.expectIdentifierName()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(lexer.TokLBrace); err != nil {
		return nil, err
	}
	decl := &ast.ClassDecl{Pos: pos, Name: nameTok.Literal, Exported: exported}
	for p.peekType() != lexer.TokRBrace && p.peekType() != lexer.TokEOF {
		p.skipSemicolons()
		if p.peekType() == lexer.TokRBrace || p.peekType() == lexer.TokEOF {
			break
		}
		isStatic := false
		for {
			if p.peekType() == lexer.TokStatic {
				isStatic = true
				p.advance()
				continue
			}
			if p.peekType() == lexer.TokIdent {
				switch p.peek().Literal {
				case "private", "public", "protected", "readonly":
					p.advance()
					continue
				}
			}
			break
		}
		memberTok := p.peek()
		if !isIdentifierName(memberTok.Type) {
			return nil, p.errorf(memberTok, "expected class member name")
		}
		p.advance()
		memberPos := p.pos2ast(memberTok)
		name := memberTok.Literal
		if p.peekType() == lexer.TokLParen {
			params, err := p.parseParamsAfterOpen()
			if err != nil {
				return nil, err
			}
			var returnType *ast.Type
			if name != "constructor" && p.peekType() == lexer.TokColon {
				p.advance()
				returnType, err = p.parseType()
				if err != nil {
					return nil, err
				}
			}
			body, err := p.parseBlock()
			if err != nil {
				return nil, err
			}
			method := ast.ClassMethod{Pos: memberPos, Name: name, IsStatic: isStatic, Params: params, ReturnType: returnType, Body: body}
			if name == "constructor" {
				decl.Constructor = &method
			} else {
				decl.Methods = append(decl.Methods, method)
			}
			continue
		}
		if _, err := p.expect(lexer.TokColon); err != nil {
			return nil, err
		}
		typeAnnot, err := p.parseType()
		if err != nil {
			return nil, err
		}
		var init ast.Expression
		if p.peekType() == lexer.TokAssign {
			p.advance()
			init, err = p.parseExpr()
			if err != nil {
				return nil, err
			}
		}
		prop := ast.ClassProperty{Pos: memberPos, Name: name, IsStatic: isStatic, Type: typeAnnot, Value: init}
		if isStatic {
			decl.StaticProps = append(decl.StaticProps, prop)
		} else {
			decl.Properties = append(decl.Properties, prop)
		}
		p.skipSemicolons()
	}
	if _, err := p.expect(lexer.TokRBrace); err != nil {
		return nil, err
	}
	return decl, nil
}

func (p *Parser) parseParamsAfterOpen() ([]*ast.Param, error) {
	return p.parseParamsOnly()
}

func (p *Parser) parseExport() (ast.Statement, error) {
	tok := p.advance() // export
	pos := p.pos2ast(tok)
	switch p.peekType() {
	case lexer.TokDeclare:
		return p.parseDeclare(true)
	case lexer.TokType:
		return p.parseTypeDecl()
	case lexer.TokInterface:
		return p.parseInterfaceDecl()
	case lexer.TokClass:
		p.pos--
		return p.parseClassDecl(true)
	case lexer.TokFunction:
		p.pos--
		return p.parseFnDecl()
	case lexer.TokLet, lexer.TokConst:
		stmt, err := p.parseLetDecl()
		if err != nil {
			return nil, err
		}
		decl, ok := stmt.(*ast.LetDecl)
		if !ok {
			return nil, p.errorf(tok, "export only supports simple let/const declarations")
		}
		decl.Exported = true
		decl.Pos = pos
		return decl, nil
	case lexer.TokDefault:
		p.advance()
		value, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		return &ast.LetDecl{Pos: pos, Name: "default", IsConst: true, Exported: true, DefaultExport: true, Value: value}, nil
	}
	return nil, p.errorf(p.peek(), "unsupported export form")
}

func (p *Parser) parseImport() (*ast.ImportDecl, error) {
	tok := p.advance() // import
	pos := p.pos2ast(tok)
	decl := &ast.ImportDecl{Pos: pos}

	if p.peekType() != lexer.TokLBrace {
		nameTok, err := p.expectIdentifierName()
		if err != nil {
			return nil, err
		}
		decl.DefaultName = nameTok.Literal
		if p.peekType() == lexer.TokComma {
			p.advance()
			if p.peekType() != lexer.TokLBrace {
				return nil, p.errorf(p.peek(), "expected named import list after comma in import")
			}
		}
	}

	if p.peekType() == lexer.TokLBrace {
		p.advance()
		for p.peekType() != lexer.TokRBrace && p.peekType() != lexer.TokEOF {
			nameTok, err := p.expectIdentifierName()
			if err != nil {
				return nil, err
			}
			decl.Names = append(decl.Names, nameTok.Literal)
			if p.peekType() == lexer.TokComma {
				p.advance()
			}
		}
		if _, err := p.expect(lexer.TokRBrace); err != nil {
			return nil, err
		}
	}
	if decl.DefaultName == "" && len(decl.Names) == 0 {
		return nil, p.errorf(tok, "import requires a default name or named import list")
	}
	if _, err := p.expect(lexer.TokFrom); err != nil {
		return nil, err
	}
	srcTok, err := p.expect(lexer.TokString)
	if err != nil {
		return nil, err
	}
	decl.Source = srcTok.Literal

	if p.peekType() == lexer.TokIdent && p.peek().Literal == "assert" {
		p.advance()
		if _, err := p.expect(lexer.TokLBrace); err != nil {
			return nil, err
		}
		keyTok := p.peek()
		if keyTok.Type != lexer.TokType {
			return nil, p.errorf(keyTok, "expected import assertion key \"type\", got %q", keyTok.Literal)
		}
		p.advance()
		if _, err := p.expect(lexer.TokColon); err != nil {
			return nil, err
		}
		valueTok, err := p.expect(lexer.TokString)
		if err != nil {
			return nil, err
		}
		if valueTok.Literal != "shell" {
			return nil, p.errorf(valueTok, "expected import assertion type \"shell\", got %q", valueTok.Literal)
		}
		if _, err := p.expect(lexer.TokRBrace); err != nil {
			return nil, err
		}
		decl.AssertType = valueTok.Literal
	}

	return decl, nil
}

func (p *Parser) parseLetDecl() (ast.Statement, error) {
	tok := p.advance() // let or const
	pos := p.pos2ast(tok)
	isConst := tok.Type == lexer.TokConst
	if p.peekType() == lexer.TokLBracket {
		p.advance()
		var names []string
		for p.peekType() != lexer.TokRBracket && p.peekType() != lexer.TokEOF {
			nameTok, err := p.expectIdentifierName()
			if err != nil {
				return nil, err
			}
			names = append(names, nameTok.Literal)
			if p.peekType() == lexer.TokComma {
				p.advance()
			}
		}
		if _, err := p.expect(lexer.TokRBracket); err != nil {
			return nil, err
		}
		if _, err := p.expect(lexer.TokAssign); err != nil {
			return nil, err
		}
		value, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		return &ast.DestructureDecl{Pos: pos, Names: names, IsConst: isConst, Value: value}, nil
	}

	nameTok, err := p.expectIdentifierName()
	if err != nil {
		return nil, err
	}
	var typeAnnot *ast.Type
	if p.peekType() == lexer.TokColon {
		p.advance()
		typeAnnot, err = p.parseType()
		if err != nil {
			return nil, err
		}
	}
	if _, err := p.expect(lexer.TokAssign); err != nil {
		return nil, err
	}
	value, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	return &ast.LetDecl{Pos: pos, Name: nameTok.Literal, IsConst: isConst, TypeAnnot: typeAnnot, Value: value}, nil
}

func (p *Parser) parseType() (*ast.Type, error) {
	tok := p.advance()
	pos := p.pos2ast(tok)
	var t *ast.Type
	switch tok.Type {
	case lexer.TokLBracket:
		var first *ast.Type
		for p.peekType() != lexer.TokRBracket && p.peekType() != lexer.TokEOF {
			elem, err := p.parseType()
			if err != nil {
				return nil, err
			}
			if first == nil {
				first = elem
			}
			if p.peekType() == lexer.TokComma {
				p.advance()
			}
		}
		if _, err := p.expect(lexer.TokRBracket); err != nil {
			return nil, err
		}
		if first == nil {
			first = &ast.Type{Kind: ast.TypeString}
		}
		t = &ast.Type{Kind: ast.TypeList, Elem: first, Pos: pos}
	case lexer.TokTypeString:
		t = &ast.Type{Kind: ast.TypeString, Pos: pos}
	case lexer.TokTypeNumber:
		t = &ast.Type{Kind: ast.TypeNumber, Pos: pos}
	case lexer.TokTypeBoolean:
		t = &ast.Type{Kind: ast.TypeBoolean, Pos: pos}
	case lexer.TokTypeStatus:
		t = &ast.Type{Kind: ast.TypeStatus, Pos: pos}
	case lexer.TokTypeList:
		if _, err := p.expect(lexer.TokLt); err != nil {
			return nil, err
		}
		elem, err := p.parseType()
		if err != nil {
			return nil, err
		}
		if p.peekType() != lexer.TokRAngle {
			return nil, p.errorf(p.peek(), "expected '>' to close list<...>")
		}
		p.advance()
		t = &ast.Type{Kind: ast.TypeList, Elem: elem, Pos: pos}
	case lexer.TokTypeArray:
		if p.peekType() == lexer.TokLt {
			p.advance() // consume <
			elem, err := p.parseType()
			if err != nil {
				return nil, err
			}
			if p.peekType() != lexer.TokRAngle {
				return nil, p.errorf(p.peek(), "expected '>' to close Array<...>")
			}
			p.advance()
			t = &ast.Type{Kind: ast.TypeList, Elem: elem, Pos: pos}
		} else {
			t = &ast.Type{Kind: ast.TypeList, Elem: &ast.Type{Kind: ast.TypeString}, Pos: pos}
		}
	case lexer.TokIdent:
		if alias, ok := p.typeAliases[tok.Literal]; ok {
			t = cloneType(alias)
		} else if tok.Literal == "Set" && p.peekType() == lexer.TokLt {
			p.advance()
			elem, err := p.parseType()
			if err != nil {
				return nil, err
			}
			if p.peekType() != lexer.TokRAngle {
				return nil, p.errorf(p.peek(), "expected '>' to close Set<...>")
			}
			p.advance()
			t = &ast.Type{Kind: ast.TypeSet, Elem: elem, Pos: pos}
		} else if tok.Literal == "Record" && p.peekType() == lexer.TokLt {
			p.advance()
			if _, err := p.parseType(); err != nil {
				return nil, err
			}
			if _, err := p.expect(lexer.TokComma); err != nil {
				return nil, err
			}
			elem, err := p.parseType()
			if err != nil {
				return nil, err
			}
			if p.peekType() != lexer.TokRAngle {
				return nil, p.errorf(p.peek(), "expected '>' to close Record<...>")
			}
			p.advance()
			t = &ast.Type{Kind: ast.TypeObject, Elem: elem, Pos: pos}
		} else if tok.Literal == "undefined" {
			t = &ast.Type{Kind: ast.TypeString, Pos: pos}
		} else {
			t = &ast.Type{Kind: ast.TypeString, Pos: pos}
		}
	default:
		return nil, p.errorf(tok, "expected type, got %q", tok.Literal)
	}
	for p.peekType() == lexer.TokPipe {
		p.advance()
		if _, err := p.parseType(); err != nil {
			return nil, err
		}
	}
	// Postfix [] syntax: string[] → list<string>
	for p.peekType() == lexer.TokLBracket {
		p.advance() // consume [
		if _, err := p.expect(lexer.TokRBracket); err != nil {
			return nil, err
		}
		t = &ast.Type{Kind: ast.TypeList, Elem: t, Pos: pos}
	}
	return t, nil
}

func cloneType(t *ast.Type) *ast.Type {
	if t == nil {
		return nil
	}
	clone := *t
	clone.Elem = cloneType(t.Elem)
	return &clone
}

func (p *Parser) parseFnDecl() (*ast.FnDecl, error) {
	var exported bool
	tok := p.peek()
	pos := p.pos2ast(tok)

	if tok.Type == lexer.TokExport {
		exported = true
		p.advance()
	}
	if _, err := p.expect(lexer.TokFunction); err != nil {
		return nil, err
	}
	nameTok, err := p.expectIdentifierName()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(lexer.TokLParen); err != nil {
		return nil, err
	}

	var params []*ast.Param
	for p.peekType() != lexer.TokRParen && p.peekType() != lexer.TokEOF {
		pname, err := p.expectIdentifierName()
		if err != nil {
			return nil, err
		}
		var ptype *ast.Type
		if p.peekType() == lexer.TokColon {
			p.advance()
			ptype, err = p.parseType()
			if err != nil {
				return nil, err
			}
		}
		params = append(params, &ast.Param{Pos: p.pos2ast(pname), Name: pname.Literal, Type: ptype})
		if p.peekType() == lexer.TokComma {
			p.advance()
		}
	}
	if _, err := p.expect(lexer.TokRParen); err != nil {
		return nil, err
	}

	var returnType *ast.Type
	if p.peekType() == lexer.TokColon {
		p.advance()
		returnType, err = p.parseType()
		if err != nil {
			return nil, err
		}
	}

	body, err := p.parseBlock()
	if err != nil {
		return nil, err
	}
	return &ast.FnDecl{
		Pos:        pos,
		Name:       nameTok.Literal,
		Exported:   exported,
		Params:     params,
		ReturnType: returnType,
		Body:       body,
	}, nil
}

func (p *Parser) parseBlock() (*ast.Block, error) {
	tok, err := p.expect(lexer.TokLBrace)
	if err != nil {
		return nil, err
	}
	block := &ast.Block{Pos: p.pos2ast(tok)}

	for p.peekType() != lexer.TokRBrace && p.peekType() != lexer.TokEOF {
		stmt, err := p.parseStatement()
		if err != nil {
			return nil, err
		}
		block.Statements = append(block.Statements, stmt)
		p.skipSemicolons()
	}
	if _, err := p.expect(lexer.TokRBrace); err != nil {
		return nil, err
	}
	return block, nil
}

func (p *Parser) parseReturn() (*ast.ReturnStmt, error) {
	tok := p.advance() // return
	pos := p.pos2ast(tok)

	if p.peekType() == lexer.TokRBrace || p.peekType() == lexer.TokEOF {
		return &ast.ReturnStmt{Pos: pos}, nil
	}
	val, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	return &ast.ReturnStmt{Pos: pos, Value: val}, nil
}

func (p *Parser) parseIf() (*ast.IfStmt, error) {
	tok := p.advance() // if
	pos := p.pos2ast(tok)

	if _, err := p.expect(lexer.TokLParen); err != nil {
		return nil, err
	}
	cond, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(lexer.TokRParen); err != nil {
		return nil, err
	}
	then, err := p.parseControlFlowBody()
	if err != nil {
		return nil, err
	}
	stmt := &ast.IfStmt{Pos: pos, Condition: cond, Then: then}
	p.skipSemicolons()

	for p.peekType() == lexer.TokElse {
		p.advance()
		if p.peekType() == lexer.TokIf {
			p.advance()
			if _, err := p.expect(lexer.TokLParen); err != nil {
				return nil, err
			}
			eicond, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			if _, err := p.expect(lexer.TokRParen); err != nil {
				return nil, err
			}
			eibody, err := p.parseControlFlowBody()
			if err != nil {
				return nil, err
			}
			stmt.ElseIfs = append(stmt.ElseIfs, &ast.ElseIf{
				Pos:       p.pos2ast(p.peek()),
				Condition: eicond,
				Body:      eibody,
			})
			p.skipSemicolons()
		} else {
			elseBody, err := p.parseControlFlowBody()
			if err != nil {
				return nil, err
			}
			stmt.Else = elseBody
			break
		}
	}
	return stmt, nil
}

func (p *Parser) parseControlFlowBody() (*ast.Block, error) {
	if p.peekType() == lexer.TokLBrace {
		return p.parseBlock()
	}
	pos := p.pos2ast(p.peek())
	stmt, err := p.parseStatement()
	if err != nil {
		return nil, err
	}
	return &ast.Block{Pos: pos, Statements: []ast.Statement{stmt}}, nil
}

func (p *Parser) parseFor() (ast.Statement, error) {
	tok := p.advance() // for
	pos := p.pos2ast(tok)

	if _, err := p.expect(lexer.TokLParen); err != nil {
		return nil, err
	}

	if p.peekType() == lexer.TokLet || p.peekType() == lexer.TokConst {
		return p.parseDeclaredFor(pos)
	}
	if p.peekType() == lexer.TokIdent {
		// Save position, consume ident, peek next
		nameTok := p.peek()
		p.advance()
		if p.peekType() == lexer.TokIn || p.peekType() == lexer.TokOf {
			p.advance()
			iter, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			if _, err := p.expect(lexer.TokRParen); err != nil {
				return nil, err
			}
			body, err := p.parseControlFlowBody()
			if err != nil {
				return nil, err
			}
			return &ast.ForStmt{Pos: pos, VarName: nameTok.Literal, Iterator: iter, Body: body}, nil
		}
		// Must be C-style with bare assignment: i = 0; cond; update
		// We already consumed the ident — parse as Assignment init
		if _, err := p.expect(lexer.TokAssign); err != nil {
			return nil, p.errorf(p.peek(), "expected 'in' or '=' after loop variable, got %q", p.peek().Literal)
		}
		initVal, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		initStmt := &ast.Assignment{Pos: pos, Name: nameTok.Literal, Value: initVal}
		return p.parseCStyleForRest(pos, initStmt)
	}
	return nil, p.errorf(p.peek(), "expected loop variable or 'let' in for statement")
}

func (p *Parser) parseCStyleFor(pos ast.Pos) (*ast.CStyleForStmt, error) {
	initTok := p.advance() // let or const
	nameTok, err := p.expectIdentifierName()
	if err != nil {
		return nil, err
	}
	// Optional type annotation
	var typeAnnot *ast.Type
	if p.peekType() == lexer.TokColon {
		p.advance()
		typeAnnot, err = p.parseType()
		if err != nil {
			return nil, err
		}
	}
	if _, err := p.expect(lexer.TokAssign); err != nil {
		return nil, err
	}
	initVal, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	_ = initTok
	initStmt := &ast.LetDecl{Pos: pos, Name: nameTok.Literal, TypeAnnot: typeAnnot, Value: initVal}
	return p.parseCStyleForRest(pos, initStmt)
}

func (p *Parser) parseDeclaredFor(pos ast.Pos) (ast.Statement, error) {
	tok := p.advance()
	isConst := tok.Type == lexer.TokConst
	nameTok, err := p.expectIdentifierName()
	if err != nil {
		return nil, err
	}
	var typeAnnot *ast.Type
	if p.peekType() == lexer.TokColon {
		p.advance()
		typeAnnot, err = p.parseType()
		if err != nil {
			return nil, err
		}
	}
	if p.peekType() == lexer.TokIn || p.peekType() == lexer.TokOf {
		p.advance()
		iter, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(lexer.TokRParen); err != nil {
			return nil, err
		}
		body, err := p.parseControlFlowBody()
		if err != nil {
			return nil, err
		}
		_ = typeAnnot
		_ = isConst
		return &ast.ForStmt{Pos: pos, VarName: nameTok.Literal, Iterator: iter, Body: body}, nil
	}
	if _, err := p.expect(lexer.TokAssign); err != nil {
		return nil, err
	}
	initVal, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	initStmt := &ast.LetDecl{Pos: pos, Name: nameTok.Literal, IsConst: isConst, TypeAnnot: typeAnnot, Value: initVal}
	return p.parseCStyleForRest(pos, initStmt)
}

func compoundAssignment(name string, pos ast.Pos, op string, value ast.Expression) *ast.Assignment {
	return &ast.Assignment{
		Pos:  pos,
		Name: name,
		Value: &ast.BinaryExpr{
			Pos:   pos,
			Op:    op,
			Left:  &ast.IdentExpr{Pos: pos, Name: name},
			Right: value,
		},
	}
}

func (p *Parser) parseCStyleForRest(pos ast.Pos, init ast.Statement) (*ast.CStyleForStmt, error) {
	if _, err := p.expect(lexer.TokSemicolon); err != nil {
		return nil, err
	}
	cond, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(lexer.TokSemicolon); err != nil {
		return nil, err
	}
	update, err := p.parseCStyleForUpdate()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(lexer.TokRParen); err != nil {
		return nil, err
	}
	body, err := p.parseControlFlowBody()
	if err != nil {
		return nil, err
	}
	return &ast.CStyleForStmt{Pos: pos, Init: init, Condition: cond, Update: update, Body: body}, nil
}

func (p *Parser) parseCStyleForUpdate() (ast.Statement, error) {
	tok := p.peek()
	pos := p.pos2ast(tok)
	nameTok, err := p.expectIdentifierName()
	if err != nil {
		return nil, err
	}
	name := nameTok.Literal
	switch p.peekType() {
	case lexer.TokPlusPlus:
		p.advance()
		one := &ast.IntLit{Pos: pos, Value: 1}
		return &ast.Assignment{Pos: pos, Name: name, Value: &ast.BinaryExpr{Pos: pos, Op: "+", Left: &ast.IdentExpr{Pos: pos, Name: name}, Right: one}}, nil
	case lexer.TokMinusMinus:
		p.advance()
		one := &ast.IntLit{Pos: pos, Value: 1}
		return &ast.Assignment{Pos: pos, Name: name, Value: &ast.BinaryExpr{Pos: pos, Op: "-", Left: &ast.IdentExpr{Pos: pos, Name: name}, Right: one}}, nil
	case lexer.TokAssign:
		p.advance()
		val, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		return &ast.Assignment{Pos: pos, Name: name, Value: val}, nil
	case lexer.TokPlusAssign, lexer.TokMinusAssign, lexer.TokStarAssign:
		opTok := p.advance()
		val, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		op := strings.TrimSuffix(opTok.Literal, "=")
		return compoundAssignment(name, pos, op, val), nil
	}
	return nil, p.errorf(p.peek(), "expected ++, --, =, +=, -=, or *= in for loop update clause")
}

func (p *Parser) parseWhile() (*ast.WhileStmt, error) {
	tok := p.advance() // while
	pos := p.pos2ast(tok)

	if _, err := p.expect(lexer.TokLParen); err != nil {
		return nil, err
	}
	cond, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(lexer.TokRParen); err != nil {
		return nil, err
	}
	body, err := p.parseControlFlowBody()
	if err != nil {
		return nil, err
	}
	return &ast.WhileStmt{Pos: pos, Condition: cond, Body: body}, nil
}

func (p *Parser) parseTry() (*ast.TryStmt, error) {
	tok := p.advance() // try
	pos := p.pos2ast(tok)

	body, err := p.parseBlock()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(lexer.TokCatch); err != nil {
		return nil, err
	}
	if _, err := p.expect(lexer.TokLParen); err != nil {
		return nil, err
	}
	catchVarTok, err := p.expect(lexer.TokIdent)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(lexer.TokColon); err != nil {
		return nil, err
	}
	if _, err := p.expect(lexer.TokTypeStatus); err != nil {
		return nil, err
	}
	if _, err := p.expect(lexer.TokRParen); err != nil {
		return nil, err
	}
	catchBody, err := p.parseBlock()
	if err != nil {
		return nil, err
	}
	return &ast.TryStmt{Pos: pos, Body: body, CatchVar: catchVarTok.Literal, Catch: catchBody}, nil
}

func (p *Parser) parseSwitch() (*ast.SwitchStmt, error) {
	tok := p.advance()
	pos := p.pos2ast(tok)
	if _, err := p.expect(lexer.TokLParen); err != nil {
		return nil, err
	}
	value, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(lexer.TokRParen); err != nil {
		return nil, err
	}
	if _, err := p.expect(lexer.TokLBrace); err != nil {
		return nil, err
	}
	stmt := &ast.SwitchStmt{Pos: pos, Value: value}
	for p.peekType() != lexer.TokRBrace && p.peekType() != lexer.TokEOF {
		caseTok := p.peek()
		casePos := p.pos2ast(caseTok)
		var swCase ast.SwitchCase
		if p.peekType() == lexer.TokCase {
			p.advance()
			caseValue, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			swCase = ast.SwitchCase{Pos: casePos, Value: caseValue}
		} else if p.peekType() == lexer.TokDefault {
			p.advance()
			swCase = ast.SwitchCase{Pos: casePos, IsDefault: true}
		} else {
			return nil, p.errorf(caseTok, "expected case or default in switch")
		}
		if _, err := p.expect(lexer.TokColon); err != nil {
			return nil, err
		}
		body := &ast.Block{Pos: casePos}
		for p.peekType() != lexer.TokCase && p.peekType() != lexer.TokDefault && p.peekType() != lexer.TokRBrace && p.peekType() != lexer.TokEOF {
			bodyStmt, err := p.parseStatement()
			if err != nil {
				return nil, err
			}
			body.Statements = append(body.Statements, bodyStmt)
			p.skipSemicolons()
		}
		swCase.Body = body
		stmt.Cases = append(stmt.Cases, swCase)
	}
	if _, err := p.expect(lexer.TokRBrace); err != nil {
		return nil, err
	}
	return stmt, nil
}

func (p *Parser) parseDeclare(exported bool) (ast.Statement, error) {
	tok := p.advance() // declare
	pos := p.pos2ast(tok)
	if p.peekType() == lexer.TokFunction {
		fn, err := p.parseDeclareFunction(pos)
		if err != nil {
			return nil, err
		}
		fn.Exported = exported
		return fn, nil
	}
	// Consume the sub-keyword (function, const, type, namespace, var, interface, etc.)
	// then consume everything until we hit a top-level statement boundary.
	if p.peekType() != lexer.TokEOF {
		p.advance() // consume: function | const | type | namespace | var | interface | ...
	}
	depth := 0
	for p.peekType() != lexer.TokEOF {
		t := p.peekType()
		if depth == 0 {
			switch t {
			case lexer.TokLet, lexer.TokConst, lexer.TokFunction,
				lexer.TokExport, lexer.TokImport, lexer.TokDeclare,
				lexer.TokIf, lexer.TokFor, lexer.TokWhile, lexer.TokSwitch,
				lexer.TokTry, lexer.TokReturn, lexer.TokBreak,
				lexer.TokContinue:
				return &ast.DeclareStmt{Pos: pos}, nil
			}
		}
		if t == lexer.TokLBrace {
			depth++
		} else if t == lexer.TokRBrace {
			if depth == 0 {
				return &ast.DeclareStmt{Pos: pos}, nil
			}
			depth--
			if depth == 0 {
				p.advance()
				return &ast.DeclareStmt{Pos: pos}, nil
			}
		}
		p.advance()
	}
	return &ast.DeclareStmt{Pos: pos}, nil
}

func (p *Parser) parseTypeDecl() (ast.Statement, error) {
	tok := p.advance()
	pos := p.pos2ast(tok)
	var aliasName string
	genericAlias := false
	if p.peekType() == lexer.TokIdent {
		aliasName = p.advance().Literal
	}
	if p.peekType() == lexer.TokLt {
		genericAlias = true
		p.advance()
		depth := 1
		for p.peekType() != lexer.TokEOF && depth > 0 {
			if p.peekType() == lexer.TokLt {
				depth++
			} else if p.peekType() == lexer.TokRAngle {
				depth--
			}
			p.advance()
		}
	}
	p.skipSemicolons()
	if p.peekType() == lexer.TokAssign {
		p.advance()
		saved := p.pos
		if aliasName != "" && !genericAlias {
			if t, err := p.parseType(); err == nil {
				p.typeAliases[aliasName] = t
			}
		}
		p.pos = saved
	}
	return p.skipToStmtBoundary(pos)
}

func (p *Parser) parseInterfaceDecl() (ast.Statement, error) {
	tok := p.advance()
	pos := p.pos2ast(tok)
	if p.peekType() == lexer.TokIdent {
		p.advance()
	}
	depth := 0
	for p.peekType() != lexer.TokEOF {
		t := p.peekType()
		if t == lexer.TokLBrace {
			depth++
		} else if t == lexer.TokRBrace {
			depth--
			p.advance()
			if depth == 0 {
				return &ast.DeclareStmt{Pos: pos}, nil
			}
			continue
		}
		p.advance()
	}
	return &ast.DeclareStmt{Pos: pos}, nil
}

func (p *Parser) skipToStmtBoundary(pos ast.Pos) (ast.Statement, error) {
	depth := 0
	for p.peekType() != lexer.TokEOF {
		t := p.peekType()
		if depth == 0 {
			switch t {
			case lexer.TokSemicolon:
				p.advance()
				return &ast.DeclareStmt{Pos: pos}, nil
			case lexer.TokLet, lexer.TokConst, lexer.TokFunction,
				lexer.TokExport, lexer.TokImport, lexer.TokDeclare,
				lexer.TokType, lexer.TokInterface,
				lexer.TokIf, lexer.TokFor, lexer.TokWhile, lexer.TokSwitch,
				lexer.TokTry, lexer.TokReturn, lexer.TokBreak,
				lexer.TokContinue:
				return &ast.DeclareStmt{Pos: pos}, nil
			}
		}
		if t == lexer.TokLBrace {
			depth++
			p.advance()
		} else if t == lexer.TokRBrace {
			if depth == 0 {
				return &ast.DeclareStmt{Pos: pos}, nil
			}
			depth--
			p.advance()
		} else {
			p.advance()
		}
	}
	return &ast.DeclareStmt{Pos: pos}, nil
}

func (p *Parser) parseDeclareFunction(pos ast.Pos) (*ast.DeclareFnStmt, error) {
	if _, err := p.expect(lexer.TokFunction); err != nil {
		return nil, err
	}
	nameTok, err := p.expectIdentifierName()
	if err != nil {
		return nil, err
	}
	params, err := p.parseParamsOnly()
	if err != nil {
		return nil, err
	}
	var returnType *ast.Type
	if p.peekType() == lexer.TokColon {
		p.advance()
		returnType, err = p.parseType()
		if err != nil {
			return nil, err
		}
	}
	if p.peekType() == lexer.TokSemicolon {
		p.advance()
	}
	return &ast.DeclareFnStmt{Pos: pos, Name: nameTok.Literal, Params: params, ReturnType: returnType}, nil
}

func (p *Parser) parseParamsOnly() ([]*ast.Param, error) {
	if _, err := p.expect(lexer.TokLParen); err != nil {
		return nil, err
	}
	var params []*ast.Param
	for p.peekType() != lexer.TokRParen && p.peekType() != lexer.TokEOF {
		pname, err := p.expectIdentifierName()
		if err != nil {
			return nil, err
		}
		var ptype *ast.Type
		if p.peekType() == lexer.TokColon {
			p.advance()
			ptype, err = p.parseType()
			if err != nil {
				return nil, err
			}
		}
		params = append(params, &ast.Param{Pos: p.pos2ast(pname), Name: pname.Literal, Type: ptype})
		if p.peekType() == lexer.TokComma {
			p.advance()
		}
	}
	if _, err := p.expect(lexer.TokRParen); err != nil {
		return nil, err
	}
	return params, nil
}

func (p *Parser) parsePostfixFrom(expr ast.Expression) (ast.Expression, error) {
	return p.parsePostfixLoop(expr)
}

func (p *Parser) parsePostfixLoop(expr ast.Expression) (ast.Expression, error) {
	for {
		optional := false
		hasDot := false
		dotTok := lexer.Token{}
		if p.peekType() == lexer.TokQuestion && p.peekN(1).Type == lexer.TokDot {
			optional = true
			hasDot = true
			p.advance()
			dotTok = p.advance()
		} else if p.peekType() == lexer.TokDot {
			hasDot = true
			dotTok = p.advance()
		}

		if optional && p.peekType() == lexer.TokLParen {
			return nil, p.errorf(p.peek(), "optional function calls are not supported")
		}
		if p.peekType() == lexer.TokLBracket && (optional || !hasDot) {
			tok := p.advance()
			pos := p.pos2ast(tok)
			index, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			if _, err := p.expect(lexer.TokRBracket); err != nil {
				return nil, err
			}
			expr = &ast.IndexExpr{Pos: pos, Expr: expr, Index: index, Optional: optional}
			continue
		}
		if hasDot {
			pos := p.pos2ast(dotTok)
			nameTok, err := p.expectIdentifierName()
			if err != nil {
				return nil, err
			}
			if p.peekType() == lexer.TokLParen {
				args, err := p.parseArgList()
				if err != nil {
					return nil, err
				}
				expr = &ast.MethodCallExpr{Pos: pos, Receiver: expr, Method: nameTok.Literal, Args: args, Optional: optional}
			} else {
				expr = &ast.PropertyExpr{Pos: pos, Receiver: expr, Property: nameTok.Literal, Optional: optional}
			}
			continue
		}
		break
	}
	return expr, nil
}

func (p *Parser) parseCmdExpr() (*ast.CmdExpr, error) {
	tok := p.advance() // $
	pos := p.pos2ast(tok)
	if _, err := p.expect(lexer.TokLParen); err != nil {
		return nil, err
	}
	var args []ast.Expression
	for p.peekType() != lexer.TokRParen && p.peekType() != lexer.TokEOF {
		arg, err := p.parseArgExpr()
		if err != nil {
			return nil, err
		}
		args = append(args, arg)
		if p.peekType() == lexer.TokComma {
			p.advance()
		}
	}
	if _, err := p.expect(lexer.TokRParen); err != nil {
		return nil, err
	}
	if len(args) == 0 {
		return nil, p.errorf(tok, "$() requires at least a command name")
	}
	return &ast.CmdExpr{Pos: pos, Args: args}, nil
}

func (p *Parser) parseIdentStmt() (ast.Statement, error) {
	nameTok := p.peek()
	pos := p.pos2ast(nameTok)

	if p.peekType() == lexer.TokIdent {
		name := nameTok.Literal

		if name == "console" {
			expr, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			return &ast.ExprStmt{Pos: pos, Expr: expr}, nil
		}

		if ast.IsBuiltin(name) {
			expr, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			return &ast.ExprStmt{Pos: pos, Expr: expr}, nil
		}

		p.advance()
		if p.peekType() == lexer.TokPlusPlus || p.peekType() == lexer.TokMinusMinus {
			opTok := p.advance()
			op := "+"
			if opTok.Type == lexer.TokMinusMinus {
				op = "-"
			}
			return &ast.Assignment{Pos: pos, Name: name, Value: &ast.BinaryExpr{Pos: pos, Op: op, Left: &ast.IdentExpr{Pos: pos, Name: name}, Right: &ast.IntLit{Pos: pos, Value: 1}}}, nil
		}
		if p.peekType() == lexer.TokAssign {
			p.advance()
			val, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			return &ast.Assignment{Pos: pos, Name: name, Value: val}, nil
		}

		if p.peekType() == lexer.TokPlusAssign || p.peekType() == lexer.TokMinusAssign || p.peekType() == lexer.TokStarAssign {
			opTok := p.advance()
			val, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			op := strings.TrimSuffix(opTok.Literal, "=")
			return compoundAssignment(name, pos, op, val), nil
		}

		if p.peekType() == lexer.TokLBracket && p.peekN(1).Type != lexer.TokRBracket {
			p.advance()
			index, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			if _, err := p.expect(lexer.TokRBracket); err != nil {
				return nil, err
			}
			if _, err := p.expect(lexer.TokAssign); err != nil {
				return nil, err
			}
			val, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			return &ast.IndexAssignStmt{Pos: pos, Name: name, Index: index, Value: val}, nil
		}

		if p.peekType() == lexer.TokDot && isIdentifierName(p.peekN(1).Type) && p.peekN(2).Type == lexer.TokAssign {
			p.advance()
			propTok := p.advance()
			p.advance()
			val, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			return &ast.PropertyAssignStmt{Pos: pos, Object: name, Property: propTok.Literal, Value: val}, nil
		}

		if p.peekType() == lexer.TokDot || p.peekType() == lexer.TokLBracket || p.peekType() == lexer.TokQuestion || p.peekType() == lexer.TokPipe {
			p.pos--
			expr, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			return &ast.ExprStmt{Pos: pos, Expr: expr}, nil
		}

		if p.peekType() == lexer.TokLParen {
			args, err := p.parseArgList()
			if err != nil {
				return nil, err
			}
			expr := &ast.FnCallExpr{Pos: pos, Name: name, Args: args}
			return &ast.ExprStmt{Pos: pos, Expr: expr}, nil
		}
	}

	expr, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	return &ast.ExprStmt{Pos: pos, Expr: expr}, nil
}

func (p *Parser) parseThisStmt() (ast.Statement, error) {
	tok := p.advance()
	pos := p.pos2ast(tok)
	if p.peekType() == lexer.TokDot && isIdentifierName(p.peekN(1).Type) && p.peekN(2).Type == lexer.TokAssign {
		p.advance()
		propTok := p.advance()
		p.advance()
		val, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		return &ast.PropertyAssignStmt{Pos: pos, Object: "this", Property: propTok.Literal, Value: val}, nil
	}
	p.pos--
	expr, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	return &ast.ExprStmt{Pos: pos, Expr: expr}, nil
}

func (p *Parser) parseExpr() (ast.Expression, error) {
	return p.parseArrow()
}

func (p *Parser) parseArrow() (ast.Expression, error) {
	if isIdentifierName(p.peekType()) && p.peekN(1).Type == lexer.TokArrow {
		nameTok := p.advance()
		p.advance()
		if p.peekType() == lexer.TokLBrace {
			block, err := p.parseBlock()
			if err != nil {
				return nil, err
			}
			return &ast.ArrowExpr{Pos: p.pos2ast(nameTok), Params: []*ast.Param{{Pos: p.pos2ast(nameTok), Name: nameTok.Literal}}, BlockBody: block}, nil
		}
		body, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		return &ast.ArrowExpr{Pos: p.pos2ast(nameTok), Params: []*ast.Param{{Pos: p.pos2ast(nameTok), Name: nameTok.Literal}}, Body: body}, nil
	}
	if p.peekType() == lexer.TokLParen && p.hasParenArrowAhead() {
		return p.parseParenArrow()
	}
	return p.parseAs()
}

func (p *Parser) parseAs() (ast.Expression, error) {
	left, err := p.parseTernary()
	if err != nil {
		return nil, err
	}
	for p.peekType() == lexer.TokIdent && p.peek().Literal == "as" {
		tok := p.advance()
		t, err := p.parseType()
		if err != nil {
			return nil, err
		}
		left = &ast.AsExpr{Pos: p.pos2ast(tok), Expr: left, Type: t}
	}
	return left, nil
}

func (p *Parser) hasParenArrowAhead() bool {
	depth := 0
	for i := p.pos; i < len(p.tokens); i++ {
		switch p.tokens[i].Type {
		case lexer.TokLParen:
			depth++
		case lexer.TokRParen:
			depth--
			if depth == 0 {
				return p.peekN(i-p.pos+1).Type == lexer.TokArrow
			}
		case lexer.TokEOF:
			return false
		}
	}
	return false
}

func (p *Parser) parseParenArrow() (ast.Expression, error) {
	openTok := p.advance()
	pos := p.pos2ast(openTok)
	var params []*ast.Param
	if p.peekType() != lexer.TokRParen {
		for {
			nameTok, err := p.expectIdentifierName()
			if err != nil {
				return nil, err
			}
			param := &ast.Param{Pos: p.pos2ast(nameTok), Name: nameTok.Literal}
			if p.peekType() == lexer.TokColon {
				p.advance()
				t, err := p.parseType()
				if err != nil {
					return nil, err
				}
				param.Type = t
			}
			params = append(params, param)
			if p.peekType() != lexer.TokComma {
				break
			}
			p.advance()
		}
	}
	if _, err := p.expect(lexer.TokRParen); err != nil {
		return nil, err
	}
	if _, err := p.expect(lexer.TokArrow); err != nil {
		return nil, err
	}
	if p.peekType() == lexer.TokLBrace {
		block, err := p.parseBlock()
		if err != nil {
			return nil, err
		}
		return &ast.ArrowExpr{Pos: pos, Params: params, BlockBody: block}, nil
	}
	body, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	return &ast.ArrowExpr{Pos: pos, Params: params, Body: body}, nil
}

func (p *Parser) parseTernary() (ast.Expression, error) {
	left, err := p.parseNullish()
	if err != nil {
		return nil, err
	}
	for p.peekType() == lexer.TokQuestion {
		if p.peekN(1).Type == lexer.TokDot {
			break
		}
		if p.hasTernaryColonAhead() {
			tok := p.advance()
			thenExpr, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			if _, err := p.expect(lexer.TokColon); err != nil {
				return nil, err
			}
			elseExpr, err := p.parseTernary()
			if err != nil {
				return nil, err
			}
			left = &ast.TernaryExpr{Pos: p.pos2ast(tok), Condition: left, Then: thenExpr, Else: elseExpr}
			continue
		}
		tok := p.advance()
		left = &ast.PropagateExpr{Pos: p.pos2ast(tok), Expr: left}
	}
	return left, nil
}

func (p *Parser) parseNullish() (ast.Expression, error) {
	left, err := p.parseOr()
	if err != nil {
		return nil, err
	}
	for p.peekType() == lexer.TokQuestionQuestion {
		tok := p.advance()
		pos := p.pos2ast(tok)
		right, err := p.parseOr()
		if err != nil {
			return nil, err
		}
		left = &ast.BinaryExpr{Pos: pos, Op: "??", Left: left, Right: right}
	}
	return left, nil
}

func (p *Parser) hasTernaryColonAhead() bool {
	groupDepth := 0
	ternaryDepth := 0
	for i := p.pos + 1; i < len(p.tokens); i++ {
		tok := p.tokens[i]
		switch tok.Type {
		case lexer.TokLParen, lexer.TokLBracket, lexer.TokLBrace:
			groupDepth++
		case lexer.TokRParen, lexer.TokRBracket, lexer.TokRBrace:
			if groupDepth == 0 {
				return false
			}
			groupDepth--
		case lexer.TokQuestion:
			if groupDepth == 0 {
				ternaryDepth++
			}
		case lexer.TokColon:
			if groupDepth != 0 {
				continue
			}
			if ternaryDepth == 0 {
				return true
			}
			ternaryDepth--
		case lexer.TokComma, lexer.TokSemicolon, lexer.TokEOF:
			if groupDepth == 0 {
				return false
			}
		}
	}
	return false
}

func (p *Parser) parseOr() (ast.Expression, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for p.peekType() == lexer.TokOr {
		tok := p.advance()
		pos := p.pos2ast(tok)
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		left = &ast.BinaryExpr{Pos: pos, Op: "||", Left: left, Right: right}
	}
	return left, nil
}

func (p *Parser) parseAnd() (ast.Expression, error) {
	left, err := p.parseComparison()
	if err != nil {
		return nil, err
	}
	for p.peekType() == lexer.TokAnd {
		tok := p.advance()
		pos := p.pos2ast(tok)
		right, err := p.parseComparison()
		if err != nil {
			return nil, err
		}
		left = &ast.BinaryExpr{Pos: pos, Op: "&&", Left: left, Right: right}
	}
	return left, nil
}

func (p *Parser) parseComparison() (ast.Expression, error) {
	left, err := p.parseAddSub()
	if err != nil {
		return nil, err
	}
	switch p.peekType() {
	case lexer.TokEq, lexer.TokStrictEq, lexer.TokNeq, lexer.TokStrictNeq, lexer.TokLt, lexer.TokLte, lexer.TokRAngle, lexer.TokGte:
		tok := p.advance()
		pos := p.pos2ast(tok)
		right, err := p.parseAddSub()
		if err != nil {
			return nil, err
		}
		op := tok.Literal
		if op == "===" {
			op = "=="
		} else if op == "!==" {
			op = "!="
		}
		return &ast.BinaryExpr{Pos: pos, Op: op, Left: left, Right: right}, nil
	}
	return left, nil
}

func (p *Parser) parseAddSub() (ast.Expression, error) {
	left, err := p.parseMulDiv()
	if err != nil {
		return nil, err
	}
	for p.peekType() == lexer.TokPlus || p.peekType() == lexer.TokMinus {
		tok := p.advance()
		pos := p.pos2ast(tok)
		right, err := p.parseMulDiv()
		if err != nil {
			return nil, err
		}
		left = &ast.BinaryExpr{Pos: pos, Op: tok.Literal, Left: left, Right: right}
	}
	return left, nil
}

func (p *Parser) parseMulDiv() (ast.Expression, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for p.peekType() == lexer.TokStar || p.peekType() == lexer.TokSlash || p.peekType() == lexer.TokPercent {
		tok := p.advance()
		pos := p.pos2ast(tok)
		right, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		left = &ast.BinaryExpr{Pos: pos, Op: tok.Literal, Left: left, Right: right}
	}
	return left, nil
}

func (p *Parser) parseUnary() (ast.Expression, error) {
	if p.peekType() == lexer.TokPlusPlus || p.peekType() == lexer.TokMinusMinus {
		tok := p.advance()
		pos := p.pos2ast(tok)
		nameTok, err := p.expectIdentifierName()
		if err != nil {
			return nil, err
		}
		op := "+"
		if tok.Type == lexer.TokMinusMinus {
			op = "-"
		}
		return &ast.UpdateExpr{Pos: pos, Op: op, Name: nameTok.Literal}, nil
	}
	if p.peekType() == lexer.TokBang {
		tok := p.advance()
		pos := p.pos2ast(tok)
		expr, err := p.parsePostfix()
		if err != nil {
			return nil, err
		}
		return &ast.UnaryExpr{Pos: pos, Op: "!", Expr: expr}, nil
	}
	if p.peekType() == lexer.TokMinus {
		tok := p.advance()
		pos := p.pos2ast(tok)
		expr, err := p.parsePostfix()
		if err != nil {
			return nil, err
		}
		return &ast.UnaryExpr{Pos: pos, Op: "-", Expr: expr}, nil
	}
	return p.parsePostfix()
}

func (p *Parser) parsePostfix() (ast.Expression, error) {
	expr, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}
	return p.parsePostfixLoop(expr)
}

func (p *Parser) parsePrimary() (ast.Expression, error) {
	tok := p.peek()
	pos := p.pos2ast(tok)

	switch tok.Type {
	case lexer.TokInt:
		p.advance()
		n, _ := strconv.ParseInt(tok.Literal, 10, 64)
		return &ast.IntLit{Pos: pos, Value: n}, nil

	case lexer.TokFloat:
		p.advance()
		return &ast.FloatLit{Pos: pos, Value: tok.Literal}, nil

	case lexer.TokString:
		p.advance()
		return &ast.StringLit{Pos: pos, Value: tok.Literal}, nil

	case lexer.TokTrue:
		p.advance()
		return &ast.BoolLit{Pos: pos, Value: true}, nil

	case lexer.TokFalse:
		p.advance()
		return &ast.BoolLit{Pos: pos, Value: false}, nil

	case lexer.TokRawString:
		p.advance()
		return &ast.RawStringLit{Pos: pos, Value: tok.Literal}, nil

	case lexer.TokTemplateLit:
		p.advance()
		return p.parseTemplateLiteral(pos, tok)
	case lexer.TokEllipsis:
		p.advance()
		expr, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		return &ast.SpreadExpr{Pos: pos, Expr: expr}, nil

	case lexer.TokDollar:
		return p.parseCmdExpr()

	case lexer.TokNew:
		p.advance()
		nameTok, err := p.expectIdentifierName()
		if err != nil {
			return nil, err
		}
		typeArgs, err := p.parseOptionalTypeArgs()
		if err != nil {
			return nil, err
		}
		args, err := p.parseArgList()
		if err != nil {
			return nil, err
		}
		return &ast.NewExpr{Pos: pos, ClassName: nameTok.Literal, TypeArgs: typeArgs, Args: args}, nil

	case lexer.TokThis:
		p.advance()
		return &ast.ThisExpr{Pos: pos}, nil

	case lexer.TokLBracket:
		return p.parseListLit()

	case lexer.TokLBrace:
		return p.parseObjectLit()

	case lexer.TokLParen:
		p.advance()
		inner, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(lexer.TokRParen); err != nil {
			return nil, err
		}
		return inner, nil

	case lexer.TokIdent, lexer.TokTypeString, lexer.TokTypeNumber, lexer.TokTypeBoolean, lexer.TokTypeStatus, lexer.TokTypeList, lexer.TokTypeArray, lexer.TokFrom:
		name := tok.Literal
		p.advance()
		if name == "undefined" {
			return &ast.UndefinedLit{Pos: pos}, nil
		}
		if name == "null" {
			return &ast.NullLit{Pos: pos}, nil
		}
		if name == "String" && p.peekType() == lexer.TokDot && isIdentifierName(p.peekN(1).Type) && p.peekN(1).Literal == "raw" && p.peekN(2).Type == lexer.TokTemplateLit {
			p.advance()
			p.advance()
			tmpl := p.advance()
			return &ast.RawStringLit{Pos: pos, Value: tmpl.Literal}, nil
		}
		if name == "Number" && p.peekType() == lexer.TokDot && isIdentifierName(p.peekN(1).Type) {
			p.advance()
			memberTok := p.advance()
			if p.peekType() == lexer.TokLParen {
				args, err := p.parseArgList()
				if err != nil {
					return nil, err
				}
				return &ast.BuiltinCallExpr{Pos: pos, Name: "Number." + memberTok.Literal, Args: args}, nil
			}
			switch memberTok.Literal {
			case "MAX_SAFE_INTEGER":
				return &ast.IntLit{Pos: pos, Value: 9007199254740991}, nil
			case "MIN_SAFE_INTEGER":
				return &ast.IntLit{Pos: pos, Value: -9007199254740991}, nil
			case "EPSILON":
				return &ast.FloatLit{Pos: pos, Value: "2.220446049250313e-16"}, nil
			}
			return &ast.PropertyExpr{Pos: pos, Receiver: &ast.IdentExpr{Pos: pos, Name: name}, Property: memberTok.Literal}, nil
		}

		if name == "Array" && p.peekType() == lexer.TokDot && (p.peekN(1).Literal == "from" || p.peekN(1).Literal == "of") {
			methodTok := p.peekN(1)
			p.advance()
			p.advance()
			args, err := p.parseArgList()
			if err != nil {
				return nil, err
			}
			return &ast.BuiltinCallExpr{Pos: pos, Name: "Array." + methodTok.Literal, Args: args}, nil
		}

		if name == "console" && p.peekType() == lexer.TokDot {
			p.advance() // consume .
			methodTok, err := p.expect(lexer.TokIdent)
			if err != nil {
				return nil, err
			}
			fullName := "console." + methodTok.Literal
			if !ast.IsBuiltin(fullName) {
				return nil, p.errorf(methodTok, "unknown console method %q", methodTok.Literal)
			}
			args, err := p.parseArgList()
			if err != nil {
				return nil, err
			}
			return &ast.BuiltinCallExpr{Pos: pos, Name: fullName, Args: args}, nil
		}

		if ast.IsBuiltin(name) {
			args, err := p.parseArgList()
			if err != nil {
				return nil, err
			}
			return &ast.BuiltinCallExpr{Pos: pos, Name: name, Args: args}, nil
		}

		if p.peekType() == lexer.TokLParen {
			args, err := p.parseArgList()
			if err != nil {
				return nil, err
			}
			return &ast.FnCallExpr{Pos: pos, Name: name, Args: args}, nil
		}

		return &ast.IdentExpr{Pos: pos, Name: name}, nil
	}

	return nil, p.errorf(tok, "unexpected token %q in expression", tok.Literal)
}

func (p *Parser) parseOptionalTypeArgs() ([]*ast.Type, error) {
	if p.peekType() != lexer.TokLt {
		return nil, nil
	}
	p.advance()
	var args []*ast.Type
	for p.peekType() != lexer.TokRAngle && p.peekType() != lexer.TokEOF {
		t, err := p.parseType()
		if err != nil {
			return nil, err
		}
		args = append(args, t)
		if p.peekType() == lexer.TokComma {
			p.advance()
		}
	}
	if p.peekType() != lexer.TokRAngle {
		return nil, p.errorf(p.peek(), "expected '>' to close type argument list")
	}
	p.advance()
	return args, nil
}

func (p *Parser) parseListLit() (*ast.ListLit, error) {
	tok := p.advance() // [
	pos := p.pos2ast(tok)

	var elems []ast.Expression
	for p.peekType() != lexer.TokRBracket && p.peekType() != lexer.TokEOF {
		elem, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		elems = append(elems, elem)
		if p.peekType() == lexer.TokComma {
			p.advance()
		}
	}
	if _, err := p.expect(lexer.TokRBracket); err != nil {
		return nil, err
	}
	return &ast.ListLit{Pos: pos, Elements: elems}, nil
}

func (p *Parser) parseObjectLit() (*ast.ObjectLit, error) {
	tok := p.advance()
	pos := p.pos2ast(tok)
	var fields []ast.ObjectField
	for p.peekType() != lexer.TokRBrace && p.peekType() != lexer.TokEOF {
		keyTok := p.peek()
		if !isIdentifierName(keyTok.Type) && keyTok.Type != lexer.TokString {
			return nil, p.errorf(keyTok, "expected object field name")
		}
		p.advance()
		var val ast.Expression
		if p.peekType() == lexer.TokColon {
			p.advance()
			var err error
			val, err = p.parseExpr()
			if err != nil {
				return nil, err
			}
		} else if keyTok.Type == lexer.TokIdent {
			val = &ast.IdentExpr{Pos: p.pos2ast(keyTok), Name: keyTok.Literal}
		} else {
			return nil, p.errorf(keyTok, "expected ':' after object field name")
		}
		fields = append(fields, ast.ObjectField{Pos: p.pos2ast(keyTok), Key: keyTok.Literal, Value: val})
		if p.peekType() == lexer.TokComma {
			p.advance()
		}
	}
	if _, err := p.expect(lexer.TokRBrace); err != nil {
		return nil, err
	}
	return &ast.ObjectLit{Pos: pos, Fields: fields}, nil
}

func (p *Parser) parseArgList() ([]ast.Expression, error) {
	if _, err := p.expect(lexer.TokLParen); err != nil {
		return nil, err
	}
	var args []ast.Expression
	for p.peekType() != lexer.TokRParen && p.peekType() != lexer.TokEOF {
		arg, err := p.parseArgExpr()
		if err != nil {
			return nil, err
		}
		args = append(args, arg)
		if p.peekType() == lexer.TokComma {
			p.advance()
		}
	}
	if _, err := p.expect(lexer.TokRParen); err != nil {
		return nil, err
	}
	return args, nil
}

func (p *Parser) parseArgExpr() (ast.Expression, error) {
	if p.peekType() == lexer.TokEllipsis {
		tok := p.advance()
		expr, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		return &ast.SpreadExpr{Pos: p.pos2ast(tok), Expr: expr}, nil
	}
	return p.parseExpr()
}

func (p *Parser) parseTemplateLiteral(pos ast.Pos, tok lexer.Token) (ast.Expression, error) {
	parts := make([]string, 0, 4)
	exprs := make([]ast.Expression, 0, 4)
	start := 0
	for i := 0; i < len(tok.Literal); i++ {
		if tok.Literal[i] != '$' || i+1 >= len(tok.Literal) || tok.Literal[i+1] != '{' {
			continue
		}
		parts = append(parts, tok.Literal[start:i])
		depth := 1
		j := i + 2
		for j < len(tok.Literal) && depth > 0 {
			switch tok.Literal[j] {
			case '{':
				depth++
			case '}':
				depth--
			}
			j++
		}
		if depth != 0 {
			return nil, p.errorf(tok, "unterminated template interpolation")
		}
		exprSrc := strings.TrimSpace(tok.Literal[i+2 : j-1])
		if exprSrc == "" {
			return nil, p.errorf(tok, "empty template interpolation")
		}
		expr, err := parseEmbeddedExpr(exprSrc, tok.File)
		if err != nil {
			return nil, p.errorf(tok, "invalid template interpolation %q: %v", exprSrc, err)
		}
		exprs = append(exprs, expr)
		start = j
		i = j - 1
	}
	parts = append(parts, tok.Literal[start:])
	return &ast.TemplateLit{Pos: pos, Value: tok.Literal, Parts: parts, Exprs: exprs}, nil
}

func parseEmbeddedExpr(src, file string) (ast.Expression, error) {
	l := lexer.New(src, file)
	tokens, err := l.Tokenize()
	if err != nil {
		return nil, err
	}
	p := New(tokens, file)
	expr, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if p.peekType() != lexer.TokEOF {
		return nil, p.errorf(p.peek(), "unexpected token %q in template interpolation", p.peek().Literal)
	}
	return expr, nil
}

func sanitizeShellVarRefs(body string) []string {
	var refs []string
	i := 0
	for i < len(body) {
		if body[i] == '$' && i+1 < len(body) && body[i+1] == '{' {
			j := i + 2
			for j < len(body) && body[j] != '}' {
				j++
			}
			if j < len(body) {
				refs = append(refs, body[i+2:j])
			}
			i = j + 1
		} else {
			i++
		}
	}
	return refs
}

func ExtractShellVarRefs(body string) []string {
	return sanitizeShellVarRefs(body)
}

func NormalizeModulePath(path string) string {
	path = strings.TrimPrefix(path, "./")
	path = strings.ReplaceAll(path, "/", "__")
	path = strings.ReplaceAll(path, "-", "_")
	return path
}
