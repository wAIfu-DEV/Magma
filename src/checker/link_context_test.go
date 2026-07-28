package checker

import (
	mt "Magma/src/types"
	"reflect"
	"testing"
)

func TestParseNameSingle(t *testing.T) {
	name := &mt.NodeNameSingle{Name: "value"}

	got := parseName(name)
	want := parsedName{First: "value"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseName() = %#v, want %#v", got, want)
	}
	if got := flattenName(name); got != "value" {
		t.Fatalf("flattenName() = %q, want %q", got, "value")
	}
}

func TestParseNameComposite(t *testing.T) {
	name := &mt.NodeNameComposite{Parts: []string{"module", "Type", "field"}}

	got := parseName(name)
	want := parsedName{First: "module", HasParts: true, Parts: []string{"Type", "field"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseName() = %#v, want %#v", got, want)
	}
	if got := flattenName(name); got != "module.Type.field" {
		t.Fatalf("flattenName() = %q, want %q", got, "module.Type.field")
	}
}

func TestLastNameToken(t *testing.T) {
	single := &mt.NodeNameSingle{Tk: mt.Token{Repr: "single"}}
	if got := lastNameToken(single); got.Repr != "single" {
		t.Fatalf("single token = %q, want %q", got.Repr, "single")
	}

	composite := &mt.NodeNameComposite{Tokens: []mt.Token{{Repr: "module"}, {Repr: "member"}}}
	if got := lastNameToken(composite); got.Repr != "member" {
		t.Fatalf("composite token = %q, want %q", got.Repr, "member")
	}
}
