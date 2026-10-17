package analysis

import (
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
	"os"
)

// DetectColor เฉลี่ยสีจากจุดตัวอย่างประมาณ 64 x 64 จุด แล้วคืนชื่อสีเดียว
// เป็นการดูค่า RGB เท่านั้น ไม่วิเคราะห์วัตถุ แนวภาพ หรือปีที่ถ่าย
func DetectColor(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	picture, _, err := image.Decode(file)
	if err != nil {
		return "", fmt.Errorf("อ่านรูปเพื่อวิเคราะห์สีไม่ได้: %w", err)
	}
	bounds := picture.Bounds()
	stepX, stepY := max(1, bounds.Dx()/64), max(1, bounds.Dy()/64)
	var red, green, blue, count int64
	for y := bounds.Min.Y; y < bounds.Max.Y; y += stepY {
		for x := bounds.Min.X; x < bounds.Max.X; x += stepX {
			pixel := color.NRGBAModel.Convert(picture.At(x, y)).(color.NRGBA)
			if pixel.A < 128 {
				continue
			}
			red += int64(pixel.R)
			green += int64(pixel.G)
			blue += int64(pixel.B)
			count++
		}
	}
	if count == 0 {
		return "", nil
	}
	r, g, b := red/count, green/count, blue/count
	// สีใกล้กันทั้งสามช่องเป็นสีขาว ดำ หรือเทา
	if max(r, g, b)-min(r, g, b) < 30 {
		if r > 220 {
			return "สีขาว", nil
		}
		if r < 35 {
			return "สีดำ", nil
		}
		return "สีเทา", nil
	}
	// สีผสมพื้นฐาน: เหลือง/ส้ม ม่วง และฟ้า
	if r > b*5/4 && g > b*5/4 && min(r, g)*2 >= max(r, g) {
		if r > g*3/2 {
			return "สีส้ม", nil
		}
		return "สีเหลือง", nil
	}
	if r > g*5/4 && b > g*5/4 && min(r, b)*2 >= max(r, b) {
		return "สีม่วง", nil
	}
	if g > r*5/4 && b > r*5/4 && min(g, b)*2 >= max(g, b) {
		return "สีฟ้า", nil
	}
	if r >= g && r >= b {
		return "สีแดง", nil
	}
	if g >= r && g >= b {
		return "สีเขียว", nil
	}
	return "สีน้ำเงิน", nil
}
