<template>
  <div class="filter-bar">
    <div class="top-row">
      <!-- ตัวเลือก AND / OR Mode -->
      <div class="mode-selector-wrapper">
        <span class="mode-label">โหมดการค้นหา:</span>
        <div class="mode-toggle">
          <button
            type="button"
            class="mode-btn"
            :class="{ active: mode === 'OR' }"
            @click="setMode('OR')"
          >
            <span class="mode-title">OR (อย่างน้อย 1 Tag)</span>
          </button>
          <button
            type="button"
            class="mode-btn"
            :class="{ active: mode === 'AND' }"
            @click="setMode('AND')"
          >
            <span class="mode-title">AND (ครบทุก Tag)</span>
          </button>
        </div>
      </div>

      <!-- ปุ่มล้างตัวกรองทั้งหมด -->
      <button
        v-if="selectedTags.length > 0"
        type="button"
        class="clear-all-btn"
        @click="emit('clear')"
      >
        <span>🗑️</span> ล้างตัวกรอง ({{ selectedTags.length }})
      </button>
    </div>

    <!-- คำอธิบายโหมดปัจจุบัน -->
    <div class="mode-description">
      <span v-if="mode === 'OR'" class="desc-text">
        💡 <strong>โหมด OR:</strong> แสดงรูปภาพที่มี Tag ใด Tag
        หนึ่งในเงื่อนไขการค้นหา
      </span>
      <span v-else class="desc-text">
        🎯 <strong>โหมด AND:</strong> แสดงเฉพาะรูปภาพที่มี Tag
        ครบทุกรายการที่เลือกเท่านั้น
      </span>
    </div>

    <!-- รายการ Selected Tag Chips -->
    <div v-if="selectedTags.length > 0" class="selected-tags-container">
      <span class="filter-intro">Tag ที่เลือกอยู่:</span>
      <div class="chips-list">
        <span v-for="tag in selectedTags" :key="tag" class="selected-chip">
          <span class="chip-name">#{{ tag }}</span>
          <button
            type="button"
            class="remove-chip-btn"
            :title="`ลบ #${tag}`"
            @click="emit('remove', tag)"
          >
            ×
          </button>
        </span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { MatchMode } from "~/types/photo";

const props = defineProps<{
  selectedTags: string[];
  mode: MatchMode;
}>();

const emit = defineEmits<{
  (e: "update:mode", mode: MatchMode): void;
  (e: "remove", tag: string): void;
  (e: "clear"): void;
}>();

function setMode(newMode: MatchMode) {
  if (props.mode !== newMode) {
    emit("update:mode", newMode);
  }
}
</script>

<style scoped>
.filter-bar {
  display: flex;
  flex-direction: column;
  gap: 12px;
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  border-radius: 12px;
  padding: 14px 18px;
}

.top-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 12px;
}

.mode-selector-wrapper {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.mode-label {
  font-size: 0.88rem;
  font-weight: 600;
  color: #475569;
}

.mode-toggle {
  display: inline-flex;
  background: #e2e8f0;
  border-radius: 8px;
  padding: 3px;
  gap: 2px;
}

.mode-btn {
  appearance: none;
  border: none;
  background: transparent;
  padding: 6px 14px;
  border-radius: 6px;
  font-size: 0.85rem;
  font-weight: 600;
  color: #64748b;
  cursor: pointer;
  transition: all 0.2s ease;
}

.mode-btn.active {
  background: #ffffff;
  color: #1d4ed8;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
}

.clear-all-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  background: #fee2e2;
  color: #b91c1c;
  border: 1px solid #fecaca;
  padding: 6px 14px;
  border-radius: 8px;
  font-size: 0.85rem;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.15s ease;
}

.clear-all-btn:hover {
  background: #fecaca;
  color: #991b1b;
}

.mode-description {
  font-size: 0.82rem;
  color: #64748b;
  line-height: 1.4;
}

.desc-text strong {
  color: #334155;
}

.selected-tags-container {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
  padding-top: 10px;
  border-top: 1px dashed #cbd5e1;
}

.filter-intro {
  font-size: 0.85rem;
  font-weight: 600;
  color: #475569;
}

.chips-list {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.selected-chip {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 10px 4px 12px;
  border-radius: 9999px;
  background: #2563eb;
  color: #ffffff;
  font-size: 0.85rem;
  font-weight: 600;
  box-shadow: 0 1px 3px rgba(37, 99, 235, 0.2);
}

.remove-chip-btn {
  appearance: none;
  border: none;
  background: rgba(255, 255, 255, 0.25);
  color: #ffffff;
  width: 18px;
  height: 18px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  font-size: 0.95rem;
  line-height: 1;
  padding: 0;
  transition: all 0.15s ease;
}

.remove-chip-btn:hover {
  background: rgba(255, 255, 255, 0.4);
}
</style>
