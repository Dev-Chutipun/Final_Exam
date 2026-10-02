// Package auth ระบุผู้ใช้ปัจจุบันของคำขอ
//
// ตอนนี้มีแค่การอ่านและใส่ userID ใน context
// middleware จริงที่อ่าน cookie session แล้วเรียก WithUserID จะมาในส่วนที่ 2 (docs/02-auth.md)
// production code ไม่มี header หรือ query ใดๆ สำหรับสลับผู้ใช้
package auth

import "context"

type ctxKey struct{}

// WithUserID คืน context ที่มี userID ใช้โดย middleware ของ session (และ fake middleware ใน _test.go)
func WithUserID(ctx context.Context, userID int64) context.Context {
	return context.WithValue(ctx, ctxKey{}, userID)
}

// UserID คืน id ของผู้ใช้ที่ login อยู่ ถ้าไม่ได้ login คืน false
func UserID(ctx context.Context) (int64, bool) {
	id, ok := ctx.Value(ctxKey{}).(int64)
	return id, ok
}
