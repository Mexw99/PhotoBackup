<template>
  <div class="backup-page">
    <div class="page-heading">
      <div>
        <h1>Backup รูปภาพ</h1>
        <p class="muted">
          เลือกโฟลเดอร์จาก Flash Drive แล้วตรวจสอบรายการรูปภาพก่อนเริ่ม Backup
        </p>
        <p class="muted">
          ย้ายรูปภาพไปยังปลายทาง และลบไฟล์ต้นทางเฉพาะรูปที่ย้ายสำเร็จ
        </p>
      </div>
    </div>

    <!-- ======== ส่วนเลือกโฟลเดอร์ ======== -->
    <section class="panel">
      <!-- โฟลเดอร์ต้นทาง -->
      <div class="field-group">
        <label for="source-path">โฟลเดอร์ต้นทาง</label>
        <div class="path-row">
          <input
            id="source-path"
            v-model="sourcePath"
            readonly
            placeholder="ยังไม่ได้เลือกโฟลเดอร์"
          />
          <button
            class="secondary-button"
            type="button"
            :disabled="backupRunning || loading || settingsSaving"
            @click="chooseSourceFolder"
          >
            {{ sourcePath ? "เลือกใหม่" : "เลือกโฟลเดอร์" }}
          </button>
          <button
            v-if="sourcePath"
            class="clear-button"
            type="button"
            :disabled="backupRunning || loading || settingsSaving"
            @click="clearSourceFolder"
          >
            ยกเลิก
          </button>
        </div>
      </div>

      <!-- โฟลเดอร์ปลายทาง -->
      <div class="field-group" style="margin-top: 16px">
        <label for="dest-path">โฟลเดอร์ปลายทาง (Backup ไปที่)</label>
        <div class="path-row">
          <input
            id="dest-path"
            v-model="destPath"
            readonly
            placeholder="ยังไม่ได้เลือกโฟลเดอร์ปลายทาง"
          />
          <button
            class="secondary-button"
            type="button"
            :disabled="backupRunning || loading || settingsSaving"
            @click="chooseDestFolder"
          >
            {{ destPath ? "เลือกใหม่" : "เลือกโฟลเดอร์" }}
          </button>
          <button
            v-if="destPath"
            class="clear-button"
            type="button"
            :disabled="backupRunning || loading || settingsSaving"
            @click="clearDestFolder"
          >
            ยกเลิก
          </button>
        </div>
      </div>

      <div class="folder-actions">
        <button
          class="clear-button"
          type="button"
          :disabled="!sourcePath || backupRunning || loading || settingsSaving"
          @click="readSourceFiles"
        >
          ตรวจรายการใหม่
        </button>
        <button
          class="clear-button"
          type="button"
          :disabled="!sourcePath || !destPath || backupRunning || loading || settingsSaving"
          @click="saveSettings"
        >
          {{ settingsSaving ? "กำลังบันทึก..." : "บันทึกโฟลเดอร์" }}
        </button>
        <span class="muted">ระบบจะบันทึกโฟลเดอร์ให้อีกครั้งเมื่อเริ่ม Backup</span>
      </div>

      <!-- ข้อความสถานะ -->
      <div v-if="errorMessage" class="message error-message">
        {{ errorMessage }}
      </div>
      <div v-if="settingsMessage" class="message success-message" role="status">
        {{ settingsMessage }}
      </div>
      <div
        v-if="sourcePath && !loading && !errorMessage && imageFiles.length > 0"
        class="message success-message"
      >
        พบรูปภาพ {{ imageFiles.length }} ไฟล์ที่รองรับ
      </div>
      <div v-if="loading" class="message">
        กำลังค้นหารูปภาพในโฟลเดอร์ย่อย...
      </div>
    </section>

    <!-- ======== รายการรูปภาพ ======== -->
    <section class="panel">
      <div class="section-heading">
        <h2>รายการรูปภาพ</h2>
        <span class="count-badge">{{ imageFiles.length }}</span>
      </div>

      <div v-if="!imageFiles.length && !loading" class="empty-state">
        <span v-if="backupDone && !errorMessage">
          ย้ายรูปภาพแล้ว ไม่มีรูปภาพเหลือในโฟลเดอร์ต้นทาง
        </span>
        <span v-else-if="sourcePath">
          ไม่พบรูปภาพที่อ่านได้ กรุณาเลือกโฟลเดอร์ที่มีไฟล์ .jpg, .jpeg หรือ .png
        </span>
        <span v-else>เลือกโฟลเดอร์เพื่อแสดงรายการไฟล์ .jpg, .jpeg และ .png</span>
      </div>
      <ul v-else class="file-list">
        <li v-for="file in imageFiles" :key="file.path" class="file-item">
          <span class="file-name">{{ file.name }}</span>
          <span class="file-size">{{ formatBytes(file.size) }}</span>
        </li>
      </ul>

      <!-- ปุ่มเริ่ม Backup -->
      <div v-if="imageFiles.length > 0" style="margin-top: 16px">
        <button
          class="primary-button"
          type="button"
          :disabled="!sourcePath || !destPath || backupRunning || loading || settingsSaving"
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
        <div
          class="progress-bar"
          :style="{ width: progressPercent + '%' }"
        ></div>
      </div>
      <p class="progress-text">
        {{ progressCurrent }} / {{ progressTotal }} ไฟล์
        <span v-if="progressName"> — {{ progressName }}</span>
        <span class="elapsed-text"> · ใช้เวลา {{ elapsedLabel }}</span>
      </p>

      <!-- สรุปผล -->
      <div v-if="backupDone" class="result-summary">
        <div class="result-card success">
          <strong>{{ backupSummary.success }}</strong>
          <span>ย้ายสำเร็จ</span>
        </div>
        <div class="result-card skipped">
          <strong>{{ backupSummary.skipped }}</strong>
          <span>ใช้ไฟล์ปลายทางเดิม</span>
        </div>
        <div class="result-card error">
          <strong>{{ backupSummary.failed }}</strong>
          <span>ล้มเหลว</span>
        </div>
      </div>

      <div v-if="backupDone && skippedResults.length > 0" class="skipped-list">
        <h3>รูปที่ใช้ไฟล์ปลายทางเดิมและย้ายสำเร็จ:</h3>
        <ul class="file-list">
          <li
            v-for="result in skippedResults"
            :key="result.originalPath"
            class="file-item"
          >
            <span class="file-name">{{ result.originalName }}</span>
            <span class="file-size">ตรวจไฟล์เดิมแล้ว ลบต้นทางสำเร็จ</span>
          </li>
        </ul>
      </div>

      <!-- รายการที่ล้มเหลว -->
      <div
        v-if="backupDone && failedResults.length > 0"
        style="margin-top: 12px"
      >
        <h3 style="color: #b91c1c; margin-bottom: 8px">
          ไฟล์ที่ยังย้ายไม่สำเร็จ (ยังอยู่ต้นทาง):
        </h3>
        <ul class="file-list">
          <li
            v-for="r in failedResults"
            :key="r.originalPath"
            class="file-item error-item"
          >
            <span class="file-name">{{ r.originalName }}</span>
            <span class="file-size" style="color: #b91c1c">{{ r.error }}</span>
          </li>
        </ul>
      </div>

      <!-- รายการที่สำเร็จ -->
      <div
        v-if="backupDone && successResults.length > 0"
        style="margin-top: 12px"
      >
        <h3 style="margin-bottom: 8px">รูปที่ย้ายไปยังปลายทางแล้ว:</h3>
        <ul class="file-list">
          <li
            v-for="r in successResults"
            :key="r.backupPath"
            class="file-item success-item"
          >
            <span class="file-name">{{ r.originalName }}</span>
            <span class="file-size">→ {{ r.backupPath }}</span>
          </li>
        </ul>
      </div>
    </section>

    <section class="panel ai-status-panel">
      <div class="section-heading">
        <h2>สถานะการวิเคราะห์ AI</h2>
      </div>
      <p class="muted">
        AI ทำงานแยกจาก Backup และอ่านจากไฟล์ปลายทางที่ตรวจสอบแล้ว
      </p>
      <div class="ai-status-grid">
        <span>รอคิว <strong>{{ aiQueueStatus.queued }}</strong></span>
        <span>กำลังวิเคราะห์ <strong>{{ aiQueueStatus.processing }}</strong></span>
        <span>เสร็จแล้ว <strong>{{ aiQueueStatus.completed }}</strong></span>
        <span>รอลองใหม่ <strong>{{ aiQueueStatus.retryWait }}</strong></span>
        <span>ล้มเหลว <strong>{{ aiQueueStatus.failed }}</strong></span>
      </div>
      <p v-if="aiQueueStatus.queued || aiQueueStatus.processing || aiQueueStatus.retryWait" class="ai-elapsed-text">
        กำลังทำงานมาแล้ว <strong>{{ aiElapsedLabel }}</strong>
      </p>
      <p v-else-if="aiQueueStatus.completed" class="ai-elapsed-text">
        วิเคราะห์รอบล่าสุดใช้เวลา <strong>{{ aiElapsedLabel }}</strong>
      </p>
      <div class="ai-provider-status" :class="`state-${aiProviderStatus.state}`">
        <strong>Provider:</strong> {{ aiProviderStatus.provider || "ยังไม่ได้ตั้งค่า" }}
        <span v-if="aiProviderStatus.model"> · Model: {{ aiProviderStatus.model }}</span>
        <span> · {{ aiProviderStatus.message || "" }}</span>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { useBackup } from "~/composables/useBackup";

const {
  sourcePath,
  destPath,
  imageFiles,
  loading,
  errorMessage,
  settingsMessage,
  settingsSaving,
  backupRunning,
  backupDone,
  progressCurrent,
  progressTotal,
  progressName,
  elapsedLabel,
  backupSummary,
  progressPercent,
  successResults,
  skippedResults,
  failedResults,
  aiQueueStatus,
  aiElapsedLabel,
  aiProviderStatus,
  readSourceFiles,
  chooseSourceFolder,
  clearSourceFolder,
  chooseDestFolder,
  clearDestFolder,
  saveSettings,
  startBackup,
  formatBytes,
} = useBackup();
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

.folder-actions {
  align-items: center;
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  margin-top: 16px;
}

.folder-actions span {
  font-size: 0.85rem;
}

.skipped-list {
  margin-top: 12px;
}

.skipped-list h3 {
  margin-bottom: 8px;
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

.ai-provider-status {
  border-radius: 8px;
  margin-top: 12px;
  padding: 10px 12px;
  font-size: 0.86rem;
}

.state-configured,
.state-ready {
  background: #f0fdf4;
  color: #166534;
}

.state-disabled,
.state-not_running,
.state-model_missing,
.state-model_unusable {
  background: #fff7ed;
  color: #9a3412;
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

.elapsed-text {
  color: #1d4ed8;
  font-weight: 700;
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

.result-card.skipped {
  background: #eff6ff;
  color: #1d4ed8;
}

.ai-status-panel {
  background: #f8fafc;
}

.ai-status-grid {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  margin-top: 12px;
}

.ai-status-grid span {
  background: #fff;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  color: #475569;
  padding: 8px 12px;
}

.ai-status-grid strong {
  color: #1d4ed8;
  margin-left: 4px;
}

@media (max-width: 650px) {
  .path-row {
    align-items: stretch;
    flex-direction: column;
  }
}
</style>
