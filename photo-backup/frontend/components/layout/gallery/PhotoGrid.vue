<template>
  <div class="photo-grid">
    <div v-if="photos.length === 0" class="empty-state">
      {{ emptyText }}
    </div>
    <div v-else class="grid">
      <div v-for="photo in photos" :key="photo.id" class="photo-card">
        <div class="photo-icon">📷</div>
        <div class="photo-info">
          <strong>{{ photo.originalName }}</strong>
          <small>{{ photo.backedUpAt }}</small>
          <small class="path">{{ photo.backupPath }}</small>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
interface PhotoItem {
  id: number;
  originalName: string;
  backupPath: string;
  backedUpAt: string;
}

defineProps<{
  photos: PhotoItem[];
  emptyText?: string;
}>();
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
}

.path {
  overflow-wrap: anywhere;
}
</style>