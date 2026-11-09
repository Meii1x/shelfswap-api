package auth

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// รัน: go test ./auth -count=1 -v
var secret = []byte("0123456789abcdef0123456789abcdef")

func issuer() *Issuer { return &Issuer{Secret: secret, TTL: time.Hour} }

func TestRealTokenPasses(t *testing.T) {
	tok, err := issuer().Issue(7, "member")
	if err != nil {
		t.Fatal(err)
	}
	id, role, err := issuer().Parse(tok)
	if err != nil || id != 7 || role != "member" {
		t.Fatalf("ตั๋วจริงต้องผ่าน ได้ id=%d role=%q err=%v", id, role, err)
	}
}

func sign(t *testing.T, method jwt.SigningMethod, key any, c jwt.Claims) string {
	t.Helper()
	s, err := jwt.NewWithClaims(method, c).SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func claims(exp *jwt.NumericDate, role string) Claims {
	return Claims{Role: role, RegisteredClaims: jwt.RegisteredClaims{Subject: "7", ExpiresAt: exp}}
}

// ตั๋วปลอม 6 แบบ ทุกใบต้องถูกปฏิเสธ
func TestForgedTokensRejected(t *testing.T) {
	future := jwt.NewNumericDate(time.Now().Add(time.Hour))
	past := jwt.NewNumericDate(time.Now().Add(-time.Hour))

	// 1) แก้ payload ของตั๋วจริงจาก member เป็น admin โดยไม่เซ็นใหม่
	real, _ := issuer().Issue(7, "member")
	parts := strings.Split(real, ".")
	parts[1] = base64.RawURLEncoding.EncodeToString([]byte(`{"role":"admin","sub":"7","exp":9999999999}`))
	edited := strings.Join(parts, ".")

	// 3) alg none
	none := sign(t, jwt.SigningMethodNone, jwt.UnsafeAllowNoneSignatureType, claims(future, "admin"))

	cases := map[string]string{
		"1 แก้ payload": edited,
		"2 เซ็นด้วยกุญแจอื่น": sign(t, jwt.SigningMethodHS256, []byte("another-key-another-key-another!!"), claims(future, "admin")),
		"3 alg none":    none,
		"4 HS512":       sign(t, jwt.SigningMethodHS512, secret, claims(future, "admin")),
		"5 ไม่มี exp":   sign(t, jwt.SigningMethodHS256, secret, claims(nil, "admin")),
		"6 หมดอายุแล้ว": sign(t, jwt.SigningMethodHS256, secret, claims(past, "admin")),
	}
	for name, tok := range cases {
		if _, _, err := issuer().Parse(tok); err == nil {
			t.Errorf("%s: ตั๋วปลอมหลุดผ่านได้!", name)
		}
	}
}
