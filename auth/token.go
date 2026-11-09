package auth

import (
	"errors"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims คือข้อมูลที่เขียนไว้บนตั๋ว ใครก็ถอดอ่านได้ (base64) ห้ามใส่ของลับ
type Claims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
}

// Issuer คือห้องออกตั๋ว ถือกุญแจลับไว้คนเดียว
type Issuer struct {
	Secret []byte
	TTL    time.Duration
}

// Issue ออกตั๋วให้ผู้ใช้หนึ่งคน
func (i *Issuer) Issue(userID int64, role string) (string, error) {
	now := time.Now()
	claims := Claims{
		Role: role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.FormatInt(userID, 10),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(i.TTL)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(i.Secret)
}

// Parse ตรวจตั๋ว ถ้าผ่านคืนเลขผู้ใช้กับบทบาทที่เขียนไว้บนตั๋ว
func (i *Issuer) Parse(tokenString string) (int64, string, error) {
	var c Claims
	_, err := jwt.ParseWithClaims(tokenString, &c,
		func(t *jwt.Token) (any, error) { return i.Secret, nil },
		jwt.WithValidMethods([]string{"HS256"}), // กัน alg none และ HS512
		jwt.WithExpirationRequired(),            // กันตั๋วไม่มีวันหมดอายุ
	)
	if err != nil {
		return 0, "", err
	}
	id, err := strconv.ParseInt(c.Subject, 10, 64)
	if err != nil || id <= 0 {
		return 0, "", errors.New("bad subject")
	}
	return id, c.Role, nil
}
