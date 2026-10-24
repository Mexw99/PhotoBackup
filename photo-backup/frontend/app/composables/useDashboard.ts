import { ref, onMounted, onUnmounted } from "vue";
import {
  GetDatabasePath,
  GetDatabaseStatus,
  GetPhotoCount,
  GetRecentPhotos,
  GetTagCount,
  GetTags,
  GetAnalysisStatus,
  RetryFailedAnalysis,
} from "~~/wailsjs/go/main/App";
import { EventsOn } from "~~/wailsjs/runtime/runtime";
import type { PhotoSummary, TagItem } from "~/types/photo";
import type { service } from "~~/wailsjs/go/models";

export function useDashboard() {
  const databaseStatus = ref("กำลังตรวจสอบ...");
  const databasePath = ref("");
  const photoCount = ref(0);
  const tagCount = ref(0);
  const recentPhotos = ref<PhotoSummary[]>([]);
  const tags = ref<TagItem[]>([]);
  const analysisStatus = ref<service.AnalysisSummary>({
    enabled: false,
    pending: 0,
    processing: 0,
    completed: 0,
    failed: 0,
  });
  const errorMessage = ref("");
  const retryingAnalysis = ref(false);

  async function loadDashboard() {
    try {
      const [status, path, photosCount, tagsCount, photos, tagList, aiStatus] =
        await Promise.all([
          GetDatabaseStatus(),
          GetDatabasePath(),
          GetPhotoCount(),
          GetTagCount(),
          GetRecentPhotos(6),
          GetTags(),
          GetAnalysisStatus(),
        ]);

      databaseStatus.value = status;
      databasePath.value = path;
      photoCount.value = photosCount;
      tagCount.value = tagsCount;
      recentPhotos.value = photos;
      tags.value = tagList;
      analysisStatus.value = aiStatus;
    } catch (error) {
      errorMessage.value = "ไม่สามารถโหลดข้อมูลจาก Database ได้";

      console.error(error);
    }
  }

  async function retryFailedAnalysis() {
    if (retryingAnalysis.value) return;
    retryingAnalysis.value = true;
    try {
      await RetryFailedAnalysis();
      await loadDashboard();
    } finally {
      retryingAnalysis.value = false;
    }
  }

  let stopAnalysis: (() => void) | undefined;
  onMounted(() => {
    stopAnalysis = EventsOn("photo:analysis", loadDashboard);
    loadDashboard();
  });
  onUnmounted(() => stopAnalysis?.());

  return {
    databaseStatus,
    databasePath,
    photoCount,
    tagCount,
    recentPhotos,
    tags,
    analysisStatus,
    retryingAnalysis,
    retryFailedAnalysis,
    errorMessage,
  };
}
