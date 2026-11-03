package main

import (
	"bytes"
	"testing"
)

// grid builds a 2-bit BMP whose pixel at (x, y) has value f(x, y).
func grid(w, h int, topDown bool, f func(x, y int) byte) *bmp {
	img := &bmp{width: w, height: h, bpp: 2, topDown: topDown,
		palette: []byte{0, 0, 0, 0, 0x55, 0x55, 0x55, 0, 0xaa, 0xaa, 0xaa, 0, 0xff, 0xff, 0xff, 0}}
	img.rows = make([][]byte, h)
	for y := range h {
		img.rows[y] = make([]byte, rowBytes(w, 2))
		for x := range w {
			img.setPixel(img.rows[y], x, f(x, y))
		}
	}
	return img
}

func values(img *bmp) [][]byte {
	out := make([][]byte, img.height)
	for y, row := range img.rows {
		for x := range img.width {
			out[y] = append(out[y], img.pixel(row, x))
		}
	}
	return out
}

func TestEncodeParseRoundTrip(t *testing.T) {
	for _, topDown := range []bool{true, false} {
		src := grid(5, 3, topDown, func(x, y int) byte { return byte(x+y) % 4 })
		got, err := parseBMP(src.encode())
		if err != nil {
			t.Fatal(err)
		}
		if got.width != 5 || got.height != 3 || got.topDown != topDown || !bytes.Equal(got.palette, src.palette) {
			t.Fatalf("topDown=%v: header changed: %+v", topDown, got)
		}
		if !equal(values(got), values(src)) {
			t.Fatalf("topDown=%v: pixels changed", topDown)
		}
	}
}

func TestFillMirrorsRows(t *testing.T) {
	// Rows 0..3 have value y; padding 4 rows to 8 adds 2 rows at each end.
	src := grid(3, 4, true, func(x, y int) byte { return byte(y) })
	got, err := fill(src, 3, 8)
	if err != nil {
		t.Fatal(err)
	}
	want := []byte{1, 0, 0, 1, 2, 3, 3, 2}
	for y, row := range values(got) {
		for _, v := range row {
			if v != want[y] {
				t.Fatalf("row %d = %v, want %d", y, row, want[y])
			}
		}
	}
}

func TestFillMirrorsColumns(t *testing.T) {
	// Columns 0..2 have value x; padding 3 columns to 7 adds 2 at each side.
	src := grid(3, 2, false, func(x, y int) byte { return byte(x) })
	got, err := fill(src, 7, 2)
	if err != nil {
		t.Fatal(err)
	}
	want := []byte{1, 0, 0, 1, 2, 2, 1}
	for _, row := range values(got) {
		if !bytes.Equal(row, want) {
			t.Fatalf("row = %v, want %v", row, want)
		}
	}
}

func TestFillKeepsCoverCentred(t *testing.T) {
	// CrossPoint draws a 480x720 cover at y = (800-720)/2 = 40.
	src := grid(4, 720, true, func(x, y int) byte { return byte(y % 4) })
	got, err := fill(src, 4, 800)
	if err != nil {
		t.Fatal(err)
	}
	for y := range 720 {
		if !bytes.Equal(got.rows[y+40], src.rows[y]) {
			t.Fatalf("cover row %d moved", y)
		}
	}
}

func TestFillRejectsCoverThatIsNotFit(t *testing.T) {
	src := grid(3, 3, true, func(x, y int) byte { return 0 })
	if _, err := fill(src, 4, 4); err == nil {
		t.Fatal("padding both sides should fail")
	}
	if _, err := fill(src, 3, 2); err == nil {
		t.Fatal("a cover taller than the screen should fail")
	}
}

func equal(a, b [][]byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !bytes.Equal(a[i], b[i]) {
			return false
		}
	}
	return true
}
