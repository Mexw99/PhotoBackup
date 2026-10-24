# คู่มืออ่านโค้ดและเตรียม Progress/Technical Demo

PhotoBackup ใช้ Wails v2 + Nuxt 4 + Go + SQLite/GORM สำหรับย้ายรูปจาก Flash Drive หรือโทรศัพท์ไปยังปลายทาง
ระบบยังคงพฤติกรรมเดิมของงานกลางภาค คือ Copy → Verify → บันทึก SQLite → ลบต้นทางเป็นขั้นตอนสุดท้าย
และเพิ่ม AI Pipeline ที่แยกจาก Backup เพื่อไม่ให้การวิเคราะห์ทำให้การย้ายไฟล์ช้า/ล้มเหลว

## สรุป Feature ตามเกณฑ์

| เกณฑ์ | ไฟล์/เทคนิคที่ใช้ |
| --- | --- |
| Wails + Nuxt, Windows/macOS | `main.go`, `app*.go`, `frontend/app/pages/`, `wails.json` |
| SQLite + GORM | `database/database.go`, `model/`, `repository/` |
| Backup เทียบเท่ากลางภาค | `backup/transfer.go`, `backup/verify.go`, `service/backup.go` |
| วิเคราะห์ Description/Tag ด้วย AI | `analysis/gemini.go`, `analysis/openai.go`, `analysis/ollama.go`, `service/ai_queue.go` |
| ค้นหาหลายรูปและ Tag Cloud | `repository/search.go`, `app_photos.go`, `usePhotoSearch.ts`, `tag-cloud.vue` |

## Flow สำคัญที่ควรอธิบายใน Demo

### 1. เปิดโปรแกรมและต่อชั้นระบบ

`main.go` เปิด `database.OpenDefault()` แล้วประกอบ `Repository → Service → App → Wails`.
`database.OpenAt()` เรียก GORM `AutoMigrate` เพิ่ม/ตรวจตารางโดยไม่ลบข้อมูลเดิม
เมื่อ Wails เรียก `App.startup()` จะส่ง context ให้ Service และเริ่ม AI Queue Worker ถ้ามี Analyzer ที่เปิดใช้งาน

### 2. Backup และการป้องกันไฟล์ต้นฉบับ

`frontend/app/pages/backup.vue` เรียก `useBackup.ts` ซึ่งเรียก Wails binding `App.StartBackup`.
`service.StartBackup` ตรวจโฟลเดอร์ด้วย `backup.ValidateFolders` และค้นไฟล์ด้วย `backup.DiscoverImages`.
`backupPhoto` ทำตามลำดับนี้:

1. `analysis.ValidateImage` ใช้ `image.DecodeConfig` อ่าน header โดยไม่โหลด pixel ทั้งไฟล์
2. `backup.CopyImage` ใช้ `O_EXCL` จึงไม่เขียนทับไฟล์เดิม และเติม `(1)`, `(2)` เมื่อชื่อซ้ำ
3. `backup.EqualFiles` เปรียบเทียบ byte ทีละ 64 KB
4. `analysis.SHA256File` คำนวณ Hash แบบ Streaming จากไฟล์ปลายทางที่ผ่าน Verify แล้ว
5. `repository.SaveBackup` บันทึก `photos`, `files` และความสัมพันธ์ใน transaction
6. สร้าง `AIJob` แบบ best-effort หลัง DB สำเร็จ งาน AI ไม่ทำให้ Backup ล้มเหลว
7. `finishMove` ตรวจปลายทางและ source ก่อนเรียก `os.Remove`

ถ้า Copy/Verify/DB ล้มเหลว จะลบเฉพาะสำเนาใหม่และเก็บต้นฉบับไว้
ถ้าลบต้นทางไม่ได้ จะเก็บทั้งต้นทาง ปลายทาง และ DB ไว้เพื่อกด Retry ได้

### 3. Content Hash และ Cache

`model.Photo.ContentHash` เป็น nullable เพื่อรองรับข้อมูลเก่า และมี index
`model.AIResult` เก็บ `ContentHash`, `AnalysisProfile`, `Provider`, `Model`, `PromptVersion`, `Description`, `TagsJSON`.
Cache Key คือ `ContentHash + AnalysisProfile + PromptVersion` โดย Provider/Model เป็นข้อมูลอ้างอิง ไม่ใช่ส่วนของ Key
ไฟล์คนละชื่อที่มี byte เหมือนกันจึงใช้ผลเดียวกันได้ ส่วนไฟล์ที่เนื้อหาเปลี่ยนจะมี Hash ใหม่

`repository.FindAIResult` ตรวจ Cache ก่อนเรียก Analyzer
`repository.SaveAIResultAndApply` ใช้ transaction เดียว upsert Cache และเติม Description/Tags ให้ทุก Photo ที่ Hash เดียวกัน

### 4. AI Job Queue

`model.AIJob` เก็บสถานะ `queued`, `processing`, `completed`, `failed`, `retry_wait`, จำนวนครั้ง, error และเวลา
Unique Key ของ Job คือ `PhotoID + AnalysisProfile + PromptVersion` จึงไม่สร้างงานซ้ำ
เมื่อเริ่มโปรแกรม `ResetProcessingAIJobs` เปลี่ยนงานที่ค้าง `processing` กลับเป็น `queued`
Worker อ่านเฉพาะ `Photo.BackupPath` และตรวจ SHA-256 ซ้ำก่อนส่ง AI ห้ามอ่าน/แก้/ลบต้นฉบับ

หน้า Backup แสดงสถานะผ่าน `App.GetAIQueueStatus` และ event `ai:progress` จึงเปลี่ยนไปหน้าอื่นแล้วกลับมาดูได้

### 5. Provider, ค่าใช้จ่าย และ Retry

`analysis/router.go` เลือก Provider เพียงตัวเดียวจาก `AI_PROVIDER`; ค่า `all` หรือรายการคั่น comma ไม่ถูกหมุนอัตโนมัติ
ค่าเริ่มต้น `AI_ALLOW_CLOUD=false` และ `AI_ALLOW_LOCAL=false` เพื่อไม่เรียกบริการเองเมื่อเปิดโปรแกรม
เปิด Cloud หรือ Local ต้องตั้งค่าเองใน environment

HTTP 429/5xx และ network error เป็นกลุ่มที่ Retry ได้ตาม `AI_MAX_RETRIES`
รองรับ `Retry-After` และ Exponential Backoff
`AI_DAILY_LIMIT` นับคำขอที่เรียก Provider จริงในตาราง `ai_usages`; Cache Hit ไม่ถูกนับ
Error ที่เก็บใน `AIJob.LastError` จะ redact ค่า Key จาก environment

### 6. Local Ollama และ Cloud Preprocessing

เมื่อเลือก Ollamaและอนุญาต Local แล้ว `OllamaAnalyzer.HealthCheck` เรียก `/api/tags` เพื่อแยก `ready`, `not_running`, `model_missing`, `model_unusable`
ไม่ดาวน์โหลด Model และไม่บังคับ GPU NVIDIA; CPU ใช้ได้แต่ช้ากว่า ควรตั้ง `AI_MAX_CONCURRENCY=1`

Gemini/OpenAI ใช้ `readImageForCloudAI` resize ภาพใหญ่ในหน่วยความจำด้านยาวไม่เกิน 1600px
ไม่เขียนทับไฟล์ Backup และ ContentHash คำนวณก่อน preprocessing เสมอ
PNG จะคงรูปแบบ PNG เพื่อไม่ทำลาย transparency; EXIF orientation ยังเป็นข้อจำกัดที่ต้องทดสอบกับภาพจริง

## ตารางฐานข้อมูล

| ตาราง | หน้าที่ |
| --- | --- |
| `photos` | path ต้นทาง/ปลายทาง, ขนาด, เวลา, `content_hash`, Description |
| `tags` | ชื่อ Tag กลาง |
| `photo_tags` | ความสัมพันธ์หลายต่อหลายระหว่าง Photo กับ Tag |
| `files` | ประวัติ Backup ตามงานกลางภาค |
| `settings` | source/destination ที่ผู้ใช้บันทึก |
| `ai_results` | Cache ผล AI ตาม Content Hash/Profile/Prompt |
| `ai_jobs` | Queue งาน AI ที่ทำต่อหลังเปิดโปรแกรมใหม่ได้ |
| `ai_usages` | ตัวนับคำขอ Provider จริงรายวัน |

Migration ใช้ GORM `AutoMigrate` เพิ่มคอลัมน์/ตาราง/Index เท่านั้น
Integration tests ใน `database/database_test.go` ใช้ SQLite ใน `t.TempDir()` และตรวจว่า Description, Tags, `photo_tags`, files, settings เดิมยังอยู่

## เส้นทาง Search

`pages/search.vue` เก็บช่องคำค้นและ Tag ที่เลือก → `usePhotoSearch.ts` → `App.SearchPhotoLibrary(query, tags)`
→ `service.SearchPhotoLibrary` → `repository.SearchPhotos`.
Repository trim ช่องว่าง แปลงตัวพิมพ์เป็น lower-case และค้น `original_name` หรือ `description` ด้วย `LIKE`
ถ้ามี Tag จะใช้ subquery ของ `photo_tags`/`tags` แบบ OR และ `Preload("Tags")` เพื่อแสดงหลายรูปพร้อม Tag
`GetTagCloud` นับจำนวนรูปต่อ Tag และ `tag-cloud.vue` จำกัดพื้นที่พร้อม scroll เพื่อไม่ให้ Tag จำนวนมากทำให้หน้าแตก

## การตั้งค่าและความปลอดภัย

ใช้ `.env.example` และ `config.example.json` เป็นตัวอย่างเท่านั้น ห้ามใส่ API Key จริงใน Git
หากเคยใส่ Key จริงใน `config.json`, แชต หรือไฟล์ที่แชร์ ให้ revoke และออก Key ใหม่
ห้ามพิมพ์ Key ใน log/error/test และไม่ควรส่ง `config.json` ให้เพื่อนหรือ commit ขึ้น remote

## คำสั่งตรวจสอบ

```powershell
go build ./...
go vet ./...
go test ./...
wails generate module
cd frontend
npm run typecheck
npm run generate
cd ..
wails build
```

Unit/Integration tests ใช้ไฟล์ชั่วคราวและ Fake Analyzer/HTTP test เท่านั้น ไม่เรียก Gemini, OpenAI หรือ Ollama จริง
ยังต้องทดสอบแยกบน macOS, Flash Drive จริง, Ollama จริง และ Cloud API จริงก่อนสาธิต Final

## งาน Performance ที่แยกไว้

Search Pagination, Virtual Scrolling และ Persistent Thumbnail ยังไม่รวมใน AI Pipeline นี้
ควรทำเป็นงานถัดไปหลังตรวจความถูกต้องของ Backup, Hash, Cache และ Queue แล้ว
