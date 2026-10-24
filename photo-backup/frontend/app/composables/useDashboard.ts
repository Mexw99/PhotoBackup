import { ref, onMounted } from "vue";
import {
  GetDatabasePath,
  GetDatabaseStatus,
  GetPhotoCount,
  GetRecentPhotos,
  GetTagCount,
  GetTags,
} from "~~/wailsjs/go/main/App";
import type { PhotoSummary, TagItem } from "~/types/photo";

export function useDashboard() {
  const databaseStatus = ref("กำลังตรวจสอบ...");
  const databasePath = ref("");
  const photoCount = ref(0);
  const tagCount = ref(0);
  const recentPhotos = ref<PhotoSummary[]>([]);
  const tags = ref<TagItem[]>([]);
  const errorMessage = ref("");

  onMounted(async () => {
    try {
      const [status, path, photosCount, tagsCount, photos, tagList] =
        await Promise.all([
          GetDatabaseStatus(),
          GetDatabasePath(),
          GetPhotoCount(),
          GetTagCount(),
          GetRecentPhotos(6),
          GetTags(),
        ]);

      databaseStatus.value = status;
      databasePath.value = path;
      photoCount.value = photosCount;
      tagCount.value = tagsCount;
      recentPhotos.value = photos;
      tags.value = tagList;
    } catch (error) {
      errorMessage.value = "ไม่สามารถโหลดข้อมูลจาก Database ได้";

      console.error(error);
    }
  });

  return {
    databaseStatus,
    databasePath,
    photoCount,
    tagCount,
    recentPhotos,
    tags,
    errorMessage,
  };
}
