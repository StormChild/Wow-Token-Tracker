package assets

import (
	"bytes"
	"encoding/binary"
	"image/png"
	"testing"

	"github.com/StormChild/Wow-Token-Tracker/internal/tracker"
)

func TestIconsAreICO(t *testing.T) {
	seen := map[string]bool{}
	for _, l := range []tracker.Light{tracker.Unknown, tracker.Green, tracker.Red} {
		b := Icon(l)
		if len(b) < 22 || !bytes.Equal(b[:4], []byte{0, 0, 1, 0}) {
			t.Fatalf("%v: bad ICO header", l)
		}
		if n := binary.LittleEndian.Uint16(b[4:6]); n != 1 {
			t.Fatalf("%v: image count %d", l, n)
		}
		size := binary.LittleEndian.Uint32(b[14:18])
		off := binary.LittleEndian.Uint32(b[18:22])
		if int(off+size) != len(b) {
			t.Fatalf("%v: directory size/offset wrong", l)
		}
		img, err := png.Decode(bytes.NewReader(b[off:]))
		if err != nil {
			t.Fatalf("%v: embedded PNG: %v", l, err)
		}
		if img.Bounds().Dx() != 32 || img.Bounds().Dy() != 32 {
			t.Fatalf("%v: size %v", l, img.Bounds())
		}
		seen[string(b)] = true
	}
	if len(seen) != 3 {
		t.Fatal("icons are not all different")
	}
}

func TestIconCentreColour(t *testing.T) {
	b := Icon(tracker.Green)
	off := binary.LittleEndian.Uint32(b[18:22])
	img, _ := png.Decode(bytes.NewReader(b[off:]))
	r, g, bl, a := img.At(16, 16).RGBA()
	if r>>8 != 0x2E || g>>8 != 0xCC || bl>>8 != 0x71 || a>>8 != 0xFF {
		t.Fatalf("centre = %x %x %x %x", r>>8, g>>8, bl>>8, a>>8)
	}
	if _, _, _, a := img.At(0, 0).RGBA(); a != 0 {
		t.Fatal("corner not transparent")
	}
}
