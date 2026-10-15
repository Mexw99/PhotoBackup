<template>
  <div class="search-bar-container">
    <div class="search-input-wrapper">
      <span class="search-icon">🔍</span>
      <input
        ref="inputRef"
        v-model="query"
        type="text"
        class="search-input"
        placeholder="ค้นหาหรือเลือก Tag (เช่น ทะเล, ภูเขา, งานแต่ง)..."
        @keydown.enter.prevent="handleEnter"
        @keydown.down.prevent="highlightNext"
        @keydown.up.prevent="highlightPrev"
        @keydown.esc="closeSuggestions"
        @focus="isOpen = true"
      />
      <button
        v-if="query"
        type="button"
        class="clear-input-btn"
        title="ล้างข้อความ"
        @click="query = ''; isOpen = false"
      >
        ×
      </button>
      <button
        type="button"
        class="add-tag-btn"
        :disabled="!query.trim()"
        @click="commitQueryTag"
      >
        + เพิ่มค้นหา
      </button>
    </div>

    <!-- Dropdown แนะนำ Tag ที่มีอยู่ในระบบจริง -->
    <div
      v-if="isOpen && filteredTags.length > 0"
      class="suggestions-dropdown"
    >
      <div class="dropdown-header">Tag ที่มีในระบบ</div>
      <button
        v-for="(tag, index) in filteredTags"
        :key="tag.id"
        type="button"
        class="suggestion-item"
        :class="{ 'is-highlighted': index === highlightedIndex }"
        @click="selectTag(tag.name)"
        @mouseenter="highlightedIndex = index"
      >
        <span class="tag-title">#{{ tag.name }}</span>
        <span v-if="tag.photoCount !== undefined" class="tag-meta">
          {{ tag.photoCount }} รูป
        </span>
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from "vue";

export interface AvailableTag {
  id: number;
  name: string;
  photoCount?: number;
}

const props = defineProps<{
  availableTags: AvailableTag[];
  selectedTags: string[];
}>();

const emit = defineEmits<{
  (e: "select", tagName: string): void;
}>();

const query = ref("");
const isOpen = ref(false);
const highlightedIndex = ref(-1);
const inputRef = ref<HTMLInputElement | null>(null);

const filteredTags = computed(() => {
  const q = query.value.trim().toLowerCase();
  const selectedSet = new Set(props.selectedTags.map((t) => t.toLowerCase()));

  // แสดงแท็กที่ยังไม่ถูกเลือก และตรงกับคำค้นหา
  return props.availableTags.filter((tag) => {
    if (selectedSet.has(tag.name.toLowerCase())) return false;
    if (!q) return true;
    return tag.name.toLowerCase().includes(q);
  });
});

function selectTag(name: string) {
  emit("select", name);
  query.value = "";
  isOpen.value = false;
  highlightedIndex.value = -1;
  inputRef.value?.focus();
}

function commitQueryTag() {
  const trimmed = query.value.trim();
  if (!trimmed) return;

  // ตรวจสอบว่ามีแท็กที่ตรงกับในระบบหรือไม่
  const matched = props.availableTags.find(
    (t) => t.name.toLowerCase() === trimmed.toLowerCase(),
  );

  if (matched) {
    selectTag(matched.name);
    return;
  }

  const highlighted = highlightedIndex.value >= 0 ? filteredTags.value[highlightedIndex.value] : undefined;
  if (highlighted) {
    selectTag(highlighted.name);
    return;
  }

  const firstTag = filteredTags.value[0];
  if (firstTag) {
    selectTag(firstTag.name);
    return;
  }

  // ถ้าไม่มีแท็กในระบบ ให้ใช้คำที่พิมพ์ค้นหา
  selectTag(trimmed);
}

function handleEnter() {
  const highlighted = highlightedIndex.value >= 0 ? filteredTags.value[highlightedIndex.value] : undefined;
  if (highlighted) {
    selectTag(highlighted.name);
  } else {
    commitQueryTag();
  }
}

function highlightNext() {
  if (!isOpen.value) {
    isOpen.value = true;
    return;
  }
  if (highlightedIndex.value < filteredTags.value.length - 1) {
    highlightedIndex.value++;
  } else {
    highlightedIndex.value = 0;
  }
}

function highlightPrev() {
  if (highlightedIndex.value > 0) {
    highlightedIndex.value--;
  } else {
    highlightedIndex.value = filteredTags.value.length - 1;
  }
}

function closeSuggestions() {
  isOpen.value = false;
  highlightedIndex.value = -1;
}

function handleClickOutside(event: MouseEvent) {
  const target = event.target as HTMLElement | null;
  if (!target?.closest(".search-bar-container")) {
    closeSuggestions();
  }
}

onMounted(() => {
  window.addEventListener("click", handleClickOutside);
});

onUnmounted(() => {
  window.removeEventListener("click", handleClickOutside);
});
</script>

<style scoped>
.search-bar-container {
  position: relative;
  width: 100%;
}

.search-input-wrapper {
  display: flex;
  align-items: center;
  background: #ffffff;
  border: 1.5px solid #cbd5e1;
  border-radius: 12px;
  padding: 4px 6px 4px 14px;
  transition: all 0.2s ease;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.03);
}

.search-input-wrapper:focus-within {
  border-color: #2563eb;
  box-shadow: 0 0 0 3px rgba(37, 99, 235, 0.15);
}

.search-icon {
  font-size: 1.1rem;
  margin-right: 10px;
  user-select: none;
  opacity: 0.7;
}

.search-input {
  flex: 1;
  border: none;
  outline: none;
  font-size: 0.95rem;
  color: #1e293b;
  background: transparent;
  padding: 8px 0;
}

.search-input::placeholder {
  color: #94a3b8;
}

.clear-input-btn {
  background: transparent;
  border: none;
  color: #94a3b8;
  font-size: 1.25rem;
  width: 28px;
  height: 28px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  cursor: pointer;
  margin-right: 6px;
  transition: all 0.15s ease;
}

.clear-input-btn:hover {
  background: #f1f5f9;
  color: #475569;
}

.add-tag-btn {
  background: #2563eb;
  color: #ffffff;
  border: none;
  padding: 8px 16px;
  border-radius: 8px;
  font-size: 0.88rem;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.15s ease;
  white-space: nowrap;
}

.add-tag-btn:hover:not(:disabled) {
  background: #1d4ed8;
}

.add-tag-btn:disabled {
  background: #e2e8f0;
  color: #94a3b8;
  cursor: not-allowed;
}

.suggestions-dropdown {
  position: absolute;
  top: calc(100% + 6px);
  left: 0;
  right: 0;
  z-index: 50;
  background: #ffffff;
  border: 1px solid #e2e8f0;
  border-radius: 12px;
  box-shadow: 0 10px 25px -5px rgba(0, 0, 0, 0.1), 0 8px 10px -6px rgba(0, 0, 0, 0.05);
  max-height: 240px;
  overflow-y: auto;
  padding: 6px;
}

.dropdown-header {
  font-size: 0.75rem;
  font-weight: 700;
  text-transform: uppercase;
  color: #64748b;
  padding: 6px 10px 4px;
  letter-spacing: 0.05em;
}

.suggestion-item {
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 12px;
  background: transparent;
  border: none;
  border-radius: 8px;
  cursor: pointer;
  text-align: left;
  transition: background-color 0.15s ease;
}

.suggestion-item:hover,
.suggestion-item.is-highlighted {
  background: #eff6ff;
}

.tag-title {
  color: #1e40af;
  font-weight: 600;
  font-size: 0.9rem;
}

.tag-meta {
  color: #64748b;
  font-size: 0.8rem;
}
</style>
