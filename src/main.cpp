#include "raylib-cpp.hpp"

int screenWidth = 800;
int screenHeight = 450;

void UpdateDrawFrame(void);

int main() {
  raylib::Window window(screenWidth, screenHeight,
                        "raylib-cpp [core] example - basic window");
  SetTargetFPS(60);

  while (!window.ShouldClose()) {
    UpdateDrawFrame();
  }
  return 0;
}

void UpdateDrawFrame(void) {
  BeginDrawing();

  ClearBackground(RAYWHITE);

  DrawText("Congrats! You created your first raylib-cpp window!", 160, 200, 20,
           LIGHTGRAY);

  EndDrawing();
}
