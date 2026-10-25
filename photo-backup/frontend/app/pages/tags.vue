<template>
  <div class="tags-page">
    <header>
      <h1>จัดการ Tag</h1>
      <p>AI จะสร้าง Tag และคำอธิบายหลัง Backup เสร็จ และสามารถเพิ่ม Tag เองได้</p>
    </header>

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
      <TagCloud
        :tags="tagCloud"
        :clickable="false"
        deletable
        :disabled="loading || deletingTagID !== null"
        @delete="deleteTag"
      />
    </section>

    <section class="panel">
      <h2>รูปภาพที่ Backup แล้ว</h2>
      <p v-if="loading">กำลังโหลด...</p>
      <PhotoGrid
        v-else
        :photos="photos"
        :selected-i-ds="selectedIDs"
        empty-text="ยังไม่มีรูปภาพที่ Backup แล้ว"
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
}

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
