# `std/raylib`

## Example

```magma
raylib.initWindow(800, 450, "Magma")
defer raylib.closeWindow()
raylib.setTargetFPS(60)
while raylib.windowShouldClose() == false:
    raylib.beginDrawing()
    raylib.clearBackground(raylib.rayWhite())
    raylib.endDrawing()
..
```

Initial Windows bindings for raylib 5.5 using its shared-library distribution.
The import library is linked automatically, and the `bundle` declaration copies
`raylib.dll` beside the resulting executable after a successful build.

The initial surface includes window management, timing, drawing basic 2D
shapes and text, keyboard input, mouse input, config flags, key constants, and
the standard raylib color palette. See `samples/raylib_test.mg`.

## Types

- `Color(r u8, g u8, b u8, a u8)` is an RGBA color.
- `Vector2(x f32, y f32)` and `Rectangle(x f32, y f32, width f32, height f32)` mirror the corresponding raylib value types.
- `Texture`, `Image`, `GlyphInfo`, and `Font` mirror raylib 5.5's font-related value types.

## API surface

- Window: `initWindow`, `closeWindow`, `windowShouldClose`, `isWindowReady`, `isWindowFullscreen`, `isWindowHidden`, `isWindowMinimized`, `isWindowMaximized`, `isWindowFocused`, `isWindowResized`, `setWindowState`, `clearWindowState`, `toggleFullscreen`, `maximizeWindow`, `minimizeWindow`, `restoreWindow`, `setWindowSize`, `screenWidth`, `screenHeight`, `renderWidth`, and `renderHeight`.
- Timing and drawing: `setTargetFPS`, `frameTime`, `time`, `fps`, `beginDrawing`, `endDrawing`, `clearBackground`, `drawPixel`, `drawLine`, `drawCircle`, `drawRectangle`, `drawRectangleLines`, `drawTextC`, `drawText`, `drawFPS`, `measureTextC`, and `measureText`.
- Fonts: `fontDefault`, `loadFont`, `loadFontEx`, `loadFontFromImage`, `loadFontFromMemory`, `isFontValid`, `unloadFont`, `exportFontAsCode`, `drawTextExC`, `drawTextEx`, `drawTextPro`, `drawTextCodepoint`, `drawTextCodepoints`, `setTextLineSpacing`, `measureTextExC`, `measureTextEx`, `glyphIndex`, `glyphInfo`, and `glyphAtlasRectangle`.
- Keyboard: `isKeyPressed`, `isKeyPressedRepeat`, `isKeyDown`, `isKeyReleased`, `isKeyUp`, `keyPressed`, `charPressed`, and `setExitKey`. Constants: `keySpace`, `keyEscape`, `keyEnter`, `keyTab`, `keyBackspace`, `keyInsert`, `keyDelete`, `keyRight`, `keyLeft`, `keyDown`, `keyUp`, `keyHome`, `keyEnd`, `keyF1` through `keyF12`, `keyLeftShift`, `keyLeftControl`, `keyLeftAlt`, `keyRightShift`, `keyRightControl`, `keyRightAlt`, `keyA`, `keyD`, `keyS`, and `keyW`.
- Mouse and cursor: `isMouseButtonPressed`, `isMouseButtonDown`, `isMouseButtonReleased`, `isMouseButtonUp`, `mouseX`, `mouseY`, `setMousePosition`, `mouseWheelMove`, `showCursor`, `hideCursor`, `isCursorHidden`, `enableCursor`, `disableCursor`, and `isCursorOnScreen`. Constants: `mouseButtonLeft`, `mouseButtonRight`, `mouseButtonMiddle`, `mouseButtonSide`, `mouseButtonExtra`, `mouseButtonForward`, and `mouseButtonBack`.
- Configuration: `flagVsyncHint`, `flagFullscreenMode`, `flagWindowResizable`, `flagWindowUndecorated`, `flagWindowHidden`, `flagWindowMinimized`, `flagWindowMaximized`, `flagWindowUnfocused`, `flagWindowTopmost`, `flagWindowAlwaysRun`, `flagWindowTransparent`, `flagWindowHighDpi`, `flagWindowMousePassthrough`, `flagBorderlessWindowedMode`, `flagMsaa4xHint`, and `flagInterlacedHint`.
- `color(r, g, b, a)` constructs a color. Palette: `lightGray`, `gray`, `darkGray`, `yellow`, `gold`, `orange`, `pink`, `red`, `maroon`, `green`, `lime`, `darkGreen`, `skyBlue`, `blue`, `darkBlue`, `purple`, `violet`, `darkPurple`, `beige`, `brown`, `darkBrown`, `white`, `black`, `blank`, `magenta`, and `rayWhite`.

Functions ending in `C` take an existing null-terminated UTF-8 pointer and are
appropriate for drawing loops. The `str` wrappers allocate a temporary C string
and are intended for setup or occasional calls.

Font-related structs use ordinary Raylib 5.5 declarations. Magma's C ABI
lowering maps aggregate arguments and returns to the selected target's native
calling convention, including Windows x64 indirect values, System V AMD64
`byval`/`sret` values and register classes, and AArch64 indirect values. Other
texture, image, audio, and model APIs can use the same declarations as their
bindings are expanded.
