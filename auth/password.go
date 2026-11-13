package auth

import "golang.org/x/crypto/bcrypt"

// HashPassword แปลงรหัสผ่านเป็นลายนิ้วมือ bcrypt ที่ย้อนกลับไม่ได้ (มี salt ในตัว hash ซ้ำได้ผลไม่เหมือนเดิม)
func HashPassword(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	return string(b), err
}

// CheckPassword ตอบว่ารหัสที่พิมพ์มาตรงกับลายนิ้วมือที่เก็บไว้ไหม
func CheckPassword(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}
