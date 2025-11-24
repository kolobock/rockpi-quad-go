package oled

import (
	"image"
	"image/color"
	"testing"
)

func TestClearImage(t *testing.T) {
	ctrl := &Controller{
		img: image.NewGray(image.Rect(0, 0, 128, 32)),
	}

	for y := 0; y < 32; y++ {
		for x := 0; x < 128; x++ {
			ctrl.img.SetGray(x, y, color.Gray{Y: 255})
		}
	}

	ctrl.clearImage()

	for y := 0; y < 32; y++ {
		for x := 0; x < 128; x++ {
			if ctrl.img.GrayAt(x, y).Y != 0 {
				t.Errorf("pixel at (%d, %d) = %v, want 0", x, y, ctrl.img.GrayAt(x, y).Y)
			}
		}
	}
}

func TestRotateImage180(t *testing.T) {
	ctrl := &Controller{}
	src := image.NewGray(image.Rect(0, 0, 4, 4))

	src.SetGray(0, 0, color.Gray{Y: 255})
	src.SetGray(3, 3, color.Gray{Y: 200})

	dst := ctrl.rotateImage180(src)

	if dst.GrayAt(3, 3).Y != 255 {
		t.Errorf("rotated pixel at (3,3) = %v, want 255", dst.GrayAt(3, 3).Y)
	}
	if dst.GrayAt(0, 0).Y != 200 {
		t.Errorf("rotated pixel at (0,0) = %v, want 200", dst.GrayAt(0, 0).Y)
	}
}

func TestConstants(t *testing.T) {
	if displayWidth != 128 {
		t.Errorf("displayWidth = %v, want 128", displayWidth)
	}
	if displayHeight != 32 {
		t.Errorf("displayHeight = %v, want 32", displayHeight)
	}
}
