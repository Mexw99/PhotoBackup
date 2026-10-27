<template>
  <div class="dashboard">
    <section class="intro">
      <h1>PhotoBackup</h1>
      <p>โปรแกรมสำรองรูปภาพจาก Flash Drive</p>
    </section>

    <section class="summary">
      <article class="summary-card">
        <span>สถานะ Database</span>
        <strong>{{ databaseStatus }}</strong>
      </article>

      <article class="summary-card">
        <span>รูปภาพทั้งหมด</span>
        <strong>{{ photoCount }}</strong>
      </article>

      <article class="summary-card">
        <span>Tag ทั้งหมด</span>
        <strong>{{ tagCount }}</strong>
      </article>

      <article class="summary-card">
        <span>AI วิเคราะห์แล้ว</span>
        <strong>{{ analysisStatus.completed }}</strong>
        <small v-if="!analysisStatus.enabled">ยังไม่ได้ตั้งค่า AI</small>
        <small v-else-if="analysisStatus.pending || analysisStatus.processing || analysisStatus.retrying">
          รอ {{ analysisStatus.pending + analysisStatus.processing + analysisStatus.retrying }} รูป
        </small>
        <small v-else-if="analysisStatus.failed">ล้มเหลว {{ analysisStatus.failed }} รูป</small>
        <button
          v-if="analysisStatus.failed"
          type="button"
          class="retry-analysis-button"
          :disabled="retryingAnalysis"
          @click="retryFailedAnalysis"
        >
          {{ retryingAnalysis ? "กำลังลองใหม่..." : "ลองวิเคราะห์ใหม่" }}
        </button>
      </article>
    </section>

    <p v-if="errorMessage" class="error">
      {{ errorMessage }}
    </p>

    <section class="content-grid">
      <article class="panel">
        <div class="panel-header">
          <h2>รูปภาพล่าสุด</h2>
          <span>{{ recentPhotos.length }} รูป</span>
        </div>

        <p v-if="recentPhotos.length === 0" class="empty">
          ยังไม่มีรูปภาพในระบบ
        </p>

        <div v-else class="photo-list">
          <div v-for="photo in recentPhotos" :key="photo.id" class="photo-item">
            <img
              v-if="photo.thumbnail"
              :src="photo.thumbnail"
              class="photo-thumb"
              :alt="photo.originalName"
            />
            <div v-else class="photo-icon">📷</div>

            <div>
              <strong>{{ photo.originalName }}</strong>
              <small>{{ photo.backedUpAt }}</small>
              <small v-if="photo.description">{{ photo.description }}</small>
              <small>AI: {{ analysisStatusLabel(photo.analysisStatus) }}</small>
              <small>{{ photo.backupPath }}</small>
            </div>
          </div>
        </div>
      </article>

      <article class="panel">
        <div class="panel-header">
          <h2>Tag ในระบบ</h2>
          <span>{{ tags.length }} Tag</span>
        </div>

        <p v-if="tags.length === 0" class="empty">ยังไม่มี Tag ในระบบ</p>

        <div v-else class="tag-list">
          <span v-for="tag in tags" :key="tag.id" class="tag">
            #{{ tag.name }}
          </span>
        </div>
      </article>
    </section>

    <p class="database-path">
      ตำแหน่ง Database:
      <code>{{ databasePath }}</code>
    </p>
  </div>
</template>

<script setup lang="ts">
import { useDashboard } from "~/composables/useDashboard";

const {
  databaseStatus,
  databasePath,
  photoCount,
  tagCount,
  recentPhotos,
  tags,
  analysisStatus,
  retryingAnalysis,
  retryFailedAnalysis,
  errorMessage,
} = useDashboard();

function analysisStatusLabel(status: string) {
  return {
    pending: "รอวิเคราะห์",
    processing: "กำลังวิเคราะห์",
    retrying: "กำลัง Retry / รอลองใหม่",
    completed: "วิเคราะห์แล้ว",
    failed: "วิเคราะห์ไม่สำเร็จ",
  }[status] || "รอวิเคราะห์";
}
</script>

<style scoped>
.dashboard {
  color: #172033;
}

.intro {
  margin-bottom: 24px;
}

.intro h1 {
  margin-bottom: 8px;
}

.intro p {
  color: #64748b;
  margin: 0;
}

.summary {
  display: grid;
  gap: 16px;
  grid-template-columns: repeat(4, 1fr);
  margin-bottom: 24px;
}

.summary-card,
.panel {
  background: #ffffff;
  border: 1px solid #e5e7eb;
  border-radius: 12px;
  padding: 20px;
}

.summary-card span {
  color: #64748b;
  display: block;
  font-size: 14px;
  margin-bottom: 8px;
}

.summary-card strong {
  color: #2563eb;
  font-size: 24px;
}

.summary-card small {
  color: #64748b;
  display: block;
  margin-top: 6px;
}

.retry-analysis-button {
  background: #e2e8f0;
  border: 0;
  border-radius: 6px;
  color: #334155;
  cursor: pointer;
  font-size: 12px;
  margin-top: 8px;
  padding: 6px 8px;
}

.retry-analysis-button:disabled {
  cursor: not-allowed;
  opacity: 0.6;
}

.content-grid {
  display: grid;
  gap: 24px;
  grid-template-columns: 1.5fr 1fr;
}

.panel-header {
  align-items: center;
  display: flex;
  justify-content: space-between;
  margin-bottom: 18px;
}

.panel-header h2 {
  font-size: 20px;
  margin: 0;
}

.panel-header span {
  color: #64748b;
  font-size: 14px;
}

.empty {
  color: #94a3b8;
  margin: 0;
}

.photo-list {
  display: grid;
  gap: 12px;
}

.photo-item {
  align-items: center;
  background: #f8fafc;
  border-radius: 8px;
  display: flex;
  gap: 12px;
  padding: 12px;
}

.photo-icon {
  align-items: center;
  background: #dbeafe;
  border-radius: 8px;
  display: flex;
  font-size: 24px;
  height: 48px;
  justify-content: center;
  width: 48px;
}

.photo-thumb {
  border-radius: 8px;
  flex-shrink: 0;
  height: 48px;
  object-fit: cover;
  width: 48px;
}

.photo-item strong,
.photo-item small {
  display: block;
}

.photo-item strong {
  margin-bottom: 4px;
}

.photo-item small {
  color: #64748b;
  font-size: 12px;
  margin-top: 2px;
  overflow-wrap: anywhere;
}

.tag-list {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.tag {
  background: #dbeafe;
  border-radius: 999px;
  color: #1d4ed8;
  padding: 8px 12px;
}

.database-path {
  color: #64748b;
  margin-top: 24px;
}

.database-path code {
  display: block;
  margin-top: 8px;
  overflow-wrap: anywhere;
}

.error {
  background: #fee2e2;
  border-radius: 8px;
  color: #b91c1c;
  margin-bottom: 24px;
  padding: 12px;
}

@media (max-width: 700px) {
  .summary,
  .content-grid {
    grid-template-columns: 1fr;
  }
}
</style>
