import { ref, onMounted } from "vue";
import {
  GetTagCloud,
  GetTags,
  SearchPhotos,
  GetTaggedPhotos,
} from "~~/wailsjs/go/main/App";
import type {
  PhotoItem,
  MatchMode,
  TagCloudItem,
  AvailableTag,
} from "~/types/photo";

export function usePhotoSearch() {
  // ข้อมูลสถานะ
  const availableTags = ref<AvailableTag[]>([]);
  const tagCloud = ref<TagCloudItem[]>([]);
  const photos = ref<PhotoItem[]>([]);
  const selectedTags = ref<string[]>([]);
  const matchMode = ref<MatchMode>("OR");

  const initialLoading = ref(true);
  const searching = ref(false);
  const error = ref<string | null>(null);

  // Modal state
  const selectedPhotoForModal = ref<PhotoItem | null>(null);

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
        err instanceof Error
          ? err.message
          : "ไม่สามารถโหลดข้อมูลจากฐานข้อมูลได้";
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
  function openPhotoDetail(photo: PhotoItem) {
    selectedPhotoForModal.value = photo;
  }

  onMounted(() => {
    loadInitialData();
  });

  return {
    availableTags,
    tagCloud,
    photos,
    selectedTags,
    matchMode,
    initialLoading,
    searching,
    error,
    selectedPhotoForModal,
    isTagMatched,
    loadInitialData,
    executeSearch,
    addTag,
    removeTag,
    toggleTag,
    setMatchMode,
    clearFilters,
    openPhotoDetail,
  };
}
