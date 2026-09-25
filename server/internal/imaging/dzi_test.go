package imaging

import (
	"image"
	"testing"
)

func TestDZILevels(t *testing.T) {
	d := DZI{Width: 3000, Height: 2051}
	if d.MaxLevel() != 12 {
		t.Errorf("max level %d, want 12 (2^12 = 4096 >= 3000)", d.MaxLevel())
	}
	if w, h := d.LevelSize(12); w != 3000 || h != 2051 {
		t.Errorf("full size %dx%d", w, h)
	}
	if w, h := d.LevelSize(11); w != 1500 || h != 1026 {
		t.Errorf("level 11 %dx%d", w, h)
	}
	if w, h := d.LevelSize(0); w != 1 || h != 1 {
		t.Errorf("level 0 %dx%d", w, h)
	}
}

func TestTilesOverlapByOne(t *testing.T) {
	d := DZI{Width: 600, Height: 300}
	l := d.MaxLevel()
	if got := d.TileRect(l, 0, 0); got != image.Rect(0, 0, 255, 255) {
		t.Errorf("first tile %v", got)
	}
	if got := d.TileRect(l, 1, 0); got != image.Rect(253, 0, 509, 255) {
		t.Errorf("second tile %v", got)
	}
	if got := d.TileRect(l, 2, 1); got != image.Rect(507, 253, 600, 300) {
		t.Errorf("corner tile %v", got)
	}
}
