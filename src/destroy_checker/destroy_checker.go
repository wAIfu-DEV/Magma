// Package destroychecker implements Magma's optional, warning-only ownership
// analysis. It deliberately tracks direct local variables only: fields,
// indexed values, pointers, and partial moves are outside its model.
package destroychecker

import (
	"Magma/src/types"
	"fmt"
	"os"
	"strings"
)

// Enabled is the single switch for the pass. The parser retains annotations
// even when this is false, but compilation behavior is otherwise unchanged.
const Enabled = true

type State uint8

const (
	stateLive State = iota
	stateConsumed
	stateMaybeConsumed
	stateBorrowed
	stateConditional
)

type Diagnostic struct {
	FilePath string
	Line     uint32
	Column   uint32
	Message  string
}

type flow struct {
	states     map[*types.NodeExprVarDef]State
	deferred   map[*types.NodeExprVarDef]bool
	conditions map[*types.NodeExprVarDef]*types.NodeExprVarDef
	scopes     []deferScope
	terminated bool
}

type deferScope struct {
	locals   map[*types.NodeExprVarDef]bool
	deferred []*types.NodeStmtDefer
}

type analyzer struct {
	shared      *types.SharedState
	file        *types.FileCtx
	diagnostics []Diagnostic
	seen        map[string]bool
	loopBreaks  [][]flow
	loopNext    [][]flow
	loopDepths  []int
}

func cloneFlow(in flow) flow {
	out := flow{states: map[*types.NodeExprVarDef]State{}, deferred: map[*types.NodeExprVarDef]bool{}, conditions: map[*types.NodeExprVarDef]*types.NodeExprVarDef{}, terminated: in.terminated}
	for variable, state := range in.states {
		out.states[variable] = state
	}
	for variable, pending := range in.deferred {
		out.deferred[variable] = pending
	}
	for variable, condition := range in.conditions {
		out.conditions[variable] = condition
	}
	for _, scope := range in.scopes {
		copyScope := deferScope{locals: map[*types.NodeExprVarDef]bool{}, deferred: append([]*types.NodeStmtDefer(nil), scope.deferred...)}
		for variable := range scope.locals {
			copyScope.locals[variable] = true
		}
		out.scopes = append(out.scopes, copyScope)
	}
	return out
}

func variableName(variable *types.NodeExprVarDef) string {
	if name, ok := variable.Name.(*types.NodeNameSingle); ok {
		return name.Name
	}
	return "<value>"
}

func directVariable(expr types.NodeExpr) *types.NodeExprVarDef {
	name, ok := expr.(*types.NodeExprName)
	if !ok || len(name.MemberAccesses) != 0 {
		return nil
	}
	variable, _ := name.AssociatedNode.(*types.NodeExprVarDef)
	return variable
}

func variableToken(variable *types.NodeExprVarDef) types.Token {
	if variable == nil {
		return types.Token{}
	}
	switch name := variable.Name.(type) {
	case *types.NodeNameSingle:
		return name.Tk
	case *types.NodeNameComposite:
		if len(name.Tokens) != 0 {
			return name.Tokens[0]
		}
	}
	return types.Token{}
}

func (a *analyzer) warn(token types.Token, message string) {
	key := fmt.Sprintf("%s\x00%d\x00%d\x00%s", a.file.FilePath, token.Pos.Line, token.Pos.Col, message)
	if a.seen[key] {
		return
	}
	a.seen[key] = true
	a.diagnostics = append(a.diagnostics, Diagnostic{
		FilePath: a.file.FilePath,
		Line:     token.Pos.Line,
		Column:   token.Pos.Col,
		Message:  message,
	})
}

func (a *analyzer) structFor(kind types.NodeTypeKind) *types.StructDef {
	absolute, ok := kind.(*types.NodeTypeAbsolute)
	if !ok {
		return nil
	}
	separator := strings.Index(absolute.AbsoluteName, ".")
	if separator < 0 {
		return nil
	}
	module, name := absolute.AbsoluteName[:separator], absolute.AbsoluteName[separator+1:]
	for _, file := range a.shared.Files {
		if file.PackageName == module {
			return file.GlNode.StructDefs[name]
		}
	}
	return nil
}

func (a *analyzer) destructible(nodeType *types.NodeType) bool {
	if nodeType == nil {
		return false
	}
	if named, ok := nodeType.KindNode.(*types.NodeTypeNamed); ok {
		if single, ok := named.NameNode.(*types.NodeNameSingle); ok {
			for _, file := range a.shared.Files {
				if len(file.GlNode.PrimitiveDestructors[single.Name]) != 0 {
					return true
				}
			}
		}
	}
	definition := a.structFor(nodeType.KindNode)
	return definition != nil && len(definition.Destructors) != 0
}

func (a *analyzer) tracked(out *flow, variable *types.NodeExprVarDef) bool {
	if variable == nil {
		return false
	}
	state, exists := out.states[variable]
	return exists && state != stateBorrowed
}

func (a *analyzer) use(out *flow, variable *types.NodeExprVarDef) {
	if variable == nil {
		return
	}
	state, exists := out.states[variable]
	if !exists || state == stateBorrowed {
		return
	}
	if state != stateLive {
		a.warn(variableToken(variable), fmt.Sprintf("destructible value '%s' may be used after ownership was transferred", variableName(variable)))
	}
}

func (a *analyzer) consume(out *flow, variable *types.NodeExprVarDef, reason string) {
	if variable == nil || !a.destructible(variable.Type) {
		return
	}
	state, exists := out.states[variable]
	if !exists || state == stateBorrowed {
		a.warn(variableToken(variable), fmt.Sprintf("borrowed destructible value '%s' cannot be consumed (%s)", variableName(variable), reason))
		return
	}
	if state != stateLive {
		a.warn(variableToken(variable), fmt.Sprintf("destructible value '%s' may be consumed more than once (%s)", variableName(variable), reason))
		return
	}
	if out.deferred[variable] {
		a.warn(variableToken(variable), fmt.Sprintf("destructible value '%s' is transferred while a deferred destructor is pending", variableName(variable)))
	}
	out.states[variable] = stateConsumed
}

func (a *analyzer) borrowExpr(out *flow, expr types.NodeExpr) {
	switch node := expr.(type) {
	case *types.NodeExprName:
		a.use(out, directVariable(node))
	case *types.NodeExprBinary:
		a.borrowExpr(out, node.Left)
		a.borrowExpr(out, node.Right)
	case *types.NodeExprUnary:
		a.borrowExpr(out, node.Operand)
	case *types.NodeExprMemberAccess:
		// Partial ownership is intentionally ignored. The target is not marked
		// consumed, but ordinary use-after-move of the whole target is visible.
		a.borrowExpr(out, node.Target)
	case *types.NodeExprSubscript:
		a.borrowExpr(out, node.Target)
		a.borrowExpr(out, node.Expr)
	case *types.NodeExprAddrof:
		// Pointer escape is explicitly outside the first draft.
	case *types.NodeExprCall:
		a.call(out, node)
	case *types.NodeExprTry:
		a.borrowExpr(out, node.Call)
		a.checkTryFailure(out)
	case *types.NodeExprStructInit:
		a.transferStructFields(out, node)
	}
}

// A struct constructor is an ownership boundary for its fields. Tracked local
// values placed into the aggregate move into it; borrowed values remain borrows.
func (a *analyzer) transferStructFields(out *flow, init *types.NodeExprStructInit) {
	for _, field := range init.Fields {
		a.transferValue(out, field.Expression)
	}
}

func (a *analyzer) call(out *flow, call *types.NodeExprCall) {
	definition := call.AssociatedFnDef
	if call.IsFuncPointer {
		functionType, ok := call.FuncPtrType.KindNode.(*types.NodeTypeFunc)
		if !ok {
			for _, argument := range call.Args {
				a.borrowExpr(out, argument)
			}
			return
		}
		for index, argument := range call.Args {
			consuming := index < len(functionType.Args) && functionType.Args[index].Owned
			if consuming {
				a.consume(out, directVariable(argument), "consuming function-pointer argument")
				if directVariable(argument) == nil {
					a.borrowExpr(out, argument)
				}
			} else {
				a.borrowExpr(out, argument)
			}
		}
		return
	}
	if definition == nil {
		for _, argument := range call.Args {
			a.borrowExpr(out, argument)
		}
		return
	}

	if call.IsMemberFunc && call.MemberOwnerExpr != nil {
		if definition.IsDestructor {
			a.consume(out, directVariable(call.MemberOwnerExpr), "destructor call")
		} else {
			a.borrowExpr(out, call.MemberOwnerExpr)
		}
	} else if call.IsMemberFunc && call.MemberOwnerName != nil {
		if definition.IsDestructor {
			a.consume(out, directVariable(call.MemberOwnerName), "destructor call")
		} else {
			a.borrowExpr(out, call.MemberOwnerName)
		}
	}

	offset := 0
	if len(definition.Class.ArgsNode.Args) > 0 && definition.Class.ArgsNode.Args[0].Name == "this" {
		offset = 1
	}
	for index, argument := range call.Args {
		parameterIndex := index + offset
		consuming := !definition.IsExternal && parameterIndex < len(definition.Class.ArgsNode.Args) && definition.Class.ArgsNode.Args[parameterIndex].TypeNode.Owned
		if consuming {
			a.consume(out, directVariable(argument), "consuming argument")
			if directVariable(argument) == nil {
				a.borrowExpr(out, argument)
			}
		} else {
			a.borrowExpr(out, argument)
		}
	}
}

func (a *analyzer) expression(out *flow, expr types.NodeExpr) {
	switch node := expr.(type) {
	case *types.NodeExprVarDef:
		if a.destructible(node.Type) {
			out.states[node] = stateLive
			a.addLocal(out, node)
		}
	case *types.NodeExprVarDefAssign:
		a.valueInto(out, node.VarDef, node.AssignExpr)
		a.addLocal(out, node.VarDef)
	case *types.NodeExprAssign:
		a.assignment(out, node)
	case *types.NodeExprCall:
		a.call(out, node)
		if node.InfType != nil && node.InfType.Owned && a.destructible(node.InfType) {
			a.warn(node.Tk, "owned destructible call result is discarded")
		}
	case *types.NodeExprTry:
		a.expression(out, node.Call)
		a.checkTryFailure(out)
	case *types.NodeExprDestructureAssign:
		a.call(out, node.Call)
		if node.Call.InfType != nil && node.Call.InfType.Owned && a.destructible(node.ValueDef.Type) {
			out.states[&node.ValueDef] = stateConditional
			if out.conditions == nil {
				out.conditions = map[*types.NodeExprVarDef]*types.NodeExprVarDef{}
			}
			out.conditions[&node.ValueDef] = &node.ErrDef
			a.addLocal(out, &node.ValueDef)
		}
	default:
		a.borrowExpr(out, expr)
	}
}

func (a *analyzer) addLocal(out *flow, variable *types.NodeExprVarDef) {
	if len(out.scopes) != 0 && a.destructible(variable.Type) {
		out.scopes[len(out.scopes)-1].locals[variable] = true
	}
}

// transferValue evaluates a value used in an ownership-transfer position and
// reports whether ownership was actually produced. Plain-returning calls and
// borrowed locals stay borrowed; owned calls and owned locals transfer.
func (a *analyzer) transferValue(out *flow, value types.NodeExpr) bool {
	switch node := value.(type) {
	case *types.NodeExprName:
		source := directVariable(node)
		if source == nil {
			a.borrowExpr(out, value)
			return false
		}
		if a.tracked(out, source) {
			a.consume(out, source, "assignment")
			return true
		}
		a.borrowExpr(out, value)
		return false
	case *types.NodeExprCall:
		a.call(out, node)
		return node.InfType != nil && node.InfType.Owned && a.destructible(node.InfType)
	case *types.NodeExprTry:
		owned := a.transferValue(out, node.Call)
		a.checkTryFailure(out)
		return owned
	case *types.NodeExprStructInit:
		a.transferStructFields(out, node)
		return a.destructible(node.Type)
	default:
		a.borrowExpr(out, value)
		return false
	}
}

func (a *analyzer) checkTryFailure(out *flow) {
	failure := cloneFlow(*out)
	// A failed try follows an implicit error-propagation edge. Execute all
	// registered defers on that edge without turning declarations preceding
	// the try into additional exit-path obligations.
	a.unwindTryFailure(&failure)
}

func (a *analyzer) setDestinationOwnership(out *flow, destination *types.NodeExprVarDef, owned bool) {
	if !a.destructible(destination.Type) {
		return
	}
	if state, exists := out.states[destination]; exists && state == stateLive {
		a.warn(variableToken(destination), fmt.Sprintf("assignment overwrites live destructible value '%s'", variableName(destination)))
	}
	delete(out.deferred, destination)
	if owned {
		out.states[destination] = stateLive
	} else {
		out.states[destination] = stateBorrowed
	}
}

func partialValue(value types.NodeExpr) bool {
	switch node := value.(type) {
	case *types.NodeExprMemberAccess, *types.NodeExprSubscript:
		return true
	case *types.NodeExprName:
		return len(node.MemberAccesses) != 0
	}
	return false
}

func (a *analyzer) valueInto(out *flow, destination *types.NodeExprVarDef, value types.NodeExpr) {
	owned := a.transferValue(out, value)
	if !owned && destination.Type != nil && destination.Type.Owned && partialValue(value) {
		// Partial moves are intentionally not analysed. An explicit `$T` local
		// may claim a field/indexed value, but calls still derive ownership only
		// from their return annotation.
		owned = true
	}
	a.setDestinationOwnership(out, destination, owned)
}

func (a *analyzer) assignment(out *flow, assignment *types.NodeExprAssign) {
	owned := a.transferValue(out, assignment.Right)

	if destination := directVariable(assignment.Left); destination != nil {
		a.setDestinationOwnership(out, destination, owned)
		return
	}
	// A field or indexed destination is an ownership escape. Its contents are
	// the container's responsibility and are not represented in this pass.
	a.borrowExpr(out, assignment.Left)
}

func mergeState(left, right State) State {
	if left == right {
		return left
	}
	return stateMaybeConsumed
}

func mergeFlows(left, right flow) flow {
	if left.terminated {
		return right
	}
	if right.terminated {
		return left
	}
	out := cloneFlow(left)
	for variable, rightState := range right.states {
		leftState, exists := out.states[variable]
		if !exists {
			out.states[variable] = mergeState(stateBorrowed, rightState)
		} else {
			out.states[variable] = mergeState(leftState, rightState)
		}
	}
	for variable, leftState := range left.states {
		if _, exists := right.states[variable]; !exists {
			out.states[variable] = mergeState(leftState, stateBorrowed)
		}
	}
	for variable, pending := range right.deferred {
		out.deferred[variable] = out.deferred[variable] || pending
	}
	for variable, state := range out.states {
		if state != stateConditional {
			delete(out.conditions, variable)
		}
	}
	return out
}

func (a *analyzer) checkExit(out *flow) {
	exit := cloneFlow(*out)
	a.unwindTo(&exit, 0)
	for variable, state := range exit.states {
		if state == stateLive || state == stateMaybeConsumed || state == stateConditional {
			a.warn(variableToken(variable), fmt.Sprintf("destructible value '%s' is not consumed on every exit path", variableName(variable)))
		}
	}
}

func (a *analyzer) runDeferred(out *flow, deferred *types.NodeStmtDefer) {
	if deferred.IsBody {
		a.body(out, &deferred.Body)
	} else {
		a.expression(out, deferred.Expression)
	}
}

func (a *analyzer) unwindScope(out *flow, checkLocals bool) {
	index := len(out.scopes) - 1
	scope := out.scopes[index]
	out.scopes = out.scopes[:index]
	for i := len(scope.deferred) - 1; i >= 0; i-- {
		a.runDeferred(out, scope.deferred[i])
	}
	for variable := range scope.locals {
		state := out.states[variable]
		if checkLocals && (state == stateLive || state == stateMaybeConsumed || state == stateConditional) {
			a.warn(variableToken(variable), fmt.Sprintf("destructible value '%s' is not consumed on every scope exit path", variableName(variable)))
		}
		delete(out.states, variable)
		delete(out.deferred, variable)
		delete(out.conditions, variable)
	}
}

func (a *analyzer) unwindTo(out *flow, depth int) {
	for len(out.scopes) > depth {
		a.unwindScope(out, true)
	}
}

func (a *analyzer) unwindTryFailure(out *flow) {
	for len(out.scopes) > 0 {
		a.unwindScope(out, false)
	}
}

func errorPredicate(expr types.NodeExpr) (*types.NodeExprVarDef, bool, bool) {
	call, ok := expr.(*types.NodeExprCall)
	if !ok || call.AssociatedFnDef == nil || call.AssociatedFnDef.ErrorPredicate == types.ErrorPredicateNone {
		return nil, false, false
	}
	var owner *types.NodeExprVarDef
	if call.MemberOwnerExpr != nil {
		owner = directVariable(call.MemberOwnerExpr)
	} else if call.MemberOwnerName != nil {
		owner = directVariable(call.MemberOwnerName)
	}
	if owner == nil {
		return nil, false, false
	}
	return owner, call.AssociatedFnDef.ErrorPredicate == types.ErrorPredicateOk, true
}

func refineConditionalOwnership(in flow, errVariable *types.NodeExprVarDef, success bool) flow {
	out := cloneFlow(in)
	for variable, condition := range out.conditions {
		if condition != errVariable {
			continue
		}
		if success {
			out.states[variable] = stateLive
		} else {
			out.states[variable] = stateBorrowed
		}
		delete(out.conditions, variable)
	}
	return out
}

func predicateFlows(in flow, expr types.NodeExpr) (flow, flow) {
	errVariable, successOnTrue, ok := errorPredicate(expr)
	if !ok {
		return cloneFlow(in), cloneFlow(in)
	}
	return refineConditionalOwnership(in, errVariable, successOnTrue), refineConditionalOwnership(in, errVariable, !successOnTrue)
}

func (a *analyzer) conditional(out *flow, statement *types.NodeStmtIf) {
	a.borrowExpr(out, statement.CondExpr)
	branches := []flow{}
	first, remaining := predicateFlows(*out, statement.CondExpr)
	a.body(&first, &statement.Body)
	branches = append(branches, first)

	next := statement.NextCondStmt
	hasElse := false
	for next != nil {
		switch branch := next.(type) {
		case *types.NodeStmtIf:
			candidate, falseFlow := predicateFlows(remaining, branch.CondExpr)
			a.borrowExpr(&candidate, branch.CondExpr)
			a.body(&candidate, &branch.Body)
			branches = append(branches, candidate)
			remaining = falseFlow
			next = branch.NextCondStmt
		case *types.NodeStmtElse:
			hasElse = true
			candidate := cloneFlow(remaining)
			a.body(&candidate, &branch.Body)
			branches = append(branches, candidate)
			next = nil
		}
	}
	if !hasElse {
		branches = append(branches, remaining)
	}
	merged := branches[0]
	for _, branch := range branches[1:] {
		merged = mergeFlows(merged, branch)
	}
	*out = merged
}

func (a *analyzer) statement(out *flow, statement types.NodeStatement) {
	if out.terminated {
		return
	}
	switch node := statement.(type) {
	case *types.NodeStmtExpr:
		a.expression(out, node.Expression)
	case *types.NodeStmtRet:
		if node.OwnerFuncType != nil && node.OwnerFuncType.Owned {
			// Evaluate the complete return expression. Calls may consume owned
			// arguments and constructors may move locals into aggregate fields.
			a.transferValue(out, node.Expression)
		} else if init, ok := node.Expression.(*types.NodeExprStructInit); ok {
			// Aggregate construction transfers its fields even when the outer
			// return type itself is not ownership-tracked.
			a.transferStructFields(out, init)
		} else {
			a.borrowExpr(out, node.Expression)
		}
		a.unwindTo(out, 0)
		a.checkExit(out)
		out.terminated = true
	case *types.NodeStmtThrow:
		a.borrowExpr(out, node.Expression)
		a.unwindTo(out, 0)
		a.checkExit(out)
		out.terminated = true
	case *types.NodeStmtIf:
		a.conditional(out, node)
	case *types.NodeStmtWhile:
		a.borrowExpr(out, node.CondExpr)
		a.loopBreaks = append(a.loopBreaks, nil)
		a.loopNext = append(a.loopNext, nil)
		a.loopDepths = append(a.loopDepths, len(out.scopes))
		iteration := cloneFlow(*out)
		a.body(&iteration, &node.Body)
		loopIndex := len(a.loopBreaks) - 1
		if !iteration.terminated {
			a.loopNext[loopIndex] = append(a.loopNext[loopIndex], iteration)
		}
		breaks := a.loopBreaks[loopIndex]
		if isLiteralTrue(node.CondExpr) {
			if len(breaks) == 0 {
				out.terminated = true
			} else {
				*out = breaks[0]
				for _, broken := range breaks[1:] {
					*out = mergeFlows(*out, broken)
				}
			}
		} else {
			for _, next := range a.loopNext[loopIndex] {
				*out = mergeFlows(*out, next)
			}
			for _, broken := range breaks {
				*out = mergeFlows(*out, broken)
			}
		}
		a.loopBreaks = a.loopBreaks[:loopIndex]
		a.loopNext = a.loopNext[:loopIndex]
		a.loopDepths = a.loopDepths[:loopIndex]
	case *types.NodeStmtBreak:
		if len(a.loopBreaks) != 0 {
			index := len(a.loopBreaks) - 1
			a.unwindTo(out, a.loopDepths[index])
			exit := cloneFlow(*out)
			exit.terminated = false
			a.loopBreaks[index] = append(a.loopBreaks[index], exit)
			out.terminated = true
		}
	case *types.NodeStmtContinue:
		if len(a.loopNext) != 0 {
			index := len(a.loopNext) - 1
			a.unwindTo(out, a.loopDepths[index])
			next := cloneFlow(*out)
			next.terminated = false
			a.loopNext[index] = append(a.loopNext[index], next)
			out.terminated = true
		}
	case *types.NodeStmtDefer:
		if len(out.scopes) != 0 {
			out.scopes[len(out.scopes)-1].deferred = append(out.scopes[len(out.scopes)-1].deferred, node)
		}
	}
}

func (a *analyzer) body(out *flow, body *types.NodeBody) {
	depth := len(out.scopes)
	out.scopes = append(out.scopes, deferScope{locals: map[*types.NodeExprVarDef]bool{}})
	for _, statement := range body.Statements {
		a.statement(out, statement)
	}
	if len(out.scopes) > depth {
		a.unwindTo(out, depth)
	}
}

func isLiteralTrue(expr types.NodeExpr) bool {
	literal, ok := expr.(*types.NodeExprLit)
	return ok && literal.LitType == types.TokLitBool && literal.Value == "true"
}

func (a *analyzer) function(function *types.NodeFuncDef) {
	out := flow{states: map[*types.NodeExprVarDef]State{}, deferred: map[*types.NodeExprVarDef]bool{}, conditions: map[*types.NodeExprVarDef]*types.NodeExprVarDef{}, scopes: []deferScope{{locals: map[*types.NodeExprVarDef]bool{}}}}
	// Parameter declaration nodes are manufactured by scope_info. Seed owned
	// parameters so an unused one still produces an exit warning.
	for _, fnScope := range a.file.ScopeTree.DeclFuncs {
		if fnScope.Func != function {
			continue
		}
		for _, argument := range function.Class.ArgsNode.Args {
			variable := fnScope.Scope.DeclVars[argument.Name]
			if variable != nil && variable.Type != nil && variable.Type.Owned && a.destructible(variable.Type) {
				out.states[variable] = stateLive
				out.scopes[0].locals[variable] = true
			}
		}
	}
	a.body(&out, &function.Body)
	if !out.terminated {
		a.unwindTo(&out, 0)
		a.checkExit(&out)
	}
}

func validateDestructors(a *analyzer, global *types.NodeGlobal) {
	// A destructor marks a consuming operation; its result type is unrestricted.
}

// Check runs the analysis and returns all warnings without changing the AST.
func Check(shared *types.SharedState) []Diagnostic {
	if !Enabled {
		return nil
	}
	diagnostics := []Diagnostic{}
	for _, file := range shared.Files {
		a := &analyzer{shared: shared, file: file, seen: map[string]bool{}}
		validateDestructors(a, file.GlNode)
		for _, declaration := range file.GlNode.Declarations {
			if function, ok := declaration.(*types.NodeFuncDef); ok {
				a.function(function)
			}
		}
		diagnostics = append(diagnostics, a.diagnostics...)
	}
	return diagnostics
}

// Run is the compiler pipeline hook. Warnings are intentionally non-fatal.
func Run(shared *types.SharedState) {
	for _, diagnostic := range Check(shared) {
		fmt.Fprintf(os.Stderr, "%s:%d:%d: warning: %s\n", diagnostic.FilePath, diagnostic.Line, diagnostic.Column, diagnostic.Message)
	}
}
