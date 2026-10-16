# PhotoBackup frontend

Nuxt 4 ทำงานแบบ client-side rendering ภายใน Wails WebView โดยใช้ hash routing
และสร้าง static output ไปที่ `dist/` สำหรับฝังใน Go binary

```powershell
npm ci
npm run typecheck
npm run generate
```

เริ่มพัฒนาแอปพร้อม Go bridge ด้วย `wails dev` จากโฟลเดอร์ด้านบน
คำสั่ง `npm run dev` เปิดเฉพาะ frontend; native APIs ต้องเรียกจาก Wails

- `app/pages/`: template และการประกอบคอมโพเนนต์ตาม route
- `app/layouts/`: โครงหน้าจอร่วม
- `app/components/`: ส่วนแสดงผลที่นำกลับมาใช้ซ้ำ
- `app/composables/`: state, API calls และ lifecycle ของแต่ละหน้า
- `app/types/`: ชนิดข้อมูลร่วมที่อ้างอิง Wails generated types
- `app/assets/css/`: CSS ส่วนกลาง
- `config/`: build/dev helpers
- `wailsjs/`: generated code จาก `wails generate module` หรือ `wails dev`

ดูรายละเอียดที่ [แนวทางจัดโครงสร้างโปรเจกต์](../docs/PROJECT_STRUCTURE.md)
