<template>
  <main class="backup-page">
    <div class="page-heading">
      <div>
        <p class="eyebrow">PhotoBackup</p>
        <h1>Backup รูปภาพ</h1>
        <p class="muted">เลือกโฟลเดอร์จาก Flash Drive แล้วตรวจสอบรายการรูปภาพก่อนเริ่ม Backup</p>
      </div>
    </div>

    <section class="panel">
      <div class="field-group">
        <label for="source-path">โฟลเดอร์ต้นทาง</label>
        <div class="path-row">
          <input id="source-path" v-model="sourcePath" readonly placeholder="ยังไม่ได้เลือกโฟลเดอร์" />
          <button class="secondary-button" type="button" :disabled="loading" @click="chooseSourceFolder">
            {{ sourcePath ? "เลือกใหม่" : "เลือกโฟลเดอร์" }}
          </button>
          <button v-if="sourcePath" class="clear-button" type="button" :disabled="loading" @click="clearSourceFolder">
            ยกเลิกการเลือก
          </button>
        </div>
      </div>

      <div v-if="errorMessage" class="message error-message">{{ errorMessage }}</div>
      <div v-if="sourcePath && !loading && !errorMessage" class="message success-message">
        พบรูปภาพ {{ imageFiles.length }} ไฟล์ที่รองรับ
      </div>
      <div v-if="loading" class="message">กำลังค้นหารูปภาพในโฟลเดอร์ย่อย...</div>
    </section>

    <section class="panel">
      <div class="section-heading">
        <h2>รายการรูปภาพ</h2>
        <span class="count-badge">{{ imageFiles.length }}</span>
      </div>

      <div v-if="!imageFiles.length && !loading" class="empty-state">
        เลือกโฟลเดอร์เพื่อแสดงรายการไฟล์ .jpg, .jpeg และ .png
      </div>
      <ul v-else class="file-list">
        <li v-for="file in imageFiles" :key="file.path" class="file-item">
          <span class="file-name">{{ file.name }}</span>
          <span class="file-size">{{ formatBytes(file.size) }}</span>
        </li>
      </ul>
    </section>
  </main>
</template>

<script setup lang="ts">
import { ListImageFiles, SelectSourceFolder } from "../../wailsjs/go/main/App";

interface ImageFile {
  name: string;
  path: string;
  size: number;
}

const sourcePath = ref("");
const imageFiles = ref<ImageFile[]>([]);
const loading = ref(false);
const errorMessage = ref("");

async function chooseSourceFolder() {
  errorMessage.value = "";
  loading.value = true;

  try {
    const selectedPath = await SelectSourceFolder();
    if (!selectedPath) return;

    sourcePath.value = selectedPath;
    imageFiles.value = await ListImageFiles(selectedPath);
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : "ไม่สามารถอ่านโฟลเดอร์ได้";
    imageFiles.value = [];
  } finally {
    loading.value = false;
  }
}

function clearSourceFolder() {
  sourcePath.value = "";
  imageFiles.value = [];
  errorMessage.value = "";
}

function formatBytes(size: number) {
  if (size < 1024) return `${size} B`;
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KB`;
  return `${(size / (1024 * 1024)).toFixed(1)} MB`;
}
</script>

<style scoped>
.backup-page {
  display: grid;
  gap: 20px;
  padding: 12px 0;
}

.page-heading h1,
.section-heading h2 {
  margin: 0;
}

.eyebrow {
  color: #2563eb;
  font-size: 0.8rem;
  font-weight: 700;
  letter-spacing: 0.08em;
  margin: 0 0 6px;
  text-transform: uppercase;
}

.muted,
.empty-state {
  color: #64748b;
}

.panel {
  background: #fff;
  border: 1px solid #e2e8f0;
  border-radius: 12px;
  padding: 20px;
}

.field-group {
  display: grid;
  gap: 8px;
}

.field-group label {
  font-weight: 700;
}

.path-row {
  display: flex;
  gap: 10px;
}

input {
  background: #f8fafc;
  border: 1px solid #cbd5e1;
  border-radius: 8px;
  flex: 1;
  min-width: 0;
  padding: 10px 12px;
}

button {
  border: 0;
  border-radius: 8px;
  cursor: pointer;
  font-weight: 700;
  padding: 10px 16px;
}

.secondary-button {
  background: #2563eb;
  color: #fff;
}

.clear-button {
  background: #e2e8f0;
  color: #334155;
}

button:disabled {
  cursor: not-allowed;
  opacity: 0.6;
}

.message {
  background: #f8fafc;
  border-radius: 8px;
  color: #475569;
  margin-top: 14px;
  padding: 10px 12px;
}

.success-message {
  background: #f0fdf4;
  color: #166534;
}

.error-message {
  background: #fef2f2;
  color: #b91c1c;
}

.section-heading {
  align-items: center;
  display: flex;
  gap: 10px;
  margin-bottom: 14px;
}

.count-badge {
  background: #dbeafe;
  border-radius: 999px;
  color: #1d4ed8;
  font-size: 0.85rem;
  padding: 3px 9px;
}

.empty-state {
  border: 1px dashed #cbd5e1;
  border-radius: 8px;
  padding: 24px;
  text-align: center;
}

.file-list {
  display: grid;
  gap: 8px;
  list-style: none;
  margin: 0;
  max-height: 420px;
  overflow: auto;
  padding: 0;
}

.file-item {
  align-items: center;
  background: #f8fafc;
  border-radius: 8px;
  display: flex;
  gap: 12px;
  justify-content: space-between;
  padding: 10px 12px;
}

.file-name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.file-size {
  color: #64748b;
  flex: 0 0 auto;
  font-size: 0.85rem;
}

@media (max-width: 650px) {
  .path-row {
    align-items: stretch;
    flex-direction: column;
  }
}
</style>
