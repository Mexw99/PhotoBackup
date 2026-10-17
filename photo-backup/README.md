# PhotoBackup

แอปจัดการรูปภาพบนเดสก์ท็อปด้วย Wails v2, Go, Nuxt 4 และ SQLite/GORM
รองรับเลือกโฟลเดอร์ สำรองรูป เพิ่ม Tag และค้นหาด้วยเงื่อนไข AND/OR

หน้า Tag มีปุ่ม “ลบ” ข้างแต่ละ Tag ใน Tag Cloud เพื่อลบ Tag ออกจาก SQLite
และยกเลิกความสัมพันธ์กับทุกรูป โดยเก็บรูปภาพไว้ตามเดิม
ปุ่ม × บน Tag ของรูปแต่ละรูปยังใช้สำหรับเอา Tag ออกจากรูปนั้นเพียงรูปเดียว

## เริ่มพัฒนา

ต้องมี Go ตาม `go.mod`, Node.js/npm และ Wails CLI v2
รันคำสั่งจากโฟลเดอร์ `photo-backup`:

```powershell
cd frontend
npm ci
cd ..
wails dev
```

Wails จะเริ่ม Nuxt และสร้าง JavaScript/TypeScript bindings ใน `frontend/wailsjs`
การเปิด Nuxt ในเบราว์เซอร์เพียงอย่างเดียวไม่มี native Go bridge สำหรับเรียกฐานข้อมูลหรือ dialog

## ตรวจสอบและ build

```powershell
go test ./...
go vet ./...
cd frontend
npm run typecheck
cd ..
wails build
```

`wails build` เรียก `npm run generate` และฝัง `frontend/dist` ลงในโปรแกรม
ผลลัพธ์อยู่ใน `build/bin/`

อ่าน [โครงสร้างและแนวทางเพิ่มไฟล์](docs/PROJECT_STRUCTURE.md)
และ [คู่มือฐานข้อมูล](docs/database/DATABASE_GUIDE.html)
แผนแบ่งงานเดิมอยู่ใน `docs/planning/`

ฐานข้อมูลเก็บที่ `os.UserConfigDir()/PhotoBackup/photo-backup.db` ตามระบบปฏิบัติการ
การสำรองรูปปัจจุบันเป็นการ **ย้ายไฟล์** ไปยังปลายทางและลบต้นทางหลังย้ายสำเร็จ
