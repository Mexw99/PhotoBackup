import { ref, onMounted } from "vue";
import { GetTagCloud, SearchPhotoLibrary } from "~~/wailsjs/go/main/App";
import type { PhotoItem, TagCloudItem } from "~/types/photo";

export function usePhotoSearch() {
  const tagCloud = ref<TagCloudItem[]>([]);
  const photos = ref<PhotoItem[]>([]);
  const searchText = ref("");
  const selectedTags = ref<string[]>([]);
  const initialLoading = ref(true);
  const searching = ref(false);
  const error = ref<string | null>(null);
  const selectedPhotoForModal = ref<PhotoItem | null>(null);
  let searchRequest = 0;

  function isTagMatched(tagName: string) {
    return selectedTags.value.some(
      (tag) => tag.toLowerCase() === tagName.toLowerCase(),
    );
  }

  async function loadInitialData() {
    initialLoading.value = true;
    error.value = null;

    try {
      tagCloud.value = (await GetTagCloud()) ?? [];
      await executeSearch();
    } catch (err) {
      error.value = String(err || "ไม่สามารถโหลดข้อมูลจากฐานข้อมูลได้");
    } finally {
      initialLoading.value = false;
    }
  }

  async function executeSearch() {
    const request = ++searchRequest;
    searching.value = true;
    error.value = null;

    try {
      const results = await SearchPhotoLibrary(
        searchText.value.trim(),
        selectedTags.value,
      );
      // หากกดตัวกรองต่อเนื่อง ให้แสดงผลจากคำขอล่าสุดเท่านั้น
      if (request !== searchRequest) return;
      photos.value = results ?? [];

      const openedPhoto = selectedPhotoForModal.value;
      if (openedPhoto) {
        selectedPhotoForModal.value =
          photos.value.find((photo) => photo.id === openedPhoto.id) ?? openedPhoto;
      }
    } catch (err) {
      if (request === searchRequest) {
        error.value = String(err || "เกิดข้อผิดพลาดในการค้นหารูปภาพ");
      }
    } finally {
      if (request === searchRequest) searching.value = false;
    }
  }

  function toggleTag(tagName: string) {
    const name = tagName.trim();
    if (!name) return;
    if (isTagMatched(name)) {
      selectedTags.value = selectedTags.value.filter(
        (tag) => tag.toLowerCase() !== name.toLowerCase(),
      );
    } else {
      selectedTags.value = [...selectedTags.value, name];
    }
    executeSearch();
  }

  function openPhotoDetail(photo: PhotoItem) {
    selectedPhotoForModal.value = photo;
  }

  onMounted(loadInitialData);

  return {
    tagCloud,
    photos,
    searchText,
    selectedTags,
    initialLoading,
    searching,
    error,
    selectedPhotoForModal,
    isTagMatched,
    loadInitialData,
    executeSearch,
    toggleTag,
    openPhotoDetail,
  };
}
