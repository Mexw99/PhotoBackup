# PhotoBackup

แอปจัดการรูปภาพบนเดสก์ท็อปด้วย Wails v2, Go, Nuxt 4 และ SQLite/GORM
รองรับบันทึกโฟลเดอร์ต้นทาง/ปลายทาง ย้ายรูปไปเก็บที่ปลายทาง และวิเคราะห์ภาพด้วย AI เป็น Description/Tag
ระบบเก็บ SHA-256 ของไฟล์ Backup เพื่อใช้ Cache ผลวิเคราะห์ซ้ำ และมี Persistent AI Job Queue
ค้นหาชื่อไฟล์ คำอธิบาย และ Tag Cloud ได้หลาย Tag โดยผลการวิเคราะห์จะแสดงในรายละเอียดรูป

อ่าน [คู่มือเตรียมตรวจความก้าวหน้าและไล่อ่านโค้ด](docs/PROGRESS_DEMO.md)
สำหรับหัวข้อที่ทำเพิ่ม วิธีสาธิต และไฟล์สำคัญตามเกณฑ์ตรวจรอบนี้

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
แผนแบ่งงานเดิมอยู่ใน `docs/planning/`
[คู่มือฐานข้อมูลเดิม](docs/database/DATABASE_GUIDE.html) ใช้อ่านพื้นฐาน GORM และความสัมพันธ์ Tag
เป็นเอกสารเวอร์ชันก่อนหน้า ตัวอย่างการสำรอง การค้นหา และ schema บางส่วนต่างจากรอบนี้
ให้ดูคู่มือ Progress Demo สำหรับพฤติกรรมล่าสุด และดู `model/` กับ `database/database.go` สำหรับ schema ปัจจุบัน

ฐานข้อมูลปัจจุบันเก็บที่ `photo-backup.db` ใน working directory ที่ใช้เปิดโปรแกรม (`database.DefaultPath`)
บน macOS การเปิดจาก `.app` อาจทำให้ working directory เขียนไม่ได้ จึงควรตรวจ path จาก `GetDatabasePath()` ก่อนใช้งานจริง
การสำรองรูปเป็นการ **ย้ายไฟล์จริง** รองรับ JPG, JPEG และ PNG
ระบบเตรียมไฟล์ปลายทางและบันทึก SQLite ให้สำเร็จก่อนลบต้นทาง
เมื่อย้ายสำเร็จ รูปจะอยู่ที่ปลายทางและหายจากโฟลเดอร์ต้นทาง
Backup ใช้ worker 2–4 ตัว และคำนวณ SHA-256 แบบ Streaming หลัง copy/verify สำเร็จ
AI ไม่ทำให้ Backup ต้องรอ: งานจะถูกบันทึกในตาราง `ai_jobs` และ Worker แยกจะอ่านจากไฟล์ปลายทางเท่านั้น
ถ้าปลายทางและ DB พร้อมแล้วแต่ลบต้นทางไม่ได้ ระบบรายงานล้มเหลวและเก็บข้อมูลทั้งสองฝั่งไว้
เมื่อกดใหม่จะเทียบข้อมูลไฟล์จริงก่อนใช้ปลายทางเดิมและลบต้นทางให้เสร็จ

## การตั้งค่า AI แบบปลอดภัย

ค่าเริ่มต้นเลือก Ollama แบบ Local เพื่อให้เปิดโปรแกรมแล้วพร้อมวิเคราะห์โดยไม่เกิดค่าใช้จ่าย Cloud
ส่วน Gemini/OpenAI ยังคงปิดไว้และต้องเปิดใช้งานเองผ่าน environment เท่านั้น

```powershell
$env:AI_PROVIDER = "ollama"      # หรือ gemini / openai / all
$env:AI_ALLOW_CLOUD = "true"      # ใช้กับ Gemini/OpenAI เมื่อยอมรับค่าใช้จ่ายแล้ว
$env:AI_ALLOW_LOCAL = "true"      # Ollama ไม่ใช้ Cloud API และไม่มี API Key
$env:GEMINI_API_KEYS = "KEY_1,KEY_2"
$env:GEMINI_MODEL = "gemini-2.5-flash"
$env:AI_MAX_CONCURRENCY = "1"
$env:AI_MAX_RETRIES = "3"
$env:AI_DAILY_LIMIT = "0"          # 0 = ไม่จำกัด; Cache Hit ไม่ถูกนับ
```

เมื่อใช้ `AI_PROVIDER=all` ระบบจะใช้เฉพาะ Provider ที่มีการตั้งค่าและได้รับอนุญาต
เช่น `AI_ALLOW_CLOUD=true` พร้อม Gemini/OpenAI key และ/หรือ
`AI_ALLOW_LOCAL=true` สำหรับ Ollama จากนั้นจะหมุน Provider เมื่อคำขอหนึ่งล้มเหลว
ผลสำเร็จจะถูกเก็บ Cache ตาม ContentHash จึงไม่ส่งภาพเดิมไปวิเคราะห์ซ้ำ

### Local YOLO + Ollama

ระบบ Local จะใช้ YOLO สร้าง Tag วัตถุ และใช้ Ollama สร้าง Description พร้อมกัน
โดย YOLO worker จะเปิด Python process ค้างไว้ ไม่เปิดโมเดลใหม่ทุกภาพ

เตรียม YOLO บนเครื่องพัฒนาเพียงครั้งเดียว:

```powershell
python -m venv .venv-yolo
.\.venv-yolo\Scripts\Activate.ps1
python -m pip install ultralytics onnxruntime
```

วาง `yolo11n.pt` ไว้ข้าง `wails.exe` หรือในโฟลเดอร์โปรเจกต์ และติดตั้ง Ollama พร้อมโมเดล:

```powershell
ollama pull qwen2.5vl:3b
```

ถ้าเครื่องปลายทางใช้ path ต่างกัน ให้ตั้ง `YOLO_PYTHON`, `YOLO_MODEL_PATH` และ
`YOLO_SCRIPT` ใน environment หรือ `config.json` โดยไม่ต้องใส่ API Key

สำหรับชุดแจกจ่าย Windows ให้รัน:

```powershell
wails build
.\scripts\package-windows.ps1
```

สคริปต์จะสร้าง `tools/bin/windows/yolo_worker.exe` และคัดลอกโมเดลไปที่
`models/yolo11n.pt` ทำให้เครื่องปลายทางไม่ต้องติดตั้ง Python หรือ Ultralytics
เพื่อใช้ Tag; Ollama ยังคงเป็นตัวเลือกเสริมสำหรับ Description

### ทดสอบจาก Source โดยไม่ติดตั้ง Python/YOLO เพิ่ม

เมื่อ Repository มีไฟล์สองรายการนี้แล้ว:

```text
models/yolo11n.pt
tools/bin/windows/yolo_worker.exe
```

สมาชิกที่ติดตั้ง Go, Node.js และ Wails CLI ตามปกติสามารถสั่ง `wails dev`
จากโฟลเดอร์ `photo-backup` แล้วใช้ Backup และวิเคราะห์ Tag ด้วย YOLO ได้ทันที
โดยไม่ต้องติดตั้ง Python, Ultralytics หรือ Ollama เพิ่มในเครื่องนั้น

ถ้า Git ไม่ได้ดาวน์โหลดไฟล์ LFS ให้อัตโนมัติ ให้ติดตั้ง Git LFS แล้วสั่ง
`git lfs pull` หนึ่งครั้งหลัง clone เพื่อดึงโมเดลและ worker ตัวจริงลงมา
เนื่องจากไฟล์ worker มีขนาดใหญ่และไม่สามารถเก็บเป็นไฟล์ Git ปกติได้

โหมด Portable นี้ให้ Tag จาก YOLO และ Description จาก BLIP โดยไม่ต้องติดตั้ง
Python หรือ Ollama หากมีไฟล์ `models/blip-image-captioning-base` และ
`tools/bin/windows/caption_worker.exe` ครบ ส่วน Ollama ยังคงเป็น fallback หรือ
ตัวเลือกสำหรับเครื่องที่มีโมเดล Vision อยู่แล้ว

Caption จะถูกส่งต่อให้โมเดล `models/opus-mt-en-mul` แปลเป็นภาษาไทยแบบ Local
ถ้าโมเดลแปลมีอยู่ครบ หากไม่มีโมเดลแปล Worker จะยังทำงานต่อและคืน Caption ภาษาอังกฤษ

Worker มีขนาดใหญ่เนื่องจากรวม PyTorch/Ultralytics ไว้ภายใน จึงควรใช้ Git LFS
หรือส่งเป็น ZIP หาก Repository ไม่รองรับไฟล์ขนาดใหญ่

ห้าม commit `config.json` หรือ API Key จริง แนะนำให้ revoke Key ที่เคยอยู่ในไฟล์/แชตและออก Key ใหม่
ดูค่าตัวอย่างที่ `config.example.json` และ `.env.example` โดยไม่มี Key จริง

## AI Pipeline แบบย่อ

`Backup → CopyImage → EqualFiles → SHA256File(ปลายทาง) → SaveBackup → สร้าง AIJob → ลบต้นทาง`

Worker จะตรวจ `BackupPath` และ Hash อีกครั้ง, ตรวจ Cache ด้วย `ContentHash + AnalysisProfile + PromptVersion`,
เรียก Provider เฉพาะเมื่อ Cache Miss, แล้วบันทึก `AIResult`, `Photo.Description` และ `photo_tags` ใน transaction เดียว
Job ที่ค้างสถานะ `processing` จะกลับเป็น `queued` เมื่อเปิดโปรแกรมใหม่
