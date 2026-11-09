-- ShelfSwap mock data (รันหลัง schema.sql)
-- บน Supabase: SQL Editor > New query > วางทั้งไฟล์ > Run  (รันซ้ำได้ เพราะล้างข้อมูลก่อนทุกครั้ง)
--
-- บัญชีทดลอง (รหัสผ่านทุกบัญชี = password123  สร้าง hash ใหม่ได้ด้วย go run ./cmd/hashpw "รหัส")
--   admin@shelfswap.test    แอดมิน (admin)
--   mei@shelfswap.test      เมย์   มีคำขอจองรอตอบ 1 รายการ (ของชิ้นที่ 13)
--   ploy@shelfswap.test     พลอย
--   ton@shelfswap.test      ต้น    มีประกาศรออนุมัติ 2 รายการ
--   fah@shelfswap.test      ฟ้า
--   banned@shelfswap.test   ถูกแบนแล้ว (login ได้ 403, ประกาศของคนนี้ไม่ขึ้นหน้าร้าน)
--
-- รูปสินค้าใช้ placehold.co ไปก่อน (ภาพสีพื้นมีชื่อสินค้า) อัปโหลดรูปจริงแล้วแก้ image_url ทีหลังได้

BEGIN;

TRUNCATE reservations, listings, categories, users RESTART IDENTITY CASCADE;

-- ---------------------------------------------------------------------------
-- users
-- ---------------------------------------------------------------------------
INSERT INTO users (id, email, password_hash, display_name, phone, role, is_banned, created_at) VALUES
 (1, 'admin@shelfswap.test',  '$2a$10$FqtFpi7cy6jqM2xy/7AopOyX9lzUG1jJQLJcEWhiKKMqoL/yoAeaq', 'แอดมิน ShelfSwap', '',             'admin',  FALSE, now() - interval '60 days'),
 (2, 'mei@shelfswap.test',    '$2a$10$FqtFpi7cy6jqM2xy/7AopOyX9lzUG1jJQLJcEWhiKKMqoL/yoAeaq', 'เมย์',             '080-000-0001', 'member', FALSE, now() - interval '45 days'),
 (3, 'ploy@shelfswap.test',   '$2a$10$FqtFpi7cy6jqM2xy/7AopOyX9lzUG1jJQLJcEWhiKKMqoL/yoAeaq', 'พลอย',            '080-000-0002', 'member', FALSE, now() - interval '40 days'),
 (4, 'ton@shelfswap.test',    '$2a$10$FqtFpi7cy6jqM2xy/7AopOyX9lzUG1jJQLJcEWhiKKMqoL/yoAeaq', 'ต้น',              '080-000-0003', 'member', FALSE, now() - interval '38 days'),
 (5, 'fah@shelfswap.test',    '$2a$10$FqtFpi7cy6jqM2xy/7AopOyX9lzUG1jJQLJcEWhiKKMqoL/yoAeaq', 'ฟ้า',              '080-000-0004', 'member', FALSE, now() - interval '30 days'),
 (6, 'banned@shelfswap.test', '$2a$10$FqtFpi7cy6jqM2xy/7AopOyX9lzUG1jJQLJcEWhiKKMqoL/yoAeaq', 'บัญชีต้องสงสัย',     '',             'member', TRUE,  now() - interval '10 days');

-- ---------------------------------------------------------------------------
-- categories
-- ---------------------------------------------------------------------------
INSERT INTO categories (id, name, slug) VALUES
 (1, 'หนังสือเรียน',              'textbooks'),
 (2, 'หนังสือทั่วไป & นิยาย',       'books'),
 (3, 'เครื่องคิดเลข & อิเล็กทรอนิกส์', 'electronics'),
 (4, 'เครื่องเขียน',               'stationery'),
 (5, 'อุปกรณ์ศิลปะ & เขียนแบบ',    'art-supplies'),
 (6, 'ชุดนักศึกษา & ชุดแล็บ',      'uniforms'),
 (7, 'อุปกรณ์กีฬา',               'sports'),
 (8, 'ของใช้ในหอ',                'dorm');

-- ---------------------------------------------------------------------------
-- listings 35 ชิ้น: available 24 / reserved 3 / sold 3 / pending 3 / rejected 1 / ของคนถูกแบน 1 (ไม่ขึ้นหน้าร้าน)
-- ---------------------------------------------------------------------------
INSERT INTO listings (id, seller_id, category_id, title, description, price, condition, image_url, status, reject_reason, created_at) VALUES
-- หนังสือเรียน
 (1,  2, 1, 'Thomas'' Calculus 14th Edition', 'ใช้เรียน Calculus 1-2 มีจดโน้ตดินสอบางหน้า ลบได้ ปกมีรอยเล็กน้อย', 450, 'good', 'https://placehold.co/600x450/dbeafe/1e3a8a.png?text=Calculus', 'available', '', now() - interval '20 days'),
 (2,  3, 1, 'Campbell Biology 11th Edition', 'ใช้เทอมเดียว สภาพดีมาก ไม่มีขีดเขียน', 650, 'like_new', 'https://placehold.co/600x450/dbeafe/1e3a8a.png?text=Biology', 'available', '', now() - interval '18 days'),
 (3,  4, 1, 'Fundamentals of Physics (Halliday) 10th Ed.', 'มีไฮไลต์บทที่ 1-8 แถมชีทสรุปสูตร', 520, 'good', 'https://placehold.co/600x450/dbeafe/1e3a8a.png?text=Physics', 'reserved', '', now() - interval '17 days'),
 (4,  5, 1, 'Database System Concepts 7th Ed.', 'ปกหลุดนิดหน่อย เนื้อในครบ เหมาะกับวิชา Database', 380, 'fair', 'https://placehold.co/600x450/dbeafe/1e3a8a.png?text=Database', 'available', '', now() - interval '15 days'),
 (5,  2, 1, 'ชีทสรุป Data Structures (เข้าเล่ม)', 'สรุปเองครบทุกบท มีตัวอย่างโค้ด Java', 80, 'good', 'https://placehold.co/600x450/dbeafe/1e3a8a.png?text=Data+Structures', 'available', '', now() - interval '12 days'),
 (6,  3, 1, 'Introduction to Algorithms (CLRS) 3rd Ed.', 'เล่มหนา สภาพดี มีชื่อเขียนหน้าแรก', 590, 'good', 'https://placehold.co/600x450/dbeafe/1e3a8a.png?text=Algorithms', 'sold', '', now() - interval '25 days'),
 (7,  4, 1, 'หนังสือเตรียมสอบ TOEIC ฉบับล่าสุด', 'ทำแบบฝึกหัดไปแค่ 2 บท', 150, 'like_new', 'https://placehold.co/600x450/dbeafe/1e3a8a.png?text=TOEIC', 'pending', '', now() - interval '1 day'),
-- หนังสือทั่วไป & นิยาย
 (8,  5, 2, 'Atomic Habits ฉบับภาษาไทย', 'อ่านจบรอบเดียว', 180, 'like_new', 'https://placehold.co/600x450/fce7f3/9d174d.png?text=Atomic+Habits', 'available', '', now() - interval '14 days'),
 (9,  2, 2, 'Harry Potter เล่ม 1-3 (แปลไทย)', 'ขายยกชุด กระดาษเหลืองตามเวลา', 600, 'good', 'https://placehold.co/600x450/fce7f3/9d174d.png?text=Harry+Potter+1-3', 'available', '', now() - interval '13 days'),
 (10, 3, 2, 'The Psychology of Money ฉบับภาษาไทย', 'สภาพดี มีที่คั่นหนังสือแถม', 160, 'good', 'https://placehold.co/600x450/fce7f3/9d174d.png?text=Psychology+of+Money', 'reserved', '', now() - interval '11 days'),
 (11, 4, 2, 'มังงะ One Piece เล่ม 1-10', 'มีรอยพับมุมบางเล่ม', 450, 'fair', 'https://placehold.co/600x450/fce7f3/9d174d.png?text=One+Piece+1-10', 'available', '', now() - interval '9 days'),
 (12, 5, 2, 'Sapiens ฉบับภาษาไทย', 'ปกแข็ง', 220, 'good', 'https://placehold.co/600x450/fce7f3/9d174d.png?text=Sapiens', 'sold', '', now() - interval '28 days'),
-- เครื่องคิดเลข & อิเล็กทรอนิกส์
 (13, 2, 3, 'เครื่องคิดเลข Casio fx-991EX', 'ใช้สอบได้ทุกวิชา ถ่านใหม่ มีฝาครอบ', 550, 'like_new', 'https://placehold.co/600x450/e0e7ff/3730a3.png?text=Casio+fx-991EX', 'reserved', '', now() - interval '10 days'),
 (14, 4, 3, 'เครื่องคิดเลข Casio fx-570ES Plus', 'ปุ่มยังดีทุกปุ่ม', 280, 'good', 'https://placehold.co/600x450/e0e7ff/3730a3.png?text=Casio+fx-570ES', 'available', '', now() - interval '16 days'),
 (15, 3, 3, 'เมาส์ไร้สาย Logitech M331', 'เสียงคลิกเงียบ ใช้ถ่าน AA', 250, 'good', 'https://placehold.co/600x450/e0e7ff/3730a3.png?text=Wireless+Mouse', 'available', '', now() - interval '8 days'),
 (16, 5, 3, 'พาวเวอร์แบงก์ 20000mAh', 'ชาร์จเร็ว มีสาย USB-C แถม', 350, 'good', 'https://placehold.co/600x450/e0e7ff/3730a3.png?text=Power+Bank', 'available', '', now() - interval '7 days'),
 (17, 4, 3, 'หูฟัง Sony WH-CH520', 'ใช้ไม่ถึง 3 เดือน ครบกล่อง', 900, 'like_new', 'https://placehold.co/600x450/e0e7ff/3730a3.png?text=Headphones', 'pending', '', now() - interval '12 hours'),
 (18, 2, 3, 'ชุด Arduino Uno Starter Kit', 'บอร์ด + เซนเซอร์ 10 ชนิด + สายจัมเปอร์ ใช้ในวิชา Embedded', 400, 'good', 'https://placehold.co/600x450/e0e7ff/3730a3.png?text=Arduino+Kit', 'available', '', now() - interval '6 days'),
-- เครื่องเขียน
 (19, 3, 4, 'ปากกาเจล Pilot Juice 0.5 (10 ด้าม)', 'ของใหม่ยังไม่แกะ ซื้อมาเกิน', 120, 'new', 'https://placehold.co/600x450/fef3c7/92400e.png?text=Gel+Pens', 'available', '', now() - interval '5 days'),
 (20, 5, 4, 'กระดาษ A4 ถนอมสายตา 1 รีม + แฟ้ม 3 เล่ม', 'ยังไม่แกะห่อ', 90, 'new', 'https://placehold.co/600x450/fef3c7/92400e.png?text=A4+Paper', 'available', '', now() - interval '4 days'),
 (21, 2, 4, 'ปากกาไฮไลต์ Stabilo 6 สี', 'หมึกยังเหลือเยอะ', 85, 'like_new', 'https://placehold.co/600x450/fef3c7/92400e.png?text=Highlighters', 'sold', '', now() - interval '22 days'),
-- อุปกรณ์ศิลปะ & เขียนแบบ
 (22, 4, 5, 'ไม้ที T-square 90 ซม. + ไม้ฉาก', 'ใช้วิชาเขียนแบบปี 1', 300, 'good', 'https://placehold.co/600x450/ede9fe/5b21b6.png?text=T-square', 'available', '', now() - interval '19 days'),
 (23, 3, 5, 'สีน้ำ Winsor & Newton Cotman 24 สี', 'ใช้ไปไม่กี่ครั้ง ตลับครบ', 480, 'like_new', 'https://placehold.co/600x450/ede9fe/5b21b6.png?text=Watercolor+24', 'available', '', now() - interval '3 days'),
 (24, 5, 5, 'กระดานเขียนแบบ A2 พร้อมขาตั้ง', 'มีรอยขีดข่วน ใช้งานได้ปกติ ต้องมารับเอง', 750, 'fair', 'https://placehold.co/600x450/ede9fe/5b21b6.png?text=Drawing+Board', 'available', '', now() - interval '21 days'),
 (25, 2, 5, 'ชุดดินสอสเก็ตช์ Faber-Castell 12 แท่ง', 'ของใหม่', 150, 'new', 'https://placehold.co/600x450/ede9fe/5b21b6.png?text=Sketch+Pencils', 'rejected', 'รูปไม่ใช่รูปสินค้าจริง กรุณาใส่รูปที่ถ่ายเองแล้วส่งใหม่', now() - interval '2 days'),
-- ชุดนักศึกษา & ชุดแล็บ
 (26, 5, 6, 'เสื้อกาวน์ (ชุดแล็บ) ไซซ์ M', 'ซักรีดแล้ว ไม่มีคราบ', 200, 'good', 'https://placehold.co/600x450/dcfce7/166534.png?text=Lab+Coat+M', 'available', '', now() - interval '9 days'),
 (27, 3, 6, 'ชุดนักศึกษาหญิง เสื้อ + กระโปรง ไซซ์ S', 'ใส่ไม่กี่ครั้ง', 250, 'like_new', 'https://placehold.co/600x450/dcfce7/166534.png?text=Uniform+S', 'available', '', now() - interval '6 days'),
 (28, 4, 6, 'แว่นตานิรภัยห้องแล็บ (Safety Goggles)', 'มีถุงผ้าใส่', 60, 'good', 'https://placehold.co/600x450/dcfce7/166534.png?text=Safety+Goggles', 'available', '', now() - interval '5 days'),
-- อุปกรณ์กีฬา
 (29, 2, 7, 'ไม้แบด Yonex Astrox 3 DG', 'เอ็นใหม่ 24 ปอนด์ พร้อมซอง', 900, 'good', 'https://placehold.co/600x450/ffedd5/9a3412.png?text=Badminton+Racket', 'available', '', now() - interval '4 days'),
 (30, 4, 7, 'เสื่อโยคะ 6 มม.', 'ซื้อมาแล้วไม่ค่อยได้ใช้', 200, 'like_new', 'https://placehold.co/600x450/ffedd5/9a3412.png?text=Yoga+Mat', 'available', '', now() - interval '3 days'),
 (31, 5, 7, 'ลูกบาสเกตบอล Molten GG7X', 'ลมยังดี', 600, 'good', 'https://placehold.co/600x450/ffedd5/9a3412.png?text=Basketball', 'pending', '', now() - interval '6 hours'),
-- ของใช้ในหอ
 (32, 3, 8, 'พัดลมตั้งโต๊ะ 12 นิ้ว', 'ลมแรง ปรับได้ 3 ระดับ', 350, 'good', 'https://placehold.co/600x450/e0f2fe/075985.png?text=Desk+Fan', 'available', '', now() - interval '2 days'),
 (33, 4, 8, 'หม้อหุงข้าว 1 ลิตร', 'ใช้ได้ปกติ ฝามีรอย', 300, 'fair', 'https://placehold.co/600x450/e0f2fe/075985.png?text=Rice+Cooker', 'available', '', now() - interval '1 day'),
 (34, 6, 3, 'iPhone ราคาถูกมาก โอนก่อนส่งของ', 'ทักไลน์ด่วน', 1500, 'good', 'https://placehold.co/600x450/e5e7eb/374151.png?text=Suspicious', 'available', '', now() - interval '8 days'),
 (35, 5, 8, 'โคมไฟอ่านหนังสือ LED', 'ปรับแสงได้ 3 โทน ชาร์จ USB', 180, 'like_new', 'https://placehold.co/600x450/e0f2fe/075985.png?text=Desk+Lamp', 'available', '', now() - interval '1 day');

UPDATE listings SET updated_at = created_at;

-- ---------------------------------------------------------------------------
-- reservations (ให้สถานะตรงกับ listings ด้านบน)
-- ---------------------------------------------------------------------------
INSERT INTO reservations (id, listing_id, buyer_id, message, status, created_at, decided_at) VALUES
 (1, 13, 3, 'สนใจค่ะ นัดรับที่หอในได้ไหมคะ',         'pending',   now() - interval '1 day',   NULL),
 (2, 3,  2, 'ขอรับวันพุธหลังเลิกเรียนได้ไหมคะ',        'accepted',  now() - interval '3 days',  now() - interval '2 days'),
 (3, 10, 5, 'จองค่ะ รับพรุ่งนี้ที่ตึกคณะ',              'pending',   now() - interval '5 hours', NULL),
 (4, 6,  4, 'เอาครับ',                              'completed', now() - interval '20 days', now() - interval '19 days'),
 (5, 12, 2, 'สนใจค่ะ',                              'completed', now() - interval '24 days', now() - interval '23 days'),
 (6, 21, 5, 'ขอซื้อค่ะ',                             'completed', now() - interval '18 days', now() - interval '18 days'),
 (7, 14, 5, 'ลดได้ไหมคะ',                            'rejected',  now() - interval '10 days', now() - interval '9 days'),
 (8, 1,  4, 'จองไว้ก่อนครับ',                        'cancelled', now() - interval '7 days',  now() - interval '6 days');

-- ใส่ id เองแล้ว ต้องเลื่อนตัวนับ BIGSERIAL ตาม ไม่งั้น INSERT ครั้งต่อไปจะชน primary key
SELECT setval(pg_get_serial_sequence('users', 'id'),        (SELECT MAX(id) FROM users));
SELECT setval(pg_get_serial_sequence('categories', 'id'),   (SELECT MAX(id) FROM categories));
SELECT setval(pg_get_serial_sequence('listings', 'id'),     (SELECT MAX(id) FROM listings));
SELECT setval(pg_get_serial_sequence('reservations', 'id'), (SELECT MAX(id) FROM reservations));

COMMIT;
