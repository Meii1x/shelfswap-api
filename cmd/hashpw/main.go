// hashpw สร้าง bcrypt hash ไว้ใส่ใน seed.sql
// ใช้: go run ./cmd/hashpw "รหัสผ่าน"
package main

import (
	"fmt"
	"os"

	"shelfswap/auth"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Println(`ใช้: go run ./cmd/hashpw "รหัสผ่าน"`)
		os.Exit(1)
	}
	h, err := auth.HashPassword(os.Args[1])
	if err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}
	fmt.Println(h)
}
