package semantics

import "github.com/victor141516/besht/internal/ast"

func ValidateObjectSurface(stmts []ast.Statement) error {
	return ValidateObjectSurfaceWithTypes(stmts, nil)
}

func ValidateObjectSurfaceWithTypes(stmts []ast.Statement, varTypes map[string]*ast.Type) error {
	objectValues := map[string]bool{}
	for name, typ := range varTypes {
		if typ != nil && typ.Kind == ast.TypeObject {
			objectValues[name] = true
		}
	}
	v := objectSurfaceValidator{
		objectValues:                    objectValues,
		objectAliases:                   map[string]string{},
		objectValuesWithUnsupportedData: map[string]bool{},
		processEnvAliases:               map[string]bool{},
		classStaticObjects:              map[string]bool{},
		classStaticUnsupportedObjects:   map[string]bool{},
		fnReturnObjects:                 collectObjectSurfaceFunctionReturns(stmts),
	}
	return v.stmts(stmts)
}

type objectSurfaceValidator struct {
	objectValues                    map[string]bool
	objectAliases                   map[string]string
	objectValuesWithUnsupportedData map[string]bool
	processEnvAliases               map[string]bool
	classStaticObjects              map[string]bool
	classStaticUnsupportedObjects   map[string]bool
	fnReturnObjects                 map[string]bool
}

func (v *objectSurfaceValidator) stmts(stmts []ast.Statement) error {
	for _, stmt := range stmts {
		if err := v.stmt(stmt); err != nil {
			return err
		}
	}
	return nil
}

func (v *objectSurfaceValidator) block(block *ast.Block) error {
	if block == nil {
		return nil
	}
	return v.stmts(block.Statements)
}

func (v *objectSurfaceValidator) stmt(stmt ast.Statement) error {
	switch s := stmt.(type) {
	case nil, *ast.ImportDecl, *ast.DeclareStmt, *ast.DeclareFnStmt, *ast.BreakStmt, *ast.ContinueStmt:
		return nil
	case *ast.LetDecl:
		if err := v.expr(s.Value); err != nil {
			return err
		}
		v.trackLet(s.Name, s.Value)
		return nil
	case *ast.DestructureDecl:
		return v.expr(s.Value)
	case *ast.Assignment:
		if err := v.expr(s.Value); err != nil {
			return err
		}
		v.trackAssignment(s.Name, s.Value)
		return nil
	case *ast.IndexAssignStmt:
		if err := v.expr(s.Index); err != nil {
			return err
		}
		if err := v.expr(s.Value); err != nil {
			return err
		}
		v.trackObjectMutation(s.Name, s.Value)
		return nil
	case *ast.PropertyAssignStmt:
		if err := v.expr(s.Value); err != nil {
			return err
		}
		v.trackObjectMutation(s.Object, s.Value)
		return nil
	case *ast.FnDecl:
		return v.withParamScope(s.Params, false, func() error { return v.block(s.Body) })
	case *ast.ClassDecl:
		for i := range s.StaticProps {
			if _, ok := s.StaticProps[i].Value.(*ast.ObjectLit); ok {
				name := s.Name + "." + s.StaticProps[i].Name
				v.classStaticObjects[name] = true
				if objectSurfaceHasUnsupportedValue(s.StaticProps[i].Value) {
					v.classStaticUnsupportedObjects[name] = true
				} else {
					delete(v.classStaticUnsupportedObjects, name)
				}
			}
		}
		for i := range s.Properties {
			if err := v.expr(s.Properties[i].Value); err != nil {
				return err
			}
		}
		for i := range s.StaticProps {
			if err := v.expr(s.StaticProps[i].Value); err != nil {
				return err
			}
		}
		if s.Constructor != nil {
			if err := v.withParamScope(s.Constructor.Params, true, func() error { return v.block(s.Constructor.Body) }); err != nil {
				return err
			}
		}
		for i := range s.Methods {
			if err := v.withParamScope(s.Methods[i].Params, !s.Methods[i].IsStatic, func() error { return v.block(s.Methods[i].Body) }); err != nil {
				return err
			}
		}
		for i := range s.Accessors {
			if err := v.withParamScope(s.Accessors[i].Params, !s.Accessors[i].IsStatic, func() error { return v.block(s.Accessors[i].Body) }); err != nil {
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
		return v.expr(s.Expr)
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
	}
	return nil
}

func (v *objectSurfaceValidator) expr(expr ast.Expression) error {
	switch e := expr.(type) {
	case nil:
		return nil
	case *ast.ObjectLit:
		for _, field := range e.Fields {
			if field.Spread != nil {
				if err := v.expr(field.Spread); err != nil {
					return err
				}
				if v.isProcessEnvValue(field.Spread) || !v.isObjectValue(field.Spread) {
					return &SemanticError{Pos: field.Pos, Message: "object spread requires an object literal or named object"}
				}
				if v.hasUnsupportedObjectValue(field.Spread) {
					return &SemanticError{Pos: field.Pos, Message: "object spread only supports scalar object values"}
				}
				continue
			}
			if err := validateObjectKey(field.Key); err != nil {
				return &SemanticError{Pos: field.Pos, Message: err.Error()}
			}
			if err := v.expr(field.Value); err != nil {
				return err
			}
		}
	case *ast.BuiltinCallExpr:
		if e.Name == "Object.keys" || e.Name == "Object.values" || e.Name == "Object.entries" || e.Name == "Object.fromEntries" || e.Name == "Object.hasOwn" || e.Name == "Object.assign" {
			wantArgs := 1
			if e.Name == "Object.hasOwn" {
				wantArgs = 2
			}
			if e.Name == "Object.assign" {
				wantArgs = 0
			}
			if e.Name == "Object.assign" && len(e.Args) < 1 {
				return &SemanticError{Pos: e.Pos, Message: "Object.assign() takes at least 1 argument"}
			}
			if wantArgs > 0 && len(e.Args) != wantArgs {
				return &SemanticError{Pos: e.Pos, Message: e.Name + "() takes " + objectSurfaceArgCount(wantArgs)}
			}
			if e.Name == "Object.fromEntries" {
				for _, arg := range e.Args {
					if err := v.expr(arg); err != nil {
						return err
					}
				}
				return nil
			}
			if v.isProcessEnvValue(e.Args[0]) || !v.isObjectValue(e.Args[0]) {
				return &SemanticError{Pos: e.Pos, Message: e.Name + "() requires an object literal or named object"}
			}
			if e.Name == "Object.assign" {
				if _, isThis := unwrapAsExpr(e.Args[0]).(*ast.ThisExpr); isThis {
					return &SemanticError{Pos: e.Pos, Message: "Object.assign() requires an object literal or named object"}
				}
				for _, arg := range e.Args[1:] {
					if v.isProcessEnvValue(arg) || !v.isObjectValue(arg) {
						return &SemanticError{Pos: e.Pos, Message: "Object.assign() requires an object literal or named object"}
					}
				}
			}
			if (e.Name == "Object.values" || e.Name == "Object.entries" || e.Name == "Object.assign") && v.hasUnsupportedObjectValue(e.Args[0]) {
				return &SemanticError{Pos: e.Pos, Message: e.Name + "() only supports scalar object values"}
			}
			if e.Name == "Object.assign" {
				for _, arg := range e.Args[1:] {
					if v.hasUnsupportedObjectValue(arg) {
						return &SemanticError{Pos: e.Pos, Message: "Object.assign() only supports scalar object values"}
					}
				}
			}
		}
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
	case *ast.MethodCallExpr:
		if err := v.expr(e.Receiver); err != nil {
			return err
		}
		if isObjectSurfaceReduceMethod(e.Method) && len(e.Args) >= 2 {
			if arrow, ok := e.Args[0].(*ast.ArrowExpr); ok {
				if err := v.expr(e.Args[1]); err != nil {
					return err
				}
				objectParams := map[string]bool{}
				if len(arrow.Params) > 0 && v.isObjectValue(e.Args[1]) {
					objectParams[arrow.Params[0].Name] = true
				}
				if err := v.withArrowParamScope(arrow, objectParams, func() error {
					if err := v.expr(arrow.Body); err != nil {
						return err
					}
					return v.block(arrow.BlockBody)
				}); err != nil {
					return err
				}
				for _, arg := range e.Args[2:] {
					if err := v.expr(arg); err != nil {
						return err
					}
				}
				return nil
			}
		}
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
	case *ast.UpdateExpr:
		return nil
	case *ast.IndexExpr:
		if err := v.expr(e.Expr); err != nil {
			return err
		}
		return v.expr(e.Index)
	case *ast.PropertyExpr:
		return v.expr(e.Receiver)
	case *ast.ListLit:
		for _, elem := range e.Elements {
			if err := v.expr(elem); err != nil {
				return err
			}
		}
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
		return v.withArrowParamScope(e, nil, func() error {
			if err := v.expr(e.Body); err != nil {
				return err
			}
			return v.block(e.BlockBody)
		})
	case *ast.NewExpr:
		for _, arg := range e.Args {
			if err := v.expr(arg); err != nil {
				return err
			}
		}
	}
	return nil
}

func (v *objectSurfaceValidator) trackLet(name string, expr ast.Expression) {
	if v.isProcessEnvValue(expr) {
		v.processEnvAliases[name] = true
	} else {
		delete(v.processEnvAliases, name)
	}
	if v.isObjectValue(expr) {
		v.objectValues[name] = true
		root := v.objectRootForExpr(expr)
		if root == "" {
			root = name
		}
		v.objectAliases[name] = root
		if v.hasUnsupportedObjectValue(expr) {
			v.objectValuesWithUnsupportedData[root] = true
		} else {
			delete(v.objectValuesWithUnsupportedData, root)
		}
	} else {
		delete(v.objectValues, name)
		delete(v.objectAliases, name)
		delete(v.objectValuesWithUnsupportedData, name)
	}
}

func (v *objectSurfaceValidator) trackAssignment(name string, expr ast.Expression) {
	v.trackLet(name, expr)
}

func (v *objectSurfaceValidator) isObjectValue(expr ast.Expression) bool {
	switch e := expr.(type) {
	case *ast.AsExpr:
		return v.isObjectValue(e.Expr)
	case *ast.ObjectLit:
		return true
	case *ast.IdentExpr:
		return v.objectValues[e.Name]
	case *ast.FnCallExpr:
		return v.fnReturnObjects[e.Name]
	case *ast.BuiltinCallExpr:
		return (e.Name == "Object.assign" && len(e.Args) >= 1) || (e.Name == "Object.fromEntries" && len(e.Args) == 1)
	case *ast.PropertyExpr:
		if ident, ok := e.Receiver.(*ast.IdentExpr); ok {
			return v.classStaticObjects[ident.Name+"."+e.Property]
		}
	case *ast.MethodCallExpr:
		return isObjectSurfaceReduceMethod(e.Method) && len(e.Args) >= 2 && v.isObjectValue(e.Args[1])
	}
	return false
}

func isObjectSurfaceReduceMethod(method string) bool {
	return method == "reduce" || method == "reduceRight"
}

func collectObjectSurfaceFunctionReturns(stmts []ast.Statement) map[string]bool {
	out := map[string]bool{}
	for _, stmt := range stmts {
		switch s := stmt.(type) {
		case *ast.FnDecl:
			out[s.Name] = s.ReturnType != nil && s.ReturnType.Kind == ast.TypeObject
		case *ast.DeclareFnStmt:
			out[s.Name] = s.ReturnType != nil && s.ReturnType.Kind == ast.TypeObject
		}
	}
	return out
}

func (v *objectSurfaceValidator) objectRootForName(name string) string {
	if root, ok := v.objectAliases[name]; ok {
		return root
	}
	if v.objectValues[name] {
		return name
	}
	return ""
}

func (v *objectSurfaceValidator) objectRootForExpr(expr ast.Expression) string {
	switch e := expr.(type) {
	case *ast.AsExpr:
		return v.objectRootForExpr(e.Expr)
	case *ast.IdentExpr:
		return v.objectRootForName(e.Name)
	case *ast.BuiltinCallExpr:
		if e.Name == "Object.assign" && len(e.Args) >= 1 {
			return v.objectRootForExpr(e.Args[0])
		}
		if e.Name == "Object.fromEntries" && len(e.Args) == 1 {
			return ""
		}
	case *ast.PropertyExpr:
		if ident, ok := e.Receiver.(*ast.IdentExpr); ok && v.classStaticObjects[ident.Name+"."+e.Property] {
			return ident.Name + "." + e.Property
		}
	}
	return ""
}

func (v *objectSurfaceValidator) trackObjectMutation(name string, value ast.Expression) {
	root := v.objectRootForName(name)
	if root == "" {
		return
	}
	if objectSurfaceValueUnsupported(value) || v.hasUnsupportedObjectValue(value) {
		v.objectValuesWithUnsupportedData[root] = true
	}
}

func (v *objectSurfaceValidator) isProcessEnvValue(expr ast.Expression) bool {
	if isProcessEnvExpr(expr) {
		return true
	}
	ident, ok := expr.(*ast.IdentExpr)
	return ok && v.processEnvAliases[ident.Name]
}

func (v *objectSurfaceValidator) hasUnsupportedObjectValue(expr ast.Expression) bool {
	switch e := expr.(type) {
	case *ast.AsExpr:
		return v.hasUnsupportedObjectValue(e.Expr)
	case *ast.ObjectLit:
		for _, field := range e.Fields {
			if field.Spread != nil {
				if v.hasUnsupportedObjectValue(field.Spread) {
					return true
				}
				continue
			}
			if objectSurfaceValueUnsupported(field.Value) {
				return true
			}
		}
		return false
	case *ast.IdentExpr:
		return v.objectValuesWithUnsupportedData[v.objectRootForName(e.Name)]
	case *ast.BuiltinCallExpr:
		if e.Name == "Object.assign" {
			for _, arg := range e.Args {
				if v.hasUnsupportedObjectValue(arg) {
					return true
				}
			}
		}
		if e.Name == "Object.fromEntries" {
			return false
		}
	case *ast.PropertyExpr:
		if ident, ok := e.Receiver.(*ast.IdentExpr); ok {
			return v.classStaticUnsupportedObjects[ident.Name+"."+e.Property]
		}
	case *ast.MethodCallExpr:
		return isObjectSurfaceReduceMethod(e.Method) && len(e.Args) >= 2 && v.hasUnsupportedObjectValue(e.Args[1])
	}
	return false
}

func (v *objectSurfaceValidator) withScope(fn func() error) error {
	savedObjects := v.objectValues
	savedAliases := v.objectAliases
	savedUnsupported := v.objectValuesWithUnsupportedData
	savedProcessEnv := v.processEnvAliases
	v.objectValues = cloneBoolMap(savedObjects)
	v.objectAliases = cloneStringMap(savedAliases)
	v.objectValuesWithUnsupportedData = cloneBoolMap(savedUnsupported)
	v.processEnvAliases = cloneBoolMap(savedProcessEnv)
	err := fn()
	v.objectValues = savedObjects
	v.objectAliases = savedAliases
	v.objectValuesWithUnsupportedData = savedUnsupported
	v.processEnvAliases = savedProcessEnv
	return err
}

func (v *objectSurfaceValidator) withParamScope(params []*ast.Param, includeThis bool, fn func() error) error {
	return v.withScope(func() error {
		if includeThis {
			v.objectValues["this"] = true
			v.objectAliases["this"] = "this"
		}
		for _, param := range params {
			if param.Type != nil && param.Type.Kind == ast.TypeObject {
				v.objectValues[param.Name] = true
				v.objectAliases[param.Name] = param.Name
				delete(v.objectValuesWithUnsupportedData, param.Name)
			} else {
				delete(v.objectValues, param.Name)
				delete(v.objectAliases, param.Name)
				delete(v.objectValuesWithUnsupportedData, param.Name)
			}
			delete(v.processEnvAliases, param.Name)
		}
		return fn()
	})
}

func (v *objectSurfaceValidator) withArrowParamScope(arrow *ast.ArrowExpr, objectParams map[string]bool, fn func() error) error {
	return v.withScope(func() error {
		for _, param := range arrow.Params {
			if objectParams[param.Name] || (param.Type != nil && param.Type.Kind == ast.TypeObject) {
				v.objectValues[param.Name] = true
				v.objectAliases[param.Name] = param.Name
				delete(v.objectValuesWithUnsupportedData, param.Name)
			} else {
				delete(v.objectValues, param.Name)
				delete(v.objectAliases, param.Name)
				delete(v.objectValuesWithUnsupportedData, param.Name)
			}
			delete(v.processEnvAliases, param.Name)
		}
		return fn()
	})
}

func cloneBoolMap(src map[string]bool) map[string]bool {
	dst := make(map[string]bool, len(src))
	for k, val := range src {
		dst[k] = val
	}
	return dst
}

func cloneStringMap(src map[string]string) map[string]string {
	dst := make(map[string]string, len(src))
	for k, val := range src {
		dst[k] = val
	}
	return dst
}

func objectSurfaceArgCount(n int) string {
	if n == 1 {
		return "1 argument"
	}
	return "2 arguments"
}

func objectSurfaceHasUnsupportedValue(expr ast.Expression) bool {
	as, ok := expr.(*ast.AsExpr)
	if ok {
		return objectSurfaceHasUnsupportedValue(as.Expr)
	}
	obj, ok := expr.(*ast.ObjectLit)
	if !ok {
		return false
	}
	for _, field := range obj.Fields {
		if field.Spread != nil {
			continue
		}
		if objectSurfaceValueUnsupported(field.Value) {
			return true
		}
	}
	return false
}

func objectSurfaceValueUnsupported(expr ast.Expression) bool {
	switch value := unwrapAsExpr(expr).(type) {
	case *ast.ListLit, *ast.ObjectLit, *ast.NewExpr, *ast.CmdExpr:
		return true
	case *ast.BuiltinCallExpr:
		return value.Name == "Object.keys" || value.Name == "Object.values" || value.Name == "Object.entries" || value.Name == "Object.assign" || value.Name == "Object.fromEntries" || value.Name == "Array.from" || value.Name == "Array.of" || value.Name == "fetch"
	case *ast.MethodCallExpr:
		switch value.Method {
		case "map", "filter", "slice", "concat", "reverse", "sort", "push", "pop", "shift", "unshift", "readStdoutLines":
			return true
		}
	}
	return false
}

func unwrapAsExpr(expr ast.Expression) ast.Expression {
	for {
		as, ok := expr.(*ast.AsExpr)
		if !ok {
			return expr
		}
		expr = as.Expr
	}
}

func unsupportedObjectValueType(t *ast.Type) bool {
	if t == nil {
		return false
	}
	switch t.Kind {
	case ast.TypeList, ast.TypeSet, ast.TypeObject, ast.TypeCommand, ast.TypeFetchResponse:
		return true
	}
	return false
}
