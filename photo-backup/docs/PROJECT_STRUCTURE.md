# โครงสร้าง PhotoBackup และแนวทางเพิ่มไฟล์

อิงบทเรียนใน `C:\Users\thatr\Downloads\Wails\Wails` โดยเฉพาะ
`04-Wails-Nuxt-IPC/03-Layer Architeccture.md`, `05-Wails Application/04-Backend.md`,
`05-Wails Application/06-Layers.md`, `03-Front End/01-Basic of Nuxt/05-Layout.md`
และ `03-Front End/02-Vue Engine/05-Composable & Plugin.md`

## โครงสร้าง

```text
photo-backup/
├── main.go                     # เปิด DB และประกอบ Repository → Service → App
├── app.go                      # Wails lifecycle และสถานะฐานข้อมูล
├── app_backup.go               # Native dialog และส่ง progress event
├── app_photos.go                # API ค้นหารูป และแปลงผลลัพธ์เป็น DTO
├── app_tags.go                  # API จัดการ Tag
├── app_types.go                 # DTO เดิมของ main.App สำหรับ Wails bindings
├── model/                      # GORM schema: Photo, Tag, PhotoTag, Setting, FileRecord
│   ├── photo.go                # ข้อมูลรูป คำอธิบาย สถานะ AI และเวลาแก้ไขต้นทาง
│   ├── photo_tag.go            # แยกความสัมพันธ์ Tag ที่มาจาก manual/AI
│   ├── tag.go                  # Tag และความสัมพันธ์ many-to-many
│   ├── setting.go              # ตาราง settings: ต้นทาง/ปลายทาง
│   └── file_record.go          # ตาราง files: ประวัติสำรองตามแนวกลางภาค
├── database/                   # เปิด SQLite, migration, ปิด connection
├── repository/                 # GORM queries และ transaction
│   ├── repository.go           # Store interface, constructor, context
│   ├── photo.go
│   ├── tag.go
│   ├── search.go
│   ├── settings.go
│   └── repository_test.go
├── service/                    # ประสานงาน use case และกำหนด DB timeout
│   ├── service.go
│   ├── backup.go
│   ├── photo.go
│   ├── tag.go
│   ├── settings.go
│   └── service_test.go
├── backup/                     # การทำงานกับไฟล์ แยกจาก Wails และฐานข้อมูล
│   ├── scanner.go              # ค้นหารูปภาพ
│   ├── validation.go           # ตรวจต้นทาง/ปลายทางไม่ให้ซ้อนกัน
│   ├── transfer.go             # เตรียมไฟล์ปลายทางและจัดการชื่อซ้ำก่อนย้ายสำเร็จ
│   ├── verify.go               # เปรียบเทียบข้อมูลไฟล์จริงก่อนลบต้นทาง
│   └── scanner_test.go
├── analysis/                   # Gemini Flash แยกจาก UI และฐานข้อมูล
│   ├── vision.go               # ส่งรูปและรับ JSON description/tags
│   └── vision_test.go
├── frontend/
│   ├── app/
│   │   ├── app.vue             # NuxtLayout ครอบ NuxtPage
│   │   ├── layouts/default.vue # Header, main, slot และ CSS ของโครงหน้า
│   │   ├── pages/              # เส้นทาง /, /backup, /tags, /search
│   │   ├── components/
│   │   │   ├── layout/         # app-header.vue
│   │   │   ├── gallery/        # photo-grid.vue, tag-editor.vue
│   │   │   ├── search/         # photo-detail-modal.vue แสดงรายละเอียดรูป
│   │   │   └── tag-cloud/      # tag-cloud.vue
│   │   ├── composables/        # State และการเรียก API ของแต่ละหน้า
│   │   ├── types/              # ชนิดข้อมูลร่วมที่อ้างอิง Wails generated types
│   │   └── assets/css/main.css # สไตล์ส่วนกลาง
│   ├── config/                # HMR workaround ของ Wails ตามบทเรียน
│   ├── public/                # ไฟล์ static เช่น favicon
│   ├── wailsjs/               # Wails สร้างให้ ห้ามแก้ด้วยมือ
│   ├── nuxt.config.ts         # CSR, hash routing, CSS, dist output
│   └── dist/                  # ผลลัพธ์ npm run generate
├── docs/
│   ├── PROJECT_STRUCTURE.md
│   ├── PROGRESS_DEMO.md        # เกณฑ์รอบนี้ วิธีสาธิต และคู่มือไล่อ่านโค้ด
│   ├── database/              # คู่มือฐานข้อมูล HTML/CSS/JS
│   └── planning/              # แผนแบ่งงานเดิมเพื่อใช้อ้างอิง
├── build/                     # ไอคอน manifest installer และ bin ที่ build ได้
└── wails.json                 # การเชื่อม Wails กับ Nuxt
```

## การไหลของงาน

หน้า Vue → composable → `wailsjs/go/main/App` → `app_*.go` → Service → Repository → SQLite

เมื่อสำรองรูป Service เรียก `backup` เพื่อค้นหารูป แล้วตรวจข้อมูลสำรองเดิมและชื่อซ้ำ
`backup` เตรียมไฟล์ปลายทางด้วย worker สูงสุด 4 ตัว จากนั้น Repository บันทึกรูปและประวัติ `files`
ใน transaction เดียว แล้ว Service จึงลบต้นทางด้วย `finishMove` เป็นขั้นสุดท้าย
หลังบันทึก DB แล้ว Service ส่ง Photo ID เข้าคิว AI เพื่อสร้าง description และ Tag แบบเบื้องหลัง; จำนวน worker ปรับตาม API key และ concurrency ที่ตั้งไว้
หากเป็นการทำต่อจากครั้งก่อน จะเทียบ bytes ผ่าน `backup.EqualFiles` ก่อนใช้ปลายทางเดิมและลบต้นทาง
`Skipped` หมายถึงใช้ปลายทางเดิมและลบต้นทางสำเร็จแล้ว ไม่ใช่ข้ามโดยทิ้งต้นทางไว้
ส่วน `app_backup.go` แปลง callback เป็น event `backup:progress` เพื่ออัปเดต UI
Service จึงทดสอบได้โดยไม่ต้องเปิด Wails WebView

หน้า Search ส่งชื่อไฟล์, Tag ที่เลือก, หมายเลขหน้า และขนาดหน้าผลลัพธ์ไปยัง `SearchPhotoPage(query, tags, page, size)` เพื่อจำกัดจำนวนภาพที่โหลดพร้อมกัน
ค้นเฉพาะชื่อไฟล์ และแสดงรูปที่มี Tag ใด Tag หนึ่งจากหลาย Tag ที่เลือก
ไม่มี Tag ที่เลือกจะไม่กรอง Tag; เมื่อไม่มีชื่อไฟล์ด้วยจะแสดงรูปทั้งหมด

การลบ Tag ทั้งระบบใช้ `App.DeleteTag` → `Service.DeleteTag` → `Repository.DeleteTag`
โดย Repository ลบความสัมพันธ์ใน `photo_tags` และแถวใน `tags` ภายใน transaction เดียว
ส่วน `RemoveTagFromPhoto` ใช้ลบเฉพาะความสัมพันธ์ของรูปหนึ่งรูปตามเดิม

## กติกาการเพิ่มไฟล์

- หน้าใหม่สร้างใน `frontend/app/pages/` และเพิ่มเมนูใน `components/layout/app-header.vue`
- โครงหน้าจอที่ใช้ซ้ำสร้างใน `app/layouts/` โดยแสดงเนื้อหาผ่าน `<slot />`
- `app.vue` ประกอบ `<NuxtLayout>` กับ `<NuxtPage>`; `layouts/default.vue` ดูแล Header และ `<main>` กลาง ส่วนแต่ละ Page มีเฉพาะเนื้อหาของหน้านั้น
- CSS ของโครงหน้าอยู่ใน `<style scoped>` ของ Layout; `app/assets/css/main.css` เก็บเฉพาะสไตล์ที่ใช้ทั่วแอป
- ชื่อไฟล์ Page และ Component ใช้ภาษาอังกฤษตัวพิมพ์เล็กแบบ `kebab-case` ตามบท Routing; ชื่อแท็ก Component ใน template ยังใช้ PascalCase ได้
- คอมโพเนนต์จัดกลุ่มตามหน้าที่ เช่น gallery หรือ search ไม่ซ้อน gallery ไว้ใน layout
- State และ async API calls ของหน้าให้อยู่ใน `app/composables/use*.ts`
- ชนิดข้อมูลร่วมอยู่ใน `app/types/` และอ้างอิง generated types เพื่อลดการประกาศซ้ำ
- ใช้ `~/` สำหรับไฟล์ภายใน `frontend/app` และ `~~/wailsjs/` สำหรับ bridge ที่ frontend root
- Public method ที่ให้หน้าเว็บเรียกอยู่ใน `app_*.go`; ให้ทำหน้าที่รับคำสั่งและจัดรูปผลลัพธ์
- งานประสานหลายขั้นตอนอยู่ใน `service/`; SQL/GORM และ transaction อยู่ใน `repository/`
- Schema อยู่ใน `model/`; งาน filesystem อยู่ใน `backup/`; การวิเคราะห์ AI อยู่ใน `analysis/`
- งานฐานข้อมูลใน Service ใช้ context ของแอปพร้อม timeout 10 วินาทีต่อ operation
- เมื่อแก้ public methods หรือ DTO ให้รัน `wails generate module` และตรวจ typecheck ใหม่

## การปรับบทเรียนให้เข้ากับโปรเจกต์เดิม

ใช้ชื่อโฟลเดอร์เอกพจน์ `model`, `repository`, `service` ตามบทเรียน โดยแยก database
และงานไฟล์เป็นแพ็กเกจเฉพาะ คง GORM models ที่เขียนอยู่เดิมและ AutoMigrate ไว้
จึงไม่เพิ่ม GORM Gen หรือสร้าง query helpers ที่ยังไม่มีผู้เรียกใช้
ไม่เพิ่ม controller REST, system server หรือระบบสมัครสมาชิกจากตัวอย่าง Foodie
เพราะ PhotoBackup เรียก Go ผ่าน Wails IPC

คง `main.App` เป็นจุดเชื่อม Wails และ event `backup:progress` พร้อมเพิ่ม API/DTO สำหรับความสามารถใหม่
ใช้ GORM AutoMigrate เพิ่ม schema ที่ต้องใช้ในฐานข้อมูลเดิม
ตำแหน่งฐานข้อมูลคือ `os.UserConfigDir()/PhotoBackup/photo-backup.db`
ไม่เก็บฐานข้อมูลผู้ใช้ไว้ใน source tree

**พฤติกรรมการสำรองปัจจุบัน:** ย้ายไฟล์ โดยลบต้นทางหลังไฟล์ปลายทางและ SQLite พร้อมแล้ว
รองรับ JPG, JPEG และ PNG โดยเตรียมไฟล์หลายรายการพร้อมกัน แต่บันทึก SQLite ทีละรายการเพื่อป้องกัน database lock
ชื่อซ้ำในต้นทาง/ปลายทาง/ประวัติ Database จะถูกตรวจจากชื่อไฟล์เท่านั้น (ไม่เทียบ bytes) ข้ามและเก็บต้นฉบับไว้ พร้อมแสดงชื่อ/ตำแหน่ง/เหตุผลในผลลัพธ์หน้า Backup; ไม่เขียนทับหรือสร้างชื่อ `(1)`
ตาราง `settings/files` ใช้รูปแบบจากแกนงานกลางภาค แต่ไม่ได้นำเข้าฐานข้อมูลของโปรเจกต์อ้างอิงอัตโนมัติ

Tag จาก AI และ Tag ที่ผู้ใช้เพิ่มใช้ตาราง `tags/photo_tags` เดียวกัน โดยคอลัมน์ `source` แยก `ai` กับ `manual`
การวิเคราะห์ซ้ำจะลบเฉพาะความสัมพันธ์ Tag ที่มาจาก AI และคง Tag ของผู้ใช้ไว้
ลบ Tag ที่หน้า Tag คือการลบแถวและความสัมพันธ์ ไม่ลบรูป
ไม่มีการวิเคราะห์สีด้วยโค้ดของเราเองแล้ว การวิเคราะห์ AI มีสถานะ `pending`, `processing`, `completed`, `failed`

## ตรวจสอบก่อนส่งงาน

จากโฟลเดอร์ `photo-backup`:

```powershell
go test ./...
go vet ./...
wails generate module
cd frontend
npm run typecheck
npm run generate
cd ..
wails build
```

Go tests ใช้ SQLite และรูปจำลองใน temporary directories ไม่ใช้ฐานข้อมูลหรือรูปจริงของผู้ใช้
ให้ตรวจเส้นทาง Backup → บันทึก DB → ลบต้นทาง → ค้นชื่อไฟล์/Tag → ลบ Tag
รวมทั้งการลองใหม่เมื่อลบต้นทางไม่สำเร็จ และการส่ง context ลงไปถึง Repository
