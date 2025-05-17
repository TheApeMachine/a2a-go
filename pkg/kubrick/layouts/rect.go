package layouts

// Position represents a 2D coordinate in the terminal
type Position struct {
	Row int
	Col int
}

// Size represents dimensions in the terminal
type Size struct {
	Width  int
	Height int
}

// Rect represents a rectangular region in the terminal
type Rect struct {
	Pos  Position
	Size Size
}