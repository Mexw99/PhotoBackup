<template>
  <form class="tag-editor" @submit.prevent="submit">
    <input v-model="name" placeholder="ชื่อ Tag เช่น ทะเล" />
    <button type="submit" :disabled="!name.trim() || !props.photoIDs.length">
      เพิ่ม Tag ให้รูปที่เลือก
    </button>
  </form>
</template>

<script setup lang="ts">
import { ref } from 'vue';

const props = defineProps<{
  photoIDs: number[]
}>()

const emit = defineEmits<{
  add: [photoIDs: number[], name: string]
}>()

const name = ref("")

function submit() {
  const cleanName = name.value.trim()
  if (!cleanName || props.photoIDs.length === 0) return

  emit("add", [...props.photoIDs], cleanName)
  name.value = ""
}
</script>

<style scoped>
.tag-editor {
  display: flex;
  gap: 10px;
}

input {
  flex: 1;
  min-width: 0;
  padding: 10px 12px;
  border: 1px solid #cbd5e1;
  border-radius: 8px;
}

button {
  padding: 10px 16px;
  border: 0;
  border-radius: 8px;
  background: #2563eb;
  color: white;
  font-weight: 700;
}

button:disabled {
  opacity: 0.55;
}

@media (max-width: 600px) {
  .tag-editor {
    flex-direction: column;
  }
}
</style>