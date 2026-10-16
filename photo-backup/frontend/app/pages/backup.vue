<template>
  <main class="backup-page">
    <div class="page-heading">
      <div>
        <h1>Backup รูปภาพ</h1>
        <p class="muted">เลือกโฟลเดอร์จาก Flash Drive แล้วตรวจสอบรายการรูปภาพก่อนเริ่ม Backup</p>
      </div>
    </div>

    <!-- ======== ส่วนเลือกโฟลเดอร์ ======== -->
    <section class="panel">
      <!-- โฟลเดอร์ต้นทาง -->
      <div class="field-group">
        <label for="source-path">โฟลเดอร์ต้นทาง</label>
        <div class="path-row">
          <input id="source-path" v-model="sourcePath" readonly placeholder="ยังไม่ได้เลือกโฟลเดอร์" />
          <button class="secondary-button" type="button" :disabled="backupRunning" @click="chooseSourceFolder">
            {{ sourcePath ? "เลือกใหม่" : "เลือกโฟลเดอร์" }}
          </button>
          <button v-if="sourcePath" class="clear-button" type="button" :disabled="backupRunning" @click="clearSourceFolder">
            ยกเลิก
          </button>
        </div>
      </div>

      <!-- โฟลเดอร์ปลายทาง -->
      <div class="field-group" style="margin-top: 16px;">
        <label for="dest-path">โฟลเดอร์ปลายทาง (Backup ไปที่)</label>
        <div class="path-row">
          <input id="dest-path" v-model="destPath" readonly placeholder="ยังไม่ได้เลือกโฟลเดอร์ปลายทาง" />
          <button class="secondary-button" type="button" :disabled="backupRunning" @click="chooseDestFolder">
            {{ destPath ? "เลือกใหม่" : "เลือกโฟลเดอร์" }}
          </button>
          <button v-if="destPath" class="clear-button" type="button" :disabled="backupRunning" @click="clearDestFolder">
            ยกเลิก
          </button>
        </div>
      </div>

      <!-- ข้อความสถานะ -->
      <div v-if="errorMessage" class="message error-message">{{ errorMessage }}</div>
      <div v-if="sourcePath && !loading && !errorMessage && imageFiles.length > 0" class="message success-message">
        พบรูปภาพ {{ imageFiles.length }} ไฟล์ที่รองรับ
      </div>
      <div v-if="loading" class="message">กำลังค้นหารูปภาพในโฟลเดอร์ย่อย...</div>
    </section>

    <!-- ======== รายการรูปภาพ ======== -->
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

      <!-- ปุ่มเริ่ม Backup -->
      <div v-if="imageFiles.length > 0" style="margin-top: 16px;">
        <button
          class="primary-button"
          type="button"
          :disabled="!sourcePath || !destPath || backupRunning"
          @click="startBackup"
        >
          {{ backupRunning ? "กำลัง Backup..." : "เริ่ม Backup" }}
        </button>
      </div>
    </section>

    <!-- ======== Progress ======== -->
    <section v-if="backupRunning || backupDone" class="panel">
      <div class="section-heading">
        <h2>{{ backupDone ? "ผลลัพธ์ Backup" : "กำลัง Backup..." }}</h2>
      </div>

      <!-- Progress Bar -->
      <div class="progress-container">
        <div class="progress-bar" :style="{ width: progressPercent + '%' }"></div>
      </div>
      <p class="progress-text">
        {{ progressCurrent }} / {{ progressTotal }} ไฟล์
        <span v-if="progressName"> — {{ progressName }}</span>
      </p>

      <!-- สรุปผล -->
      <div v-if="backupDone" class="result-summary">
        <div class="result-card success">
          <strong>{{ backupSummary.success }}</strong>
          <span>สำเร็จ</span>
        </div>
        <div class="result-card error" v-if="backupSummary.failed > 0">
          <strong>{{ backupSummary.failed }}</strong>
          <span>ล้มเหลว</span>
        </div>
      </div>

      <!-- รายการที่ล้มเหลว -->
      <div v-if="backupDone && failedResults.length > 0" style="margin-top: 12px;">
        <h3 style="color: #b91c1c; margin-bottom: 8px;">ไฟล์ที่ Backup ไม่สำเร็จ:</h3>
        <ul class="file-list">
          <li v-for="r in failedResults" :key="r.originalPath" class="file-item error-item">
            <span class="file-name">{{ r.originalName }}</span>
            <span class="file-size" style="color: #b91c1c;">{{ r.error }}</span>
          </li>
        </ul>
      </div>

      <!-- รายการที่สำเร็จ -->
      <div v-if="backupDone && successResults.length > 0" style="margin-top: 12px;">
        <h3 style="margin-bottom: 8px;">รูปที่ Backup สำเร็จ:</h3>
        <ul class="file-list">
          <li v-for="r in successResults" :key="r.backupPath" class="file-item success-item">
            <span class="file-name">{{ r.originalName }}</span>
            <span class="file-size">→ {{ r.backupPath }}</span>
          </li>
        </ul>
      </div>
    </section>
  </main>
</template>

<script setup lang="ts">
import {
  ListImageFiles,
  SelectSourceFolder,
  SelectDestFolder,
  StartBackup,
} from "../../wailsjs/go/main/App";
import { EventsOn, EventsOff } from "../../wailsjs/runtime/runtime";

interface ImageFile {
  name: string;
  path: string;
  size: number;
}

interface BackupResult {
  originalName: string;
  originalPath: string;
  backupPath: string;
  fileSize: number;
  success: boolean;
  error: string;
}

interface BackupSummary {
  total: number;
  success: number;
  failed: number;
  results: BackupResult[];
}

// ---------- State ----------
const sourcePath = ref("");
const destPath = ref("");
const imageFiles = ref<ImageFile[]>([]);
const loading = ref(false);
const errorMessage = ref("");

// Progress
const backupRunning = ref(false);
const backupDone = ref(false);
const progressCurrent = ref(0);
const progressTotal = ref(0);
const progressName = ref("");
const backupSummary = ref<BackupSummary>({ total: 0, success: 0, failed: 0, results: [] });

const progressPercent = computed(() => {
  if (progressTotal.value === 0) return 0;
  return Math.round((progressCurrent.value / progressTotal.value) * 100);
});

const successResults = computed(() => backupSummary.value.results.filter((r) => r.success));
const failedResults = computed(() => backupSummary.value.results.filter((r) => !r.success));

// ---------- เลือกโฟลเดอร์ต้นทาง ----------
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
  resetBackupState();
}

// ---------- เลือกโฟลเดอร์ปลายทาง ----------
async function chooseDestFolder() {
  try {
    const selectedPath = await SelectDestFolder();
    if (selectedPath) {
      destPath.value = selectedPath;
    }
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : "ไม่สามารถเลือกโฟลเดอร์ได้";
  }
}

function clearDestFolder() {
  destPath.value = "";
}

// ---------- เริ่ม Backup ----------
async function startBackup() {
  if (!sourcePath.value || !destPath.value) return;

  resetBackupState();
  backupRunning.value = true;
  errorMessage.value = "";

  // ดักรับ event progress จาก Go backend
  EventsOn("backup:progress", (data: any) => {
    progressCurrent.value = data.current;
    progressTotal.value = data.total;
    progressName.value = data.name;
  });

  try {
    const result = await StartBackup(sourcePath.value, destPath.value);
    backupSummary.value = result;
    backupDone.value = true;
    imageFiles.value = await ListImageFiles(sourcePath.value);
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : "Backup ล้มเหลว";
  } finally {
    backupRunning.value = false;
    EventsOff("backup:progress");
  }
}

function resetBackupState() {
  backupDone.value = false;
  progressCurrent.value = 0;
  progressTotal.value = 0;
  progressName.value = "";
  backupSummary.value = { total: 0, success: 0, failed: 0, results: [] };
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

.primary-button {
  background: #16a34a;
  color: #fff;
  font-size: 1rem;
  padding: 12px 24px;
  width: 100%;
}

.primary-button:disabled {
  background: #94a3b8;
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
  max-height: 300px;
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

.success-item {
  background: #f0fdf4;
  border-left: 3px solid #16a34a;
}

.error-item {
  background: #fef2f2;
  border-left: 3px solid #b91c1c;
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

/* ======== Progress Bar ======== */
.progress-container {
  background: #e2e8f0;
  border-radius: 999px;
  height: 14px;
  overflow: hidden;
  width: 100%;
}

.progress-bar {
  background: linear-gradient(90deg, #2563eb, #16a34a);
  border-radius: 999px;
  height: 100%;
  transition: width 0.3s ease;
}

.progress-text {
  color: #475569;
  font-size: 0.9rem;
  margin-top: 8px;
}

/* ======== ผลลัพธ์ ======== */
.result-summary {
  display: flex;
  gap: 12px;
  margin-top: 14px;
}

.result-card {
  border-radius: 10px;
  padding: 14px 20px;
  text-align: center;
  flex: 1;
}

.result-card strong {
  display: block;
  font-size: 1.5rem;
}

.result-card.success {
  background: #f0fdf4;
  color: #166534;
}

.result-card.error {
  background: #fef2f2;
  color: #b91c1c;
}

@media (max-width: 650px) {
  .path-row {
    align-items: stretch;
    flex-direction: column;
  }
}
</style>