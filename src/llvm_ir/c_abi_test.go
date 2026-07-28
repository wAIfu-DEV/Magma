package llvmir_test

import (
	magmatarget "Magma/src/target"
	"strings"
	"testing"
)

const aggregateABISource = `mod main

Vector2(x f32, y f32)
Rectangle(x f32, y f32, width f32, height f32)
Texture(id u32, width i32, height i32, mipmaps i32, format i32)
Font(baseSize i32, glyphCount i32, glyphPadding i32, texture Texture, recs ptr, glyphs ptr)

ext getFont GetFontDefault() Font
ext validFont IsFontValid(font Font) bool
ext measure MeasureTextEx(font Font, text u8*, size f32, spacing f32) Vector2
ext atlas GetGlyphAtlasRec(font Font, codepoint i32) Rectangle

@export_name("magma_font_score")
fontScore(font Font) i32:
    ret font.baseSize + font.glyphCount
..

main() void:
    font := getFont()
    validFont(font)
    measure(font, none, 20.0, 2.0)
    atlas(font, 65)
..
`

func TestExternalAggregateCABILowering(t *testing.T) {
	tests := []struct {
		name   string
		target magmatarget.Target
		want   []string
	}{
		{
			name:   "windows-x64",
			target: magmatarget.Target{Arch: "x86_64", OS: "windows", PointerBits: 64},
			want: []string{
				"call void @GetFontDefault(ptr sret(",
				"call i1 @IsFontValid(ptr ",
				"call i64 @MeasureTextEx(ptr ",
				"call void @GetGlyphAtlasRec(ptr sret(",
				"define i32 @magma_font_score(ptr %cabi.arg.0)",
			},
		},
		{
			name:   "sysv-x64",
			target: magmatarget.Target{Arch: "x86_64", OS: "linux", PointerBits: 64},
			want: []string{
				"call void @GetFontDefault(ptr sret(",
				"call i1 @IsFontValid(ptr byval(",
				"call <2 x float> @MeasureTextEx(ptr byval(",
				"call { <2 x float>, <2 x float> } @GetGlyphAtlasRec(ptr byval(",
				"define i32 @magma_font_score(ptr byval(",
			},
		},
		{
			name:   "aarch64",
			target: magmatarget.Target{Arch: "aarch64", OS: "darwin", PointerBits: 64},
			want: []string{
				"call void @GetFontDefault(ptr sret(",
				"call i1 @IsFontValid(ptr ",
				"call %struct.",
				"@MeasureTextEx(ptr ",
				"define i32 @magma_font_score(ptr %cabi.arg.0)",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ir, err := compileSourceTarget(t, aggregateABISource, &test.target)
			if err != nil {
				t.Fatalf("compile aggregate ABI probe: %v", err)
			}
			for _, want := range test.want {
				if !strings.Contains(ir, want) {
					t.Fatalf("generated IR is missing %q", want)
				}
			}
		})
	}
}
