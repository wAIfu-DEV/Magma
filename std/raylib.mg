mod raylib

# Requires raylib.dll beside executable
@platform("windows")
link "../vendor/raylib/win/raylibdll.lib"

@platform("linux", "freebsd", "netbsd", "openbsd")
link "../vendor/raylib/linux/raylib.so" 

@platform("darwin")
link "../vendor/raylib/mac/raylib.dylib" 

use "allocator.mg" alc
use "strings.mg"   strings
use "cast.mg"      cast

# ABI-safe public value types. Color is packed to a u32 before crossing the C
# boundary because Win64 passes this four-byte C struct as an integer value.
Color(r u8, g u8, b u8, a u8)

Vector2(
    x f32
    y f32
)

Rectangle(
    x f32
    y f32
    width f32
    height f32
)

ext ext_InitWindow InitWindow(width i32, height i32, title u8*) void
ext ext_CloseWindow CloseWindow() void
ext ext_WindowShouldClose WindowShouldClose() bool
ext ext_IsWindowReady IsWindowReady() bool
ext ext_IsWindowFullscreen IsWindowFullscreen() bool
ext ext_IsWindowHidden IsWindowHidden() bool
ext ext_IsWindowMinimized IsWindowMinimized() bool
ext ext_IsWindowMaximized IsWindowMaximized() bool
ext ext_IsWindowFocused IsWindowFocused() bool
ext ext_IsWindowResized IsWindowResized() bool
ext ext_SetWindowState SetWindowState(flags u32) void
ext ext_ClearWindowState ClearWindowState(flags u32) void
ext ext_ToggleFullscreen ToggleFullscreen() void
ext ext_MaximizeWindow MaximizeWindow() void
ext ext_MinimizeWindow MinimizeWindow() void
ext ext_RestoreWindow RestoreWindow() void
ext ext_SetWindowSize SetWindowSize(width i32, height i32) void
ext ext_GetScreenWidth GetScreenWidth() i32
ext ext_GetScreenHeight GetScreenHeight() i32
ext ext_GetRenderWidth GetRenderWidth() i32
ext ext_GetRenderHeight GetRenderHeight() i32

ext ext_SetTargetFPS SetTargetFPS(targetFps i32) void
ext ext_GetFrameTime GetFrameTime() f32
ext ext_GetTime GetTime() f64
ext ext_GetFPS GetFPS() i32

ext ext_BeginDrawing BeginDrawing() void
ext ext_EndDrawing EndDrawing() void
ext ext_ClearBackground ClearBackground(packedColor u32) void
ext ext_DrawPixel DrawPixel(x i32, y i32, packedColor u32) void
ext ext_DrawLine DrawLine(startX i32, startY i32, endX i32, endY i32, packedColor u32) void
ext ext_DrawCircle DrawCircle(centerX i32, centerY i32, radius f32, packedColor u32) void
ext ext_DrawRectangle DrawRectangle(x i32, y i32, width i32, height i32, packedColor u32) void
ext ext_DrawRectangleLines DrawRectangleLines(x i32, y i32, width i32, height i32, packedColor u32) void
ext ext_DrawText DrawText(text u8*, x i32, y i32, fontSize i32, packedColor u32) void
ext ext_DrawFPS DrawFPS(x i32, y i32) void
ext ext_MeasureText MeasureText(text u8*, fontSize i32) i32

ext ext_IsKeyPressed IsKeyPressed(key i32) bool
ext ext_IsKeyPressedRepeat IsKeyPressedRepeat(key i32) bool
ext ext_IsKeyDown IsKeyDown(key i32) bool
ext ext_IsKeyReleased IsKeyReleased(key i32) bool
ext ext_IsKeyUp IsKeyUp(key i32) bool
ext ext_GetKeyPressed GetKeyPressed() i32
ext ext_GetCharPressed GetCharPressed() i32
ext ext_SetExitKey SetExitKey(key i32) void

ext ext_IsMouseButtonPressed IsMouseButtonPressed(button i32) bool
ext ext_IsMouseButtonDown IsMouseButtonDown(button i32) bool
ext ext_IsMouseButtonReleased IsMouseButtonReleased(button i32) bool
ext ext_IsMouseButtonUp IsMouseButtonUp(button i32) bool
ext ext_GetMouseX GetMouseX() i32
ext ext_GetMouseY GetMouseY() i32
ext ext_SetMousePosition SetMousePosition(x i32, y i32) void
ext ext_GetMouseWheelMove GetMouseWheelMove() f32
ext ext_ShowCursor ShowCursor() void
ext ext_HideCursor HideCursor() void
ext ext_IsCursorHidden IsCursorHidden() bool
ext ext_EnableCursor EnableCursor() void
ext ext_DisableCursor DisableCursor() void
ext ext_IsCursorOnScreen IsCursorOnScreen() bool

pub color(r u8, g u8, b u8, a u8) Color:
    ret Color(r=r, g=g, b=b, a=a)
..

packColor(c Color) u32:
    packed u64 = cast.u8to64(c.r)
    packed = packed | (cast.u8to64(c.g) << 8)
    packed = packed | (cast.u8to64(c.b) << 16)
    packed = packed | (cast.u8to64(c.a) << 24)
    ret cast.u64to32(packed)
..

pub lightGray() Color: ret color(200, 200, 200, 255) ..
pub gray() Color: ret color(130, 130, 130, 255) ..
pub darkGray() Color: ret color(80, 80, 80, 255) ..
pub yellow() Color: ret color(253, 249, 0, 255) ..
pub gold() Color: ret color(255, 203, 0, 255) ..
pub orange() Color: ret color(255, 161, 0, 255) ..
pub pink() Color: ret color(255, 109, 194, 255) ..
pub red() Color: ret color(230, 41, 55, 255) ..
pub maroon() Color: ret color(190, 33, 55, 255) ..
pub green() Color: ret color(0, 228, 48, 255) ..
pub lime() Color: ret color(0, 158, 47, 255) ..
pub darkGreen() Color: ret color(0, 117, 44, 255) ..
pub skyBlue() Color: ret color(102, 191, 255, 255) ..
pub blue() Color: ret color(0, 121, 241, 255) ..
pub darkBlue() Color: ret color(0, 82, 172, 255) ..
pub purple() Color: ret color(200, 122, 255, 255) ..
pub violet() Color: ret color(135, 60, 190, 255) ..
pub darkPurple() Color: ret color(112, 31, 126, 255) ..
pub beige() Color: ret color(211, 176, 131, 255) ..
pub brown() Color: ret color(127, 106, 79, 255) ..
pub darkBrown() Color: ret color(76, 63, 47, 255) ..
pub white() Color: ret color(255, 255, 255, 255) ..
pub black() Color: ret color(0, 0, 0, 255) ..
pub blank() Color: ret color(0, 0, 0, 0) ..
pub magenta() Color: ret color(255, 0, 255, 255) ..
pub rayWhite() Color: ret color(245, 245, 245, 255) ..

# Allocating UTF-8 wrapper intended for setup code. Hot drawing loops should
# convert stable text once with strings.toCstr and call drawTextC.
pub initWindow(width i32, height i32, title str) void:
    titleC u8* = strings.toCstrNoCopy(title)
    ext_InitWindow(width, height, titleC)
..

pub closeWindow() void: ext_CloseWindow() ..
pub windowShouldClose() bool: ret ext_WindowShouldClose() ..
pub isWindowReady() bool: ret ext_IsWindowReady() ..
pub isWindowFullscreen() bool: ret ext_IsWindowFullscreen() ..
pub isWindowHidden() bool: ret ext_IsWindowHidden() ..
pub isWindowMinimized() bool: ret ext_IsWindowMinimized() ..
pub isWindowMaximized() bool: ret ext_IsWindowMaximized() ..
pub isWindowFocused() bool: ret ext_IsWindowFocused() ..
pub isWindowResized() bool: ret ext_IsWindowResized() ..
pub setWindowState(flags u32) void: ext_SetWindowState(flags) ..
pub clearWindowState(flags u32) void: ext_ClearWindowState(flags) ..
pub toggleFullscreen() void: ext_ToggleFullscreen() ..
pub maximizeWindow() void: ext_MaximizeWindow() ..
pub minimizeWindow() void: ext_MinimizeWindow() ..
pub restoreWindow() void: ext_RestoreWindow() ..
pub setWindowSize(width i32, height i32) void: ext_SetWindowSize(width, height) ..
pub screenWidth() i32: ret ext_GetScreenWidth() ..
pub screenHeight() i32: ret ext_GetScreenHeight() ..
pub renderWidth() i32: ret ext_GetRenderWidth() ..
pub renderHeight() i32: ret ext_GetRenderHeight() ..

pub setTargetFPS(targetFps i32) void: ext_SetTargetFPS(targetFps) ..
pub frameTime() f32: ret ext_GetFrameTime() ..
pub time() f64: ret ext_GetTime() ..
pub fps() i32: ret ext_GetFPS() ..

pub beginDrawing() void: ext_BeginDrawing() ..
pub endDrawing() void: ext_EndDrawing() ..
pub clearBackground(c Color) void: ext_ClearBackground(packColor(c)) ..
pub drawPixel(x i32, y i32, c Color) void: ext_DrawPixel(x, y, packColor(c)) ..
pub drawLine(startX i32, startY i32, endX i32, endY i32, c Color) void: ext_DrawLine(startX, startY, endX, endY, packColor(c)) ..
pub drawCircle(centerX i32, centerY i32, radius f32, c Color) void: ext_DrawCircle(centerX, centerY, radius, packColor(c)) ..
pub drawRectangle(x i32, y i32, width i32, height i32, c Color) void: ext_DrawRectangle(x, y, width, height, packColor(c)) ..
pub drawRectangleLines(x i32, y i32, width i32, height i32, c Color) void: ext_DrawRectangleLines(x, y, width, height, packColor(c)) ..
pub drawTextC(text u8*, x i32, y i32, fontSize i32, c Color) void: ext_DrawText(text, x, y, fontSize, packColor(c)) ..
pub drawFPS(x i32, y i32) void: ext_DrawFPS(x, y) ..
pub measureTextC(text u8*, fontSize i32) i32: ret ext_MeasureText(text, fontSize) ..

pub drawText(text str, x i32, y i32, fontSize i32, c Color) void:
    textC u8* = strings.toCstrNoCopy(text)
    ext_DrawText(textC, x, y, fontSize, packColor(c))
..

pub measureText(text str, fontSize i32) i32:
    textC u8* = strings.toCstrNoCopy(text)
    width i32 = ext_MeasureText(textC, fontSize)
    ret width
..

pub isKeyPressed(key i32) bool: ret ext_IsKeyPressed(key) ..
pub isKeyPressedRepeat(key i32) bool: ret ext_IsKeyPressedRepeat(key) ..
pub isKeyDown(key i32) bool: ret ext_IsKeyDown(key) ..
pub isKeyReleased(key i32) bool: ret ext_IsKeyReleased(key) ..
pub isKeyUp(key i32) bool: ret ext_IsKeyUp(key) ..
pub keyPressed() i32: ret ext_GetKeyPressed() ..
pub charPressed() i32: ret ext_GetCharPressed() ..
pub setExitKey(key i32) void: ext_SetExitKey(key) ..

pub isMouseButtonPressed(button i32) bool: ret ext_IsMouseButtonPressed(button) ..
pub isMouseButtonDown(button i32) bool: ret ext_IsMouseButtonDown(button) ..
pub isMouseButtonReleased(button i32) bool: ret ext_IsMouseButtonReleased(button) ..
pub isMouseButtonUp(button i32) bool: ret ext_IsMouseButtonUp(button) ..
pub mouseX() i32: ret ext_GetMouseX() ..
pub mouseY() i32: ret ext_GetMouseY() ..
pub setMousePosition(x i32, y i32) void: ext_SetMousePosition(x, y) ..
pub mouseWheelMove() f32: ret ext_GetMouseWheelMove() ..
pub showCursor() void: ext_ShowCursor() ..
pub hideCursor() void: ext_HideCursor() ..
pub isCursorHidden() bool: ret ext_IsCursorHidden() ..
pub enableCursor() void: ext_EnableCursor() ..
pub disableCursor() void: ext_DisableCursor() ..
pub isCursorOnScreen() bool: ret ext_IsCursorOnScreen() ..

# ConfigFlags values.
pub flagVsyncHint() u32: ret 0x00000040 ..
pub flagFullscreenMode() u32: ret 0x00000002 ..
pub flagWindowResizable() u32: ret 0x00000004 ..
pub flagWindowUndecorated() u32: ret 0x00000008 ..
pub flagWindowHidden() u32: ret 0x00000080 ..
pub flagWindowMinimized() u32: ret 0x00000200 ..
pub flagWindowMaximized() u32: ret 0x00000400 ..
pub flagWindowUnfocused() u32: ret 0x00000800 ..
pub flagWindowTopmost() u32: ret 0x00001000 ..
pub flagWindowAlwaysRun() u32: ret 0x00000100 ..
pub flagWindowTransparent() u32: ret 0x00000010 ..
pub flagWindowHighDpi() u32: ret 0x00002000 ..
pub flagWindowMousePassthrough() u32: ret 0x00004000 ..
pub flagBorderlessWindowedMode() u32: ret 0x00008000 ..
pub flagMsaa4xHint() u32: ret 0x00000020 ..
pub flagInterlacedHint() u32: ret 0x00010000 ..

# Common KeyboardKey values. Letters and digits use their ASCII values.
pub keySpace() i32: ret 32 ..
pub keyA() i32: ret 65 ..
pub keyD() i32: ret 68 ..
pub keyS() i32: ret 83 ..
pub keyW() i32: ret 87 ..
pub keyEscape() i32: ret 256 ..
pub keyEnter() i32: ret 257 ..
pub keyTab() i32: ret 258 ..
pub keyBackspace() i32: ret 259 ..
pub keyInsert() i32: ret 260 ..
pub keyDelete() i32: ret 261 ..
pub keyRight() i32: ret 262 ..
pub keyLeft() i32: ret 263 ..
pub keyDown() i32: ret 264 ..
pub keyUp() i32: ret 265 ..
pub keyHome() i32: ret 268 ..
pub keyEnd() i32: ret 269 ..
pub keyF1() i32: ret 290 ..
pub keyF2() i32: ret 291 ..
pub keyF3() i32: ret 292 ..
pub keyF4() i32: ret 293 ..
pub keyF5() i32: ret 294 ..
pub keyF6() i32: ret 295 ..
pub keyF7() i32: ret 296 ..
pub keyF8() i32: ret 297 ..
pub keyF9() i32: ret 298 ..
pub keyF10() i32: ret 299 ..
pub keyF11() i32: ret 300 ..
pub keyF12() i32: ret 301 ..
pub keyLeftShift() i32: ret 340 ..
pub keyLeftControl() i32: ret 341 ..
pub keyLeftAlt() i32: ret 342 ..
pub keyRightShift() i32: ret 344 ..
pub keyRightControl() i32: ret 345 ..
pub keyRightAlt() i32: ret 346 ..

pub mouseButtonLeft() i32: ret 0 ..
pub mouseButtonRight() i32: ret 1 ..
pub mouseButtonMiddle() i32: ret 2 ..
pub mouseButtonSide() i32: ret 3 ..
pub mouseButtonExtra() i32: ret 4 ..
pub mouseButtonForward() i32: ret 5 ..
pub mouseButtonBack() i32: ret 6 ..
