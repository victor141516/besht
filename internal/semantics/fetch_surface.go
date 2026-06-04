package semantics

import (
	"fmt"

	"github.com/victor141516/besht/internal/ast"
)

func ValidateFetchSurface(stmts []ast.Statement) error {
	return ValidateFetchSurfaceWithTypes(stmts, nil)

}

func ValidateFetchSurfaceWithTypes(stmts []ast.Statement, varTypes map[string]*ast.Type) error {
	fetchVars := make(map[string]bool)
	for name, typ := range varTypes {
		if typ != nil && typ.Kind == ast.TypeFetchResponse {
			fetchVars[name] = true
		}
	}
	return (&fetchSurfaceValidator{fetchVars: fetchVars}).stmts(stmts)
}

type fetchSurfaceValidator struct {
	fetchVars map[string]bool
}

func (v *fetchSurfaceValidator) child() *fetchSurfaceValidator {
	cloned := make(map[string]bool, len(v.fetchVars))
	for name, ok := range v.fetchVars {
		cloned[name] = ok
	}
	return &fetchSurfaceValidator{fetchVars: cloned}
}

func (v *fetchSurfaceValidator) childWithoutParams(params []*ast.Param) *fetchSurfaceValidator {
	child := v.child()
	for _, param := range params {
		delete(child.fetchVars, param.Name)
	}
	return child
}

func (v *fetchSurfaceValidator) setVar(name string, expr ast.Expression) {
	if v.isFetchResponse(expr) {
		v.fetchVars[name] = true
		return
	}
	delete(v.fetchVars, name)
}

func (v *fetchSurfaceValidator) stmts(stmts []ast.Statement) error {
	for _, stmt := range stmts {
		if err := v.stmt(stmt); err != nil {
			return err
		}
	}
	return nil
}

func (v *fetchSurfaceValidator) block(block *ast.Block) error {
	if block == nil {
		return nil
	}
	return v.child().stmts(block.Statements)
}

func (v *fetchSurfaceValidator) stmt(stmt ast.Statement) error {
	switch s := stmt.(type) {
	case nil, *ast.ImportDecl, *ast.DeclareStmt, *ast.DeclareFnStmt, *ast.BreakStmt, *ast.ContinueStmt:
		return nil
	case *ast.LetDecl:
		if err := v.expr(s.Value); err != nil {
			return err
		}
		v.setVar(s.Name, s.Value)
	case *ast.DestructureDecl:
		return v.expr(s.Value)
	case *ast.Assignment:
		if err := v.expr(s.Value); err != nil {
			return err
		}
		v.setVar(s.Name, s.Value)
	case *ast.IndexAssignStmt:
		if v.fetchVars[s.Name] {
			return &SemanticError{Pos: s.Pos, Message: "FetchResponse does not support index assignment"}
		}
		if err := v.expr(s.Index); err != nil {
			return err
		}
		return v.expr(s.Value)
	case *ast.PropertyAssignStmt:
		if v.fetchVars[s.Object] {
			return &SemanticError{Pos: s.Pos, Message: fmt.Sprintf("FetchResponse has no property %q; status, ok, headers, json(), and body are not supported yet", s.Property)}
		}
		return v.expr(s.Value)
	case *ast.FnDecl:
		return v.childWithoutParams(s.Params).block(s.Body)
	case *ast.ClassDecl:
		return v.classDecl(s)
	case *ast.Block:
		return v.block(s)
	case *ast.IfStmt:
		if err := v.expr(s.Condition); err != nil {
			return err
		}
		if err := v.block(s.Then); err != nil {
			return err
		}
		for _, elseif := range s.ElseIfs {
			if err := v.expr(elseif.Condition); err != nil {
				return err
			}
			if err := v.block(elseif.Body); err != nil {
				return err
			}
		}
		return v.block(s.Else)
	case *ast.ForStmt:
		if err := v.expr(s.Iterator); err != nil {
			return err
		}
		child := v.child()
		delete(child.fetchVars, s.VarName)
		return child.block(s.Body)
	case *ast.CStyleForStmt:
		child := v.child()
		if err := child.stmt(s.Init); err != nil {
			return err
		}
		if err := child.expr(s.Condition); err != nil {
			return err
		}
		if err := child.stmt(s.Update); err != nil {
			return err
		}
		return child.block(s.Body)
	case *ast.WhileStmt:
		if err := v.expr(s.Condition); err != nil {
			return err
		}
		return v.block(s.Body)
	case *ast.TryStmt:
		if err := v.block(s.Body); err != nil {
			return err
		}
		return v.block(s.Catch)
	case *ast.SwitchStmt:
		if err := v.expr(s.Value); err != nil {
			return err
		}
		for _, swCase := range s.Cases {
			if !swCase.IsDefault {
				if err := v.expr(swCase.Value); err != nil {
					return err
				}
			}
			if err := v.block(swCase.Body); err != nil {
				return err
			}
		}
	case *ast.ReturnStmt:
		return v.expr(s.Value)
	case *ast.ExitStmt:
		return v.expr(s.Code)
	case *ast.ExprStmt:
		return v.expr(s.Expr)
	}
	return nil
}

func (v *fetchSurfaceValidator) classDecl(s *ast.ClassDecl) error {
	for _, prop := range s.Properties {
		if err := v.expr(prop.Value); err != nil {
			return err
		}
	}
	for _, prop := range s.StaticProps {
		if err := v.expr(prop.Value); err != nil {
			return err
		}
	}
	if s.Constructor != nil {
		if err := v.childWithoutParams(s.Constructor.Params).block(s.Constructor.Body); err != nil {
			return err
		}
	}
	for _, method := range s.Methods {
		if err := v.childWithoutParams(method.Params).block(method.Body); err != nil {
			return err
		}
	}
	for _, accessor := range s.Accessors {
		if err := v.childWithoutParams(accessor.Params).block(accessor.Body); err != nil {
			return err
		}
	}
	return nil
}

func (v *fetchSurfaceValidator) expr(expr ast.Expression) error {
	if expr == nil {
		return nil
	}
	switch e := expr.(type) {
	case *ast.IntLit, *ast.FloatLit, *ast.StringLit, *ast.BoolLit, *ast.UndefinedLit, *ast.NullLit, *ast.IdentExpr, *ast.ThisExpr, *ast.UpdateExpr:
		return nil
	case *ast.TemplateLit:
		for _, part := range e.Exprs {
			if err := v.expr(part); err != nil {
				return err
			}
		}
	case *ast.ListLit:
		for _, elem := range e.Elements {
			if err := v.expr(elem); err != nil {
				return err
			}
		}
	case *ast.ObjectLit:
		for _, field := range e.Fields {
			if field.Spread != nil {
				if err := v.expr(field.Spread); err != nil {
					return err
				}
				continue
			}
			if err := v.expr(field.Value); err != nil {
				return err
			}
		}
	case *ast.ArrowExpr:
		child := v.childWithoutParams(e.Params)
		if e.BlockBody != nil {
			return child.block(e.BlockBody)
		}
		return child.expr(e.Body)
	case *ast.NewExpr:
		for _, arg := range e.Args {
			if err := v.expr(arg); err != nil {
				return err
			}
		}
	case *ast.BinaryExpr:
		if err := v.expr(e.Left); err != nil {
			return err
		}
		return v.expr(e.Right)
	case *ast.TernaryExpr:
		if err := v.expr(e.Condition); err != nil {
			return err
		}
		if err := v.expr(e.Then); err != nil {
			return err
		}
		return v.expr(e.Else)
	case *ast.UnaryExpr:
		return v.expr(e.Expr)
	case *ast.PipeExpr:
		if err := v.expr(e.Left); err != nil {
			return err
		}
		return v.expr(e.Right)
	case *ast.CmdExpr:
		for _, arg := range e.Args {
			if err := v.expr(arg); err != nil {
				return err
			}
		}
	case *ast.FnCallExpr:
		for _, arg := range e.Args {
			if err := v.expr(arg); err != nil {
				return err
			}
		}
	case *ast.BuiltinCallExpr:
		if e.Name == "fetch" && len(e.Args) != 1 {
			return &SemanticError{Pos: e.Pos, Message: "fetch() takes 1 URL argument; options are not supported yet"}
		}
		for _, arg := range e.Args {
			if err := v.expr(arg); err != nil {
				return err
			}
		}
	case *ast.RangeExpr:
		if err := v.expr(e.Start); err != nil {
			return err
		}
		return v.expr(e.End)
	case *ast.PropagateExpr:
		return v.expr(e.Expr)
	case *ast.IndexExpr:
		if err := v.expr(e.Expr); err != nil {
			return err
		}
		if v.isFetchResponse(e.Expr) {
			return &SemanticError{Pos: e.Pos, Message: "FetchResponse does not support index access"}
		}
		return v.expr(e.Index)
	case *ast.MethodCallExpr:
		if err := v.expr(e.Receiver); err != nil {
			return err
		}
		for _, arg := range e.Args {
			if err := v.expr(arg); err != nil {
				return err
			}
		}
		if v.isFetchResponse(e.Receiver) {
			if e.Method != "text" {
				return &SemanticError{Pos: e.Pos, Message: fmt.Sprintf("FetchResponse has no method %q; this fetch() slice only supports text()", e.Method)}
			}
			if len(e.Args) != 0 {
				return &SemanticError{Pos: e.Pos, Message: "FetchResponse.text() takes no arguments"}
			}
		}
	case *ast.PropertyExpr:
		if err := v.expr(e.Receiver); err != nil {
			return err
		}
		if v.isFetchResponse(e.Receiver) {
			return &SemanticError{Pos: e.Pos, Message: fmt.Sprintf("FetchResponse has no property %q; status, ok, headers, json(), and body are not supported yet", e.Property)}
		}
	case *ast.SpreadExpr:
		return v.expr(e.Expr)
	case *ast.AsExpr:
		return v.expr(e.Expr)
	}
	return nil
}

func (v *fetchSurfaceValidator) isFetchResponse(expr ast.Expression) bool {
	switch e := expr.(type) {
	case *ast.BuiltinCallExpr:
		return e.Name == "fetch"
	case *ast.IdentExpr:
		return v.fetchVars[e.Name]
	case *ast.AsExpr:
		return v.isFetchResponse(e.Expr)
	}
	return false
}
