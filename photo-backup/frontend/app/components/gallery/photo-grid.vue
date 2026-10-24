<template>
  <div class="photo-grid">
    <div v-if="photos.length === 0" class="empty-state">
      {{ emptyText }}
    </div>
    <div v-else class="grid">
      <div v-for="photo in photos" :key="photo.id" class="photo-card">
        <input
          type="checkbox"
          :checked="selectedIDs.includes(photo.id)"
          @change="
            togglePhoto(photo.id, ($event.target as HTMLInputElement).checked)
          "
        />
        <img
          v-if="photo.thumbnail"
          :src="photo.thumbnail"
          class="photo-thumb"
          :alt="photo.originalName"
        />
        <div v-else class="photo-icon">📷</div>
        <div class="photo-info">
          <strong>{{ photo.originalName }}</strong>
          <small>{{ photo.backedUpAt }}</small>
          <small class="path">{{ photo.backupPath }}</small>
        </div>
        <div class="photo-tags">
          <span v-for="tag in photo.tags" :key="tag.id" class="tag-chip">
            #{{ tag.name }}
            <button type="button" @click="emit('remove-tag', photo.id, tag.id)">
              ×
            </button>
          </span>
          <span v-if="!photo.tags.length">ยังไม่มี Tag</span>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { PhotoItem } from "~/types/photo";

const props = defineProps<{
  photos: PhotoItem[];
  emptyText?: string;
  selectedIDs: number[];
}>();

const emit = defineEmits<{
  "update:selectedIDs": [ids: number[]];
  "remove-tag": [photoID: number, tagID: number];
}>();

function togglePhoto(photoID: number, checked: boolean) {
  const next = new Set(props.selectedIDs);

  if (checked) next.add(photoID);
  else next.delete(photoID);

  emit("update:selectedIDs", [...next]);
}
</script>

<style scoped>
.empty-state {
  border: 1px dashed #cbd5e1;
  border-radius: 8px;
  color: #94a3b8;
  padding: 24px;
  text-align: center;
}

.grid {
  display: grid;
  gap: 12px;
}

.photo-card {
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
  flex-shrink: 0;
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

.photo-info {
  min-width: 0;
}

.photo-info strong,
.photo-info small {
  display: block;
}

.photo-info strong {
  margin-bottom: 4px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.photo-info small {
  color: #64748b;
  font-size: 12px;
  overflow-wrap: anywhere;
}
.photo-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-top: 8px;
}

.tag-chip {
  padding: 4px 8px;
  border-radius: 999px;
  background: #dbeafe;
  color: #1d4ed8;
}

.tag-chip button {
  border: 0;
  background: transparent;
  color: inherit;
  cursor: pointer;
}
.path {
  overflow-wrap: anywhere;
}
</style>
