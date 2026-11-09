-- ShelfSwap database schema (PostgreSQL 14+ / Supabase)
-- รันไฟล์นี้ก่อน แล้วค่อยรัน seed.sql
-- บน Supabase: เปิด SQL Editor > New query > วางทั้งไฟล์ > Run

BEGIN;

-- ล้างของเก่า (รันซ้ำได้) ลำดับต้องลบลูกก่อนแม่
DROP TABLE IF EXISTS reservations;
DROP TABLE IF EXISTS listings;
DROP TABLE IF EXISTS categories;
DROP TABLE IF EXISTS users;

-- ---------------------------------------------------------------------------
-- users: สมาชิกทุกคน รวม admin  (Guest = คนที่ไม่มีแถวในตารางนี้/ไม่ได้ login)
-- ---------------------------------------------------------------------------
CREATE TABLE users (
    id             BIGSERIAL    PRIMARY KEY,
    email          VARCHAR(255) NOT NULL UNIQUE,          -- เก็บเป็นตัวเล็กเสมอ (handler แปลงให้)
    password_hash  TEXT         NOT NULL,                 -- bcrypt ห้ามเก็บรหัสจริง
    display_name   VARCHAR(50)  NOT NULL,
    phone          VARCHAR(20)  NOT NULL DEFAULT '',      -- โชว์ให้อีกฝ่ายเห็นเมื่อการจองถูกยืนยันแล้วเท่านั้น
    role           VARCHAR(10)  NOT NULL DEFAULT 'member' CHECK (role IN ('member', 'admin')),
    is_banned      BOOLEAN      NOT NULL DEFAULT FALSE,
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT now()
);

-- ---------------------------------------------------------------------------
-- categories: หมวดหมู่ที่ admin จัดการ
-- ---------------------------------------------------------------------------
CREATE TABLE categories (
    id    BIGSERIAL   PRIMARY KEY,
    name  VARCHAR(50) NOT NULL UNIQUE,
    slug  VARCHAR(50) NOT NULL UNIQUE CHECK (slug ~ '^[a-z0-9-]+$')
);

-- ---------------------------------------------------------------------------
-- listings: ประกาศขาย 1 แถว = ของ 1 ชิ้น
-- สถานะ: pending (รอ admin) -> available (ขึ้นหน้าร้าน) -> reserved (มีคนจอง) -> sold
--        pending -> rejected (admin ไม่ผ่าน ผู้ขายแก้แล้วกลับไป pending ได้)
-- ---------------------------------------------------------------------------
CREATE TABLE listings (
    id             BIGSERIAL     PRIMARY KEY,
    seller_id      BIGINT        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    category_id    BIGINT        NOT NULL REFERENCES categories (id),   -- ลบหมวดที่ยังมีของไม่ได้ (23503)
    title          VARCHAR(100)  NOT NULL,
    description    TEXT          NOT NULL DEFAULT '',
    price          INTEGER       NOT NULL CHECK (price BETWEEN 0 AND 1000000),
    condition      VARCHAR(10)   NOT NULL CHECK (condition IN ('new', 'like_new', 'good', 'fair')),
    image_url      VARCHAR(500)  NOT NULL DEFAULT '',
    status         VARCHAR(10)   NOT NULL DEFAULT 'pending'
                   CHECK (status IN ('pending', 'available', 'reserved', 'sold', 'rejected')),
    reject_reason  VARCHAR(200)  NOT NULL DEFAULT '',
    created_at     TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ   NOT NULL DEFAULT now()
);

-- หน้าร้านกรองด้วย status + category แล้วเรียงตามวันที่บ่อยที่สุด
CREATE INDEX idx_listings_status_cat_time ON listings (status, category_id, created_at DESC);
CREATE INDEX idx_listings_seller ON listings (seller_id);

-- ---------------------------------------------------------------------------
-- reservations: คำขอจอง
-- pending (รอผู้ขาย) -> accepted (ผู้ขายยืนยัน) -> completed (ผู้ขายกดขายแล้ว)
-- pending -> rejected (ผู้ขายปฏิเสธ) | cancelled (ผู้ซื้อยกเลิก)
-- ---------------------------------------------------------------------------
CREATE TABLE reservations (
    id          BIGSERIAL    PRIMARY KEY,
    listing_id  BIGINT       NOT NULL REFERENCES listings (id) ON DELETE CASCADE,
    buyer_id    BIGINT       NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    message     VARCHAR(300) NOT NULL DEFAULT '',
    status      VARCHAR(10)  NOT NULL DEFAULT 'pending'
                CHECK (status IN ('pending', 'accepted', 'rejected', 'cancelled', 'completed')),
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    decided_at  TIMESTAMPTZ
);

-- ด่านสุดท้ายกันจองซ้อน: ของ 1 ชิ้นมีการจองที่ "ยังมีผล" ได้แค่ 1 รายการ
-- ถึงโค้ด Go จะพลาด ฐานข้อมูลก็จะตอบ 23505 กลับมา
CREATE UNIQUE INDEX uq_reservations_active ON reservations (listing_id)
    WHERE status IN ('pending', 'accepted');
CREATE INDEX idx_reservations_buyer ON reservations (buyer_id);

-- Supabase เปิด REST API อัตโนมัติให้ทุกตารางใน schema public (ใครมี anon key ก็อ่านได้)
-- เปิด RLS โดยไม่มี policy = ปิดทางนั้นทั้งหมด ข้อมูลต้องผ่าน Go API ของเราเท่านั้น
-- (Go ต่อด้วย user postgres ซึ่งเป็นเจ้าของตาราง จึงไม่โดน RLS)
ALTER TABLE users        ENABLE ROW LEVEL SECURITY;
ALTER TABLE categories   ENABLE ROW LEVEL SECURITY;
ALTER TABLE listings     ENABLE ROW LEVEL SECURITY;
ALTER TABLE reservations ENABLE ROW LEVEL SECURITY;

COMMIT;
