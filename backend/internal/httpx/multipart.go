package httpx

import (
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
)

// MaxImageSize คือขนาดสูงสุดของรูปหนึ่งรูป (03 §3)
const MaxImageSize = 2 << 20 // 2 MB

// allowedImageTypes คือชนิดรูปที่รับ ตรวจจากเนื้อไฟล์ ไม่เชื่อนามสกุลหรือ Content-Type ที่ส่งมา
// ไม่รับ SVG เพราะเป็น XML ที่ฝังสคริปต์ได้
var allowedImageTypes = map[string]bool{"image/jpeg": true, "image/png": true, "image/webp": true}

// Image คือรูปที่อ่านและตรวจแล้ว
type Image struct {
	ContentType string
	Data        []byte
}

// ParseMultipart อ่าน body แบบ multipart/form-data โดยจำกัดขนาดรวม maxBytes
// ไม่ใช่ multipart → 400 · เกินเพดาน → 413 image-too-large
// ผู้เรียกต้อง defer r.MultipartForm.RemoveAll() เพื่อลบไฟล์ชั่วคราว
func ParseMultipart(w http.ResponseWriter, r *http.Request, maxBytes int64) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
	err := r.ParseMultipartForm(maxBytes)
	var tooBig *http.MaxBytesError
	switch {
	case errors.As(err, &tooBig):
		return New("image-too-large", "ขนาดข้อมูลรวมเกิน "+strconv.FormatInt(maxBytes>>20, 10)+" MB")
	case err != nil:
		return Validation("ต้องส่งเป็น multipart/form-data")
	}
	return nil
}

// ReadImage อ่านไฟล์รูปหนึ่งไฟล์ ตรวจขนาด (413) และชนิดจากเนื้อไฟล์ด้วย http.DetectContentType (415)
func ReadImage(fh *multipart.FileHeader) (Image, error) {
	if fh.Size > MaxImageSize {
		return Image{}, New("image-too-large", fh.Filename+" ใหญ่เกิน 2 MB")
	}
	f, err := fh.Open()
	if err != nil {
		return Image{}, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, MaxImageSize+1))
	if err != nil {
		return Image{}, err
	}
	if len(data) > MaxImageSize {
		return Image{}, New("image-too-large", fh.Filename+" ใหญ่เกิน 2 MB")
	}
	if len(data) == 0 {
		return Image{}, New("unsupported-image-type", fh.Filename+" เป็นไฟล์ว่าง")
	}
	ct := http.DetectContentType(data)
	if !allowedImageTypes[ct] {
		return Image{}, New("unsupported-image-type", fh.Filename+" ไม่ใช่รูป JPEG, PNG หรือ WebP")
	}
	return Image{ContentType: ct, Data: data}, nil
}
