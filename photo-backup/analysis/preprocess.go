package analysis

import (
	"bytes"
	"image"
	"image/jpeg"
	"image/png"
	"os"
)

const cloudMaxImageSide = 1600

// readImageForCloudAI ลดขนาดเฉพาะ payload ที่จะส่ง Cloud ในหน่วยความจำ
// ไม่แก้ไขไฟล์ Backup และไม่เกี่ยวข้องกับ ContentHash
func readImageForCloudAI(path string) ([]byte, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	source, _, decodeErr := image.Decode(bytes.NewReader(data))
	if decodeErr != nil {
		return nil, "", decodeErr
	}
	bounds := source.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	if width <= cloudMaxImageSide && height <= cloudMaxImageSide {
		return data, imageMIME(path), nil
	}

	scale := float64(cloudMaxImageSide) / float64(width)
	if height > width {
		scale = float64(cloudMaxImageSide) / float64(height)
	}
	destWidth := max(1, int(float64(width)*scale))
	destHeight := max(1, int(float64(height)*scale))
	destination := image.NewRGBA(image.Rect(0, 0, destWidth, destHeight))
	for y := 0; y < destHeight; y++ {
		for x := 0; x < destWidth; x++ {
			sourceX := bounds.Min.X + x*width/destWidth
			sourceY := bounds.Min.Y + y*height/destHeight
			destination.Set(x, y, source.At(sourceX, sourceY))
		}
	}

	var encoded bytes.Buffer
	if imageMIME(path) == "image/png" {
		if err := png.Encode(&encoded, destination); err != nil {
			return nil, "", err
		}
		return encoded.Bytes(), "image/png", nil
	}
	if err := jpeg.Encode(&encoded, destination, &jpeg.Options{Quality: 80}); err != nil {
		return nil, "", err
	}
	return encoded.Bytes(), "image/jpeg", nil
}
