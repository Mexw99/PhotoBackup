<template>
  <main class="tags-page">
    <header>
      <h1>จัดการ Tag</h1>
      <p>เลือกรูปภาพแล้วเพิ่ม Tag เพื่อจัดกลุ่ม</p>
    </header>

    <section class="panel">
      <p>เลือกแล้ว {{ selectedIDs.length }} รูป</p>
      <TagEditor :photo-i-ds="selectedIDs" @add="addTag" />
      <p v-if="errorMessage" class="error">{{ errorMessage }}</p>
      <p v-if="successMessage" class="success">{{ successMessage }}</p>
    </section>

    <section class="panel">
      <h2>Tag Cloud</h2>
      <TagCloud :tags="tagCloud" />
    </section>

    <section class="panel">
      <h2>รูปภาพที่ Backup แล้ว</h2>
      <p v-if="loading">กำลังโหลด...</p>
      <PhotoGrid
        v-else
        :photos="photos"
        :selected-i-ds="selectedIDs"
        empty-text="ยังไม่มีรูปภาพที่ Backup แล้ว"
        @update:selected-i-ds="selectedIDs = $event"
        @remove-tag="removeTag"
      />
    </section>
  </main>
</template>

<script setup lang="ts">
import {
  AddTagToPhotos,
  GetTagCloud,
  GetTaggedPhotos,
  RemoveTagFromPhoto,
} from "../../wailsjs/go/main/App"

import PhotoGrid from "../../components/layout/gallery/PhotoGrid.vue"
import TagEditor from "../../components/layout/gallery/TagEditor.vue"
import TagCloud from "../../components/tag-cloud/TagCloud.vue"

interface TagItem {
  id: number
  name: string
}

interface PhotoItem {
  id: number
  originalName: string
  backupPath: string
  backedUpAt: string
  tags: TagItem[]
}

interface TagCloudItem extends TagItem {
  photoCount: number
}

const photos = ref<PhotoItem[]>([])
const tagCloud = ref<TagCloudItem[]>([])
const selectedIDs = ref<number[]>([])
const loading = ref(true)
const errorMessage = ref("")
const successMessage = ref("")

async function loadData() {
  loading.value = true
  errorMessage.value = ""

  try {
    const [photoData, cloudData] = await Promise.all([
      GetTaggedPhotos(),
      GetTagCloud(),
    ])

    photos.value = photoData ?? []
    tagCloud.value = cloudData ?? []
  } catch (error) {
    errorMessage.value =
      error instanceof Error ? error.message : "โหลดข้อมูลไม่สำเร็จ"
  } finally {
    loading.value = false
  }
}

async function addTag(photoIDs: number[], name: string) {
  errorMessage.value = ""
  successMessage.value = ""

  try {
    await AddTagToPhotos(photoIDs, name)
    selectedIDs.value = []
    successMessage.value = `เพิ่ม Tag “${name}” แล้ว`
    await loadData()
  } catch (error) {
    errorMessage.value =
      error instanceof Error ? error.message : "เพิ่ม Tag ไม่สำเร็จ"
  }
}

async function removeTag(photoID: number, tagID: number) {
  errorMessage.value = ""
  successMessage.value = ""

  try {
    await RemoveTagFromPhoto(photoID, tagID)
    successMessage.value = "ลบ Tag ออกจากรูปแล้ว"
    await loadData()
  } catch (error) {
    errorMessage.value =
      error instanceof Error ? error.message : "ลบ Tag ไม่สำเร็จ"
  }
}

onMounted(loadData)
</script>

<style scoped>
.tags-page {
  display: grid;
  gap: 20px;
  padding: 12px 0;
}

header h1,
.panel h2 {
  margin: 0 0 10px;
}

header p {
  color: #64748b;
}

.panel {
  padding: 20px;
  border: 1px solid #e2e8f0;
  border-radius: 12px;
  background: white;
}

.error {
  color: #b91c1c;
}

.success {
  color: #166534;
}
</style>