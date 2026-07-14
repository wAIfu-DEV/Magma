mod main

use "../std/raylib.mg" raylib
use "../std/heap.mg"   heap
use "../std/strings.mg" strings

pub main() !void:
    a := heap.allocator()
    try raylib.initWindow(a, 800, 450, "Magma + raylib")
    defer raylib.closeWindow()
    raylib.setTargetFPS(60)

    message u8* = try strings.toCstr(a, "Hello from Magma")
    defer a.free(message)

    while raylib.windowShouldClose() == false:
        raylib.beginDrawing()
        raylib.clearBackground(raylib.rayWhite())
        raylib.drawTextC(message, 280, 210, 24, raylib.darkGray())
        raylib.endDrawing()
    ..
..
