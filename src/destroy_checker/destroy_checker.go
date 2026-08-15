// Package destroychecker implements Magma's ownership-safety analysis. Fatal
// transfer/range errors and non-fatal resource-hygiene findings share this pass
// and its control-flow model. It tracks canonical aggregate places and records
// compiler-only range proofs on authorized subscripts.
package destroychecker

import (
	"Magma/src/comp_err"
	"Magma/src/magma_types"
	place "Magma/src/safety/place"
	"Magma/src/types"
	"fmt"
	"reflect"
	"strconv"
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
	Token    types.Token
	Message  string
	// Safety marks definite invalid ownership operations. Cleanup and leak
	// findings remain resource-hygiene warnings.
	Safety  bool
	Code    string
	Related []types.DiagnosticRelated
}

type flow struct {
	states map[*types.NodeExprVarDef]State
	// absent records projected ownership places which have been moved out.
	// Roots remain in states so the existing control-flow machinery continues
	// to model declarations, branches, and cleanup scopes unchanged.
	absent     map[placeKey]types.Token
	deferred   map[*types.NodeExprVarDef]bool
	consumedAt map[*types.NodeExprVarDef]types.Token
	deferredAt map[*types.NodeExprVarDef]types.Token
	conditions map[*types.NodeExprVarDef]*types.NodeExprVarDef
	errorFacts map[*types.NodeExprVarDef]int8 // 1 = OK, -1 = non-OK
	scopes     []deferScope
	terminated bool
	ranges     map[rangeRelation]*types.RangeProof
	// provenance is compiler-only metadata for pointer and stack-backed slice
	// values. Its key is the place holding the value, not the pointed-to place.
	provenance map[placeKey]pointerProvenance
	// retentions records source storage which may remain borrowed by an owned
	// completion-bearing handle. Unlike an ordinary loan, the dependency lasts
	// until the handle is consumed by its destructor (join/await/remove/close).
	retentions map[placeKey]pointerProvenance
}

type pointerProvenance struct {
	sources    []place.Place
	stackSlice bool
	unknown    bool
}

type rangeRelation struct {
	lower, upper string
	strict       bool
}

type placeKey struct {
	root *types.NodeExprVarDef
	path string
}

type deferScope struct {
	locals   map[*types.NodeExprVarDef]bool
	deferred []*types.NodeStmtDefer
}

type analyzer struct {
	shared              *types.SharedState
	file                *types.FileCtx
	diagnostics         []Diagnostic
	seen                map[string]bool
	loopBreaks          [][]flow
	loopNext            [][]flow
	loopDepths          []int
	nextProof           uint64
	returnOrigins       map[*types.NodeFuncDef][]int
	consumePtrOrigins   map[*types.NodeFuncDef][]int
	futureUses          map[*types.NodeExprVarDef]bool
	unsafeDepth         int
	destructorReceivers map[*types.NodeExprVarDef]bool
	staticExtents       map[*types.NodeExprVarDef]uint64
}

func cloneFlow(in flow) flow {
	out := flow{states: map[*types.NodeExprVarDef]State{}, absent: map[placeKey]types.Token{}, deferred: map[*types.NodeExprVarDef]bool{}, consumedAt: map[*types.NodeExprVarDef]types.Token{}, deferredAt: map[*types.NodeExprVarDef]types.Token{}, conditions: map[*types.NodeExprVarDef]*types.NodeExprVarDef{}, errorFacts: map[*types.NodeExprVarDef]int8{}, ranges: map[rangeRelation]*types.RangeProof{}, provenance: map[placeKey]pointerProvenance{}, retentions: map[placeKey]pointerProvenance{}, terminated: in.terminated}
	for variable, state := range in.states {
		out.states[variable] = state
	}
	for key, token := range in.absent {
		out.absent[key] = token
	}
	for variable, pending := range in.deferred {
		out.deferred[variable] = pending
	}
	for variable, token := range in.consumedAt {
		out.consumedAt[variable] = token
	}
	for variable, token := range in.deferredAt {
		out.deferredAt[variable] = token
	}
	for variable, condition := range in.conditions {
		out.conditions[variable] = condition
	}
	for variable, fact := range in.errorFacts {
		out.errorFacts[variable] = fact
	}
	for relation, proof := range in.ranges {
		out.ranges[relation] = proof
	}
	for key, provenance := range in.provenance {
		copy := provenance
		copy.sources = append([]place.Place(nil), provenance.sources...)
		out.provenance[key] = copy
	}
	for key, retention := range in.retentions {
		copy := retention
		copy.sources = append([]place.Place(nil), retention.sources...)
		out.retentions[key] = copy
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
	resolved, err := place.FromExpr(expr)
	if err != nil || len(resolved.Projections) != 0 {
		return nil
	}
	return resolved.Root
}

func resolvedPlace(expr types.NodeExpr) (place.Place, bool) {
	resolved, err := place.FromExpr(expr)
	return resolved, err == nil
}

func keyFor(resolved place.Place) placeKey {
	var path strings.Builder
	for _, projection := range resolved.Projections {
		switch projection.Kind {
		case place.Field:
			fmt.Fprintf(&path, "/f:%p:%d", projection.FieldOwner, projection.FieldIndex)
		case place.ConstantIndex:
			fmt.Fprintf(&path, "/i:%d", projection.Index)
		case place.DynamicIndex:
			fmt.Fprintf(&path, "/x:%p", projection.DynamicExpr)
		case place.Dereference:
			path.WriteString("/d")
		}
	}
	return placeKey{root: resolved.Root, path: path.String()}
}

func placeName(resolved place.Place) string {
	name := variableName(resolved.Root)
	for _, projection := range resolved.Projections {
		switch projection.Kind {
		case place.Field:
			field := fmt.Sprintf("#%d", projection.FieldIndex)
			if projection.FieldOwner != nil && projection.FieldIndex >= 0 && projection.FieldIndex < len(projection.FieldOwner.FieldOrder) {
				field = projection.FieldOwner.FieldOrder[projection.FieldIndex]
			}
			name += "." + field
		case place.ConstantIndex:
			name += fmt.Sprintf("[%d]", projection.Index)
		case place.DynamicIndex:
			name += "[index]"
		case place.Dereference:
			name = "*" + name
		}
	}
	return name
}

func hasUnsupportedMoveProjection(resolved place.Place) bool {
	for _, projection := range resolved.Projections {
		if projection.Kind == place.DynamicIndex || projection.Kind == place.ConstantIndex || projection.Kind == place.Dereference {
			return true
		}
	}
	return false
}

func (a *analyzer) unsupportedMoveProjection(resolved place.Place) bool {
	if a.destructorReceivers[resolved.Root] {
		for _, projection := range resolved.Projections {
			if projection.Kind == place.DynamicIndex || projection.Kind == place.ConstantIndex {
				return true
			}
		}
		// The implicit destructor receiver is exclusively owned. Its leading
		// dereference and field projections therefore remain canonical places.
		return false
	}
	return hasUnsupportedMoveProjection(resolved)
}

func pathContains(parent, child string) bool {
	return parent == child || (len(child) > len(parent) && strings.HasPrefix(child, parent+"/"))
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

func expressionToken(expr types.NodeExpr) types.Token {
	switch node := expr.(type) {
	case *types.NodeExprName:
		return node.Tk
	case *types.NodeExprMove:
		return node.Tk
	case *types.NodeExprCall:
		return node.Tk
	case *types.NodeExprMemberAccess:
		return node.Tk
	case *types.NodeExprSubscript:
		return node.Tk
	case *types.NodeExprTry:
		return expressionToken(node.Call)
	case *types.NodeExprUnary:
		return node.Tk
	case *types.NodeExprAddrof:
		return expressionToken(node.Expr)
	case *types.NodeExprProtoView:
		return node.Tk
	}
	return types.Token{}
}

func isPointerType(node *types.NodeType) bool {
	if node == nil {
		return false
	}
	if _, ok := node.KindNode.(*types.NodeTypePointer); ok {
		return true
	}
	if named, ok := node.KindNode.(*types.NodeTypeNamed); ok {
		if single, ok := named.NameNode.(*types.NodeNameSingle); ok {
			return single.Name == "ptr"
		}
	}
	return false
}

func isOpaquePointerType(node *types.NodeType) bool {
	if node == nil {
		return false
	}
	named, ok := node.KindNode.(*types.NodeTypeNamed)
	if !ok {
		return false
	}
	single, ok := named.NameNode.(*types.NodeNameSingle)
	return ok && single.Name == "ptr"
}

func isSliceType(node *types.NodeType) bool {
	if node == nil {
		return false
	}
	_, ok := node.KindNode.(*types.NodeTypeSlice)
	return ok
}

func mergeProvenance(left, right pointerProvenance) pointerProvenance {
	out := pointerProvenance{stackSlice: left.stackSlice || right.stackSlice, unknown: left.unknown || right.unknown}
	for _, source := range append(append([]place.Place{}, left.sources...), right.sources...) {
		seen := false
		for _, old := range out.sources {
			if old.Equal(source) {
				seen = true
				break
			}
		}
		if !seen {
			out.sources = append(out.sources, source)
		}
	}
	return out
}

func (a *analyzer) provenanceForExpr(out *flow, expr types.NodeExpr) (pointerProvenance, bool) {
	switch node := expr.(type) {
	case *types.NodeExprTry:
		return a.provenanceForExpr(out, node.Call)
	case *types.NodeExprMove:
		return a.provenanceForExpr(out, node.Expr)
	case *types.NodeExprAddrof:
		if source, ok := resolvedPlace(node.Expr); ok {
			// Addressing an element of a slice points into the slice's backing
			// storage, not into the local slice descriptor itself.
			if len(source.Projections) != 0 && isSliceType(source.Root.Type) {
				if provenance, exists := out.provenance[keyFor(place.Place{Root: source.Root})]; exists {
					return provenance, true
				}
			}
			return pointerProvenance{sources: []place.Place{source}}, true
		}
		return pointerProvenance{unknown: true}, true
	case *types.NodeExprArray:
		return pointerProvenance{stackSlice: true}, true
	case *types.NodeExprName, *types.NodeExprMemberAccess:
		if holder, ok := resolvedPlace(expr); ok {
			provenance, exists := out.provenance[keyFor(holder)]
			return provenance, exists
		}
	case *types.NodeExprCall:
		if !isPointerType(node.InfType) && !isSliceType(node.InfType) {
			return pointerProvenance{}, false
		}
		if origins := a.returnOrigins[node.AssociatedFnDef]; len(origins) != 0 {
			var result pointerProvenance
			for _, index := range origins {
				if index >= 0 && index < len(node.Args) {
					if source, ok := a.provenanceForExpr(out, node.Args[index]); ok {
						result = mergeProvenance(result, source)
					}
				}
			}
			if len(result.sources) != 0 || result.stackSlice || result.unknown {
				return result, true
			}
		}
		// Native pointer results are opaque. The call-only FFI default explicitly
		// does not imply that a result aliases or retains any pointer argument.
		if node.AssociatedFnDef != nil && node.AssociatedFnDef.IsExternal {
			return pointerProvenance{unknown: true}, true
		}
		// If a visible implementation constructs the result through unsafe IR or
		// another opaque helper, its exact return origin is not expressible in the
		// AST. Conservatively retain every pointer/view input. This is also the
		// contract for indirect calls and keeps safe view wrappers source-aware.
		var result pointerProvenance
		ownerExpr := node.MemberOwnerExpr
		if ownerExpr == nil && node.MemberOwnerName != nil {
			ownerExpr = node.MemberOwnerName
		}
		if ownerExpr != nil {
			if source, ok := a.provenanceForExpr(out, ownerExpr); ok {
				result = mergeProvenance(result, source)
			} else if rangeContainerKey(node) != "" {
				if owner, resolved := resolvedPlace(ownerExpr); resolved {
					result.sources = append(result.sources, owner)
				}
			}
		}
		for _, argument := range node.Args {
			if !isPointerType(argument.GetInferredType()) && !isSliceType(argument.GetInferredType()) {
				continue
			}
			if source, ok := a.provenanceForExpr(out, argument); ok {
				result = mergeProvenance(result, source)
			}
		}
		if len(result.sources) != 0 || result.stackSlice || result.unknown {
			return result, true
		}
		// External and opaque calls may fabricate a pointer. Stage 8 decides
		// whether dereferencing such a value requires an unsafe block.
		return pointerProvenance{unknown: true}, true
	}
	return pointerProvenance{}, false
}

func (a *analyzer) setProvenance(out *flow, destination place.Place, value types.NodeExpr) {
	key := keyFor(destination)
	provenance, hasProvenance := a.provenanceForExpr(out, value)
	var nullLiteral bool
	if literal, ok := value.(*types.NodeExprLit); ok {
		nullLiteral = literal.LitType == types.TokLitNone
	}
	if len(destination.Projections) == 0 && isOpaquePointerType(destination.Root.Type) {
		valueType := value.GetInferredType()
		var typedPointer bool
		if valueType != nil {
			_, typedPointer = valueType.KindNode.(*types.NodeTypePointer)
		}
		if typedPointer && a.unsafeDepth == 0 {
			a.safetyError(expressionToken(value), "conversion from a typed pointer to ptr requires an unsafe block")
		}
	}
	// Converting the representation-free `ptr` value into a typed pointer
	// fabricates provenance which the compiler cannot validate.
	if !hasProvenance && !nullLiteral && isOpaquePointerType(value.GetInferredType()) {
		provenance = pointerProvenance{unknown: true}
		hasProvenance = true
		if a.unsafeDepth == 0 {
			a.safetyError(expressionToken(value), fmt.Sprintf("conversion from ptr to typed pointer '%s' requires an unsafe block", placeName(destination)))
		}
	}
	if _, stackArray := value.(*types.NodeExprArray); stackArray {
		provenance.sources = append(provenance.sources, destination)
	}
	for old := range out.provenance {
		if old.root == key.root && pathContains(key.path, old.path) {
			delete(out.provenance, old)
		}
	}
	if hasProvenance {
		out.provenance[key] = provenance
	}
}

// retentionForExpr returns the lifetime dependency carried by an owned handle.
// A freshly returned owned value is completion-bearing when its construction
// receives a pointer or stack-backed view. This deliberately keeps the
// contract on the safe Magma wrapper: opaque calls themselves remain call-only.
func (a *analyzer) retentionForExpr(out *flow, expr types.NodeExpr) (pointerProvenance, bool) {
	switch node := expr.(type) {
	case *types.NodeExprMove:
		return a.retentionForExpr(out, node.Expr)
	case *types.NodeExprTry:
		return a.retentionForExpr(out, node.Call)
	case *types.NodeExprName, *types.NodeExprMemberAccess:
		if holder, ok := resolvedPlace(expr); ok {
			retention, exists := out.retentions[keyFor(holder)]
			return retention, exists
		}
	case *types.NodeExprCall:
		if node.AssociatedFnDef != nil && node.AssociatedFnDef.IsExternal {
			return pointerProvenance{}, false
		}
		if node.AssociatedFnDef != nil && node.AssociatedFnDef.NoRetain {
			return pointerProvenance{}, false
		}
		if node.InfType == nil || !node.InfType.Owned || !a.destructible(node.InfType) {
			return pointerProvenance{}, false
		}
		var result pointerProvenance
		// A consuming member may transfer a completion-bearing handle into its
		// owned result (for example Listener.runAsync -> RunningListener). Carry
		// the receiver's lifetime dependency to that result before call analysis
		// consumes and clears the source place.
		if node.IsMemberFunc {
			var receiver types.NodeExpr
			if node.MemberOwnerExpr != nil {
				receiver = node.MemberOwnerExpr
			} else if node.MemberOwnerName != nil {
				receiver = node.MemberOwnerName
			}
			if receiver != nil {
				if source, ok := a.retentionForExpr(out, receiver); ok {
					result = mergeProvenance(result, source)
				}
			}
		}
		for _, argument := range node.Args {
			if !isPointerType(argument.GetInferredType()) && !isSliceType(argument.GetInferredType()) {
				continue
			}
			if source, ok := a.provenanceForExpr(out, argument); ok {
				result = mergeProvenance(result, source)
			}
		}
		if len(result.sources) != 0 || result.stackSlice {
			return result, true
		}
	}
	return pointerProvenance{}, false
}

func setRetention(out *flow, destination place.Place, retention pointerProvenance, exists bool) {
	key := keyFor(destination)
	for old := range out.retentions {
		if old.root == key.root && pathContains(key.path, old.path) {
			delete(out.retentions, old)
		}
	}
	if exists {
		out.retentions[key] = retention
	}
}

func clearRetention(out *flow, owner place.Place) {
	ownerKey := keyFor(owner)
	for key := range out.retentions {
		if key.root == ownerKey.root && (pathContains(key.path, ownerKey.path) || pathContains(ownerKey.path, key.path)) {
			delete(out.retentions, key)
		}
	}
}

func activeRetention(out *flow, holder placeKey) bool {
	state, tracked := out.states[holder.root]
	return !tracked || state == stateLive || state == stateMaybeConsumed || state == stateConditional
}

func (a *analyzer) checkRetentionReplacement(out *flow, destination place.Place, token types.Token) {
	key := keyFor(destination)
	if retention, exists := out.retentions[key]; exists && activeRetention(out, key) && (len(retention.sources) != 0 || retention.stackSlice) {
		a.safetyError(token, fmt.Sprintf("assignment discards completion-bearing handle '%s' before completion", placeName(destination)))
	}
}

func (a *analyzer) validateDereference(out *flow, unary *types.NodeExprUnary) {
	if unary.Operator != types.KwAsterisk {
		return
	}
	unary.ProvenanceChecked = true
	provenance, ok := a.provenanceForExpr(out, unary.Operand)
	if !ok {
		// A typed pointer parameter is a call-duration validity contract. Values
		// fabricated from `ptr` and opaque results carry explicit unknown metadata.
		return
	}
	if provenance.unknown {
		if a.unsafeDepth == 0 {
			a.safetyError(unary.Tk, "dereference with unknown pointer provenance requires an unsafe block")
		}
		return
	}
	for _, source := range provenance.sources {
		if _, absent := a.absentOrigin(out, source); absent {
			a.safetyError(unary.Tk, fmt.Sprintf("dereference uses expired provenance from '%s'", placeName(source)))
		}
	}
}

func (a *analyzer) validateLvalueDereferences(out *flow, expr types.NodeExpr) {
	switch node := expr.(type) {
	case *types.NodeExprUnary:
		a.validateDereference(out, node)
		a.validateLvalueDereferences(out, node.Operand)
	case *types.NodeExprMemberAccess:
		a.validateLvalueDereferences(out, node.Target)
	case *types.NodeExprSubscript:
		a.validateLvalueDereferences(out, node.Target)
	}
}

func (a *analyzer) checkLiveLoans(out *flow, changed place.Place, token types.Token, action string) {
	changedPlaces := []place.Place{changed}
	var mutationHolder *placeKey
	if count := len(changed.Projections); count != 0 {
		last := changed.Projections[count-1]
		if last.Kind == place.ConstantIndex || last.Kind == place.DynamicIndex {
			holder := changed
			holder.Projections = append([]place.Projection(nil), changed.Projections[:count-1]...)
			holderKey := keyFor(holder)
			mutationHolder = &holderKey
		}
	}
	for index, projection := range changed.Projections {
		if projection.Kind != place.Dereference {
			continue
		}
		holder := changed
		holder.Projections = append([]place.Projection(nil), changed.Projections[:index]...)
		holderKey := keyFor(holder)
		mutationHolder = &holderKey
		if provenance, ok := out.provenance[holderKey]; ok && len(provenance.sources) != 0 {
			changedPlaces = nil
			for _, source := range provenance.sources {
				expanded := source
				expanded.Projections = append(append([]place.Projection(nil), source.Projections...), changed.Projections[index+1:]...)
				changedPlaces = append(changedPlaces, expanded)
			}
		}
		break
	}
	for holder, provenance := range out.provenance {
		// Access through the pointer which created a loan is the purpose of the
		// loan, not a competing mutation. Other live aliases are still checked.
		if mutationHolder != nil && holder == *mutationHolder {
			continue
		}
		changedKey := keyFor(changed)
		if holder.root == changedKey.root && pathContains(changedKey.path, holder.path) {
			// A view or pointer may be used to initialize its own source storage;
			// that originating holder is not a competing alias.
			continue
		}
		if holder.root == nil || !a.futureUses[holder.root] {
			continue
		}
		for _, source := range provenance.sources {
			for _, actual := range changedPlaces {
				if source.Root == actual.Root && source.Overlaps(actual) {
					a.safetyError(token, fmt.Sprintf("cannot %s '%s' while a pointer to it remains live", action, placeName(actual)))
					break
				}
			}
		}
	}
}

func literalUint(expr types.NodeExpr) (uint64, bool) {
	literal, ok := expr.(*types.NodeExprLit)
	if !ok || literal.LitType != types.TokLitNum || strings.HasPrefix(literal.Value, "-") {
		return 0, false
	}
	value, err := strconv.ParseUint(literal.Value, 0, 64)
	return value, err == nil
}

func (a *analyzer) diagnostic(token types.Token, message string, safety bool) {
	a.diagnosticRelated(token, message, safety, nil)
}

func (a *analyzer) diagnosticRelated(token types.Token, message string, safety bool, related []types.DiagnosticRelated) {
	key := fmt.Sprintf("%s\x00%d\x00%d\x00%t\x00%s", a.file.FilePath, token.Pos.Line, token.Pos.Col, safety, message)
	if a.seen[key] {
		return
	}
	a.seen[key] = true
	a.diagnostics = append(a.diagnostics, Diagnostic{
		FilePath: a.file.FilePath,
		Line:     token.Pos.Line,
		Column:   token.Pos.Col,
		Token:    token,
		Message:  message,
		Safety:   safety,
		Code:     diagnosticCode(message, safety),
		Related:  related,
	})
}

func diagnosticCode(message string, safety bool) string {
	if !safety {
		return "cleanup"
	}
	switch {
	case strings.Contains(message, "requires 'move'"):
		return "missing-move"
	case strings.Contains(message, "not proven in range") || strings.Contains(message, "subscript index"):
		return "unproven-bounds"
	case strings.Contains(message, "used after") || strings.Contains(message, "use after"):
		return "use-after-move"
	case strings.Contains(message, "more than once"):
		return "double-consume"
	case strings.Contains(message, "unsafe"):
		return "unsafe-required"
	case strings.Contains(message, "pointer") || strings.Contains(message, "provenance") || strings.Contains(message, "escape"):
		return "invalid-provenance"
	default:
		return "memory-safety"
	}
}

func (a *analyzer) safetyErrorRelated(token types.Token, message string, prior types.Token, priorMessage string) {
	related := []types.DiagnosticRelated(nil)
	if prior.Pos.Line != 0 {
		related = []types.DiagnosticRelated{{FilePath: a.file.FilePath, Token: prior, Message: priorMessage}}
	}
	a.diagnosticRelated(token, message, true, related)
}

func rangeExprKey(expr types.NodeExpr) string {
	switch node := expr.(type) {
	case *types.NodeExprName:
		if variable := directVariable(node); variable != nil {
			if variable.IsConst {
				if value, ok := literalUint(variable.Initializer); ok {
					return fmt.Sprintf("c:%d", value)
				}
			}
			return fmt.Sprintf("v:%p", variable)
		}
	case *types.NodeExprLit:
		return "c:" + node.Value
	case *types.NodeExprBinary:
		if node.Operator == types.KwPlus || node.Operator == types.KwMinus || node.Operator == types.KwAsterisk {
			left, right := rangeExprKey(node.Left), rangeExprKey(node.Right)
			if left != "" && right != "" {
				return fmt.Sprintf("e:%d:(%s):(%s)", node.Operator, left, right)
			}
		}
	case *types.NodeExprCall:
		if node.AssociatedFnDef != nil && strings.HasPrefix(node.AssociatedFnDef.AbsName, "slices_") && strings.HasSuffix(node.AssociatedFnDef.AbsName, ".count") && len(node.Args) == 1 {
			if owner, ok := resolvedPlace(node.Args[0]); ok {
				return fmt.Sprintf("n:%p:%s", owner.Root, keyFor(owner).path)
			}
		}
		ownerExpr := node.MemberOwnerExpr
		if ownerExpr == nil && node.MemberOwnerName != nil {
			ownerExpr = node.MemberOwnerName
		}
		if member, ok := node.Callee.(*types.NodeExprMemberAccess); ok && ownerExpr == nil {
			ownerExpr = member.Target
		}
		if ownerExpr != nil {
			name := ""
			if member, ok := node.Callee.(*types.NodeExprMemberAccess); ok {
				name = member.Member
			}
			if name == "" && node.AssociatedFnDef != nil {
				switch named := node.AssociatedFnDef.Class.NameNode.(type) {
				case *types.NodeNameSingle:
					name = named.Name
				case *types.NodeNameComposite:
					if len(named.Parts) != 0 {
						name = named.Parts[len(named.Parts)-1]
					}
				}
			}
			if name == "count" || name == "Count" {
				if owner, ok := resolvedPlace(ownerExpr); ok {
					return fmt.Sprintf("n:%p:%s", owner.Root, keyFor(owner).path)
				}
			}
		}
	}
	return ""
}

// rangeContainerKey identifies the storage whose logical count bounds an
// ordinary subscript. Safe view-producing member calls retain the identity of
// their receiver even though the call expression itself is not a place.
func rangeContainerKey(expr types.NodeExpr) string {
	if owner, ok := resolvedPlace(expr); ok {
		return fmt.Sprintf("n:%p:%s", owner.Root, keyFor(owner).path)
	}
	call, ok := expr.(*types.NodeExprCall)
	if !ok {
		return ""
	}
	ownerExpr := call.MemberOwnerExpr
	if ownerExpr == nil && call.MemberOwnerName != nil {
		ownerExpr = call.MemberOwnerName
	}
	name := ""
	if member, memberCall := call.Callee.(*types.NodeExprMemberAccess); memberCall {
		if ownerExpr == nil {
			ownerExpr = member.Target
		}
		name = member.Member
	}
	if name == "" && call.AssociatedFnDef != nil {
		switch named := call.AssociatedFnDef.Class.NameNode.(type) {
		case *types.NodeNameSingle:
			name = named.Name
		case *types.NodeNameComposite:
			if len(named.Parts) != 0 {
				name = named.Parts[len(named.Parts)-1]
			}
		}
	}
	if name != "view" && name != "View" {
		return ""
	}
	if ownerExpr == nil {
		return ""
	}
	if owner, ok := resolvedPlace(ownerExpr); ok {
		return fmt.Sprintf("n:%p:%s", owner.Root, keyFor(owner).path)
	}
	return ""
}

func (a *analyzer) newRangeProof(guarded bool) *types.RangeProof {
	a.nextProof++
	return &types.RangeProof{ID: a.nextProof, Guarded: guarded}
}

func (a *analyzer) addRangePredicate(out *flow, expr types.NodeExpr, truth, guarded bool) *types.RangeProof {
	binary, ok := expr.(*types.NodeExprBinary)
	if !ok {
		return nil
	}
	left, right := rangeExprKey(binary.Left), rangeExprKey(binary.Right)
	if left == "" || right == "" {
		return nil
	}
	// For unsigned values, x != 0 is exactly the lower-bound fact 0 < x.
	// This is what makes a terminating empty-container guard establish that
	// count-1 cannot underflow in the continuation.
	neq := (truth && binary.Operator == types.KwCmpNeq) || (!truth && binary.Operator == types.KwCmpEq)
	if neq {
		if value, zero := literalUint(binary.Left); zero && value == 0 && !signedInteger(binary.Right.GetInferredType()) {
			proof := a.newRangeProof(guarded)
			out.ranges[rangeRelation{lower: left, upper: right, strict: true}] = proof
			return proof
		}
		if value, zero := literalUint(binary.Right); zero && value == 0 && !signedInteger(binary.Left.GetInferredType()) {
			proof := a.newRangeProof(guarded)
			out.ranges[rangeRelation{lower: right, upper: left, strict: true}] = proof
			return proof
		}
	}
	strict := false
	equal := false
	switch {
	case truth && binary.Operator == types.KwCmpLt:
		strict = true
	case truth && binary.Operator == types.KwCmpLtEq:
	case truth && binary.Operator == types.KwCmpGt:
		left, right, strict = right, left, true
	case truth && binary.Operator == types.KwCmpGtEq:
		left, right = right, left
	case truth && binary.Operator == types.KwCmpEq:
		equal = true
	case !truth && binary.Operator == types.KwCmpLt:
		left, right = right, left
	case !truth && binary.Operator == types.KwCmpLtEq:
		left, right, strict = right, left, true
	case !truth && binary.Operator == types.KwCmpGt:
	case !truth && binary.Operator == types.KwCmpGtEq:
		strict = true
	case !truth && binary.Operator == types.KwCmpNeq:
		equal = true
	default:
		return nil
	}
	if out.ranges == nil {
		out.ranges = map[rangeRelation]*types.RangeProof{}
	}
	proof := a.newRangeProof(guarded)
	out.ranges[rangeRelation{lower: left, upper: right, strict: strict}] = proof
	if equal {
		out.ranges[rangeRelation{lower: right, upper: left}] = proof
	}
	return proof
}

func (a *analyzer) addRangePredicates(out *flow, expr types.NodeExpr, truth, guarded bool) *types.RangeProof {
	if binary, ok := expr.(*types.NodeExprBinary); ok {
		// Both operands are known only on the non-short-circuiting outcomes:
		// true for conjunction and false for disjunction.
		if (binary.Operator == types.KwAndAnd && truth) || (binary.Operator == types.KwOrOr && !truth) {
			proof := a.addRangePredicates(out, binary.Left, truth, guarded)
			if right := a.addRangePredicates(out, binary.Right, truth, guarded); right != nil {
				proof = right
			}
			return proof
		}
	}
	return a.addRangePredicate(out, expr, truth, guarded)
}

func rangeProof(out *flow, lower, upper string, requireStrict bool) *types.RangeProof {
	if lower == "" || upper == "" {
		return nil
	}
	type step struct {
		key    string
		strict bool
		proof  *types.RangeProof
	}
	queue := []step{{key: lower}}
	seen := map[string]bool{}
	for len(queue) != 0 {
		current := queue[0]
		queue = queue[1:]
		marker := fmt.Sprintf("%s:%t", current.key, current.strict)
		if seen[marker] {
			continue
		}
		seen[marker] = true
		if current.key == upper && (current.strict || !requireStrict) {
			return current.proof
		}
		for relation, proof := range out.ranges {
			intrinsicStrict := false
			if relation.lower != current.key {
				currentValue, currentConst := rangeKeyUint(current.key)
				relationValue, relationConst := rangeKeyUint(relation.lower)
				if !currentConst || !relationConst || currentValue > relationValue {
					continue
				}
				intrinsicStrict = currentValue < relationValue
			}
			selected := current.proof
			if selected == nil || proof.Guarded {
				selected = proof
			}
			queue = append(queue, step{key: relation.upper, strict: current.strict || intrinsicStrict || relation.strict, proof: selected})
		}
	}
	return nil
}

func rangeKeyUint(key string) (uint64, bool) {
	if !strings.HasPrefix(key, "c:") {
		return 0, false
	}
	value, err := strconv.ParseUint(strings.TrimPrefix(key, "c:"), 0, 64)
	return value, err == nil
}

func signedInteger(node *types.NodeType) bool {
	if node == nil {
		return false
	}
	name := ""
	if named, ok := node.KindNode.(*types.NodeTypeNamed); ok {
		if single, ok := named.NameNode.(*types.NodeNameSingle); ok {
			name = single.Name
		}
	}
	desc, ok := magmatypes.NumberTypes[name]
	return ok && desc.IsSigned
}

func errorValueType(node *types.NodeType) bool {
	if node == nil {
		return false
	}
	switch kind := node.KindNode.(type) {
	case *types.NodeTypeNamed:
		if single, ok := kind.NameNode.(*types.NodeNameSingle); ok {
			return single.Name == "error"
		}
	case *types.NodeTypeAbsolute:
		return kind.AbsoluteName == "error" || kind.DisplayName == "error" || strings.HasSuffix(kind.AbsoluteName, ".error")
	case *types.NodeTypeCompilerKnown:
		return kind.Name == "error"
	}
	return false
}

func knownFailingError(expr types.NodeExpr) bool {
	call, ok := expr.(*types.NodeExprCall)
	if !ok || call.AssociatedFnDef == nil {
		return false
	}
	name := call.AssociatedFnDef.AbsName
	return strings.HasPrefix(name, "errors_") && !strings.HasSuffix(name, ".ok")
}

func knownNonNegative(expr types.NodeExpr) bool {
	literal, ok := expr.(*types.NodeExprLit)
	return ok && literal.LitType == types.TokLitNum && !strings.HasPrefix(literal.Value, "-")
}

func invalidateVariableRanges(out *flow, variable *types.NodeExprVarDef) {
	valueKey := fmt.Sprintf("v:%p", variable)
	countPrefix := fmt.Sprintf("n:%p:", variable)
	for relation := range out.ranges {
		if relation.lower == valueKey || relation.upper == valueKey ||
			strings.HasPrefix(relation.lower, countPrefix) || strings.HasPrefix(relation.upper, countPrefix) {
			delete(out.ranges, relation)
		}
	}
}

func (a *analyzer) authorizeSubscript(out *flow, node *types.NodeExprSubscript) {
	node.RangeProof = nil
	if node.BoxType == nil {
		return
	}
	if a.unsafeDepth > 0 {
		// Unsafe is the explicit source-level authorization for an access whose
		// range cannot be proven. Preserve that authorization as compiler-only
		// evidence so lowering cannot accidentally reject or invent it later.
		node.RangeProof = a.newRangeProof(false)
		return
	}
	switch node.BoxType.KindNode.(type) {
	case *types.NodeTypePointer:
		a.safetyError(node.Tk, "pointer subscript has no proven extent and requires an unsafe block")
		return
	}
	upper := rangeContainerKey(node.Target)
	if upper == "" {
		a.safetyError(node.Tk, "ordinary subscript has no stable container for a range proof")
		return
	}
	if index, constant := literalUint(node.Expr); constant {
		if target, ok := resolvedPlace(node.Target); ok {
			if extent, fixed := a.staticExtents[target.Root]; fixed && index < extent {
				node.RangeProof = a.newRangeProof(false)
				return
			}
		}
	}
	lower := rangeExprKey(node.Expr)
	// If base is positive, base-1 is strictly below every upper bound which is
	// at least base. This common guarded-last-element form is overflow-safe
	// because the positivity proof excludes underflow.
	if binary, ok := node.Expr.(*types.NodeExprBinary); ok && binary.Operator == types.KwMinus {
		if decrement, one := literalUint(binary.Right); one && decrement == 1 {
			base := rangeExprKey(binary.Left)
			if rangeProof(out, "c:0", base, true) != nil && rangeProof(out, base, upper, false) != nil {
				node.RangeProof = a.newRangeProof(false)
				return
			}
		}
	}
	if signedInteger(node.Expr.GetInferredType()) && !knownNonNegative(node.Expr) && rangeProof(out, "c:0", lower, false) == nil {
		a.safetyError(node.Tk, "signed subscript index is not proven non-negative")
		return
	}
	if proof := rangeProof(out, lower, upper, true); proof != nil {
		node.RangeProof = proof
		return
	}
	a.safetyError(node.Tk, "ordinary subscript is not proven in range; use a bounded block or checked-access API")
}

func (a *analyzer) authorizeAddressedSubscript(out *flow, node *types.NodeExprSubscript) bool {
	if node.BoxType == nil {
		return false
	}
	if _, pointer := node.BoxType.KindNode.(*types.NodeTypePointer); pointer {
		return false
	}
	upper := rangeContainerKey(node.Target)
	lower := rangeExprKey(node.Expr)
	if proof := rangeProof(out, lower, upper, false); proof != nil {
		// Address formation permits the one-past-end position. The resulting
		// pointer still cannot be dereferenced there; ordinary subscripting keeps
		// requiring a strict proof.
		node.RangeProof = proof
		a.borrowExpr(out, node.Target)
		a.borrowExpr(out, node.Expr)
		return true
	}
	if index, constant := literalUint(node.Expr); constant && index > 0 {
		predecessor := fmt.Sprintf("c:%d", index-1)
		if proof := rangeProof(out, predecessor, upper, true); proof != nil {
			node.RangeProof = proof
			a.borrowExpr(out, node.Target)
			a.borrowExpr(out, node.Expr)
			return true
		}
	}
	return false
}

func (a *analyzer) warn(token types.Token, message string) {
	a.diagnostic(token, message, false)
}

func (a *analyzer) safetyError(token types.Token, message string) {
	a.diagnostic(token, message, true)
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
	if definition == nil {
		return false
	}
	if len(definition.Destructors) != 0 {
		return true
	}
	for _, field := range definition.Fields {
		// Ownership is meaningful only when the specialized field type has a
		// destruction obligation of its own. Generic containers commonly mark
		// their type-parameter fields as owned so resources move correctly, but
		// a specialization such as Pair[bool, u64] must remain freely disposable.
		if field != nil && field.Owned && a.destructible(field) {
			return true
		}
	}
	return false
}

func (a *analyzer) tracked(out *flow, variable *types.NodeExprVarDef) bool {
	if variable == nil {
		return false
	}
	state, exists := out.states[variable]
	return exists && state != stateBorrowed
}

func (a *analyzer) use(out *flow, variable *types.NodeExprVarDef) {
	a.useAt(out, variable, variableToken(variable))
}

func (a *analyzer) useAt(out *flow, variable *types.NodeExprVarDef, token types.Token) {
	if variable == nil {
		return
	}
	state, exists := out.states[variable]
	if !exists || state == stateBorrowed {
		return
	}
	if state != stateLive {
		message := fmt.Sprintf("destructible value '%s' may be used after ownership was transferred", variableName(variable))
		prior := out.consumedAt[variable]
		if prior.Pos.Line != 0 {
			message += fmt.Sprintf(" (previous consumption at line %d, column %d)", prior.Pos.Line, prior.Pos.Col)
		}
		a.safetyErrorRelated(token, message, prior, "ownership was transferred here")
	}
}

func (a *analyzer) absentOrigin(out *flow, resolved place.Place) (types.Token, bool) {
	key := keyFor(resolved)
	for absent, token := range out.absent {
		if absent.root != key.root {
			continue
		}
		// Using a whole aggregate overlaps every absent descendant. Using a
		// field overlaps an absent ancestor or that exact projected place.
		if key.path == "" || pathContains(absent.path, key.path) || pathContains(key.path, absent.path) {
			return token, true
		}
	}
	return types.Token{}, false
}

func (a *analyzer) usePlace(out *flow, resolved place.Place, token types.Token) {
	a.useAt(out, resolved.Root, token)
	if prior, absent := a.absentOrigin(out, resolved); absent {
		message := fmt.Sprintf("ownership place '%s' may be used after it was moved", placeName(resolved))
		if prior.Pos.Line != 0 {
			message += fmt.Sprintf(" (previous move at line %d, column %d)", prior.Pos.Line, prior.Pos.Col)
		}
		a.safetyErrorRelated(token, message, prior, "ownership place was moved here")
	}
}

func (a *analyzer) allOwnedFieldsAbsent(out *flow, resolved place.Place) bool {
	if len(resolved.Projections) != 0 || resolved.Root == nil || resolved.Root.Type == nil {
		return false
	}
	definition := a.structFor(resolved.Root.Type.KindNode)
	if definition == nil {
		return false
	}
	found := false
	for _, fieldName := range definition.FieldOrder {
		fieldType := definition.Fields[fieldName]
		if !fieldType.Owned && !a.destructible(fieldType) {
			continue
		}
		found = true
		field := resolved
		field.Projections = []place.Projection{{Kind: place.Field, FieldOwner: definition, FieldIndex: definition.FieldNb[fieldName]}}
		if _, absent := out.absent[keyFor(field)]; !absent {
			return false
		}
	}
	return found
}

func (a *analyzer) movePlace(out *flow, resolved place.Place, token types.Token) bool {
	a.checkLiveLoans(out, resolved, token, "move")
	if len(resolved.Projections) == 0 {
		if !a.tracked(out, resolved.Root) {
			a.safetyError(token, fmt.Sprintf("cannot move borrowed or unowned value '%s'", variableName(resolved.Root)))
			return false
		}
		if _, absent := a.absentOrigin(out, resolved); absent {
			a.safetyError(token, fmt.Sprintf("cannot move aggregate '%s' while an ownership-bearing field is absent", placeName(resolved)))
			return false
		}
		a.consumeAt(out, resolved.Root, "explicit move", token)
		return true
	}
	if a.unsupportedMoveProjection(resolved) && a.unsafeDepth == 0 {
		a.safetyError(token, "direct ownership moves through indices or dereferences are not supported; use a checked container operation")
		return false
	}
	if a.unsupportedMoveProjection(resolved) && a.unsafeDepth > 0 {
		// Raw container implementations maintain their own occupancy metadata.
		// Unsafe permits the projected transfer but does not weaken ownership of
		// ordinary named places handled above.
		return true
	}
	if !a.tracked(out, resolved.Root) {
		a.safetyError(token, fmt.Sprintf("cannot move field from borrowed or unowned value '%s'", variableName(resolved.Root)))
		return false
	}
	if prior, absent := a.absentOrigin(out, resolved); absent {
		message := fmt.Sprintf("ownership place '%s' may be moved more than once", placeName(resolved))
		if prior.Pos.Line != 0 {
			message += fmt.Sprintf("; previous move was at line %d, column %d", prior.Pos.Line, prior.Pos.Col)
		}
		a.safetyErrorRelated(token, message, prior, "ownership place was first moved here")
		return false
	}
	if out.absent == nil {
		out.absent = map[placeKey]types.Token{}
	}
	out.absent[keyFor(resolved)] = token
	root := place.Place{Root: resolved.Root}
	if a.allOwnedFieldsAbsent(out, root) {
		out.states[resolved.Root] = stateConsumed
		if out.consumedAt == nil {
			out.consumedAt = map[*types.NodeExprVarDef]types.Token{}
		}
		out.consumedAt[resolved.Root] = token
	}
	return true
}

func (a *analyzer) reinitializePlace(out *flow, resolved place.Place) {
	key := keyFor(resolved)
	for absent := range out.absent {
		if absent.root == key.root && pathContains(key.path, absent.path) {
			delete(out.absent, absent)
		}
	}
	if len(resolved.Projections) == 0 {
		delete(out.consumedAt, resolved.Root)
	} else if out.states[resolved.Root] == stateConsumed {
		out.states[resolved.Root] = stateLive
		delete(out.consumedAt, resolved.Root)
	}
}

func (a *analyzer) consume(out *flow, variable *types.NodeExprVarDef, reason string) {
	a.consumeAt(out, variable, reason, variableToken(variable))
}

func (a *analyzer) consumeAt(out *flow, variable *types.NodeExprVarDef, reason string, token types.Token) {
	if variable == nil || !a.destructible(variable.Type) {
		return
	}
	state, exists := out.states[variable]
	if !exists || state == stateBorrowed {
		a.safetyError(token, fmt.Sprintf("borrowed destructible value '%s' cannot be consumed (%s)", variableName(variable), reason))
		return
	}
	if state != stateLive {
		message := fmt.Sprintf("destructible value '%s' may be consumed more than once (%s)", variableName(variable), reason)
		prior := out.consumedAt[variable]
		if prior.Pos.Line != 0 {
			message += fmt.Sprintf("; previous consumption was at line %d, column %d", prior.Pos.Line, prior.Pos.Col)
		}
		a.safetyErrorRelated(token, message, prior, "value was first consumed here")
		return
	}
	if out.deferred[variable] {
		message := fmt.Sprintf("destructible value '%s' is transferred while a deferred destructor is pending", variableName(variable))
		prior := out.deferredAt[variable]
		if prior.Pos.Line != 0 {
			message += fmt.Sprintf(" (defer scheduled at line %d, column %d)", prior.Pos.Line, prior.Pos.Col)
		}
		a.safetyErrorRelated(token, message, prior, "deferred destruction was scheduled here")
	}
	out.states[variable] = stateConsumed
	clearRetention(out, place.Place{Root: variable})
	if out.consumedAt == nil {
		out.consumedAt = map[*types.NodeExprVarDef]types.Token{}
	}
	out.consumedAt[variable] = token
}

func (a *analyzer) borrowExpr(out *flow, expr types.NodeExpr) {
	switch node := expr.(type) {
	case *types.NodeExprName:
		if resolved, ok := resolvedPlace(node); ok {
			a.usePlace(out, resolved, node.Tk)
		}
	case *types.NodeExprBinary:
		a.borrowExpr(out, node.Left)
		if node.Operator == types.KwAndAnd || node.Operator == types.KwOrOr {
			// The right operand is evaluated only after the left operand has
			// selected the corresponding short-circuit continuation. Retain
			// that predicate while checking accesses in the right operand, but
			// do not leak the conditional fact beyond the expression.
			saved := out.ranges
			out.ranges = cloneFlow(*out).ranges
			a.addRangePredicates(out, node.Left, node.Operator == types.KwAndAnd, false)
			a.borrowExpr(out, node.Right)
			out.ranges = saved
		} else {
			a.borrowExpr(out, node.Right)
		}
	case *types.NodeExprUnary:
		a.validateDereference(out, node)
		a.borrowExpr(out, node.Operand)
	case *types.NodeExprMemberAccess:
		// Validate every projected operation in the target (notably a
		// subscript) before using the combined place. Treating the combined
		// member as a place must not bypass the target's bounds proof.
		a.borrowExpr(out, node.Target)
		if resolved, ok := resolvedPlace(node); ok {
			a.usePlace(out, resolved, node.Tk)
		}
	case *types.NodeExprSubscript:
		a.authorizeSubscript(out, node)
		if resolved, ok := resolvedPlace(node); ok {
			a.usePlace(out, resolved, node.Tk)
		} else {
			a.borrowExpr(out, node.Target)
		}
		a.borrowExpr(out, node.Expr)
	case *types.NodeExprAddrof:
		// Address-taking still requires the indexed place itself to be proven.
		if indexed, ok := node.Expr.(*types.NodeExprSubscript); ok && a.authorizeAddressedSubscript(out, indexed) {
			return
		}
		a.borrowExpr(out, node.Expr)
	case *types.NodeExprMove:
		// A move is meaningful only in an ownership-transfer position.
		a.safetyError(node.Tk, "move expression is only valid where ownership is transferred")
		a.borrowExpr(out, node.Expr)
	case *types.NodeExprCall:
		a.call(out, node)
	case *types.NodeExprTry:
		a.borrowExpr(out, node.Call)
		a.checkTryFailure(out)
	case *types.NodeExprStructInit:
		a.transferStructFields(out, node)
	case *types.NodeExprProtoView:
		a.borrowExpr(out, node.Target)
	}
}

// A struct constructor is an ownership boundary for its fields. Tracked local
// values placed into the aggregate move into it; borrowed values remain borrows.
func (a *analyzer) transferStructFields(out *flow, init *types.NodeExprStructInit) {
	for _, field := range init.Fields {
		a.transferValue(out, field.Expression)
	}
}

func (a *analyzer) consumeExpression(out *flow, expr types.NodeExpr, reason string, token types.Token) {
	resolved, ok := resolvedPlace(expr)
	if ok {
		if _, absent := a.absentOrigin(out, resolved); absent {
			a.usePlace(out, resolved, token)
			return
		}
	}
	if !ok || len(resolved.Projections) == 0 {
		a.consumeAt(out, directVariable(expr), reason, token)
		return
	}
	// Validate projected access (including an unsafe-authorized subscript) before
	// recording the ownership transition. Lowering must receive the same range
	// evidence as an ordinary read of the projected place.
	a.borrowExpr(out, expr)
	// Calling a field destructor is the explicit consumption of that field.
	// It uses the same projected state transition as `move field`.
	a.movePlace(out, resolved, token)
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
				a.transferRequired(out, argument, "consuming function-pointer argument")
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
	if definition.IsExternal && call.InfType != nil && call.InfType.Owned && a.destructible(call.InfType) && a.unsafeDepth == 0 {
		for _, argument := range call.Args {
			if isPointerType(argument.GetInferredType()) || isSliceType(argument.GetInferredType()) {
				a.safetyError(call.Tk, "external call returning owned pointer-backed state requires an audited unsafe wrapper")
				break
			}
		}
	}

	if call.IsMemberFunc && call.MemberOwnerExpr != nil {
		if definition.IsDestructor && !definition.IsExternal {
			a.consumeExpression(out, call.MemberOwnerExpr, "destructor call", call.Tk)
			if owner, ok := resolvedPlace(call.MemberOwnerExpr); ok {
				clearRetention(out, owner)
			}
		} else {
			a.borrowExpr(out, call.MemberOwnerExpr)
		}
	} else if call.IsMemberFunc && call.MemberOwnerName != nil {
		if definition.IsDestructor && !definition.IsExternal {
			a.consumeExpression(out, call.MemberOwnerName, "destructor call", call.Tk)
			if owner, ok := resolvedPlace(call.MemberOwnerName); ok {
				clearRetention(out, owner)
			}
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
			a.transferRequired(out, argument, "consuming argument")
		} else {
			a.borrowExpr(out, argument)
		}
	}
	// Visible helpers may wrap a destructor in order to adapt its throwing-void
	// result. Propagate only effects proven to occur in the helper's
	// unconditional leading path; conditional or later calls are deliberately
	// excluded from the summary.
	for _, index := range a.consumePtrOrigins[definition] {
		if index < 0 || index >= len(call.Args) {
			continue
		}
		address, ok := call.Args[index].(*types.NodeExprAddrof)
		if !ok {
			continue
		}
		a.consumeExpression(out, address.Expr, "destructor helper call", call.Tk)
	}
}

func (a *analyzer) transferRequired(out *flow, value types.NodeExpr, reason string) bool {
	if a.transferValue(out, value) {
		return true
	}
	if source := directVariable(value); source != nil && a.destructible(source.Type) {
		a.consumeAt(out, source, reason, expressionToken(value))
		return false
	}
	if inferred := value.GetInferredType(); inferred != nil && a.destructible(inferred) {
		a.safetyError(expressionToken(value), fmt.Sprintf("%s requires an owned value, but this expression is borrowed", reason))
	}
	return false
}

func (a *analyzer) expression(out *flow, expr types.NodeExpr) {
	switch node := expr.(type) {
	case *types.NodeExprVarDef:
		if a.destructible(node.Type) {
			// A declaration without an initializer contains only the type's zero
			// value. It does not acquire ownership until a later owned assignment.
			out.states[node] = stateBorrowed
			a.addLocal(out, node)
		}
	case *types.NodeExprVarDefAssign:
		if array, ok := node.AssignExpr.(*types.NodeExprArray); ok {
			if extent, constant := literalUint(array.Length); constant {
				a.staticExtents[node.VarDef] = extent
				count := fmt.Sprintf("n:%p:", node.VarDef)
				literal := fmt.Sprintf("c:%d", extent)
				proof := a.newRangeProof(false)
				out.ranges[rangeRelation{lower: count, upper: literal}] = proof
				out.ranges[rangeRelation{lower: literal, upper: count}] = proof
			}
		}
		a.valueInto(out, node.VarDef, node.AssignExpr)
		a.addLocal(out, node.VarDef)
	case *types.NodeExprAssign:
		a.assignment(out, node)
	case *types.NodeExprCall:
		a.call(out, node)
		if node.InfType != nil && node.InfType.Owned && a.destructible(node.InfType) {
			a.warn(node.Tk, "owned destructible call result is discarded")
			if retention, ok := a.retentionForExpr(out, node); ok && (len(retention.sources) != 0 || retention.stackSlice) {
				a.safetyError(node.Tk, "completion-bearing handle retaining source storage cannot be discarded")
			}
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
	if len(out.scopes) != 0 {
		out.scopes[len(out.scopes)-1].locals[variable] = true
	}
}

// transferValue evaluates a value used in an ownership-transfer position and
// reports whether ownership was actually produced. Plain-returning calls and
// borrowed locals stay borrowed; owned calls and owned locals transfer.
func (a *analyzer) transferValue(out *flow, value types.NodeExpr) bool {
	switch node := value.(type) {
	case *types.NodeExprMove:
		resolved, ok := resolvedPlace(node.Expr)
		if !ok {
			a.safetyError(node.Tk, "move requires a local ownership place")
			a.borrowExpr(out, node.Expr)
			return false
		}
		if !a.destructible(node.Expr.GetInferredType()) {
			// Explicit movement of a freely copyable value is a semantic no-op.
			// This keeps generic transfer code valid when T specializes to a
			// primitive while retaining the same source for owned T.
			a.borrowExpr(out, node.Expr)
			return false
		}
		return a.movePlace(out, resolved, node.Tk)
	case *types.NodeExprName:
		resolved, ok := resolvedPlace(node)
		if !ok {
			a.borrowExpr(out, value)
			return false
		}
		if a.tracked(out, resolved.Root) && a.destructible(node.InfType) {
			a.safetyError(node.Tk, fmt.Sprintf("ownership transfer of named value '%s' requires 'move'", placeName(resolved)))
			a.movePlace(out, resolved, node.Tk)
			return true
		}
		a.borrowExpr(out, value)
		return false
	case *types.NodeExprMemberAccess:
		resolved, ok := resolvedPlace(node)
		if ok && a.tracked(out, resolved.Root) && a.destructible(node.InfType) {
			a.safetyError(node.Tk, fmt.Sprintf("ownership transfer of field '%s' requires 'move'", placeName(resolved)))
			a.movePlace(out, resolved, node.Tk)
			return true
		}
		a.borrowExpr(out, value)
		return false
	case *types.NodeExprSubscript:
		if node.ElemType != nil && node.ElemType.Owned && a.destructible(node.ElemType) {
			if a.unsafeDepth == 0 {
				a.safetyError(node.Tk, "direct ownership transfer through an index is not supported; use a checked container operation")
			} else {
				a.authorizeSubscript(out, node)
				a.borrowExpr(out, node.Target)
				a.borrowExpr(out, node.Expr)
				return true
			}
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
	delete(out.deferredAt, destination)
	delete(out.consumedAt, destination)
	for key := range out.absent {
		if key.root == destination {
			delete(out.absent, key)
		}
	}
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
	if destination != nil {
		if source := rangeExprKey(value); source != "" {
			destinationKey := fmt.Sprintf("v:%p", destination)
			proof := a.newRangeProof(false)
			out.ranges[rangeRelation{lower: destinationKey, upper: source}] = proof
			out.ranges[rangeRelation{lower: source, upper: destinationKey}] = proof
		}
		if source := rangeContainerKey(value); source != "" && isSliceType(destination.Type) {
			destinationCount := fmt.Sprintf("n:%p:", destination)
			proof := a.newRangeProof(false)
			out.ranges[rangeRelation{lower: destinationCount, upper: source}] = proof
			out.ranges[rangeRelation{lower: source, upper: destinationCount}] = proof
		}
	}
	retention, retained := a.retentionForExpr(out, value)
	if destination != nil {
		a.checkRetentionReplacement(out, place.Place{Root: destination}, variableToken(destination))
	}
	if destination != nil && (isPointerType(destination.Type) || isSliceType(destination.Type)) {
		a.setProvenance(out, place.Place{Root: destination}, value)
	}
	if destination != nil {
		if init, ok := value.(*types.NodeExprStructInit); ok {
			owner := a.structFor(destination.Type.KindNode)
			for _, field := range init.Fields {
				if isPointerType(field.FieldType) || isSliceType(field.FieldType) {
					a.setProvenance(out, place.Place{Root: destination, Projections: []place.Projection{{Kind: place.Field, FieldOwner: owner, FieldIndex: field.FieldIndex}}}, field.Expression)
				}
			}
		}
	}
	owned := a.transferValue(out, value)
	if !owned && destination.Type != nil && destination.Type.Owned && partialValue(value) {
		// Partial moves are intentionally not analysed. An explicit `$T` local
		// may claim a field/indexed value, but calls still derive ownership only
		// from their return annotation.
		owned = true
	}
	a.setDestinationOwnership(out, destination, owned)
	if destination != nil {
		setRetention(out, place.Place{Root: destination}, retention, owned && retained)
	}
}

func (a *analyzer) assignment(out *flow, assignment *types.NodeExprAssign) {
	retention, retained := a.retentionForExpr(out, assignment.Right)
	owned := a.transferValue(out, assignment.Right)
	// Lvalue dereferences are not otherwise visited by borrowExpr.
	a.validateLvalueDereferences(out, assignment.Left)
	if indexed, ok := assignment.Left.(*types.NodeExprSubscript); ok {
		a.authorizeSubscript(out, indexed)
		a.borrowExpr(out, indexed.Target)
		a.borrowExpr(out, indexed.Expr)
	}
	if member, ok := assignment.Left.(*types.NodeExprMemberAccess); ok {
		// Lvalue member projections must validate any subscript/dereference in
		// their target just as rvalue member projections do.
		a.borrowExpr(out, member.Target)
	}

	if destination := directVariable(assignment.Left); destination != nil {
		resolved := place.Place{Root: destination}
		a.checkRetentionReplacement(out, resolved, expressionToken(assignment.Left))
		a.checkLiveLoans(out, resolved, expressionToken(assignment.Left), "mutate")
		if isPointerType(destination.Type) || isSliceType(destination.Type) {
			a.setProvenance(out, resolved, assignment.Right)
		}
		// Rebinding invalidates only relations which depend on that value or its
		// descriptor; unrelated dominating facts remain available.
		invalidateVariableRanges(out, destination)
		a.setDestinationOwnership(out, destination, owned)
		setRetention(out, resolved, retention, owned && retained)
		if resolved, ok := resolvedPlace(assignment.Left); ok {
			a.reinitializePlace(out, resolved)
		}
		return
	}
	if destination, ok := resolvedPlace(assignment.Left); ok {
		a.checkRetentionReplacement(out, destination, expressionToken(assignment.Left))
		a.checkLiveLoans(out, destination, expressionToken(assignment.Left), "mutate")
		if isPointerType(assignment.Left.GetInferredType()) || isSliceType(assignment.Left.GetInferredType()) {
			a.setProvenance(out, destination, assignment.Right)
		}
		if _, indexed := assignment.Left.(*types.NodeExprSubscript); !indexed {
			invalidateVariableRanges(out, destination.Root)
		}
		fieldType := assignment.Left.GetInferredType()
		ownershipStorage := fieldType != nil && fieldType.Owned && a.destructible(fieldType)
		if a.unsupportedMoveProjection(destination) && a.unsafeDepth == 0 {
			if ownershipStorage {
				a.safetyError(expressionToken(assignment.Left), "direct assignment to indexed ownership storage is not supported; use a checked replace operation")
			}
			return
		}
		if ownershipStorage && !owned && !(a.unsafeDepth > 0 && hasUnsupportedMoveProjection(destination)) {
			a.safetyError(expressionToken(assignment.Right), fmt.Sprintf("assignment to ownership field '%s' requires a moved or freshly owned value", placeName(destination)))
			return
		}
		if _, absent := a.absentOrigin(out, destination); !absent && ownershipStorage && a.tracked(out, destination.Root) && !(a.unsafeDepth > 0 && hasUnsupportedMoveProjection(destination)) {
			a.warn(expressionToken(assignment.Left), fmt.Sprintf("assignment overwrites live destructible field '%s'", placeName(destination)))
		}
		a.reinitializePlace(out, destination)
		setRetention(out, destination, retention, owned && retained)
		return
	}
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
	for variable, fact := range out.errorFacts {
		if right.errorFacts[variable] != fact {
			delete(out.errorFacts, variable)
		}
	}
	// A fact remains available after a join only when every reachable branch
	// established the same relation.
	for relation := range out.ranges {
		if _, ok := right.ranges[relation]; !ok {
			delete(out.ranges, relation)
		}
	}
	for key, provenance := range right.provenance {
		if prior, ok := out.provenance[key]; ok {
			out.provenance[key] = mergeProvenance(prior, provenance)
		} else {
			out.provenance[key] = provenance
		}
	}
	for key, retention := range right.retentions {
		if prior, ok := out.retentions[key]; ok {
			out.retentions[key] = mergeProvenance(prior, retention)
		} else {
			out.retentions[key] = retention
		}
	}
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
	for key, token := range right.absent {
		if _, exists := out.absent[key]; !exists {
			out.absent[key] = token
		}
	}
	for variable, token := range right.consumedAt {
		if _, exists := out.consumedAt[variable]; !exists {
			out.consumedAt[variable] = token
		}
	}
	for variable, token := range right.deferredAt {
		if _, exists := out.deferredAt[variable]; !exists {
			out.deferredAt[variable] = token
		}
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
	a.unwindTo(&exit, 0, false)
	for variable, state := range exit.states {
		if (state == stateLive || state == stateMaybeConsumed || state == stateConditional) && !a.destructorReceivers[variable] {
			a.warn(variableToken(variable), fmt.Sprintf("destructible value '%s' is not consumed on every exit path", variableName(variable)))
		}
	}
}

func (a *analyzer) runDeferred(out *flow, deferred *types.NodeStmtDefer) {
	if owner := deferredDestructorOwner(deferred); owner != nil {
		delete(out.deferred, owner)
		delete(out.deferredAt, owner)
	}
	if deferred.IsBody {
		a.body(out, &deferred.Body)
	} else {
		a.expression(out, deferred.Expression)
	}
}

func deferredDestructorOwner(deferred *types.NodeStmtDefer) *types.NodeExprVarDef {
	if deferred == nil || deferred.IsBody || deferred.OnError {
		return nil
	}
	call, ok := deferred.Expression.(*types.NodeExprCall)
	if !ok || call.AssociatedFnDef == nil || !call.AssociatedFnDef.IsDestructor {
		return nil
	}
	if call.MemberOwnerExpr != nil {
		return directVariable(call.MemberOwnerExpr)
	}
	return directVariable(call.MemberOwnerName)
}

func (a *analyzer) unwindScope(out *flow, checkLocals bool, failing bool) {
	index := len(out.scopes) - 1
	scope := out.scopes[index]
	out.scopes = out.scopes[:index]
	for i := len(scope.deferred) - 1; i >= 0; i-- {
		if !scope.deferred[i].OnError || failing {
			a.runDeferred(out, scope.deferred[i])
		}
	}
	// Validate every retained source before deleting any local handle metadata;
	// map iteration order must not decide whether an unfinished operation is
	// diagnosed.
	for holder, retention := range out.retentions {
		if !activeRetention(out, holder) {
			continue
		}
		for _, source := range retention.sources {
			if scope.locals[source.Root] {
				a.safetyError(variableToken(source.Root), fmt.Sprintf("completion-bearing handle must be consumed before retained local place '%s' leaves scope", placeName(source)))
			}
		}
		if retention.stackSlice {
			for _, source := range retention.sources {
				if scope.locals[source.Root] {
					a.safetyError(variableToken(source.Root), "completion-bearing handle must be consumed before its stack-backed slice leaves scope")
					break
				}
			}
		}
	}
	for variable := range scope.locals {
		localPlace := place.Place{Root: variable}
		for holder, provenance := range out.provenance {
			if holder.root == variable || scope.locals[holder.root] {
				continue
			}
			for _, source := range provenance.sources {
				// Conservative alias overlap between distinct dereferenced roots is
				// useful for mutation checks, but it does not make one declaration's
				// lifetime storage belong to every unrelated local declaration.
				if source.Root == variable && source.Overlaps(localPlace) {
					a.safetyError(variableToken(variable), fmt.Sprintf("pointer or stack-backed slice to local place '%s' outlives its source scope", placeName(source)))
					break
				}
			}
		}
		state, tracked := out.states[variable]
		if checkLocals && tracked && (state == stateLive || state == stateMaybeConsumed || state == stateConditional) && !a.destructorReceivers[variable] {
			a.warn(variableToken(variable), fmt.Sprintf("destructible value '%s' is not consumed on every scope exit path", variableName(variable)))
		}
		delete(out.states, variable)
		delete(out.deferred, variable)
		delete(out.deferredAt, variable)
		delete(out.consumedAt, variable)
		for key := range out.absent {
			if key.root == variable {
				delete(out.absent, key)
			}
		}
		for key := range out.provenance {
			if key.root == variable {
				delete(out.provenance, key)
			}
		}
		for key := range out.retentions {
			if key.root == variable {
				delete(out.retentions, key)
			}
		}
		delete(out.conditions, variable)
	}
}

func (a *analyzer) unwindTo(out *flow, depth int, failing bool) {
	for len(out.scopes) > depth {
		a.unwindScope(out, true, failing)
	}
}

func (a *analyzer) unwindTryFailure(out *flow) {
	for len(out.scopes) > 0 {
		a.unwindScope(out, false, true)
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
	}
	return out
}

func predicateFlows(in flow, expr types.NodeExpr) (flow, flow) {
	errVariable, successOnTrue, ok := errorPredicate(expr)
	if !ok {
		return cloneFlow(in), cloneFlow(in)
	}
	truth := refineConditionalOwnership(in, errVariable, successOnTrue)
	falsehood := refineConditionalOwnership(in, errVariable, !successOnTrue)
	truth.errorFacts[errVariable] = -1
	if successOnTrue {
		truth.errorFacts[errVariable] = 1
	}
	falsehood.errorFacts[errVariable] = 1
	if successOnTrue {
		falsehood.errorFacts[errVariable] = -1
	}
	return truth, falsehood
}

func (a *analyzer) conditional(out *flow, statement *types.NodeStmtIf) {
	a.borrowExpr(out, statement.CondExpr)
	branches := []flow{}
	first, remaining := predicateFlows(*out, statement.CondExpr)
	a.addRangePredicates(&first, statement.CondExpr, true, false)
	a.addRangePredicates(&remaining, statement.CondExpr, false, false)
	a.body(&first, &statement.Body)
	branches = append(branches, first)

	next := statement.NextCondStmt
	hasElse := false
	for next != nil {
		switch branch := next.(type) {
		case *types.NodeStmtIf:
			candidate, falseFlow := predicateFlows(remaining, branch.CondExpr)
			a.addRangePredicates(&candidate, branch.CondExpr, true, false)
			a.addRangePredicates(&falseFlow, branch.CondExpr, false, false)
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
		if retention, ok := a.retentionForExpr(out, node.Expression); ok {
			for _, source := range retention.sources {
				for _, scope := range out.scopes {
					if scope.locals[source.Root] {
						a.safetyError(node.Tk, fmt.Sprintf("completion-bearing handle retaining local place '%s' cannot escape its source frame", placeName(source)))
						break
					}
				}
			}
			if retention.stackSlice {
				a.safetyError(node.Tk, "completion-bearing handle retaining a stack-backed slice cannot escape its source frame")
			}
		}
		if provenance, ok := a.provenanceForExpr(out, node.Expression); ok {
			if provenance.stackSlice {
				a.safetyError(node.Tk, "stack-backed slice cannot escape its source frame")
			}
			for _, source := range provenance.sources {
				// Parameters and globals are not declared in a body scope. Any
				// address rooted in a body local expires when this return executes.
				isLocal := false
				for _, scope := range out.scopes {
					if scope.locals[source.Root] {
						isLocal = true
						break
					}
				}
				if isLocal {
					a.safetyError(node.Tk, fmt.Sprintf("pointer to local place '%s' cannot escape its source frame", placeName(source)))
				}
			}
		}
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
		a.unwindTo(out, 0, false)
		a.checkExit(out)
		out.terminated = true
	case *types.NodeStmtThrow:
		a.borrowExpr(out, node.Expression)
		errVariable := directVariable(node.Expression)
		knownFailure, knownOK := knownFailingError(node.Expression), false
		if errVariable != nil {
			knownFailure = knownFailure || out.errorFacts[errVariable] < 0
			knownOK = out.errorFacts[errVariable] > 0
			for owner, condition := range out.conditions {
				if condition == errVariable {
					knownFailure = out.states[owner] == stateBorrowed
					knownOK = out.states[owner] == stateLive
					break
				}
			}
		}
		if knownOK {
			break
		}
		if errorValueType(node.Expression.GetInferredType()) && !knownFailure {
			// Throwing an error exits only for a non-OK value. Validate that
			// failure edge independently while retaining the OK continuation.
			failure := cloneFlow(*out)
			a.unwindTo(&failure, 0, true)
			a.checkExit(&failure)
			if errVariable != nil {
				*out = refineConditionalOwnership(*out, errVariable, true)
			}
		} else {
			a.unwindTo(out, 0, true)
			a.checkExit(out)
			out.terminated = true
		}
	case *types.NodeStmtIf:
		a.conditional(out, node)
	case *types.NodeStmtWhile:
		a.borrowExpr(out, node.CondExpr)
		a.loopBreaks = append(a.loopBreaks, nil)
		a.loopNext = append(a.loopNext, nil)
		a.loopDepths = append(a.loopDepths, len(out.scopes))
		iteration := cloneFlow(*out)
		a.addRangePredicates(&iteration, node.CondExpr, true, false)
		outerFuture := a.futureUses
		a.futureUses = cloneUseSet(outerFuture)
		collectBodyUses(&node.Body, a.futureUses) // account for the next iteration
		a.body(&iteration, &node.Body)
		a.futureUses = outerFuture
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
	case *types.NodeStmtFor:
		a.expression(out, node.DeclExpr)
		a.borrowExpr(out, node.BoundExpr)
		a.loopBreaks = append(a.loopBreaks, nil)
		a.loopNext = append(a.loopNext, nil)
		a.loopDepths = append(a.loopDepths, len(out.scopes))
		iteration := cloneFlow(*out)
		if declaration, ok := node.DeclExpr.(*types.NodeExprVarDefAssign); ok && declaration.VarDef != nil {
			index := fmt.Sprintf("v:%p", declaration.VarDef)
			bound := rangeExprKey(node.BoundExpr)
			start := rangeExprKey(declaration.AssignExpr)
			if bound != "" {
				proof := a.newRangeProof(false)
				iteration.ranges[rangeRelation{lower: index, upper: bound, strict: true}] = proof
			}
			if start != "" {
				iteration.ranges[rangeRelation{lower: start, upper: index, strict: false}] = a.newRangeProof(false)
			}
		}
		outerFuture := a.futureUses
		a.futureUses = cloneUseSet(outerFuture)
		collectBodyUses(&node.Body, a.futureUses) // account for the next iteration
		a.body(&iteration, &node.Body)
		a.futureUses = outerFuture
		loopIndex := len(a.loopBreaks) - 1
		if !iteration.terminated {
			a.loopNext[loopIndex] = append(a.loopNext[loopIndex], iteration)
		}
		breaks := a.loopBreaks[loopIndex]
		nextFlows := a.loopNext[loopIndex]
		for _, next := range nextFlows {
			*out = mergeFlows(*out, next)
		}
		for _, broken := range breaks {
			*out = mergeFlows(*out, broken)
		}
		a.loopBreaks = a.loopBreaks[:loopIndex]
		a.loopNext = a.loopNext[:loopIndex]
		a.loopDepths = a.loopDepths[:loopIndex]
	case *types.NodeStmtBounded:
		bounded := cloneFlow(*out)
		node.Proofs = nil
		for _, predicate := range node.Predicates {
			a.borrowExpr(&bounded, predicate)
			if proof := a.addRangePredicate(&bounded, predicate, true, true); proof != nil {
				node.Proofs = append(node.Proofs, proof)
			}
		}
		a.body(&bounded, &node.Body)
		*out = bounded
	case *types.NodeStmtUnsafe:
		a.unsafeDepth++
		a.body(out, &node.Body)
		a.unsafeDepth--
	case *types.NodeLlvm:
		if a.unsafeDepth == 0 {
			a.safetyError(node.Tk, "inline LLVM requires an unsafe block")
		}
	case *types.NodeStmtBreak:
		if len(a.loopBreaks) != 0 {
			index := len(a.loopBreaks) - 1
			a.unwindTo(out, a.loopDepths[index], false)
			exit := cloneFlow(*out)
			exit.terminated = false
			a.loopBreaks[index] = append(a.loopBreaks[index], exit)
			out.terminated = true
		}
	case *types.NodeStmtContinue:
		if len(a.loopNext) != 0 {
			index := len(a.loopNext) - 1
			a.unwindTo(out, a.loopDepths[index], false)
			next := cloneFlow(*out)
			next.terminated = false
			a.loopNext[index] = append(a.loopNext[index], next)
			out.terminated = true
		}
	case *types.NodeStmtDefer:
		if len(out.scopes) != 0 {
			out.scopes[len(out.scopes)-1].deferred = append(out.scopes[len(out.scopes)-1].deferred, node)
		}
		if owner := deferredDestructorOwner(node); owner != nil {
			out.deferred[owner] = true
			if out.deferredAt == nil {
				out.deferredAt = map[*types.NodeExprVarDef]types.Token{}
			}
			out.deferredAt[owner] = expressionToken(node.Expression)
		}
	}
}

func (a *analyzer) body(out *flow, body *types.NodeBody) {
	depth := len(out.scopes)
	out.scopes = append(out.scopes, deferScope{locals: map[*types.NodeExprVarDef]bool{}})
	outerFuture := a.futureUses
	for index, statement := range body.Statements {
		a.futureUses = cloneUseSet(outerFuture)
		for _, later := range body.Statements[index+1:] {
			collectStatementUses(later, a.futureUses)
		}
		a.statement(out, statement)
	}
	a.futureUses = outerFuture
	if len(out.scopes) > depth {
		a.unwindTo(out, depth, false)
	}
}

func cloneUseSet(in map[*types.NodeExprVarDef]bool) map[*types.NodeExprVarDef]bool {
	out := map[*types.NodeExprVarDef]bool{}
	for variable, used := range in {
		out[variable] = used
	}
	return out
}

func collectExprUses(expr types.NodeExpr, out map[*types.NodeExprVarDef]bool) {
	if expr == nil || (reflect.ValueOf(expr).Kind() == reflect.Ptr && reflect.ValueOf(expr).IsNil()) {
		return
	}
	switch node := expr.(type) {
	case *types.NodeExprName:
		if variable := directVariable(node); variable != nil {
			out[variable] = true
		}
	case *types.NodeExprVarDefAssign:
		collectExprUses(node.AssignExpr, out)
	case *types.NodeExprAssign:
		collectExprUses(node.Left, out)
		collectExprUses(node.Right, out)
	case *types.NodeExprBinary:
		collectExprUses(node.Left, out)
		collectExprUses(node.Right, out)
	case *types.NodeExprUnary:
		collectExprUses(node.Operand, out)
	case *types.NodeExprAddrof:
		collectExprUses(node.Expr, out)
	case *types.NodeExprMove:
		collectExprUses(node.Expr, out)
	case *types.NodeExprMemberAccess:
		collectExprUses(node.Target, out)
	case *types.NodeExprSubscript:
		collectExprUses(node.Target, out)
		collectExprUses(node.Expr, out)
	case *types.NodeExprCall:
		collectExprUses(node.Callee, out)
		for _, arg := range node.Args {
			collectExprUses(arg, out)
		}
		collectExprUses(node.MemberOwnerExpr, out)
		collectExprUses(node.MemberOwnerName, out)
	case *types.NodeExprTry:
		collectExprUses(node.Call, out)
	case *types.NodeExprStructInit:
		for _, field := range node.Fields {
			collectExprUses(field.Expression, out)
		}
	case *types.NodeExprProtoView:
		collectExprUses(node.Target, out)
	case *types.NodeExprArray:
		collectExprUses(node.Length, out)
		for _, entry := range node.Entries {
			collectExprUses(entry.Index, out)
			collectExprUses(entry.Value, out)
		}
	}
}

func collectBodyUses(body *types.NodeBody, out map[*types.NodeExprVarDef]bool) {
	if body == nil {
		return
	}
	for _, statement := range body.Statements {
		collectStatementUses(statement, out)
	}
}

func collectStatementUses(statement types.NodeStatement, out map[*types.NodeExprVarDef]bool) {
	switch node := statement.(type) {
	case *types.NodeStmtExpr:
		collectExprUses(node.Expression, out)
	case *types.NodeStmtRet:
		collectExprUses(node.Expression, out)
	case *types.NodeStmtThrow:
		collectExprUses(node.Expression, out)
	case *types.NodeStmtIf:
		collectExprUses(node.CondExpr, out)
		collectBodyUses(&node.Body, out)
		for next := node.NextCondStmt; next != nil; {
			switch branch := next.(type) {
			case *types.NodeStmtIf:
				collectExprUses(branch.CondExpr, out)
				collectBodyUses(&branch.Body, out)
				next = branch.NextCondStmt
			case *types.NodeStmtElse:
				collectBodyUses(&branch.Body, out)
				next = nil
			}
		}
	case *types.NodeStmtWhile:
		collectExprUses(node.CondExpr, out)
		collectBodyUses(&node.Body, out)
	case *types.NodeStmtFor:
		collectExprUses(node.DeclExpr, out)
		collectExprUses(node.BoundExpr, out)
		collectBodyUses(&node.Body, out)
	case *types.NodeStmtBounded:
		for _, predicate := range node.Predicates {
			collectExprUses(predicate, out)
		}
		collectBodyUses(&node.Body, out)
	case *types.NodeStmtUnsafe:
		collectBodyUses(&node.Body, out)
	case *types.NodeStmtDefer:
		if node.IsBody {
			collectBodyUses(&node.Body, out)
		} else {
			collectExprUses(node.Expression, out)
		}
	}
}

func isLiteralTrue(expr types.NodeExpr) bool {
	literal, ok := expr.(*types.NodeExprLit)
	return ok && literal.LitType == types.TokLitBool && literal.Value == "true"
}

func (a *analyzer) function(function *types.NodeFuncDef) {
	out := flow{states: map[*types.NodeExprVarDef]State{}, deferred: map[*types.NodeExprVarDef]bool{}, conditions: map[*types.NodeExprVarDef]*types.NodeExprVarDef{}, errorFacts: map[*types.NodeExprVarDef]int8{}, ranges: map[rangeRelation]*types.RangeProof{}, provenance: map[placeKey]pointerProvenance{}, retentions: map[placeKey]pointerProvenance{}, scopes: []deferScope{{locals: map[*types.NodeExprVarDef]bool{}}}}
	a.futureUses = map[*types.NodeExprVarDef]bool{}
	// Parameter declaration nodes are manufactured by scope_info. Seed owned
	// parameters so an unused one still produces an exit warning.
	for _, fnScope := range a.file.ScopeTree.DeclFuncs {
		if fnScope.Func != function {
			continue
		}
		for _, argument := range function.Class.ArgsNode.Args {
			variable := fnScope.Scope.DeclVars[argument.Name]
			if variable != nil && argument.Name == "this" && function.IsDestructor {
				// A destructor exclusively owns the receiver for the duration of its
				// implementation, including projected field cleanup through `this`.
				out.states[variable] = stateLive
				out.scopes[0].locals[variable] = true
				a.destructorReceivers[variable] = true
				continue
			}
			if variable != nil && variable.Type != nil && variable.Type.Owned && a.destructible(variable.Type) {
				out.states[variable] = stateLive
				out.scopes[0].locals[variable] = true
			}
		}
	}
	a.body(&out, &function.Body)
	if !out.terminated {
		a.unwindTo(&out, 0, false)
		a.checkExit(&out)
	}
}

func validateDestructors(a *analyzer, global *types.NodeGlobal) {
	// A destructor marks a consuming operation; its result type is unrestricted.
}

func functionParameterRoots(file *types.FileCtx, function *types.NodeFuncDef) map[*types.NodeExprVarDef]int {
	out := map[*types.NodeExprVarDef]int{}
	for _, fnScope := range file.ScopeTree.DeclFuncs {
		if fnScope.Func != function {
			continue
		}
		for index, argument := range function.Class.ArgsNode.Args {
			if variable := fnScope.Scope.DeclVars[argument.Name]; variable != nil {
				out[variable] = index
			}
		}
	}
	return out
}

func appendOrigin(out []int, value int) []int {
	for _, old := range out {
		if old == value {
			return out
		}
	}
	return append(out, value)
}

func inferredExprOrigins(expr types.NodeExpr, parameters map[*types.NodeExprVarDef]int, aliases map[*types.NodeExprVarDef][]int, summaries map[*types.NodeFuncDef][]int) []int {
	if expr == nil {
		return nil
	}
	if root := directVariable(expr); root != nil {
		if index, ok := parameters[root]; ok {
			return []int{index}
		}
		return append([]int(nil), aliases[root]...)
	}
	switch node := expr.(type) {
	case *types.NodeExprMove:
		return inferredExprOrigins(node.Expr, parameters, aliases, summaries)
	case *types.NodeExprCall:
		var out []int
		for _, argumentIndex := range summaries[node.AssociatedFnDef] {
			if argumentIndex < len(node.Args) {
				for _, origin := range inferredExprOrigins(node.Args[argumentIndex], parameters, aliases, summaries) {
					out = appendOrigin(out, origin)
				}
			}
		}
		return out
	}
	return nil
}

func inferBodyOrigins(body *types.NodeBody, parameters map[*types.NodeExprVarDef]int, aliases map[*types.NodeExprVarDef][]int, summaries map[*types.NodeFuncDef][]int) []int {
	var returns []int
	for _, statement := range body.Statements {
		switch node := statement.(type) {
		case *types.NodeStmtExpr:
			switch expr := node.Expression.(type) {
			case *types.NodeExprVarDefAssign:
				aliases[expr.VarDef] = inferredExprOrigins(expr.AssignExpr, parameters, aliases, summaries)
			case *types.NodeExprAssign:
				if variable := directVariable(expr.Left); variable != nil {
					aliases[variable] = inferredExprOrigins(expr.Right, parameters, aliases, summaries)
				}
			}
		case *types.NodeStmtRet:
			for _, origin := range inferredExprOrigins(node.Expression, parameters, aliases, summaries) {
				returns = appendOrigin(returns, origin)
			}
		case *types.NodeStmtIf:
			for _, origin := range inferBodyOrigins(&node.Body, parameters, cloneOrigins(aliases), summaries) {
				returns = appendOrigin(returns, origin)
			}
			for next := node.NextCondStmt; next != nil; {
				switch branch := next.(type) {
				case *types.NodeStmtIf:
					for _, origin := range inferBodyOrigins(&branch.Body, parameters, cloneOrigins(aliases), summaries) {
						returns = appendOrigin(returns, origin)
					}
					next = branch.NextCondStmt
				case *types.NodeStmtElse:
					for _, origin := range inferBodyOrigins(&branch.Body, parameters, cloneOrigins(aliases), summaries) {
						returns = appendOrigin(returns, origin)
					}
					next = nil
				}
			}
		case *types.NodeStmtUnsafe:
			for _, origin := range inferBodyOrigins(&node.Body, parameters, aliases, summaries) {
				returns = appendOrigin(returns, origin)
			}
		}
	}
	return returns
}

func cloneOrigins(in map[*types.NodeExprVarDef][]int) map[*types.NodeExprVarDef][]int {
	out := map[*types.NodeExprVarDef][]int{}
	for variable, origins := range in {
		out[variable] = append([]int(nil), origins...)
	}
	return out
}

func inferReturnOrigins(shared *types.SharedState) map[*types.NodeFuncDef][]int {
	summaries := map[*types.NodeFuncDef][]int{}
	// Iterate because a helper may return the result of another visible helper.
	for iteration := 0; iteration < 8; iteration++ {
		changed := false
		for _, file := range shared.Files {
			for _, declaration := range file.GlNode.Declarations {
				function, ok := declaration.(*types.NodeFuncDef)
				if !ok {
					continue
				}
				origins := inferBodyOrigins(&function.Body, functionParameterRoots(file, function), map[*types.NodeExprVarDef][]int{}, summaries)
				if len(origins) != len(summaries[function]) {
					summaries[function] = origins
					changed = true
				}
			}
		}
		if !changed {
			break
		}
	}
	return summaries
}

func destructorCallExpr(expr types.NodeExpr) *types.NodeExprCall {
	if attempted, ok := expr.(*types.NodeExprTry); ok {
		expr = attempted.Call
	}
	call, _ := expr.(*types.NodeExprCall)
	if call == nil || call.AssociatedFnDef == nil || !call.AssociatedFnDef.IsDestructor {
		return nil
	}
	return call
}

// inferLeadingDestructorEffects records pointer parameters whose pointee is
// consumed before the function can branch, return, or execute another call.
// This intentionally small effect summary is sound for adapter helpers such as
// `try pointer.close(); ret value` without guessing about conditional cleanup.
func inferLeadingDestructorEffects(shared *types.SharedState) map[*types.NodeFuncDef][]int {
	summaries := map[*types.NodeFuncDef][]int{}
	for _, file := range shared.Files {
		for _, declaration := range file.GlNode.Declarations {
			function, ok := declaration.(*types.NodeFuncDef)
			// Member calls carry their receiver outside call.Args, so this summary's
			// source-parameter indexing applies only to ordinary helper functions.
			if !ok || function.IsMember {
				continue
			}
			parameters := functionParameterRoots(file, function)
			for _, statement := range function.Body.Statements {
				expression, ok := statement.(*types.NodeStmtExpr)
				if !ok {
					break
				}
				call := destructorCallExpr(expression.Expression)
				if call == nil {
					break
				}
				receiver := call.MemberOwnerExpr
				if receiver == nil {
					receiver = call.MemberOwnerName
				}
				resolved, ok := resolvedPlace(receiver)
				if !ok {
					break
				}
				index, parameter := parameters[resolved.Root]
				if !parameter || !isPointerType(resolved.Root.Type) {
					break
				}
				summaries[function] = appendOrigin(summaries[function], index)
				break
			}
		}
	}
	return summaries
}

// Check runs the analysis, annotates authorized subscripts, and returns all
// ownership/range diagnostics.
func Check(shared *types.SharedState) []Diagnostic {
	if !Enabled {
		return nil
	}
	diagnostics := []Diagnostic{}
	returnOrigins := inferReturnOrigins(shared)
	consumePtrOrigins := inferLeadingDestructorEffects(shared)
	for _, file := range shared.Files {
		a := &analyzer{shared: shared, file: file, seen: map[string]bool{}, returnOrigins: returnOrigins, consumePtrOrigins: consumePtrOrigins, futureUses: map[*types.NodeExprVarDef]bool{}, destructorReceivers: map[*types.NodeExprVarDef]bool{}, staticExtents: map[*types.NodeExprVarDef]uint64{}}
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

// Run is the compiler pipeline hook. Safety diagnostics are fatal unless
// warningMode was explicitly requested; cleanup diagnostics always remain
// warnings.
func Run(shared *types.SharedState, warningMode bool) error {
	var fatal []error
	for _, diagnostic := range Check(shared) {
		out := types.Diagnostic{
			Severity: types.SeverityError,
			Code:     diagnostic.Code,
			Stage:    "ownership checking",
			Ctx:      shared.Files[diagnostic.FilePath],
			FilePath: diagnostic.FilePath,
			Token:    diagnostic.Token,
			Message:  diagnostic.Message,
			Related:  diagnostic.Related,
		}
		if !diagnostic.Safety || warningMode {
			out.Severity = types.SeverityWarning
			shared.Warnings = append(shared.Warnings, out)
			continue
		}
		copy := out
		fatal = append(fatal, &copy)
	}
	return comp_err.Join(fatal...)
}
