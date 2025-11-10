package main

import rl "github.com/gen2brain/raylib-go/raylib"

const (
	margin          float32 = 10
	main_menu_width float32 = 0.6
	button_height   float32 = 50
	button_width    float32 = 100
)

type Number interface {
	int | uint32 | uint64 | int32 | int64 | float32 | float64
}

func getMainMenuRect[num Number](width num, height num) rl.Rectangle {
	// basically just use this so that the caller doesn't have to call float32() themself
	width_f32 := float32(width)
	height_f32 := float32(height)

	inner_width := main_menu_width * width_f32
	inner_height := height_f32 - (margin * 2)

	return rl.Rectangle{
		X:      margin,
		Y:      margin,
		Width:  inner_width,
		Height: inner_height,
	}
}

func getMainMenuButtons(main_menu_rect rl.Rectangle, count int) []rl.Rectangle {
	buttons := make([]rl.Rectangle, count)

	button_cont_height := (main_menu_rect.Height / float32(len(buttons))) - (margin * 2)
	button_cont_width := (main_menu_rect.Width) - (margin * 2)

	var final_button_height = button_height
	var final_button_width = button_width

	var (
		scaled_height bool = false
		scaled_width  bool = false
	)

	// technically possible that the window goes bellow the size where 50 x 100 is possible,
	// then move onto centered scaled button
	if button_cont_height < button_height {
		final_button_height = button_cont_height
		scaled_height = true
	}

	if button_cont_width < button_width {
		final_button_width = button_cont_width
		scaled_width = true
	}

	base_top_left_x := main_menu_rect.X
	base_top_left_y := main_menu_rect.Y

	for i := range len(buttons) {
		i_f32 := float32(i)
		top_left_x := base_top_left_x + button_cont_height*i_f32

		if scaled_height {
			buttons[i].X = top_left_x
		} else {
			// here I need to center the button
			middle_x := top_left_x + button_cont_height/2
			// shifted since I need the left top corner to be slightly offset
			shifted_middle_x := middle_x - final_button_height/2

			buttons[i].X = shifted_middle_x
		}

		if scaled_width {
			buttons[i].Y = margin
		} else {
			middle_y := base_top_left_y + button_cont_width/2
			shifted_middle_y := middle_y - final_button_width/2
			buttons[i].Y = shifted_middle_y
		}

		buttons[i].Width = final_button_width
		buttons[i].Height = final_button_height
	}

	return buttons
}
