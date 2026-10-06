# Lottery Search System Design

## 1. ฐานข้อมูลและการแบ่ง Partition

- ใช้ PostgreSQL เพื่อเก็บข้อมูลงวด สลาก การจอง และคำสั่งซื้อ โดยใช้ transaction และ constraint เพื่อรักษาความถูกต้อง
- สลากประมาณ 10M ใบต่องวด หรือประมาณ 240M ใบต่อปีเมื่อมี 24 งวด เป็นจำนวนข้อมูลสะสม ไม่ใช่จำนวนแถวที่ทุก query ต้องอ่าน
- แบ่ง LIST partition ตาม `draw_id` และกำหนดให้ทุก query ระบุงวด เพื่อให้เกิด partition pruning และจัดการข้อมูลเก่าได้ง่าย
- สร้าง partition ก่อนเปิดงวด และ archive/drop ตาม retention policy หลังตรวจว่าไม่มีคำสั่งซื้อ การชำระเงิน หรือข้อมูลอ้างอิงที่ยังต้องใช้งาน
- Primary key ใช้ `(draw_id, id)` โดย `id` ระบุสลากแต่ละใบ เลขสลากเดียวกันมีได้หลายใบ

## 2. การค้นหาและ Index

เก็บ `number` เป็น string ตัวเลข 6 หลักเพื่อรักษาเลขศูนย์นำหน้า กำหนด collation เป็น `C` สำหรับการเปรียบเทียบช่วง และสร้าง `number_reversed` จาก `reverse(number)` แบบ generated column

| รูปแบบ | วิธีค้นหา |
| --- | --- |
| `123456` | Exact match บน `number` |
| `123***` | Range บน `number`: `123000` ถึง `123999` |
| `***123` | Range บน `number_reversed`: `321000` ถึง `321999` |
| `12**34` หรือ `1*3**4` | ขยายเป็นรายการเลขเต็มเมื่อมี wildcard ไม่เกิน 3 ตัว แล้วค้นหา exact match |
| รูปแบบอื่นที่มี wildcard มากกว่า 3 ตัว | ปฏิเสธพร้อมให้ผู้ใช้ระบุเลขเพิ่มในรุ่นแรก |

- `*` หมายถึงตัวเลขใดก็ได้หนึ่งหลัก รับเฉพาะ `[0-9*]` ความยาว 6 ตัว และใช้ parameterized query
- จัดประเภท exact/prefix/suffix ก่อนใช้การขยาย wildcard จึงรองรับ `123***` และ `***123` ด้วย range ตามปกติ
- Wildcard 3 ตัวให้ได้สูงสุด 1,000 หมายเลข ต้อง benchmark ก่อนกำหนดขีดจำกัดจริง
- การแปลง pattern กระจายเป็นช่วงเดียวไม่เพียงพอ เพราะอาจได้เลขที่ไม่ตรง pattern
- ใช้ partial B-tree index เฉพาะ `status = 'AVAILABLE'` ขนาดที่ลดลงขึ้นกับสัดส่วนสลากว่าง ไม่กำหนดเปอร์เซ็นต์ล่วงหน้า
- Query คืนเฉพาะสลากว่าง ไม่ได้ยืนยันสิทธิ์การจอง ผู้ใช้ต้องผ่าน atomic reserve อีกครั้ง

### การจำกัดผลลัพธ์

- คืนสูงสุด 20 ใบต่อ request และเรียง `ORDER BY number, id` ให้แน่นอน
- ใช้ keyset pagination จาก `(number, id)` หากต้องการหน้าถัดไป จำกัดจำนวนหน้าที่อนุญาตตามข้อกำหนดผลิตภัณฑ์
- หน้าถัดไปต้องใช้เงื่อนไขค้นหาเดิม ผลลัพธ์อาจเปลี่ยนตามสถานะสลากระหว่าง request
- `LIMIT` จำกัดผลลัพธ์ที่ส่งกลับ ไม่รับประกันว่าจะอ่านข้อมูลน้อย โดยเฉพาะเมื่อไม่มีผลลัพธ์ที่ยังว่าง

## 3. การจองแบบ Atomic

PostgreSQL เป็นแหล่งยืนยันสิทธิ์การจองและการขาย ใช้ conditional update ใน transaction สั้น ๆ ไม่เปิด transaction ค้างระหว่างรอชำระเงิน

```sql
UPDATE lottery
SET status = 'RESERVED',
    reservation_id = $3,
    reserved_by = $4,
    reserved_until = NOW() + INTERVAL '5 minutes'
WHERE draw_id = $1
  AND id = $2
  AND status = 'AVAILABLE'
RETURNING draw_id, id, reservation_id, reserved_until;
```

- คืนหนึ่งแถวหมายถึงจองสำเร็จ ไม่คืนแถวหมายถึงจองไม่ได้
- สร้าง reservation record ใน transaction เดียวกัน และใช้ idempotency key เพื่อให้ retry หลัง response สูญหายคืนผลเดิมได้
- ตรวจว่างวดยังเปิดขายและผู้ใช้มีสิทธิ์จอง โดย synchronize การปิดงวดกับ transaction จอง
- ถ้าจองหลายใบแบบทั้งหมดหรือไม่มีเลย ให้อัปเดตใน transaction เดียว ตรวจจำนวนครบ และเรียงลำดับการล็อกเพื่อลด deadlock
- Request แข่งกันจองใบเดียวอาจรอ row lock ต้องกำหนด timeout และ retry เฉพาะ error ที่ retry ได้
- ยังไม่จำเป็นต้องมี Redis สำหรับความถูกต้องของการจอง เพิ่ม cache/rate limiting เมื่อมีผล load test รองรับ

## 4. การหมดอายุและชำระเงิน

```text
AVAILABLE → RESERVED → PAYMENT_PENDING → SOLD
                ↓              ↓
            AVAILABLE      AVAILABLE
          (หมดเวลาจอง)   (ยืนยันล้มเหลว/ยกเลิก)
```

- Background Worker คืนเฉพาะ `RESERVED` ที่ `reserved_until` หมดอายุ โดย conditional update และล้างข้อมูลเจ้าของการจองที่ active
- ก่อนเริ่มชำระเงิน เปลี่ยนเป็น `PAYMENT_PENDING` โดยตรวจผู้ใช้ `reservation_id` และเวลาจอง พร้อมสร้าง order/payment attempt ใน transaction เดียวกัน
- การเปลี่ยนเป็น `PAYMENT_PENDING` แข่งกับการคืนสลากหมดอายุผ่าน conditional update เดียวกัน จึงมีผู้ชนะเพียงทางเดียว
- เรียก payment provider หลัง commit ใช้ idempotency key และมี reconciliation สำหรับกรณี timeout หรือแอปล่ม
- ห้ามคืน `PAYMENT_PENDING` เพราะครบ TTL เพียงอย่างเดียว ต้องยืนยันการยกเลิก/ล้มเหลวกับ provider ก่อน ถ้าสถานะยังไม่แน่นอนให้คงการถือสลากไว้และตรวจสอบซ้ำ
- Callback สำเร็จต้องตรวจความถูกต้องของ callback, order, ยอดเงิน และ reservation ที่ผูกกับสลาก แล้วเปลี่ยนเป็น `SOLD` ใน transaction
- บันทึก payment event แบบ unique และเปลี่ยนสถานะแบบ idempotent เพื่อรองรับ callback ซ้ำหรือมาผิดลำดับ
- หาก callback สำเร็จมาหลัง order ถูกยกเลิก ต้องบันทึกเพื่อ reconciliation/refund ตามนโยบาย ห้ามยึดสลากจากเจ้าของใหม่
- เก็บประวัติการจองและการชำระเงินแยกจากสถานะปัจจุบันของสลากเพื่อ audit

### งานเบื้องหลังและ Message Queue

ถ้าใช้ RabbitMQ ให้บันทึก outbox event ใน transaction เดียวกับสถานะธุรกิจ แล้วให้ publisher ส่งภายหลัง ใช้ publisher confirms, consumer แบบ idempotent และ manual ack หลัง DB commit พร้อม retry และ dead-letter handling การยืนยันสิทธิ์การขายไม่ขึ้นกับความเร็วของ queue

## 5. SQL DDL ตัวอย่าง

ตัวอย่างนี้ครอบคลุมตารางงวดและสลาก ส่วน reservation, order, payment event และ outbox ต้องมี schema เพิ่มตาม flow ข้างต้น

```sql
CREATE TABLE draw (
    id         VARCHAR(32) PRIMARY KEY,
    draw_date  DATE NOT NULL,
    status     VARCHAR(16) NOT NULL
               CHECK (status IN ('UPCOMING', 'OPEN', 'CLOSED')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE lottery (
    draw_id         VARCHAR(32) NOT NULL REFERENCES draw(id),
    id              VARCHAR(64) NOT NULL,
    number          VARCHAR(6) COLLATE "C" NOT NULL
                    CHECK (number ~ '^[0-9]{6}$'),
    number_reversed VARCHAR(6) COLLATE "C"
                    GENERATED ALWAYS AS (reverse(number)) STORED,
    status          VARCHAR(16) NOT NULL DEFAULT 'AVAILABLE'
                    CHECK (status IN (
                        'AVAILABLE', 'RESERVED', 'PAYMENT_PENDING', 'SOLD'
                    )),
    reservation_id  VARCHAR(64),
    reserved_by     VARCHAR(64),
    reserved_until  TIMESTAMPTZ,
    order_id        VARCHAR(64),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (draw_id, id),
    CHECK (
        status NOT IN ('RESERVED', 'PAYMENT_PENDING')
        OR (reservation_id IS NOT NULL
            AND reserved_by IS NOT NULL
            AND reserved_until IS NOT NULL)
    ),
    CHECK (
        status NOT IN ('PAYMENT_PENDING', 'SOLD')
        OR order_id IS NOT NULL
    )
) PARTITION BY LIST (draw_id);

CREATE TABLE lottery_p2026_01
    PARTITION OF lottery FOR VALUES IN ('2026_01');

CREATE INDEX idx_lottery_available_number
    ON lottery (number, id) WHERE status = 'AVAILABLE';

CREATE INDEX idx_lottery_available_suffix
    ON lottery (number_reversed, id) WHERE status = 'AVAILABLE';

CREATE INDEX idx_lottery_reserved_expiry
    ON lottery (reserved_until, id) WHERE status = 'RESERVED';
```

### ตัวอย่างค้นหาเลขหน้า

```sql
SELECT id, number
FROM lottery
WHERE draw_id = $1
  AND status = 'AVAILABLE'
  AND number BETWEEN '123000' AND '123999'
ORDER BY number, id
LIMIT 20;
```

### ตัวอย่างค้นหา Pattern กระจาย

`$2` คือรายการเลขเต็มที่แอปขยายจาก pattern ภายใต้ขีดจำกัดที่กำหนด

```sql
SELECT id, number
FROM lottery
WHERE draw_id = $1
  AND status = 'AVAILABLE'
  AND number = ANY($2::varchar[])
ORDER BY number, id
LIMIT 20;
```

## 6. Go Model

```go
package model

import "time"

type Draw struct {
    ID        string    `json:"id"`
    DrawDate  time.Time `json:"draw_date"`
    Status    string    `json:"status"`
    CreatedAt time.Time `json:"created_at"`
}

type Lottery struct {
    ID             string     `json:"id"`
    DrawID         string     `json:"draw_id"`
    Number         string     `json:"number"`
    NumberReversed string     `json:"-"` // DB สร้างค่าเอง
    Status         string     `json:"status"`
    ReservationID  *string    `json:"-"`
    ReservedBy     *string    `json:"-"`
    ReservedUntil  *time.Time `json:"reserved_until,omitempty"`
    OrderID        *string    `json:"-"`
    CreatedAt      time.Time  `json:"created_at"`
}
```

ใช้ response DTO แยกสำหรับ search และ reservation ไม่เปิดเผยข้อมูลเจ้าของการจองแก่ผู้ใช้คนอื่น

## 7. การตรวจสอบก่อนใช้งานจริง

- ใช้ `EXPLAIN (ANALYZE, BUFFERS)` กับข้อมูลประมาณ 10M ใบต่องวด ตรวจ partition pruning และ index สำหรับแต่ละ pattern รวมกรณีไม่พบผลลัพธ์
- Load test การค้นหา จองใบเดียวพร้อมกัน และจองคนละใบ วัด latency, lock wait, connection pool และต้นทุนการอัปเดต index
- ทดสอบจองซ้ำด้วย idempotency key, การหมดอายุแข่งกับเริ่มชำระเงิน, callback ซ้ำ/มาช้า และการล่มระหว่าง commit กับส่ง event
- กำหนด connection pool, rate limit และ timeout จากผลทดสอบ ไม่สรุปว่า PostgreSQL รองรับไม่ไหวจากจำนวนแถวเพียงอย่างเดียว
