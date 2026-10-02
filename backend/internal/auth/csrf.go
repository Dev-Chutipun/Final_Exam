package auth

import (
	"net/http"

	"devpool/internal/httpx"
)

// CSRF ครอบ mux ทั้งหมดด้วย http.CrossOriginProtection (ชั้นที่ 2 ของ 02 §5)
//
// ชั้นที่ 1 คือ cookie SameSite=Lax และชั้นที่ 3 คือบังคับ Content-Type: application/json
// ใน httpx.ReadJSON (เว็บอื่นส่ง JSON ข้ามโดเมนแบบ simple request ไม่ได้)
//
// เพิ่ม PUBLIC_BASE_URL เป็น trusted origin เพราะคำขอผ่าน rewrite ของ Next.js
// Go จะเห็น Host เป็น localhost:8080 ขณะที่ Origin คือ localhost:3000
func CSRF(publicBaseURL string, next http.Handler) (http.Handler, error) {
	cop := http.NewCrossOriginProtection()
	if err := cop.AddTrustedOrigin(publicBaseURL); err != nil {
		return nil, err
	}
	cop.SetDenyHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httpx.Write(w, httpx.New("cross-origin-request", "คำขอที่เปลี่ยนข้อมูลต้องมาจากหน้าเว็บของระบบเท่านั้น"))
	}))
	return cop.Handler(next), nil
}
