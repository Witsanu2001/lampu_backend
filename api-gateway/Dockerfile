# ใช้ Go image แบบ lightweight
FROM golang:alpine AS builder

# ตั้งค่าโฟลเดอร์ทำงาน
WORKDIR /app

# คัดลอกไฟล์จัดการ dependencies และโหลด
COPY go.mod go.sum ./
RUN go mod download

# คัดลอกโค้ดทั้งหมดแล้ว Build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o main .

# สร้าง Image ตัวสุดท้ายให้เล็กที่สุด (รันเร็วและประหยัดพื้นที่)
FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/main .

# เปิดพอร์ต 8080 ตามที่โค้ดเรารัน
EXPOSE 8080

# คำสั่งรันโปรแกรม
CMD ["./main"]