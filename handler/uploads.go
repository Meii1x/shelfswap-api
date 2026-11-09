package handler

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// UploadImage POST /api/uploads  (ต้อง login)
//
// สัญญากับฝั่ง React:
//
//	Request : multipart/form-data มีไฟล์ในฟิลด์ชื่อ "file" + Authorization: Bearer <token>
//	Response: 201 {"url": "https://<project>.supabase.co/storage/v1/object/public/listing-images/..."}
//
// React เอา url ไปใส่ช่อง imageUrl ของประกาศ แล้วบันทึกประกาศเหมือนเดิม (endpoint ประกาศไม่ต้องแก้)
//
// ทำไมไม่เก็บไฟล์ไว้บน server เอง: Render แบบฟรีล้างดิสก์ทุกครั้งที่ deploy/หลับ รูปจะหาย
// จึงส่งต่อไปเก็บที่ Supabase Storage ด้วย secret key ที่อยู่บน Render เท่านั้น (ไม่หลุดไปฝั่ง React)

const maxUploadBytes = 5 << 20 // 5 MB

// ตรวจชนิดไฟล์จาก "เนื้อไฟล์" ไม่เชื่อนามสกุลหรือ Content-Type ที่ browser ส่งมา
var allowedImageTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
}

// timeout ต้องสั้นกว่า WriteTimeout ของ server (15 วิ) จะได้ตอบ error ได้ทันก่อนโดนตัด
var storageClient = &http.Client{Timeout: 12 * time.Second}

func randomName() string {
	b := make([]byte, 12)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func (h *Handler) UploadImage(w http.ResponseWriter, r *http.Request) {
	base := strings.TrimRight(os.Getenv("SUPABASE_URL"), "/")
	key := os.Getenv("SUPABASE_SERVICE_KEY")
	bucket := os.Getenv("SUPABASE_BUCKET")
	if bucket == "" {
		bucket = "listing-images"
	}
	if base == "" || key == "" {
		writeError(w, http.StatusServiceUnavailable, "UPLOAD_DISABLED", "ยังไม่ได้ตั้งค่าที่เก็บรูป ใช้ลิงก์รูปไปก่อน")
		return
	}

	// จำกัดขนาดทั้งคำขอ (เผื่อหัว multipart อีก 1 MB) กันคนส่งไฟล์ยักษ์มาถ่วง server
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes+1<<20)
	file, _, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "BAD_FILE", "ต้องส่งรูปในฟิลด์ชื่อ file และขนาดไม่เกิน 5 MB")
		return
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxUploadBytes+1))
	if err != nil {
		writeError(w, http.StatusBadRequest, "BAD_FILE", "อ่านไฟล์ไม่สำเร็จ")
		return
	}
	if len(data) > maxUploadBytes {
		writeError(w, http.StatusBadRequest, "FILE_TOO_LARGE", "รูปต้องมีขนาดไม่เกิน 5 MB")
		return
	}
	contentType := http.DetectContentType(data)
	ext, ok := allowedImageTypes[contentType]
	if !ok {
		writeError(w, http.StatusBadRequest, "BAD_FILE_TYPE", "รองรับเฉพาะรูป JPG, PNG หรือ WEBP")
		return
	}

	// ตั้งชื่อไฟล์เอง (ไม่ใช้ชื่อที่ผู้ใช้ส่งมา) แยกโฟลเดอร์ตามผู้ใช้ สุ่มชื่อกันชนกัน
	path := fmt.Sprintf("u%d/%s%s", me(r).ID, randomName(), ext)

	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost,
		base+"/storage/v1/object/"+bucket+"/"+path, bytes.NewReader(data))
	if err != nil {
		internal(w, err)
		return
	}
	req.Header.Set("apikey", key)
	if strings.HasPrefix(key, "eyJ") { // key แบบเก่า (service_role) เป็น JWT ต้องส่งใน Authorization ด้วย
		req.Header.Set("Authorization", "Bearer "+key)
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("x-upsert", "false")

	resp, err := storageClient.Do(req)
	if err != nil {
		internal(w, fmt.Errorf("supabase storage: %w", err))
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		internal(w, fmt.Errorf("supabase storage %d: %s", resp.StatusCode, msg))
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{
		"url": base + "/storage/v1/object/public/" + bucket + "/" + path,
	})
}
