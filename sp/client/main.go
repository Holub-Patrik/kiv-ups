package main

import rl "github.com/gen2brain/raylib-go/raylib"

func main() {
	// rl.SetConfigFlags(rl.FlagWindowResizable)
	rl.InitWindow(800, 450, "raylib [core] example - basic window")
	defer rl.CloseWindow()

	// rl.SetTargetFPS(60)

	for !rl.WindowShouldClose() {
		rl.BeginDrawing()

		rl.ClearBackground(rl.Black)
		rl.DrawText("Congrats! You created your first window!", 190, 200, 20, rl.White)
		rl.DrawFPS(0, 0)

		rl.EndDrawing()
	}
}
