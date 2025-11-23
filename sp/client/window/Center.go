package window

import rl "github.com/gen2brain/raylib-go/raylib"

type CenterComponent struct {
	bounds rl.Rectangle
	child  RGComponent
}

func NewCenterComponent(given_child RGComponent) *CenterComponent {
	return &CenterComponent{child: given_child}
}

func (c *CenterComponent) Calculate(bounds rl.Rectangle) {
	c.bounds = bounds
	c.child.Calculate(bounds)
	child_bounds := c.child.GetBounds()
	// if the child doesn't take up the entire space

	new_bounds := bounds
	recalculate := false
	// center X if needed
	if child_bounds.Width < bounds.Width {
		width_diff := (bounds.Width - child_bounds.Width) / 2
		new_bounds.X += width_diff
		new_bounds.Width = child_bounds.Width
		recalculate = true
	}

	// center Y if needed
	if child_bounds.Height < bounds.Height {
		height_diff := (bounds.Height - child_bounds.Height) / 2
		new_bounds.Y += height_diff
		new_bounds.Height = child_bounds.Height
		recalculate = true
	}

	if recalculate {
		c.child.Calculate(new_bounds)
	}
}

func (c *CenterComponent) Draw(eventChannel chan<- UIEvent) {
	c.child.Draw(eventChannel)
}

func (c *CenterComponent) GetBounds() rl.Rectangle {
	return c.bounds
}

func (c *CenterComponent) SetChild(child RGComponent) {
	c.child = child
}

func (c *CenterComponent) Rebuild(old RGComponent) {
	if old == nil {
		return
	}

	if oldCC, ok := old.(*CenterComponent); ok {
		c.child.Rebuild(oldCC.child)
	}
}
