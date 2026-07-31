package utils

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
)

// encodeTestImage builds a horizontal red/blue split so a crop can be checked
// for having taken the region it was asked for rather than merely the right
// size.
func encodeTestImage(t *testing.T, w, h int, asJPEG bool) []byte {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if x < w/2 {
				img.Set(x, y, color.RGBA{R: 255, A: 255})
			} else {
				img.Set(x, y, color.RGBA{B: 255, A: 255})
			}
		}
	}

	var buf bytes.Buffer
	var err error
	if asJPEG {
		err = jpeg.Encode(&buf, img, nil)
	} else {
		err = png.Encode(&buf, img)
	}
	if err != nil {
		t.Fatalf("encoding test image: %v", err)
	}
	return buf.Bytes()
}

func TestCropImageReturnsRequestedRegion(t *testing.T) {
	for _, asJPEG := range []bool{true, false} {
		data := encodeTestImage(t, 800, 1200, asJPEG)

		out, err := CropImage(data, 100, 200, 400, 600)
		if err != nil {
			t.Fatalf("jpeg=%v: %v", asJPEG, err)
		}

		cfg, _, err := image.DecodeConfig(bytes.NewReader(out))
		if err != nil {
			t.Fatalf("jpeg=%v: decoding result: %v", asJPEG, err)
		}
		if cfg.Width != 400 || cfg.Height != 600 {
			t.Errorf("jpeg=%v: got %dx%d, want 400x600", asJPEG, cfg.Width, cfg.Height)
		}
	}
}

// The crop must be taken from the requested offset. Cropping the right-hand
// half of a red/blue split must come back blue — a crop that silently returned
// the top-left corner would still have the right dimensions.
func TestCropImageCropsAtTheRequestedOffset(t *testing.T) {
	data := encodeTestImage(t, 800, 1200, false)

	out, err := CropImage(data, 600, 0, 100, 100)
	if err != nil {
		t.Fatal(err)
	}

	img, _, err := image.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	r, g, b, _ := img.At(img.Bounds().Min.X+50, img.Bounds().Min.Y+50).RGBA()
	if b <= r || g > 0x1000 {
		t.Errorf("got rgb(%d,%d,%d), want the blue half of the source", r>>8, g>>8, b>>8)
	}
}

// A selection rounded a pixel past the edge should yield a smaller crop rather
// than an error, so the UI never has to be pixel-perfect.
func TestCropImageClampsToImageBounds(t *testing.T) {
	data := encodeTestImage(t, 800, 1200, true)

	out, err := CropImage(data, 700, 1100, 400, 600)
	if err != nil {
		t.Fatal(err)
	}

	cfg, _, err := image.DecodeConfig(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Width != 100 || cfg.Height != 100 {
		t.Errorf("got %dx%d, want the clamped 100x100", cfg.Width, cfg.Height)
	}
}

// A region entirely outside the image must fail loudly. Silently writing an
// empty image over the performer's only photo is the exact bug this replaced.
func TestCropImageRejectsRegionOutsideImage(t *testing.T) {
	data := encodeTestImage(t, 800, 1200, true)

	if _, err := CropImage(data, 900, 1300, 100, 100); err == nil {
		t.Fatal("expected an error for a region entirely outside the image")
	}
}

// Photographs must stay JPEG. Re-encoding them as PNG inflates a portrait from
// a couple of hundred KB to several MB in the blob store.
func TestCropImageKeepsJPEGEncoding(t *testing.T) {
	data := encodeTestImage(t, 800, 1200, true)

	out, err := CropImage(data, 0, 0, 800, 1200)
	if err != nil {
		t.Fatal(err)
	}

	_, format, err := image.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	if format != "jpeg" {
		t.Errorf("got %q, want jpeg", format)
	}
}

func TestCropImageRejectsUndecodableData(t *testing.T) {
	if _, err := CropImage([]byte("not an image"), 0, 0, 10, 10); err == nil {
		t.Fatal("expected an error for undecodable data")
	}
}
