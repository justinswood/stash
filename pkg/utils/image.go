package utils

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"regexp"
	"time"

	// Decoders for the formats performer/studio images arrive in. image.Decode
	// only recognises formats that have been registered by import.
	_ "image/gif"

	_ "golang.org/x/image/webp"
)

// Timeout to get the image. Includes transfer time. May want to make this
// configurable at some point.
const imageGetTimeout = time.Second * 60

// Quality used when a cropped JPEG is re-encoded. High enough that a single
// crop is visually lossless, without writing a needlessly large blob.
const croppedJPEGQuality = 92

const base64RE = `^data:.+\/(.+);base64,(.*)$`

// ProcessImageInput transforms an image string either from a base64 encoded
// string, or from a URL, and returns the image as a byte slice
func ProcessImageInput(ctx context.Context, imageInput string) ([]byte, error) {
	if imageInput == "" {
		return []byte{}, nil
	}

	regex := regexp.MustCompile(base64RE)
	if regex.MatchString(imageInput) {
		d, err := ProcessBase64Image(imageInput)
		return d, err
	}

	// assume input is a URL. Read it.
	return ReadImageFromURL(ctx, imageInput)
}

// ReadImageFromURL returns image data from a URL
func ReadImageFromURL(ctx context.Context, url string) ([]byte, error) {
	client := &http.Client{
		Transport: &http.Transport{ // ignore insecure certificates
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			Proxy:           http.ProxyFromEnvironment,
		},

		Timeout: imageGetTimeout,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	// assume is a URL for now

	// set the host of the URL as the referer
	if req.URL.Scheme != "" {
		req.Header.Set("Referer", req.URL.Scheme+"://"+req.Host+"/")
	}
	req.Header.Set("User-Agent", getUserAgent())

	resp, err := client.Do(req)

	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("http error %d", resp.StatusCode)
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return body, nil
}

// ProcessBase64Image transforms a base64 encoded string from a form post and
// returns the image itself as a byte slice.
func ProcessBase64Image(imageString string) ([]byte, error) {
	if imageString == "" {
		return nil, fmt.Errorf("empty image string")
	}

	regex := regexp.MustCompile(base64RE)
	matches := regex.FindStringSubmatch(imageString)
	var encodedString string
	if len(matches) > 2 {
		encodedString = regex.FindStringSubmatch(imageString)[2]
	} else {
		encodedString = imageString
	}
	imageData, err := GetDataFromBase64String(encodedString)
	if err != nil {
		return nil, err
	}

	return imageData, nil
}

// GetDataFromBase64String returns the given base64 encoded string as a byte slice
func GetDataFromBase64String(encodedString string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(encodedString)
}

// GetBase64StringFromData returns the given byte slice as a base64 encoded string
func GetBase64StringFromData(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

// CropImage cuts the given rectangle out of the supplied image data and
// re-encodes it. x/y/width/height are in source-image pixels, and the rectangle
// is clamped to the image bounds so an over-large or partly out-of-range
// request yields a smaller crop rather than an error or transparent padding.
//
// This exists because cropping in the browser is not dependable: producing the
// cropped pixels there requires reading them back out of a canvas, and browsers
// with anti-fingerprinting protection (Firefox/Gecko with canvas protection on,
// as Zen ships by default) return blank data for any canvas beyond a small size
// threshold — silently, with no error. Cropping here is unaffected by whatever
// the client browser is willing to hand back.
func CropImage(data []byte, x, y, width, height int) ([]byte, error) {
	src, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decoding image: %w", err)
	}

	bounds := src.Bounds()
	rect := image.Rect(
		bounds.Min.X+x,
		bounds.Min.Y+y,
		bounds.Min.X+x+width,
		bounds.Min.Y+y+height,
	).Intersect(bounds)
	if rect.Empty() {
		return nil, fmt.Errorf(
			"crop region %dx%d at %d,%d lies outside the %dx%d image",
			width, height, x, y, bounds.Dx(), bounds.Dy(),
		)
	}

	// The concrete types image.Decode returns all implement SubImage, which is a
	// cheap view onto the original pixels. Copy only if one somehow doesn't.
	var cropped image.Image
	if sub, ok := src.(interface {
		SubImage(r image.Rectangle) image.Image
	}); ok {
		cropped = sub.SubImage(rect)
	} else {
		dst := image.NewRGBA(image.Rect(0, 0, rect.Dx(), rect.Dy()))
		draw.Draw(dst, dst.Bounds(), src, rect.Min, draw.Src)
		cropped = dst
	}

	// Keep photographs as JPEG. Re-encoding them as PNG turns a 200KB portrait
	// into several megabytes for no visible gain.
	var buf bytes.Buffer
	if format == "jpeg" {
		err = jpeg.Encode(&buf, cropped, &jpeg.Options{Quality: croppedJPEGQuality})
	} else {
		err = png.Encode(&buf, cropped)
	}
	if err != nil {
		return nil, fmt.Errorf("encoding cropped image: %w", err)
	}

	return buf.Bytes(), nil
}

func ServeImage(w http.ResponseWriter, r *http.Request, image []byte) {
	contentType := http.DetectContentType(image)
	if contentType == "text/xml; charset=utf-8" || contentType == "text/plain; charset=utf-8" {
		contentType = "image/svg+xml"
	}

	w.Header().Set("Content-Type", contentType)
	ServeStaticContent(w, r, image)
}
