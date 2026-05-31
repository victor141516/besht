package checker

import "github.com/victor141516/besht/internal/ast"

func ValidateForEachSurfaceWithTypes(stmts []ast.Statement, varTypes map[string]*ast.Type) error {
	v := forEachSurfaceValidator{
		listValues: map[string]bool{},
		setValues:  map[string]bool{},
	}
	for name, typ := range varTypes {
		if typ == nil {
			continue
		}
		switch typ.Kind {
		case ast.TypeList:
			v.listValues[name] = true
		case ast.TypeSet:
			v.setValues[name] = true
		}
	}
	return v.stmts(stmts)
}

type forEachSurfaceValidator struct {
	listValues map[string]bool
	setValues  map[string]bool
}

func (v *forEachSurfaceValidator) stmts(stmts []ast.Statement) error {
	for _, stmt := range stmts {
		if err := v.stmt(stmt); err != nil {
			return err
		}
	}
	return nil
}

func (v *forEachSurfaceValidator) block(block *ast.Block) error {
	if block == nil {
		return nil
	}
	return v.stmts(block.Statements)
}

func (v *forEachSurfaceValidator) stmt(stmt ast.Statement) error {
	switch s := stmt.(type) {
	case nil, *ast.ImportDecl, *ast.DeclareStmt, *ast.DeclareFnStmt, *ast.BreakStmt, *ast.ContinueStmt:
		return nil
	case *ast.LetDecl:
		if err := v.expr(s.Value); err != nil {
			return err
		}
		v.trackBinding(s.Name, s.TypeAnnot, s.Value)
		return nil
	case *ast.DestructureDecl:
		return v.expr(s.Value)
	case *ast.Assignment:
		if err := v.expr(s.Value); err != nil {
			return err
		}
		v.trackBinding(s.Name, nil, s.Value)
		return nil
	case *ast.IndexAssignStmt:
		if err := v.expr(s.Index); err != nil {
			return err
		}
		return v.expr(s.Value)
	case *ast.PropertyAssignStmt:
		return v.expr(s.Value)
	case *ast.FnDecl:
		return v.withParamScope(s.Params, func() error { return v.block(s.Body) })
	case *ast.ClassDecl:
		if s.Constructor != nil {
			if err := v.withParamScope(s.Constructor.Params, func() error { return v.block(s.Constructor.Body) }); err != nil {
				return err
			}
		}
		for _, method := range s.Methods {
			if err := v.withParamScope(method.Params, func() error { return v.block(method.Body) }); err != nil {
				return err
			}
		}
		for _, accessor := range s.Accessors {
			if err := v.withParamScope(accessor.Params, func() error { return v.block(accessor.Body) }); err != nil {
				return err
			}
		}
	case *ast.Block:
		return v.block(s)
	case *ast.IfStmt:
		if err := v.expr(s.Condition); err != nil {
			return err
		}
		if err := v.block(s.Then); err != nil {
			return err
		}
		for _, ei := range s.ElseIfs {
			if err := v.expr(ei.Condition); err != nil {
				return err
			}
			if err := v.block(ei.Body); err != nil {
				return err
			}
		}
		return v.block(s.Else)
	case *ast.ForStmt:
		if err := v.expr(s.Iterator); err != nil {
			return err
		}
		return v.block(s.Body)
	case *ast.CStyleForStmt:
		if err := v.stmt(s.Init); err != nil {
			return err
		}
		if err := v.expr(s.Condition); err != nil {
			return err
		}
		if err := v.stmt(s.Update); err != nil {
			return err
		}
		return v.block(s.Body)
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
		for _, c := range s.Cases {
			if err := v.expr(c.Value); err != nil {
				return err
			}
			if err := v.block(c.Body); err != nil {
				return err
			}
		}
	case *ast.ReturnStmt:
		return v.expr(s.Value)
	case *ast.ExitStmt:
		return v.expr(s.Code)
	case *ast.ExprStmt:
		return v.exprStmt(s.Expr)
	}
	return nil
}

func (v *forEachSurfaceValidator) exprStmt(expr ast.Expression) error {
	if call, ok := expr.(*ast.MethodCallExpr); ok && call.Method == "forEach" && v.isListValue(call.Receiver) {
		return v.validateForEachCall(call)
	}
	return v.expr(expr)
}

func (v *forEachSurfaceValidator) expr(expr ast.Expression) error {
	switch e := expr.(type) {
	case nil, *ast.IntLit, *ast.FloatLit, *ast.StringLit, *ast.RawStringLit, *ast.TemplateLit, *ast.BoolLit, *ast.UndefinedLit, *ast.NullLit, *ast.IdentExpr, *ast.UpdateExpr, *ast.ThisExpr:
		return nil
	case *ast.MethodCallExpr:
		if e.Method == "forEach" && v.isListValue(e.Receiver) {
			return &CheckError{Pos: e.Pos, Message: "forEach() must be used as a statement"}
		}
		if err := v.expr(e.Receiver); err != nil {
			return err
		}
		for _, arg := range e.Args {
			if err := v.expr(arg); err != nil {
				return err
			}
		}
	case *ast.BuiltinCallExpr:
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
	case *ast.NewExpr:
		for _, arg := range e.Args {
			if err := v.expr(arg); err != nil {
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
			if err := v.expr(field.Value); err != nil {
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
	case *ast.IndexExpr:
		if err := v.expr(e.Expr); err != nil {
			return err
		}
		return v.expr(e.Index)
	case *ast.PropertyExpr:
		return v.expr(e.Receiver)
	case *ast.CmdExpr:
		for _, arg := range e.Args {
			if err := v.expr(arg); err != nil {
				return err
			}
		}
	case *ast.PipeExpr:
		if err := v.expr(e.Left); err != nil {
			return err
		}
		return v.expr(e.Right)
	case *ast.RangeExpr:
		if err := v.expr(e.Start); err != nil {
			return err
		}
		return v.expr(e.End)
	case *ast.PropagateExpr:
		return v.expr(e.Expr)
	case *ast.SpreadExpr:
		return v.expr(e.Expr)
	case *ast.AsExpr:
		return v.expr(e.Expr)
	case *ast.ArrowExpr:
		return v.withParamScope(e.Params, func() error {
			if err := v.expr(e.Body); err != nil {
				return err
			}
			return v.block(e.BlockBody)
		})
	}
	return nil
}

func (v *forEachSurfaceValidator) validateForEachCall(call *ast.MethodCallExpr) error {
	if err := v.expr(call.Receiver); err != nil {
		return err
	}
	if len(call.Args) != 1 {
		return &CheckError{Pos: call.Pos, Message: "forEach() takes 1 arrow callback"}
	}
	arrow, ok := call.Args[0].(*ast.ArrowExpr)
	if !ok {
		return &CheckError{Pos: call.Pos, Message: "forEach() callback must be an arrow expression"}
	}
	if len(arrow.Params) < 1 || len(arrow.Params) > 2 {
		return &CheckError{Pos: arrow.Pos, Message: "arrow callbacks take 1 or 2 parameters"}
	}
	return v.withParamScope(arrow.Params, func() error {
		if arrow.BlockBody == nil {
			return v.forEachCallbackExpr(arrow.Body)
		}
		return v.forEachCallbackBlock(arrow.BlockBody)
	})
}

func (v *forEachSurfaceValidator) forEachCallbackBlock(block *ast.Block) error {
	if block == nil {
		return nil
	}
	for _, stmt := range block.Statements {
		if err := v.forEachCallbackStmt(stmt); err != nil {
			return err
		}
	}
	return nil
}

func (v *forEachSurfaceValidator) forEachCallbackStmt(stmt ast.Statement) error {
	switch s := stmt.(type) {
	case *ast.ExprStmt:
		return v.forEachCallbackExpr(s.Expr)
	case *ast.ReturnStmt:
		return &CheckError{Pos: s.Pos, Message: "forEach() callback does not support return"}
	case *ast.BreakStmt:
		return &CheckError{Pos: s.Pos, Message: "forEach() callback does not support break"}
	case *ast.ContinueStmt:
		return &CheckError{Pos: s.Pos, Message: "forEach() callback does not support continue"}
	case *ast.IfStmt:
		if err := v.expr(s.Condition); err != nil {
			return err
		}
		if err := v.forEachCallbackBlock(s.Then); err != nil {
			return err
		}
		for _, ei := range s.ElseIfs {
			if err := v.expr(ei.Condition); err != nil {
				return err
			}
			if err := v.forEachCallbackBlock(ei.Body); err != nil {
				return err
			}
		}
		return v.forEachCallbackBlock(s.Else)
	default:
		return v.stmt(stmt)
	}
}

func (v *forEachSurfaceValidator) forEachCallbackExpr(expr ast.Expression) error {
	if v.isSideEffectExpr(expr) {
		return v.expr(expr)
	}
	return &CheckError{Pos: ast.Pos{}, Message: "forEach() callback expression must be side-effecting"}
}

func (v *forEachSurfaceValidator) isSideEffectExpr(expr ast.Expression) bool {
	switch e := expr.(type) {
	case *ast.BuiltinCallExpr:
		return e.Name == "console.log" || e.Name == "console.error" || e.Name == "exit"
	case *ast.MethodCallExpr:
		return e.Method == "run" || (e.Method == "add" && v.isSetValue(e.Receiver)) || isProcessExitCall(e)
	case *ast.FnCallExpr:
		return true
	}
	return false
}

func (v *forEachSurfaceValidator) trackBinding(name string, typ *ast.Type, expr ast.Expression) {
	if (typ != nil && typ.Kind == ast.TypeList) || v.isListValue(expr) {
		v.listValues[name] = true
	} else {
		delete(v.listValues, name)
	}
	if (typ != nil && typ.Kind == ast.TypeSet) || v.isSetValue(expr) {
		v.setValues[name] = true
	} else {
		delete(v.setValues, name)
	}
}

func (v *forEachSurfaceValidator) isListValue(expr ast.Expression) bool {
	switch e := expr.(type) {
	case *ast.AsExpr:
		return e.Type != nil && e.Type.Kind == ast.TypeList || v.isListValue(e.Expr)
	case *ast.ListLit:
		return true
	case *ast.IdentExpr:
		return v.listValues[e.Name]
	case *ast.BuiltinCallExpr:
		return e.Name == "Array.from" || e.Name == "Array.of" || e.Name == "Object.keys" || e.Name == "Object.values" || e.Name == "Object.entries"
	case *ast.MethodCallExpr:
		switch e.Method {
		case "map", "filter", "slice", "concat", "reverse", "push", "pop", "shift", "unshift":
			return v.isListValue(e.Receiver)
		}
	}
	return false
}

func (v *forEachSurfaceValidator) isSetValue(expr ast.Expression) bool {
	switch e := expr.(type) {
	case *ast.AsExpr:
		return e.Type != nil && e.Type.Kind == ast.TypeSet || v.isSetValue(e.Expr)
	case *ast.NewExpr:
		return e.ClassName == "Set"
	case *ast.IdentExpr:
		return v.setValues[e.Name]
	}
	return false
}

func (v *forEachSurfaceValidator) withParamScope(params []*ast.Param, fn func() error) error {
	savedLists := v.listValues
	savedSets := v.setValues
	v.listValues = cloneBoolMap(savedLists)
	v.setValues = cloneBoolMap(savedSets)
	for _, param := range params {
		if param.Type != nil && param.Type.Kind == ast.TypeList {
			v.listValues[param.Name] = true
		} else {
			delete(v.listValues, param.Name)
		}
		if param.Type != nil && param.Type.Kind == ast.TypeSet {
			v.setValues[param.Name] = true
		} else {
			delete(v.setValues, param.Name)
		}
	}
	err := fn()
	v.listValues = savedLists
	v.setValues = savedSets
	return err
}

func isProcessExitCall(e *ast.MethodCallExpr) bool {
	ident, ok := e.Receiver.(*ast.IdentExpr)
	return ok && ident.Name == "process" && e.Method == "exit"
}
