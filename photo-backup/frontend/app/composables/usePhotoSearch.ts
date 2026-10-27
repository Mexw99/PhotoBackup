import { computed, nextTick, ref, onMounted, onUnmounted } from "vue";
import { GetTagCloud, SearchPhotoPage } from "~~/wailsjs/go/main/App";
import type { PhotoItem, TagCloudItem } from "~/types/photo";
import { EventsOn } from "~~/wailsjs/runtime/runtime";
import { capturePhotoViewport } from "~/utils/photoViewport";

export function usePhotoSearch() {
  const pageRoot = ref<HTMLElement | null>(null);
  const tagCloud = ref<TagCloudItem[]>([]);
  const photos = ref<PhotoItem[]>([]);
  const searchText = ref("");
  const selectedTags = ref<string[]>([]);
  const initialLoading = ref(true);
  const searching = ref(false);
  const error = ref<string | null>(null);
  const selectedPhotoForModal = ref<PhotoItem | null>(null);
  const page = ref(1);
  const pageSize = 60;
  const total = ref(0);
  const pageCount = computed(() => Math.max(1, Math.ceil(total.value / pageSize)));
  let intent = 0;
  let appliedQuery = "";
  let mounted = false;
  let inFlight = false;
  let queued = false;
  let cloudRequested = true;
  let refreshTimer: ReturnType<typeof setTimeout> | undefined;
  let stopAnalysis: (() => void) | undefined;

  function isTagMatched(tagName: string) {
    return selectedTags.value.some((tag) => tag.toLowerCase() === tagName.toLowerCase());
  }

  async function runSearch() {
    if (inFlight || !mounted) return;
    inFlight = true;
    searching.value = true;
    try {
      while (queued && mounted) {
        queued = false;
        const version = intent;
        const loadCloud = cloudRequested;
        cloudRequested = false;
        try {
          const [result, cloud] = await Promise.all([
            SearchPhotoPage(appliedQuery, [...selectedTags.value], page.value, pageSize),
            loadCloud ? GetTagCloud() : Promise.resolve(null),
          ]);
          if (!mounted) return;
          if (version !== intent) {
            if (loadCloud) cloudRequested = true;
            continue;
          }
          const restore = capturePhotoViewport(pageRoot.value, new Set());
          photos.value = result.photos ?? [];
          total.value = result.total;
          if (cloud !== null) tagCloud.value = cloud ?? [];
          error.value = null;
          initialLoading.value = false;
          if (page.value > pageCount.value) {
            page.value = pageCount.value;
            queued = true;
          }
          const opened = selectedPhotoForModal.value;
          if (opened) selectedPhotoForModal.value = photos.value.find((photo) => photo.id === opened.id) ?? opened;
          await nextTick();
          if (mounted) restore();
        } catch (err) {
          if (mounted && version === intent) {
            error.value = String(err || "ไม่สามารถค้นหารูปภาพได้");
            initialLoading.value = false;
          }
          if (loadCloud) cloudRequested = true;
        }
      }
    } finally {
      inFlight = false;
      if (mounted) searching.value = false;
    }
  }

  function requestSearch(cloud = false) {
    queued = true;
    cloudRequested ||= cloud;
    void runSearch();
  }

  function loadInitialData() { requestSearch(true); }

  function executeSearch() {
    intent++;
    appliedQuery = searchText.value.trim();
    page.value = 1;
    requestSearch();
  }

  function changePage(next: number) {
    if (next < 1 || next > pageCount.value || next === page.value) return;
    intent++;
    page.value = next;
    requestSearch();
  }

  function toggleTag(tagName: string) {
    const name = tagName.trim();
    if (!name) return;
    if (isTagMatched(name)) selectedTags.value = selectedTags.value.filter((tag) => tag.toLowerCase() !== name.toLowerCase());
    else selectedTags.value = [...selectedTags.value, name];
    executeSearch();
  }

  function openPhotoDetail(photo: PhotoItem) { selectedPhotoForModal.value = photo; }

  onMounted(() => {
    mounted = true;
    stopAnalysis = EventsOn("photo:analysis", (progress: { status?: string; photoId?: number }) => {
      if (progress.status !== "completed" && !photos.value.some((photo) => photo.id === progress.photoId)) return;
      if (refreshTimer) return;
      refreshTimer = setTimeout(() => {
        refreshTimer = undefined;
        requestSearch(true);
      }, 1000);
    });
    loadInitialData();
  });
  onUnmounted(() => {
    mounted = false;
    intent++;
    stopAnalysis?.();
    if (refreshTimer) clearTimeout(refreshTimer);
  });

  return {
    pageRoot, tagCloud, photos, searchText, selectedTags, initialLoading, searching,
    error, selectedPhotoForModal, page, pageCount, total, pageSize, changePage,
    isTagMatched, loadInitialData, executeSearch, toggleTag, openPhotoDetail,
  };
}
