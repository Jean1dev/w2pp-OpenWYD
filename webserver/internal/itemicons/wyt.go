package itemicons

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
)

var (
	standardWYTWrapper    = []byte("WT10")
	proprietaryWYTWrapper = []byte{0x00, 0x4b, 0x6d, 0x4b}
)

// decodeWYT decodes the four-byte WYD wrapper followed by a true-color TGA.
// Some shipped itemicon atlases use a proprietary marker but retain the same
// TGA payload, so only that observed marker is accepted alongside WT10.
func decodeWYT(data []byte) (*image.NRGBA, error) {
	if len(data) < 22 {
		return nil, fmt.Errorf("WYT has %d bytes, want at least 22", len(data))
	}
	if !bytes.Equal(data[:4], standardWYTWrapper) && !bytes.Equal(data[:4], proprietaryWYTWrapper) {
		return nil, fmt.Errorf("unknown WYT wrapper % x", data[:4])
	}
	const base = 4
	idLength := int(data[base])
	colorMapType := data[base+1]
	imageType := data[base+2]
	width := int(uint16(data[base+12]) | uint16(data[base+13])<<8)
	height := int(uint16(data[base+14]) | uint16(data[base+15])<<8)
	bits := int(data[base+16])
	descriptor := data[base+17]
	if colorMapType != 0 || (imageType != 2 && imageType != 10) || (bits != 24 && bits != 32) || width == 0 || height == 0 || width > 8192 || height > 8192 {
		return nil, fmt.Errorf("unsupported TGA type=%d cmap=%d bits=%d size=%dx%d", imageType, colorMapType, bits, width, height)
	}
	pixelOffset := base + 18 + idLength
	if pixelOffset > len(data) {
		return nil, fmt.Errorf("TGA id extends beyond input")
	}
	pixelSize := bits / 8
	pixelCount := width * height
	pixels, err := decodeTGAPixels(data[pixelOffset:], int(imageType), pixelSize, pixelCount)
	if err != nil {
		return nil, err
	}

	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	topOrigin := descriptor&0x20 != 0
	rightOrigin := descriptor&0x10 != 0
	for sourceIndex, p := range pixels {
		x := sourceIndex % width
		y := sourceIndex / width
		if rightOrigin {
			x = width - 1 - x
		}
		if !topOrigin {
			y = height - 1 - y
		}
		img.SetNRGBA(x, y, p)
	}
	return img, nil
}

func decodeTGAPixels(data []byte, imageType, pixelSize, pixelCount int) ([]color.NRGBA, error) {
	out := make([]color.NRGBA, 0, pixelCount)
	offset := 0
	readPixel := func() (color.NRGBA, error) {
		if offset+pixelSize > len(data) {
			return color.NRGBA{}, fmt.Errorf("truncated TGA pixel data at byte %d", offset)
		}
		p := color.NRGBA{B: data[offset], G: data[offset+1], R: data[offset+2], A: 0xff}
		if pixelSize == 4 {
			p.A = data[offset+3]
		}
		offset += pixelSize
		if p.R == 0 && p.G == 0 && p.B == 0 {
			p.A = 0
		}
		return p, nil
	}

	if imageType == 2 {
		for len(out) < pixelCount {
			p, err := readPixel()
			if err != nil {
				return nil, err
			}
			out = append(out, p)
		}
		return out, nil
	}

	for len(out) < pixelCount {
		if offset >= len(data) {
			return nil, fmt.Errorf("truncated TGA RLE packet at byte %d", offset)
		}
		header := data[offset]
		offset++
		count := int(header&0x7f) + 1
		if len(out)+count > pixelCount {
			return nil, fmt.Errorf("TGA RLE packet overruns %d pixels", pixelCount)
		}
		if header&0x80 != 0 {
			p, err := readPixel()
			if err != nil {
				return nil, err
			}
			for range count {
				out = append(out, p)
			}
			continue
		}
		for range count {
			p, err := readPixel()
			if err != nil {
				return nil, err
			}
			out = append(out, p)
		}
	}
	return out, nil
}
