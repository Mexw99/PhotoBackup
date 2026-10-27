import { computed, nextTick, ref, onMounted, onUnmounted } from "vue";
import {
  AddTagToPhotos,
  DeleteTag,
  GetAnalysisStatus,
  GetTagCloud,
  GetTaggedPhotos,
  GetTaggedPhotosByIDs,
  RemoveTagFromPhoto,
} from "~~/wailsjs/go/main/App";
import type { service } from "~~/wailsjs/go/models";
import type { PhotoItem, TagCloudItem } from "~/types/photo";
import { EventsOn } from "~~/wailsjs/runtime/runtime";
import { capturePhotoViewport } from "~/utils/photoViewport";

export function usePhotoTags() {
  const pageRoot = ref<HTMLElement | null>(null);
  const photos = ref<PhotoItem[]>([]);
  const tagCloud = ref<TagCloudItem[]>([]);
  const selectedIDs = ref<number[]>([]);
  const loading = ref(true);
  const errorMessage = ref("");
  const syncError = ref("");
  const successMessage = ref("");
  const deletingTagID = ref<number | null>(null);
  const analysisStatus = ref<service.AnalysisSummary>({
    enabled: false, pending: 0, processing: 0, retrying: 0, completed: 0, failed: 0,
  });
  const totalPhotos = computed(() => {
    const s = analysisStatus.value;
    return s.pending + s.processing + s.retrying + s.completed + s.failed;
  });
  const taggedPhotos = computed(() => photos.value.filter((photo) => photo.tags.length > 0).length);
  const completedPercent = computed(() => totalPhotos.value
    ? Math.floor(analysisStatus.value.completed / totalPhotos.value * 1000) / 10 : 0);

  const dirtyIDs = new Set<number>();
  let fullRefresh = true;
  let cloudRefresh = true;
  let refreshInFlight = false;
  let mounted = false;
  let refreshTimer: ReturnType<typeof setTimeout> | undefined;
  let pollTimer: ReturnType<typeof setInterval> | undefined;
  let stopAnalysis: (() => void) | undefined;

  function scheduleRefresh(delay = 1000) {
    if (!mounted || refreshTimer || refreshInFlight) return;
    refreshTimer = setTimeout(() => {
      refreshTimer = undefined;
      void refreshData();
    }, delay);
  }

  async function refreshData() {
    if (!mounted || refreshInFlight) return;
    refreshInFlight = true;
    const full = fullRefresh;
    fullRefresh = false;
    const ids = full ? [...dirtyIDs] : [...dirtyIDs].slice(0, 100);
    ids.forEach((id) => dirtyIDs.delete(id));
    const hasChanges = full || ids.length > 0;
    const loadCloud = hasChanges || cloudRefresh;
    cloudRefresh = false;
    try {
      const [updates, cloud, status] = await Promise.all([
        full ? GetTaggedPhotos() : ids.length ? GetTaggedPhotosByIDs(ids) : Promise.resolve([]),
        loadCloud ? GetTagCloud() : Promise.resolve(null),
        GetAnalysisStatus(),
      ]);
      if (!mounted) return;
      const existing = new Map(photos.value.map((photo) => [photo.id, photo]));
      const movingIDs = new Set((updates ?? []).filter((photo) => {
        const previous = existing.get(photo.id);
        return !previous || Boolean(previous.tags.length) !== Boolean(photo.tags.length);
      }).map((photo) => photo.id));
      const restoreViewport = capturePhotoViewport(pageRoot.value, movingIDs);
      if (full) {
        photos.value = updates ?? [];
      } else {
        for (const photo of updates ?? []) {
          const previous = existing.get(photo.id);
          if (previous) Object.assign(previous, photo);
          else photos.value.push(photo);
        }
      }
      if (hasChanges) {
        photos.value.sort((a, b) => Number(b.tags.length > 0) - Number(a.tags.length > 0)
          || b.backedUpAt.localeCompare(a.backedUpAt) || b.id - a.id);
      }
      if (cloud !== null) tagCloud.value = cloud ?? [];
      analysisStatus.value = status;
      syncError.value = "";
      loading.value = false;
      await nextTick();
      if (mounted) restoreViewport();
    } catch (error) {
      if (!mounted) return;
      const restoreViewport = capturePhotoViewport(pageRoot.value, new Set());
      if (full) fullRefresh = true;
      if (loadCloud) cloudRefresh = true;
      ids.forEach((id) => dirtyIDs.add(id));
      syncError.value = "อัปเดตข้อมูลไม่สำเร็จ กำลังลองใหม่: " + (error instanceof Error ? error.message : String(error));
      loading.value = false;
      await nextTick();
      if (mounted) restoreViewport();
    } finally {
      refreshInFlight = false;
      if (mounted) {
        loading.value = false;
        if (fullRefresh || cloudRefresh || dirtyIDs.size) scheduleRefresh(syncError.value ? 3000 : dirtyIDs.size >= 100 ? 0 : 1000);
      }
    }
  }

  function refreshPhotos(ids: number[]) {
    ids.forEach((id) => dirtyIDs.add(id));
    cloudRefresh = true;
    scheduleRefresh(0);
  }

  function loadData() {
    fullRefresh = true;
    scheduleRefresh(0);
  }

  async function addTag(photoIDs: number[], name: string) {
    errorMessage.value = "";
    successMessage.value = "";
    try {
      await AddTagToPhotos(photoIDs, name);
      selectedIDs.value = [];
      successMessage.value = `เพิ่ม Tag “${name}” แล้ว`;
      refreshPhotos(photoIDs);
    } catch (error) {
      errorMessage.value = error instanceof Error ? error.message : String(error || "เพิ่ม Tag ไม่สำเร็จ");
    }
  }

  async function removeTag(photoID: number, tagID: number) {
    errorMessage.value = "";
    successMessage.value = "";
    try {
      await RemoveTagFromPhoto(photoID, tagID);
      successMessage.value = "ลบ Tag ออกจากรูปแล้ว";
      refreshPhotos([photoID]);
    } catch (error) {
      errorMessage.value = error instanceof Error ? error.message : String(error || "ลบ Tag ไม่สำเร็จ");
    }
  }

  async function deleteTag(tag: TagCloudItem) {
    if (deletingTagID.value !== null) return;
    deletingTagID.value = tag.id;
    errorMessage.value = "";
    successMessage.value = "";
    try {
      await DeleteTag(tag.id);
      successMessage.value = `ลบ Tag “${tag.name}” ออกจากฐานข้อมูลและทุกรูปแล้ว`;
      refreshPhotos(photos.value.filter((photo) => photo.tags.some((item) => item.id === tag.id)).map((photo) => photo.id));
    } catch (error) {
      errorMessage.value = error instanceof Error ? error.message : String(error || "ลบ Tag ไม่สำเร็จ");
    } finally {
      deletingTagID.value = null;
    }
  }

  onMounted(() => {
    mounted = true;
    stopAnalysis = EventsOn("photo:analysis", (progress: { photoId?: number }) => {
      if (progress?.photoId) dirtyIDs.add(progress.photoId);
      scheduleRefresh();
    });
    void refreshData();
    pollTimer = setInterval(() => scheduleRefresh(0), 5000);
  });
  onUnmounted(() => {
    mounted = false;
    stopAnalysis?.();
    if (refreshTimer) clearTimeout(refreshTimer);
    if (pollTimer) clearInterval(pollTimer);
  });

  return {
    pageRoot, photos, tagCloud, selectedIDs, loading, errorMessage, syncError,
    successMessage, deletingTagID, analysisStatus, totalPhotos, taggedPhotos,
    completedPercent, loadData, addTag, removeTag, deleteTag,
  };
}
