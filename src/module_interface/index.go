package moduleinterface

// Index is an immutable-by-convention lookup view over a validated interface.
// Milestone 3 will adapt compiler lookup APIs to consume it.
type Index struct {
	Interface            *Interface
	Functions            map[string]Function
	Structs              map[string]Struct
	Unions               map[string]Union
	Prototypes           map[string]Prototype
	Aliases              map[string]Alias
	Globals              map[string]Global
	Constants            map[string]Constant
	Reexports            map[string]string
	Imports              map[string]string
	PrimitiveDestructors map[string][]string
}

func NewIndex(value *Interface) (*Index, error) {
	if err := Validate(value); err != nil {
		return nil, err
	}
	index := &Index{
		Interface: value, Functions: map[string]Function{}, Structs: map[string]Struct{},
		Unions: map[string]Union{}, Prototypes: map[string]Prototype{}, Aliases: map[string]Alias{},
		Globals: map[string]Global{}, Constants: map[string]Constant{}, Reexports: map[string]string{}, Imports: map[string]string{},
		PrimitiveDestructors: map[string][]string{},
	}
	for _, item := range value.Functions {
		index.Functions[item.Name] = item
	}
	for _, item := range value.Structs {
		index.Structs[item.Name] = item
	}
	for _, item := range value.Unions {
		index.Unions[item.Name] = item
	}
	for _, item := range value.Prototypes {
		index.Prototypes[item.Name] = item
	}
	for _, item := range value.Aliases {
		index.Aliases[item.Name] = item
	}
	for _, item := range value.Globals {
		index.Globals[item.Name] = item
	}
	for _, item := range value.Constants {
		index.Constants[item.Name] = item
	}
	for _, item := range value.Reexports {
		index.Reexports[item.Alias] = item.ModuleID
	}
	for _, item := range value.Imports {
		index.Imports[item.Alias] = item.ModuleID
	}
	for _, item := range value.PrimitiveDestructors {
		index.PrimitiveDestructors[item.Type] = append([]string(nil), item.Symbols...)
	}
	return index, nil
}
