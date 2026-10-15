<template>
  <main class="search-page">
    <!-- Header -->
    <header class="page-header">
      <div class="header-titles">
        <h1 class="page-title">ค้นหารูปภาพด้วย Tag</h1>
        <p class="page-description">
          ค้นหาและกรองรูปภาพที่สำรองไว้ด้วย Tag โดยเลือกจาก Tag Cloud หรือพิมพ์ค้นหา
        </p>
      </div>
      <div v-if="!initialLoading && !error" class="header-stats">
        <span class="stat-badge">
          🏷️ <strong>{{ availableTags.length }}</strong> Tags ทั้งหมด
        </span>
      </div>
    </header>

    <!-- Error State -->
    <div v-if="error" class="state-banner error-banner" role="alert">
      <span class="banner-icon">⚠️</span>
      <div class="banner-content">
        <strong>เกิดข้อผิดพลาด:</strong>
        <p>{{ error }}</p>
      </div>
      <button type="button" class="retry-btn" @click="loadInitialData">
        ลองใหม่
      </button>
    </div>

    <!-- แผงค้นหาและตัวกรอง -->
    <section class="panel search-controls-panel">
      <!-- Search Bar พร้อม Auto-suggest -->
      <div class="control-row">
        <SearchBar
          :available-tags="availableTags"
          :selected-tags="selectedTags"
          @select="addTag"
        />
      </div>

      <!-- Tag Cloud -->
      <div class="cloud-section">
        <div class="section-title-row">
          <h2 class="section-title">Tag Cloud ในระบบ</h2>
          <span class="section-hint">คลิก Tag เพื่อเพิ่มหรือลบออกจากเงื่อนไขค้นหา</span>
        </div>
        <TagCloud
          :tags="tagCloud"
          :selected-tags="selectedTags"
          :clickable="true"
          @toggle="toggleTag"
        />
      </div>

      <!-- แถบจัดการ Filter และตัวเลือก AND / OR -->
      <SearchFilterBar
        v-if="selectedTags.length > 0"
        :selected-tags="selectedTags"
        :mode="matchMode"
        @update:mode="setMatchMode"
        @remove="removeTag"
        @clear="clearFilters"
      />
    </section>

    <!-- ส่วนแสดงผลลัพธ์รูปภาพ -->
    <section class="panel results-panel">
      <div class="results-header">
        <div class="results-title-group">
          <h2>ผลการค้นหา</h2>
          <span v-if="!searching && !initialLoading" class="result-count-badge">
            {{ photos.length }} รูปภาพ
          </span>
        </div>

        <div v-if="selectedTags.length > 0" class="active-filter-summary">
          กรองด้วย:
          <strong>{{ selectedTags.length }} Tag</strong> (โหมด {{ matchMode }})
        </div>
      </div>

      <!-- Loading State -->
      <div v-if="searching || initialLoading" class="state-view loading-view">
        <div class="spinner"></div>
        <p>กำลังค้นหาข้อมูลรูปภาพจากฐานข้อมูล...</p>
      </div>

      <!-- Empty State: กรณีไม่พบรูปภาพ -->
      <div
        v-else-if="photos.length === 0"
        class="state-view empty-view"
      >
        <div class="empty-icon">🔍</div>
        <h3 v-if="selectedTags.length > 0">ไม่พบรูปภาพที่ตรงกับเงื่อนไข</h3>
        <h3 v-else>ยังไม่มีรูปภาพในระบบ หรือไม่มีรูปภาพที่ผูก Tag</h3>
        <p v-if="selectedTags.length > 0">
          ลองสลับเป็นโหมด <strong>OR</strong> หรือคลิกล้างตัวกรองเพื่อดูรูปภาพทั้งหมด
        </p>
        <p v-else>
          คุณสามารถสำรองรูปภาพในหน้า Backup และจัดกลุ่มรูปภาพในหน้าจัดการ Tag
        </p>
        <button
          v-if="selectedTags.length > 0"
          type="button"
          class="reset-filters-btn"
          @click="clearFilters"
        >
          ล้างตัวกรองทั้งหมด
        </button>
      </div>

      <!-- Grid แสดงรูปภาพผลลัพธ์จริง -->
      <div v-else class="photos-grid">
        <article
          v-for="photo in photos"
          :key="photo.id"
          class="photo-card"
          tabindex="0"
          role="button"
          :aria-label="`ดูรายละเอียด ${photo.originalName}`"
          @click="openPhotoDetail(photo)"
          @keydown.enter="openPhotoDetail(photo)"
          @keydown.space.prevent="openPhotoDetail(photo)"
        >
          <div class="photo-icon-box">
            <span class="camera-icon">📷</span>
          </div>

          <div class="photo-card-info">
            <h3 class="photo-card-name" :title="photo.originalName">
              {{ photo.originalName }}
            </h3>
            <span class="photo-card-time">{{ photo.backedUpAt }}</span>
            <span class="photo-card-path" :title="photo.backupPath">
              {{ photo.backupPath }}
            </span>

            <div class="photo-card-tags">
              <span
                v-for="tag in photo.tags"
                :key="tag.id"
                class="tag-chip"
                :class="{ 'is-matching': isTagMatched(tag.name) }"
              >
                #{{ tag.name }}
              </span>
              <span v-if="!photo.tags || photo.tags.length === 0" class="no-tag-hint">
                ไม่มี Tag
              </span>
            </div>
          </div>

          <div class="photo-card-action">
            <span class="view-detail-hint">ดูรายละเอียด ➔</span>
          </div>
        </article>
      </div>
    </section>

    <!-- Modal รายละเอียดรูปภาพ -->
    <PhotoDetailModal
      :photo="selectedPhotoForModal"
      @close="selectedPhotoForModal = null"
    />
  </main>
</template>

<script setup lang="ts">
import { ref, onMounted } from "vue";
import {
  GetTagCloud,
  GetTags,
  SearchPhotos,
  GetTaggedPhotos,
} from "../../wailsjs/go/main/App";
import type { main } from "../../wailsjs/go/models";

import SearchBar from "../../components/search/SearchBar.vue";
import SearchFilterBar, { type MatchMode } from "../../components/search/SearchFilterBar.vue";
import PhotoDetailModal from "../../components/search/PhotoDetailModal.vue";
import TagCloud, { type TagCloudItem } from "../../components/tag-cloud/TagCloud.vue";

// ข้อมูลสถานะ
const availableTags = ref<Array<{ id: number; name: string; photoCount?: number }>>([]);
const tagCloud = ref<TagCloudItem[]>([]);
const photos = ref<main.TaggedPhotoSummary[]>([]);
const selectedTags = ref<string[]>([]);
const matchMode = ref<MatchMode>("OR");

const initialLoading = ref(true);
const searching = ref(false);
const error = ref<string | null>(null);

// Modal state
const selectedPhotoForModal = ref<main.TaggedPhotoSummary | null>(null);

// ตรวจสอบว่า Tag นั้นตรงกับเงื่อนไขค้นหาที่เลือกไว้หรือไม่
function isTagMatched(tagName: string): boolean {
  if (selectedTags.value.length === 0) return false;
  const target = tagName.toLowerCase();
  return selectedTags.value.some((t) => t.toLowerCase() === target);
}

// โหลดข้อมูลเริ่มต้น (Tag Cloud, รายการ Tags, รูปภาพ)
async function loadInitialData() {
  initialLoading.value = true;
  error.value = null;

  try {
    const [cloudData, allTags, initialPhotos] = await Promise.all([
      GetTagCloud(),
      GetTags(),
      selectedTags.value.length > 0
        ? SearchPhotos(selectedTags.value, matchMode.value)
        : GetTaggedPhotos(),
    ]);

    tagCloud.value = cloudData ?? [];
    availableTags.value = (allTags ?? []).map((t) => {
      const foundInCloud = (cloudData ?? []).find((c) => c.id === t.id);
      return {
        id: t.id,
        name: t.name,
        photoCount: foundInCloud?.photoCount ?? 0,
      };
    });
    photos.value = initialPhotos ?? [];
  } catch (err) {
    console.error("Load initial search data error:", err);
    error.value =
      err instanceof Error ? err.message : "ไม่สามารถโหลดข้อมูลจากฐานข้อมูลได้";
  } finally {
    initialLoading.value = false;
  }
}

// ทำการค้นหารูปภาพจากฐานข้อมูลผ่าน Wails API
async function executeSearch() {
  searching.value = true;
  error.value = null;

  try {
    if (selectedTags.value.length === 0) {
      // หากไม่มี Tag ที่เลือก ให้แสดงรูปภาพที่สำรองไว้ทั้งหมด
      const allPhotos = await GetTaggedPhotos();
      photos.value = allPhotos ?? [];
    } else {
      const results = await SearchPhotos(selectedTags.value, matchMode.value);
      photos.value = results ?? [];
    }
  } catch (err) {
    console.error("Execute search error:", err);
    error.value =
      err instanceof Error ? err.message : "เกิดข้อผิดพลาดในการค้นหารูปภาพ";
  } finally {
    searching.value = false;
  }
}

// เพิ่ม Tag เข้าสู่เงื่อนไขการค้นหา
function addTag(tagName: string) {
  const trimmed = tagName.trim();
  if (!trimmed) return;

  const exists = selectedTags.value.some(
    (t) => t.toLowerCase() === trimmed.toLowerCase(),
  );

  if (!exists) {
    selectedTags.value = [...selectedTags.value, trimmed];
    executeSearch();
  }
}

// ลบ Tag ออกจากเงื่อนไขการค้นหา
function removeTag(tagName: string) {
  selectedTags.value = selectedTags.value.filter(
    (t) => t.toLowerCase() !== tagName.toLowerCase(),
  );
  executeSearch();
}

// สลับสถานะ Tag (เพิ่ม/ลบ) เมื่อคลิกที่ Tag Cloud
function toggleTag(tagName: string) {
  const exists = selectedTags.value.some(
    (t) => t.toLowerCase() === tagName.toLowerCase(),
  );

  if (exists) {
    removeTag(tagName);
  } else {
    addTag(tagName);
  }
}

// เปลี่ยนโหมด AND / OR
function setMatchMode(newMode: MatchMode) {
  if (matchMode.value !== newMode) {
    matchMode.value = newMode;
    if (selectedTags.value.length > 0) {
      executeSearch();
    }
  }
}

// ล้างตัวกรองทั้งหมด
function clearFilters() {
  selectedTags.value = [];
  executeSearch();
}

// เปิด Modal ดูรายละเอียดรูปภาพ
function openPhotoDetail(photo: main.TaggedPhotoSummary) {
  selectedPhotoForModal.value = photo;
}

onMounted(() => {
  loadInitialData();
});
</script>

<style scoped>
.search-page {
  display: flex;
  flex-direction: column;
  gap: 20px;
  color: #1e293b;
}

/* Header */
.page-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 16px;
  padding: 8px 0;
}

.header-titles {
  max-width: 650px;
}

.page-title {
  margin: 0 0 6px;
  font-size: 1.75rem;
  font-weight: 800;
  color: #0f172a;
}

.page-description {
  margin: 0;
  color: #64748b;
  font-size: 0.95rem;
  line-height: 1.5;
}

.stat-badge {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  background: #ffffff;
  border: 1px solid #e2e8f0;
  padding: 8px 14px;
  border-radius: 9999px;
  font-size: 0.85rem;
  color: #475569;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.04);
}

.stat-badge strong {
  color: #2563eb;
}

/* Panel */
.panel {
  background: #ffffff;
  border: 1px solid #e2e8f0;
  border-radius: 14px;
  padding: 24px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.02);
}

.search-controls-panel {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.control-row {
  width: 100%;
}

.cloud-section {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.section-title-row {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 8px;
}

.section-title {
  margin: 0;
  font-size: 1.05rem;
  font-weight: 700;
  color: #334155;
}

.section-hint {
  font-size: 0.82rem;
  color: #64748b;
}

/* Results Section */
.results-panel {
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.results-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 12px;
  padding-bottom: 14px;
  border-bottom: 1px solid #f1f5f9;
}

.results-title-group {
  display: flex;
  align-items: center;
  gap: 12px;
}

.results-title-group h2 {
  margin: 0;
  font-size: 1.2rem;
  font-weight: 700;
  color: #0f172a;
}

.result-count-badge {
  background: #eff6ff;
  color: #1d4ed8;
  font-size: 0.82rem;
  font-weight: 700;
  padding: 3px 10px;
  border-radius: 9999px;
  border: 1px solid #dbeafe;
}

.active-filter-summary {
  font-size: 0.85rem;
  color: #64748b;
}

.active-filter-summary strong {
  color: #1e293b;
}

/* States */
.state-banner {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 14px 18px;
  border-radius: 10px;
}

.error-banner {
  background: #fef2f2;
  border: 1px solid #fecaca;
  color: #991b1b;
}

.banner-content {
  flex: 1;
}

.banner-content p {
  margin: 2px 0 0;
  font-size: 0.88rem;
}

.retry-btn {
  background: #b91c1c;
  color: white;
  border: none;
  padding: 6px 14px;
  border-radius: 6px;
  font-size: 0.82rem;
  font-weight: 600;
  cursor: pointer;
}

.state-view {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 48px 24px;
  text-align: center;
  color: #64748b;
  border: 1.5px dashed #cbd5e1;
  border-radius: 12px;
  background: #f8fafc;
}

.loading-view {
  gap: 16px;
}

.spinner {
  width: 36px;
  height: 36px;
  border: 3.5px solid #e2e8f0;
  border-top-color: #2563eb;
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}

.empty-view {
  gap: 10px;
}

.empty-icon {
  font-size: 2.5rem;
  opacity: 0.8;
  margin-bottom: 4px;
}

.empty-view h3 {
  margin: 0;
  font-size: 1.15rem;
  color: #334155;
}

.empty-view p {
  margin: 0;
  font-size: 0.9rem;
  max-width: 420px;
  line-height: 1.5;
}

.reset-filters-btn {
  margin-top: 10px;
  background: #2563eb;
  color: white;
  border: none;
  padding: 8px 18px;
  border-radius: 8px;
  font-size: 0.88rem;
  font-weight: 600;
  cursor: pointer;
  transition: background-color 0.15s ease;
}

.reset-filters-btn:hover {
  background: #1d4ed8;
}

/* Photos Grid */
.photos-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  gap: 16px;
}

.photo-card {
  display: flex;
  align-items: flex-start;
  gap: 14px;
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  border-radius: 12px;
  padding: 16px;
  cursor: pointer;
  position: relative;
  transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
  outline: none;
}

.photo-card:hover {
  background: #ffffff;
  border-color: #93c5fd;
  transform: translateY(-2px);
  box-shadow: 0 8px 16px -4px rgba(37, 99, 235, 0.1), 0 4px 6px -2px rgba(0, 0, 0, 0.04);
}

.photo-card:focus-visible {
  border-color: #2563eb;
  box-shadow: 0 0 0 3px rgba(37, 99, 235, 0.2);
}

.photo-icon-box {
  width: 48px;
  height: 48px;
  background: #eff6ff;
  border-radius: 10px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 24px;
  flex-shrink: 0;
  border: 1px solid #dbeafe;
}

.photo-card-info {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.photo-card-name {
  margin: 0;
  font-size: 0.95rem;
  font-weight: 700;
  color: #0f172a;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.photo-card-time {
  font-size: 0.78rem;
  color: #64748b;
}

.photo-card-path {
  font-size: 0.75rem;
  color: #94a3b8;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.photo-card-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-top: 8px;
}

.tag-chip {
  display: inline-flex;
  align-items: center;
  padding: 3px 8px;
  border-radius: 9999px;
  background: #e2e8f0;
  color: #334155;
  font-size: 0.75rem;
  font-weight: 600;
  transition: all 0.15s ease;
}

.tag-chip.is-matching {
  background: #2563eb;
  color: #ffffff;
}

.no-tag-hint {
  font-size: 0.75rem;
  color: #94a3b8;
  font-style: italic;
}

.photo-card-action {
  align-self: flex-end;
  margin-top: 8px;
  opacity: 0;
  transition: opacity 0.15s ease;
}

.photo-card:hover .photo-card-action {
  opacity: 1;
}

.view-detail-hint {
  font-size: 0.75rem;
  font-weight: 700;
  color: #2563eb;
  white-space: nowrap;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

@media (max-width: 768px) {
  .photos-grid {
    grid-template-columns: 1fr;
  }
}
</style>
