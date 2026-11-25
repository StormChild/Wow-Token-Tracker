package assets

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"math"
	"sync"

	"github.com/StormChild/Wow-Token-Tracker/internal/tracker"
)

var colours = map[tracker.Light]color.NRGBA{
	tracker.Green:   {0x2E, 0xCC, 0x71, 0xFF},
	tracker.Red:     {0xE7, 0x4C, 0x3C, 0xFF},
	tracker.Unknown: {0x95, 0xA5, 0xA6, 0xFF},
}

var (
	once  sync.Once
	icons map[tracker.Light][]byte
)

// Icon returns a 32×32 tray icon (PNG-in-ICO) for l: a filled circle with a darker rim.
func Icon(l tracker.Light) []byte {
	once.Do(func() {
		icons = map[tracker.Light][]byte{}
		for k, c := range colours {
			icons[k] = ico(circle(c))
		}
	})
	if b, ok := icons[l]; ok {
		return b
	}
	return icons[tracker.Unknown]
}

func circle(c color.NRGBA) image.Image {
	const size, r = 32, 14.5
	rim := color.NRGBA{c.R * 3 / 4, c.G * 3 / 4, c.B * 3 / 4, 0xFF}
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	for y := range size {
		for x := range size {
			d := math.Hypot(float64(x)+0.5-size/2, float64(y)+0.5-size/2)
			switch {
			case d <= r-2:
				img.SetNRGBA(x, y, c)
			case d <= r:
				img.SetNRGBA(x, y, rim)
			case d <= r+1: // anti-aliased edge
				a := rim
				a.A = uint8(255 * (r + 1 - d))
				img.SetNRGBA(x, y, a)
			}
		}
	}
	return img
}

// ico wraps a PNG in a single-image ICO container (supported since Windows Vista).
func ico(img image.Image) []byte {
	var p bytes.Buffer
	if err := png.Encode(&p, img); err != nil {
		panic(err) // encoding an in-memory NRGBA cannot fail
	}
	var b bytes.Buffer
	w := func(v any) { _ = binary.Write(&b, binary.LittleEndian, v) }
	w(uint16(0)) // reserved
	w(uint16(1)) // type: icon
	w(uint16(1)) // count
	sz := img.Bounds().Dx()
	b.WriteByte(byte(sz))
	b.WriteByte(byte(img.Bounds().Dy()))
	b.WriteByte(0)     // palette
	b.WriteByte(0)     // reserved
	w(uint16(1))       // planes
	w(uint16(32))      // bpp
	w(uint32(p.Len())) // data size
	w(uint32(6 + 16))  // data offset
	b.Write(p.Bytes())
	return b.Bytes()
}
