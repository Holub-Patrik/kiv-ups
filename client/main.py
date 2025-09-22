#!.venv/bin/python

import pyray as rl


class Window:
    def __init__(self):
        rl.init_window(800, 600, "Test app")

    def __enter__(self):
        return self

    def __exit__(self, exc_type, exc_value, traceback):
        rl.close_window()


class Drawing:
    def __init__(self):
        rl.begin_drawing()

    def __enter__(self):
        return self

    def __exit__(self, exc_type, exc_value, traceback):
        rl.end_drawing()


if __name__ == "__main__":
    with Window() as window:
        while not rl.window_should_close():
            with Drawing() as drawing:
                rl.clear_background(rl.WHITE)
                rl.draw_text("Hello world", 190, 200, 20, rl.VIOLET)
