package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"slices"
)

// bmp is a paletted, uncompressed BMP as CrossPoint writes its sleep covers:
// 2 bits per pixel and top-down rows in practice, but any of 1, 2, 4 or 8
// bits per pixel and either row order parse.
type bmp struct {
	width, height int
	bpp           int
	topDown       bool
	palette       []byte   // raw 4-byte BGRX entries
	rows          [][]byte // packed rows, top of the image first, each rowBytes long
}

func rowBytes(width, bpp int) int { return (width*bpp + 31) / 32 * 4 }

func parseBMP(b []byte) (*bmp, error) {
	if len(b) < 54 || b[0] != 'B' || b[1] != 'M' {
		return nil, errors.New("not a BMP file")
	}
	le := binary.LittleEndian
	offset := int(le.Uint32(b[10:]))
	headerSize := int(le.Uint32(b[14:]))
	width := int(int32(le.Uint32(b[18:])))
	height := int(int32(le.Uint32(b[22:])))
	bpp := int(le.Uint16(b[28:]))
	compression := le.Uint32(b[30:])

	if headerSize < 40 || offset < 14+headerSize || offset > len(b) {
		return nil, errors.New("bad BMP header")
	}
	if bpp != 1 && bpp != 2 && bpp != 4 && bpp != 8 {
		return nil, fmt.Errorf("unsupported %d bits per pixel", bpp)
	}
	if compression != 0 {
		return nil, fmt.Errorf("unsupported compression %d", compression)
	}
	img := &bmp{width: width, height: height, bpp: bpp, topDown: height < 0}
	if img.topDown {
		img.height = -height
	}
	if img.width <= 0 || img.height <= 0 {
		return nil, errors.New("bad BMP size")
	}
	stride := rowBytes(img.width, bpp)
	if len(b) < offset+stride*img.height {
		return nil, errors.New("BMP pixel data is short")
	}
	img.palette = slices.Clone(b[14+headerSize : offset])
	img.rows = make([][]byte, img.height)
	for y := range img.rows {
		fileRow := y
		if !img.topDown {
			fileRow = img.height - 1 - y
		}
		start := offset + fileRow*stride
		img.rows[y] = slices.Clone(b[start : start+stride])
	}
	return img, nil
}

func (img *bmp) encode() []byte {
	stride := rowBytes(img.width, img.bpp)
	offset := 14 + 40 + len(img.palette)
	out := make([]byte, offset+stride*img.height)
	le := binary.LittleEndian
	out[0], out[1] = 'B', 'M'
	le.PutUint32(out[2:], uint32(len(out)))
	le.PutUint32(out[10:], uint32(offset))
	le.PutUint32(out[14:], 40)
	le.PutUint32(out[18:], uint32(int32(img.width)))
	height := int32(img.height)
	if img.topDown {
		height = -height
	}
	le.PutUint32(out[22:], uint32(height))
	le.PutUint16(out[26:], 1)
	le.PutUint16(out[28:], uint16(img.bpp))
	le.PutUint32(out[34:], uint32(stride*img.height))
	le.PutUint32(out[46:], uint32(len(img.palette)/4))
	copy(out[54:], img.palette)
	for y, row := range img.rows {
		fileRow := y
		if !img.topDown {
			fileRow = img.height - 1 - y
		}
		copy(out[offset+fileRow*stride:], row)
	}
	return out
}

func (img *bmp) pixel(row []byte, x int) byte {
	bit := x * img.bpp
	shift := 8 - img.bpp - bit%8
	return row[bit/8] >> shift & (1<<img.bpp - 1)
}

func (img *bmp) setPixel(row []byte, x int, v byte) {
	bit := x * img.bpp
	shift := 8 - img.bpp - bit%8
	mask := byte(1<<img.bpp-1) << shift
	row[bit/8] = row[bit/8]&^mask | v<<shift&mask
}

// mirror maps a position outside [0, n) back into it by reflecting at the
// edges, so the padding repeats the cover's edge flipped like a mirror.
func mirror(i, n int) int {
	for i < 0 || i >= n {
		if i < 0 {
			i = -i - 1
		} else {
			i = 2*n - i - 1
		}
	}
	return i
}

// fill pads img to width x height with mirrored edges. The cover stays where
// CrossPoint centres it, (screen - image) / 2, so only the bars change. One
// side must already match the screen: that is what Fit mode produces.
func fill(img *bmp, width, height int) (*bmp, error) {
	if !(img.width == width && img.height <= height) && !(img.height == height && img.width <= width) {
		return nil, fmt.Errorf("%dx%d does not fit %dx%d along one side", img.width, img.height, width, height)
	}
	out := &bmp{width: width, height: height, bpp: img.bpp, topDown: img.topDown, palette: img.palette}
	offX, offY := (width-img.width)/2, (height-img.height)/2
	stride := rowBytes(width, img.bpp)
	out.rows = make([][]byte, height)
	for y := range out.rows {
		src := img.rows[mirror(y-offY, img.height)]
		if width == img.width {
			out.rows[y] = slices.Clone(src)
			continue
		}
		row := make([]byte, stride)
		for x := range width {
			out.setPixel(row, x, img.pixel(src, mirror(x-offX, img.width)))
		}
		out.rows[y] = row
	}
	return out, nil
}
