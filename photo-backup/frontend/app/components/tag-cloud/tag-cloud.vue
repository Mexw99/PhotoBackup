<template>
  <div v-if="tags && tags.length > 0" class="tag-cloud">
    <span v-for="tag in tags" :key="tag.id" class="tag-item">
      <button
        type="button"
        class="tag-pill"
        :class="{
          'is-selected': isSelected(tag.name),
          'is-clickable': clickable,
        }"
        :style="tagStyle(tag.photoCount)"
        :title="
          clickable
            ? `คลิกเพื่อ${isSelected(tag.name) ? 'ยกเลิก' : 'ค้นหา'} #${tag.name} (${tag.photoCount} รูป)`
            : `#${tag.name} (${tag.photoCount} รูป)`
        "
        :aria-pressed="isSelected(tag.name)"
        :disabled="disabled"
        @click="handleTagClick(tag.name)"
      >
        <span class="tag-name">#{{ tag.name }}</span>
        <span class="tag-count">{{ tag.photoCount }}</span>
      </button>
      <button
        v-if="deletable"
        type="button"
        class="delete-tag"
        :disabled="disabled"
        :aria-label="`ลบ Tag ${tag.name} จากฐานข้อมูลและทุกรูป`"
        @click="emit('delete', tag)"
      >
        ลบ
      </button>
    </span>
  </div>
  <div v-else class="empty-cloud">
    <span class="empty-icon">🏷️</span>
    <p>ยังไม่มี Tag ในฐานข้อมูล</p>
  </div>
</template>

<script setup lang="ts">
import type { TagCloudItem } from "~/types/photo";
import { computed } from "vue";

const props = withDefaults(
  defineProps<{
    tags: TagCloudItem[];
    selectedTags?: string[];
    clickable?: boolean;
    deletable?: boolean;
    disabled?: boolean;
  }>(),
  {
    selectedTags: () => [],
    clickable: true,
    deletable: false,
    disabled: false,
  },
);

const emit = defineEmits<{
  (e: "select", tagName: string): void;
  (e: "toggle", tagName: string): void;
  (e: "delete", tag: TagCloudItem): void;
}>();

const largestCount = computed(() => {
  if (!props.tags || props.tags.length === 0) return 1;
  const max = Math.max(...props.tags.map((t) => t.photoCount || 0));
  return max > 0 ? max : 1;
});

function isSelected(name: string): boolean {
  if (!props.selectedTags || props.selectedTags.length === 0) return false;
  const target = name.toLowerCase();
  return props.selectedTags.some((t) => t.toLowerCase() === target);
}

function handleTagClick(name: string) {
  if (!props.clickable) return;
  emit("select", name);
  emit("toggle", name);
}

function tagStyle(count: number) {
  const strength = count / largestCount.value;
  // ปรับ scale ตัวอักษรอย่างเหมาะสมระหว่าง 0.85rem ถึง 1.35rem ตามความถี่จริง
  const fontSize = 0.85 + strength * 0.45;
  return {
    fontSize: `${fontSize.toFixed(2)}rem`,
  };
}
</script>

<style scoped>
.tag-cloud {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px;
  padding: 8px 0;
  max-height: 280px;
  overflow-y: auto;
  align-content: flex-start;
}

.tag-item {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

.delete-tag {
  border: 1px solid #fecaca;
  border-radius: 6px;
  padding: 4px 8px;
  background: #fff1f2;
  color: #b91c1c;
  cursor: pointer;
}

.delete-tag:disabled {
  opacity: 0.5;
  cursor: wait;
}

.tag-pill {
  appearance: none;
  border: 1px solid transparent;
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 6px 14px;
  border-radius: 9999px;
  background: #f1f5f9;
  color: #334155;
  font-weight: 600;
  cursor: default;
  transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
  user-select: none;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.04);
}

.tag-pill.is-clickable {
  cursor: pointer;
  background: #eff6ff;
  color: #1e40af;
  border-color: #dbeafe;
}

.tag-pill.is-clickable:hover {
  background: #dbeafe;
  color: #1d4ed8;
  transform: translateY(-1px);
  box-shadow: 0 3px 6px rgba(37, 99, 235, 0.12);
}

.tag-pill.is-clickable:active {
  transform: translateY(0);
}

.tag-pill.is-selected {
  background: #2563eb !important;
  color: #ffffff !important;
  border-color: #1d4ed8 !important;
  box-shadow: 0 4px 10px rgba(37, 99, 235, 0.3) !important;
}

.tag-pill .tag-name {
  line-height: 1;
}

.tag-pill .tag-count {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 20px;
  height: 20px;
  padding: 0 6px;
  border-radius: 9999px;
  background: #ffffff;
  color: #475569;
  font-size: 0.75rem;
  font-weight: 700;
  line-height: 1;
}

.tag-pill.is-selected .tag-count {
  background: rgba(255, 255, 255, 0.25);
  color: #ffffff;
}

.empty-cloud {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 24px;
  color: #94a3b8;
  border: 1px dashed #e2e8f0;
  border-radius: 10px;
  gap: 6px;
}

.empty-cloud .empty-icon {
  font-size: 1.5rem;
}

.empty-cloud p {
  margin: 0;
  font-size: 0.9rem;
}
</style>
