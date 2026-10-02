package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
)

// MaxJSONBody คือเพดานขนาด body ของ endpoint ที่รับ JSON (00-overview.md §2 ข้อ 11)
const MaxJSONBody = 1 << 20 // 1 MB

// ReadJSON อ่าน body เป็น JSON แบบเข้มงวดลงใน dst
//   - จำกัด body 1 MB ด้วย http.MaxBytesReader
//   - ฟิลด์ที่ไม่รู้จักถือว่าผิด (DisallowUnknownFields)
//   - ต้องมี JSON value เดียว ห้ามมีอะไรต่อท้าย
//
// ถ้าผิด จะตอบ 400 validation ให้เองแล้วคืน false (handler แค่ return)
//
// ต้องมี Content-Type: application/json ไม่อย่างนั้นตอบ 415 (CSRF ชั้นที่ 3 ของ 02 §5):
// ฟอร์มหรือ fetch แบบ simple request จากเว็บอื่นส่ง application/json ข้ามโดเมนไม่ได้
// ถ้าไม่ผ่าน CORS preflight ซึ่ง API นี้ไม่เปิดให้
func ReadJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	if mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || mt != "application/json" {
		Write(w, New("unsupported-media-type", "ต้องส่ง Content-Type: application/json"))
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, MaxJSONBody)
	if err := DecodeJSON(r.Body, dst); err != nil {
		WriteError(w, r, err)
		return false
	}
	return true
}

// DecodeJSON อ่าน JSON หนึ่งค่าแบบเข้มงวดจาก rd (ใช้กับ body และฟิลด์ data ของ multipart)
// ผิดรูปแบบ → *Problem 400 validation
func DecodeJSON(rd io.Reader, dst any) error {
	dec := json.NewDecoder(rd)
	dec.DisallowUnknownFields()

	err := dec.Decode(dst)
	if err == nil {
		// ลองอ่านต่ออีกครั้ง ต้องได้ io.EOF แปลว่าไม่มีข้อมูลต่อท้าย
		if extra := dec.Decode(&struct{}{}); !errors.Is(extra, io.EOF) {
			err = errTrailingData
		}
	}
	if err != nil {
		return decodeProblem(err)
	}
	return nil
}

var errTrailingData = errors.New("trailing data")

// decodeProblem แปลง error ของ decoder เป็น problem 400
// ใส่ errors[].field เมื่อหาชื่อฟิลด์ได้ ถ้าหาไม่ได้ให้ errors ว่างและอธิบายใน detail (00-overview.md §2 ข้อ 15)
func decodeProblem(err error) *Problem {
	var typeErr *json.UnmarshalTypeError
	var syntaxErr *json.SyntaxError
	var tooBig *http.MaxBytesError

	switch {
	case errors.Is(err, io.EOF):
		return Validation("body ว่าง ต้องเป็น JSON")
	case errors.As(err, &tooBig):
		return Validation("body ใหญ่เกิน 1 MB")
	case errors.As(err, &typeErr) && typeErr.Field != "":
		return Validation("ชนิดข้อมูลไม่ถูกต้อง",
			FieldError{Field: typeErr.Field, Message: "ต้องเป็น " + typeErr.Type.String()})
	case errors.As(err, &typeErr):
		return Validation("ชนิดข้อมูลไม่ถูกต้อง")
	case errors.As(err, &syntaxErr), errors.Is(err, io.ErrUnexpectedEOF):
		return Validation("JSON ไม่ถูกต้อง")
	case errors.Is(err, errTrailingData):
		return Validation("body ต้องมี JSON เพียงค่าเดียว")
	}

	// encoding/json ไม่มีชนิด error เฉพาะสำหรับฟิลด์ที่ไม่รู้จัก ต้องอ่านจากข้อความ
	// รูปแบบ: json: unknown field "owner_id"
	if rest, ok := strings.CutPrefix(err.Error(), "json: unknown field "); ok {
		field, uerr := strconv.Unquote(rest)
		if uerr != nil {
			field = rest
		}
		return Validation("มีฟิลด์ที่ไม่รู้จัก", FieldError{Field: field, Message: "ไม่รู้จักฟิลด์นี้"})
	}
	return Validation("อ่าน JSON ไม่ได้: " + err.Error())
}
