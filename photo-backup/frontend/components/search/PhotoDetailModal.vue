<template>
  <Teleport to="body">
    <div
      v-if="photo"
      class="modal-backdrop"
      @click.self="emit('close')"
      @keydown.esc="emit('close')"
    >
      <div class="modal-card" role="dialog" aria-modal="true" :aria-label="photo.originalName">
        <header class="modal-header">
          <img
            v-if="photo.thumbnail"
            :src="photo.thumbnail"
            class="header-thumb"
            :alt="photo.originalName"
          />
          <div v-else class="header-icon">📷</div>
          <div class="header-info">
            <h3 class="modal-title">{{ photo.originalName }}</h3>
            <span class="modal-subtitle">ID รูปภาพ: #{{ photo.id }}</span>
          </div>
          <button
            type="button"
            class="close-modal-btn"
            title="ปิดหน้าต่าง (Esc)"
            @click="emit('close')"
          >
            ×
          </button>
        </header>

        <div class="modal-body">
          <div v-if="photo.thumbnail" class="modal-image-preview">
            <img :src="photo.thumbnail" :alt="photo.originalName" />
          </div>

          <div class="detail-row">
            <span class="detail-label">ชื่อไฟล์ต้นฉบับ:</span>
            <span class="detail-value filename">{{ photo.originalName }}</span>
          </div>

          <div class="detail-row">
            <span class="detail-label">วันที่และเวลา Backup:</span>
            <span class="detail-value">{{ photo.backedUpAt || "-" }}</span>
          </div>

          <div class="detail-row">
            <span class="detail-label">ตำแหน่งไฟล์ในระบบ:</span>
            <span class="detail-value path-value">
              <code>{{ photo.backupPath }}</code>
            </span>
          </div>

          <div class="detail-section">
            <span class="detail-label">Tag ที่ผูกกับรูปนี้ ({{ photo.tags?.length || 0 }}):</span>
            <div v-if="photo.tags && photo.tags.length > 0" class="tags-badge-list">
              <span
                v-for="tag in photo.tags"
                :key="tag.id"
                class="tag-badge"
              >
                #{{ tag.name }}
              </span>
            </div>
            <p v-else class="no-tags-text">ยังไม่มี Tag สำหรับรูปภาพนี้</p>
          </div>
        </div>

        <footer class="modal-footer">
          <button
            type="button"
            class="modal-close-action-btn"
            @click="emit('close')"
          >
            ปิด
          </button>
        </footer>
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { onMounted, onUnmounted } from "vue";

export interface PhotoDetailItem {
  id: number;
  originalName: string;
  backupPath: string;
  backedUpAt: string;
  thumbnail?: string;
  tags: Array<{
    id: number;
    name: string;
  }>;
}

const props = defineProps<{
  photo: PhotoDetailItem | null;
}>();

const emit = defineEmits<{
  (e: "close"): void;
}>();

function handleKeyDown(e: KeyboardEvent) {
  if (e.key === "Escape" && props.photo) {
    emit("close");
  }
}

onMounted(() => {
  window.addEventListener("keydown", handleKeyDown);
});

onUnmounted(() => {
  window.removeEventListener("keydown", handleKeyDown);
});
</script>

<style scoped>
.modal-backdrop {
  position: fixed;
  inset: 0;
  z-index: 1000;
  background: rgba(15, 23, 42, 0.6);
  backdrop-filter: blur(4px);
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 16px;
  animation: fadeIn 0.15s ease-out;
}

.modal-card {
  background: #ffffff;
  border-radius: 16px;
  box-shadow: 0 20px 25px -5px rgba(0, 0, 0, 0.2), 0 10px 10px -5px rgba(0, 0, 0, 0.08);
  max-width: 580px;
  width: 100%;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  animation: scaleUp 0.2s cubic-bezier(0.16, 1, 0.3, 1);
}

.modal-header {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 20px 24px;
  border-bottom: 1px solid #f1f5f9;
}

.header-icon {
  width: 44px;
  height: 44px;
  border-radius: 10px;
  background: #eff6ff;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 22px;
  flex-shrink: 0;
}

.header-thumb {
  width: 44px;
  height: 44px;
  border-radius: 10px;
  object-fit: cover;
  flex-shrink: 0;
}

.header-info {
  flex: 1;
  min-width: 0;
}

.modal-title {
  margin: 0;
  font-size: 1.15rem;
  color: #1e293b;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.modal-subtitle {
  font-size: 0.8rem;
  color: #64748b;
}

.close-modal-btn {
  background: transparent;
  border: none;
  font-size: 1.5rem;
  color: #94a3b8;
  width: 32px;
  height: 32px;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  transition: all 0.15s ease;
}

.close-modal-btn:hover {
  background: #f1f5f9;
  color: #334155;
}

.modal-body {
  padding: 24px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.modal-image-preview {
  width: 100%;
  max-height: 260px;
  border-radius: 10px;
  overflow: hidden;
  background: #0f172a;
  display: flex;
  align-items: center;
  justify-content: center;
  border: 1px solid #e2e8f0;
}

.modal-image-preview img {
  max-width: 100%;
  max-height: 260px;
  object-fit: contain;
}

.detail-row {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.detail-label {
  font-size: 0.82rem;
  font-weight: 600;
  color: #64748b;
  text-transform: uppercase;
  letter-spacing: 0.03em;
}

.detail-value {
  font-size: 0.95rem;
  color: #1e293b;
  word-break: break-all;
}

.detail-value.filename {
  font-weight: 600;
  color: #0f172a;
}

.path-value code {
  display: block;
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  padding: 8px 12px;
  border-radius: 8px;
  font-family: Consolas, monospace;
  font-size: 0.82rem;
  color: #334155;
  overflow-x: auto;
  white-space: pre-wrap;
  word-break: break-all;
}

.detail-section {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-top: 4px;
}

.tags-badge-list {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.tag-badge {
  display: inline-flex;
  align-items: center;
  padding: 4px 10px;
  border-radius: 9999px;
  background: #eff6ff;
  color: #1d4ed8;
  border: 1px solid #dbeafe;
  font-size: 0.82rem;
  font-weight: 600;
}

.no-tags-text {
  color: #94a3b8;
  font-size: 0.88rem;
  margin: 0;
  font-style: italic;
}

.modal-footer {
  padding: 16px 24px;
  border-top: 1px solid #f1f5f9;
  display: flex;
  justify-content: flex-end;
  background: #f8fafc;
}

.modal-close-action-btn {
  background: #2563eb;
  color: #ffffff;
  border: none;
  padding: 8px 20px;
  border-radius: 8px;
  font-size: 0.88rem;
  font-weight: 600;
  cursor: pointer;
  transition: background-color 0.15s ease;
}

.modal-close-action-btn:hover {
  background: #1d4ed8;
}

@keyframes fadeIn {
  from { opacity: 0; }
  to { opacity: 1; }
}

@keyframes scaleUp {
  from {
    opacity: 0;
    transform: scale(0.96);
  }
  to {
    opacity: 1;
    transform: scale(1);
  }
}
</style>
