mod main

use "../std/raylib.mg" rl

pub main() !void:
    rl.initWindow(800, 450, "raylib [core] example - basic window")
    defer rl.closeWindow()

    rl.setTargetFPS(60)

    while rl.windowShouldClose() == false:
        rl.beginDrawing()
        defer rl.endDrawing()
        
        rl.clearBackground(rl.rayWhite())

        rl.drawFPS(0, 0)
        rl.drawText("Congrats! You created your first window!", 190, 200, 20, rl.lightGray())
    ..
..
