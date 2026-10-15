<template>
  <main class="dashboard">
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
            <div class="photo-icon">📷</div>

            <div>
              <strong>{{ photo.originalName }}</strong>
              <small>{{ photo.backedUpAt }}</small>
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
  </main>
</template>

<script setup lang="ts">
import {
  GetDatabasePath,
  GetDatabaseStatus,
  GetPhotoCount,
  GetRecentPhotos,
  GetTagCount,
  GetTags,
} from "../../wailsjs/go/main/App";

import type { main } from "../../wailsjs/go/models";

const databaseStatus = ref("กำลังตรวจสอบ...");
const databasePath = ref("");
const photoCount = ref(0);
const tagCount = ref(0);
const recentPhotos = ref<main.PhotoSummary[]>([]);
const tags = ref<main.TagSummary[]>([]);
const errorMessage = ref("");

onMounted(async () => {
  try {
    const [status, path, photosCount, tagsCount, photos, tagList] =
      await Promise.all([
        GetDatabaseStatus(),
        GetDatabasePath(),
        GetPhotoCount(),
        GetTagCount(),
        GetRecentPhotos(6),
        GetTags(),
      ]);

    databaseStatus.value = status;
    databasePath.value = path;
    photoCount.value = photosCount;
    tagCount.value = tagsCount;
    recentPhotos.value = photos;
    tags.value = tagList;
  } catch (error) {
    errorMessage.value = "ไม่สามารถโหลดข้อมูลจาก Database ได้";

    console.error(error);
  }
});
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
  grid-template-columns: repeat(3, 1fr);
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
