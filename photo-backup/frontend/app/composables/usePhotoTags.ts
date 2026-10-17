import { ref, onMounted } from "vue";
import {
  AddTagToPhotos,
  DeleteTag,
  GetTagCloud,
  GetTaggedPhotos,
  RemoveTagFromPhoto,
} from "~~/wailsjs/go/main/App";
import type { PhotoItem, TagCloudItem } from "~/types/photo";

export function usePhotoTags() {
  const photos = ref<PhotoItem[]>([]);
  const tagCloud = ref<TagCloudItem[]>([]);
  const selectedIDs = ref<number[]>([]);
  const loading = ref(true);
  const errorMessage = ref("");
  const successMessage = ref("");
  const deletingTagID = ref<number | null>(null);

  async function loadData() {
    loading.value = true;
    errorMessage.value = "";

    try {
      const [photoData, cloudData] = await Promise.all([
        GetTaggedPhotos(),
        GetTagCloud(),
      ]);

      photos.value = photoData ?? [];
      tagCloud.value = cloudData ?? [];
    } catch (error) {
      errorMessage.value =
        error instanceof Error ? error.message : "โหลดข้อมูลไม่สำเร็จ";
    } finally {
      loading.value = false;
    }
  }

  async function addTag(photoIDs: number[], name: string) {
    errorMessage.value = "";
    successMessage.value = "";

    try {
      await AddTagToPhotos(photoIDs, name);
      selectedIDs.value = [];
      successMessage.value = `เพิ่ม Tag “${name}” แล้ว`;
      await loadData();
    } catch (error) {
      errorMessage.value =
        error instanceof Error ? error.message : "เพิ่ม Tag ไม่สำเร็จ";
    }
  }

  async function removeTag(photoID: number, tagID: number) {
    errorMessage.value = "";
    successMessage.value = "";

    try {
      await RemoveTagFromPhoto(photoID, tagID);
      successMessage.value = "ลบ Tag ออกจากรูปแล้ว";
      await loadData();
    } catch (error) {
      errorMessage.value =
        error instanceof Error ? error.message : "ลบ Tag ไม่สำเร็จ";
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
      await loadData();
    } catch (error) {
      errorMessage.value =
        error instanceof Error
          ? error.message
          : String(error || "ลบ Tag ไม่สำเร็จ");
    } finally {
      deletingTagID.value = null;
    }
  }

  onMounted(loadData);

  return {
    photos,
    tagCloud,
    selectedIDs,
    loading,
    errorMessage,
    successMessage,
    deletingTagID,
    loadData,
    addTag,
    removeTag,
    deleteTag,
  };
}
