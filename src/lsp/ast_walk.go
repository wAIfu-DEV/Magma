package lsp

import "reflect"

// walkAST visits exported values reachable from root while avoiding semantic
// backlinks and pointer cycles. Returning false from visit stops traversal.
func walkAST(root any, visit func(any) bool) {
	seen := map[uintptr]bool{}
	var walk func(reflect.Value) bool
	walk = func(value reflect.Value) bool {
		if !value.IsValid() {
			return true
		}
		if value.Kind() == reflect.Interface {
			if value.IsNil() {
				return true
			}
			return walk(value.Elem())
		}
		if value.Kind() == reflect.Pointer {
			if value.IsNil() || seen[value.Pointer()] {
				return true
			}
			seen[value.Pointer()] = true
			if value.CanInterface() && !visit(value.Interface()) {
				return false
			}
			return walk(value.Elem())
		}
		if value.Kind() == reflect.Struct {
			if value.CanInterface() && !visit(value.Interface()) {
				return false
			}
			valueType := value.Type()
			for i := 0; i < value.NumField(); i++ {
				field := valueType.Field(i)
				if field.PkgPath == "" && !skipASTField(field.Name) && !walk(value.Field(i)) {
					return false
				}
			}
			return true
		}
		switch value.Kind() {
		case reflect.Slice, reflect.Array:
			for i := 0; i < value.Len(); i++ {
				if !walk(value.Index(i)) {
					return false
				}
			}
		case reflect.Map:
			iterator := value.MapRange()
			for iterator.Next() {
				if !walk(iterator.Value()) {
					return false
				}
			}
		}
		return true
	}
	walk(reflect.ValueOf(root))
}

func skipASTField(name string) bool {
	switch name {
	case "Parent", "Scope", "AssociatedNode", "AssociatedFnDef", "Destructor", "Destructors":
		return true
	}
	return false
}
