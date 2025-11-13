package window

import (
	rl "github.com/gen2brain/raylib-go/raylib"
)

type RGComponent interface {
	Draw()
	Calculate(bounds rl.Rectangle, position uint64)
	GetBounds() rl.Rectangle
}

type MainMenu struct {
	bounds rl.Rectangle
	color  rl.Color
}

func (self *MainMenu) Draw() {
	// draw itself
	rl.DrawRectangleRec(self.bounds, self.color)
	rl.DrawRectangleLinesEx(self.bounds, 5, rl.White)
}

func (self *MainMenu) Calculate(outer rl.Rectangle, position uint64) {
	const width_ratio float32 = 0.7
	const height_ratio float32 = 0.6

	inner_width := (outer.Width - (margin * 2)) * width_ratio
	inner_height := (outer.Height - (margin * 2)) * height_ratio

	center_x := outer.X + outer.Width/2 - inner_width/2
	center_y := outer.Y + outer.Height/2 - inner_height/2

	self.bounds = rl.Rectangle{
		X:      center_x,
		Y:      center_y,
		Width:  inner_width,
		Height: inner_height,
	}
}

func (self *MainMenu) GetBounds() rl.Rectangle {
	return self.bounds
}

const (
	margin                float32 = 10
	hor_menu_width_ratio  float32 = 0.7
	hor_menu_height_ratio float32 = 0.6
	ver_menu_width_ratio  float32 = 0.5
	ver_menu_height_ratio float32 = 0.8
	button_height_ratio   float32 = 0.05
	button_width_ratio    float32 = 0.07
)

type Number interface {
	int | uint32 | uint64 | int32 | int64 | float32 | float64
}

func GetHorizontalMenuRect[num Number](x num, y num, width num, height num) rl.Rectangle {
	// basically just use this so that the caller doesn't have to call float32() themself
	width_f32 := float32(width)
	height_f32 := float32(height)
	x_f32 := float32(x)
	y_f32 := float32(y)

	inner_width := (width_f32 - (margin * 2)) * hor_menu_width_ratio
	inner_height := (height_f32 - (margin * 2)) * hor_menu_height_ratio

	center_x := x_f32 + width_f32/2 - inner_width/2
	center_y := y_f32 + height_f32/2 - inner_height/2

	return rl.Rectangle{
		X:      center_x,
		Y:      center_y,
		Width:  inner_width,
		Height: inner_height,
	}
}

func GetVerticalMenuRect[num Number](x num, y num, width num, height num) rl.Rectangle {
	// basically just use this so that the caller doesn't have to call float32() themself
	width_f32 := float32(width)
	height_f32 := float32(height)
	x_f32 := float32(x)
	y_f32 := float32(y)

	inner_width := (width_f32 - (margin * 2)) * ver_menu_width_ratio
	inner_height := (height_f32 - (margin * 2)) * ver_menu_height_ratio

	center_x := x_f32 + width_f32/2 - inner_width/2
	center_y := y_f32 + height_f32/2 - inner_height/2

	return rl.Rectangle{
		X:      center_x,
		Y:      center_y,
		Width:  inner_width,
		Height: inner_height,
	}
}

func GetMenuButtonsVertical(menu rl.Rectangle, count int) []rl.Rectangle {
	btn_h := float32(rl.GetScreenHeight()) * button_height_ratio
	btn_w := float32(rl.GetScreenWidth()) * button_width_ratio

	buttons := make([]rl.Rectangle, count)

	btn_cont_h := (menu.Height / float32(len(buttons))) - (margin * 2)
	btn_cont_w := (menu.Width) - (margin * 2)

	var final_btn_h = btn_h
	var final_btn_w = btn_w

	var scaled_width bool = false

	// technically possible that the window goes bellow the size where 50 x 100 is possible,
	// then move onto centered scaled button
	if btn_cont_h < btn_h {
		final_btn_h = btn_cont_h
	}

	if btn_cont_w < btn_w {
		final_btn_w = btn_cont_w
		scaled_width = true
	}

	base_top_left_x := menu.X + margin
	base_top_left_y := menu.Y + margin

	for i := range len(buttons) {
		i_f32 := float32(i)
		top_left_y := base_top_left_y + ((btn_cont_h + (margin * 2)) * i_f32)

		// here I need to center the button
		middle_x := base_top_left_x + btn_cont_w/2
		// shifted since I need the left top corner to be slightly offset
		shifted_middle_x := middle_x - final_btn_w/2
		buttons[i].X = shifted_middle_x

		if scaled_width {
			buttons[i].Y = margin
		} else {
			middle_y := top_left_y + btn_cont_h/2
			shifted_middle_y := middle_y - final_btn_h/2

			buttons[i].Y = shifted_middle_y
		}

		buttons[i].Width = final_btn_w
		buttons[i].Height = final_btn_h
	}

	return buttons
}

func GetMenuButtonsHorizontal(menu rl.Rectangle, count int) []rl.Rectangle {
	btn_h := float32(rl.GetScreenHeight()) * button_height_ratio
	btn_w := float32(rl.GetScreenWidth()) * button_width_ratio

	buttons := make([]rl.Rectangle, count)

	btn_cont_h := (menu.Height) - (margin * 2)
	btn_cont_w := (menu.Width / float32(len(buttons))) - (margin * 2)

	var final_btn_h = btn_h
	var final_btn_w = btn_w

	var scaled_height bool = false

	// technically possible that the window goes bellow the size where 50 x 100 is possible,
	// then move onto centered scaled button
	if btn_cont_h < btn_h {
		final_btn_h = btn_cont_h
		scaled_height = true
	}

	if btn_cont_w < btn_w {
		final_btn_w = btn_cont_w
	}

	base_top_left_x := menu.X + margin
	base_top_left_y := menu.Y + margin

	for i := range len(buttons) {
		i_f32 := float32(i)
		top_left_x := base_top_left_x + ((btn_cont_w + (margin * 2)) * i_f32)

		if scaled_height {
			buttons[i].X = top_left_x
		} else {
			// here I need to center the button
			middle_x := top_left_x + btn_cont_w/2
			// shifted since I need the left top corner to be slightly offset
			shifted_middle_x := middle_x - final_btn_w/2

			buttons[i].X = shifted_middle_x
		}

		middle_y := base_top_left_y + btn_cont_h/2
		shifted_middle_y := middle_y - final_btn_h/2
		buttons[i].Y = shifted_middle_y

		buttons[i].Width = final_btn_w
		buttons[i].Height = final_btn_h
	}

	return buttons
}
