<template>
  <div ref="pageRoot" class="tags-page">
    <header>
      <h1>จัดการ Tag</h1>
      <p>AI จะสร้าง Tag และคำอธิบายหลัง Backup เสร็จ และสามารถเพิ่ม Tag เองได้</p>
    </header>

    <section class="panel analysis-summary" aria-label="สรุปการวิเคราะห์รูปภาพ">
      <h2>ความคืบหน้าการวิเคราะห์</h2>
      <p class="progress-text" role="status">
        วิเคราะห์สำเร็จ {{ analysisStatus.completed.toLocaleString() }} / {{ totalPhotos.toLocaleString() }} รูป
        ({{ completedPercent }}%)
      </p>
      <progress :value="analysisStatus.completed" :max="totalPhotos || 1" aria-label="รูปที่วิเคราะห์สำเร็จ" />
      <dl class="analysis-counts">
        <div class="completed"><dt>วิเคราะห์สำเร็จ</dt><dd data-status="completed">{{ analysisStatus.completed.toLocaleString() }} รูป</dd></div>
        <div><dt>กำลังวิเคราะห์</dt><dd data-status="processing">{{ analysisStatus.processing.toLocaleString() }} รูป</dd></div>
        <div class="retrying"><dt>กำลัง Retry / รอลองใหม่</dt><dd data-status="retrying">{{ analysisStatus.retrying.toLocaleString() }} รูป</dd></div>
        <div><dt>รอคิว</dt><dd data-status="pending">{{ analysisStatus.pending.toLocaleString() }} รูป</dd></div>
        <div class="failed"><dt>วิเคราะห์ไม่สำเร็จ</dt><dd data-status="failed">{{ analysisStatus.failed.toLocaleString() }} รูป</dd></div>
      </dl>
      <p class="tag-help">มี Tag แล้ว {{ taggedPhotos.toLocaleString() }} รูป (รวมที่เพิ่มเอง) · Tag ทั้งหมด {{ tagCloud.length.toLocaleString() }} คำ</p>
      <p v-if="!loading && !analysisStatus.enabled" class="error">AI ยังไม่พร้อมใช้งาน กรุณาตรวจสอบการตั้งค่า API</p>
      <p v-if="syncError" class="error" role="alert">{{ syncError }}</p>
    </section>

    <section class="panel">
      <p>เลือกแล้ว {{ selectedIDs.length }} รูป</p>
      <TagEditor :photo-i-ds="selectedIDs" @add="addTag" />
      <p v-if="errorMessage" class="error">{{ errorMessage }}</p>
      <p v-if="successMessage" class="success">{{ successMessage }}</p>
    </section>

    <section class="panel">
      <h2>Tag Cloud</h2>
      <p class="tag-help">
        กด “ลบ” เพื่อลบ Tag จากฐานข้อมูลและทุกรูปที่ใช้ Tag นี้
      </p>
      <div class="tag-cloud-scroll">
        <TagCloud
          :tags="tagCloud"
          :clickable="false"
          deletable
          :disabled="loading || deletingTagID !== null"
          @delete="deleteTag"
        />
      </div>
    </section>

    <section class="panel">
      <h2>รูปภาพที่ Backup แล้ว</h2>
      <p class="tag-help">เรียงรูปที่มี Tag ไว้ด้านบน · อัปเดตอัตโนมัติโดยไม่โหลดรายการใหม่ทั้งหมด</p>
      <p v-if="loading">กำลังโหลด...</p>
      <PhotoGrid
        :photos="photos"
        :selected-i-ds="selectedIDs"
        :empty-text="loading ? 'กำลังโหลดรูปภาพ...' : 'ยังไม่มีรูปภาพที่ Backup แล้ว'"
        @update:selected-i-ds="selectedIDs = $event"
        @remove-tag="removeTag"
      />
    </section>
  </div>
</template>

<script setup lang="ts">
import PhotoGrid from "~/components/gallery/photo-grid.vue";
import TagEditor from "~/components/gallery/tag-editor.vue";
import TagCloud from "~/components/tag-cloud/tag-cloud.vue";
import { usePhotoTags } from "~/composables/usePhotoTags";

const {
  pageRoot,
  analysisStatus,
  totalPhotos,
  taggedPhotos,
  completedPercent,
  syncError,
  photos,
  tagCloud,
  selectedIDs,
  loading,
  errorMessage,
  successMessage,
  deletingTagID,
  addTag,
  removeTag,
  deleteTag,
} = usePhotoTags();
</script>

<style scoped>
.tags-page {
  display: grid;
  gap: 20px;
  padding: 12px 0;
  overflow-anchor: none;
}

.analysis-summary progress { width: 100%; height: 12px; accent-color: #2563eb; }
.progress-text { font-weight: 600; font-variant-numeric: tabular-nums; }
.analysis-counts { display: grid; grid-template-columns: repeat(auto-fit, minmax(150px, 1fr)); gap: 12px; }
.analysis-counts > div { padding: 14px; border-radius: 8px; background: #f1f5f9; }
.analysis-counts dt { font-size: 13px; color: #475569; }
.analysis-counts dd { margin: 8px 0 0; font-size: 22px; font-weight: 700; font-variant-numeric: tabular-nums; }
.analysis-counts .completed { background: #ecfdf5; color: #166534; }
.analysis-counts .retrying { background: #fffbeb; color: #92400e; }
.analysis-counts .failed { background: #fef2f2; color: #b91c1c; }
.tag-cloud-scroll { max-height: 240px; overflow-y: auto; }

header h1,
.panel h2 {
  margin: 0 0 10px;
}

header p,
.tag-help {
  color: #64748b;
}

.panel {
  padding: 20px;
  border: 1px solid #e2e8f0;
  border-radius: 12px;
  background: white;
}

.error {
  color: #b91c1c;
}

.success {
  color: #166534;
}
</style>
