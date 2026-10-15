<template>
  <div v-if="tags.length" class="tag-cloud">
    <span
      v-for="tag in tags"
      :key="tag.id"
      class="tag"
      :style="tagStyle(tag.photoCount)"
    >
      #{{ tag.name }}
      <small>{{ tag.photoCount }}</small>
    </span>
  </div>
  <p v-else class="empty">ยังไม่มี Tag</p>
</template>

<script setup lang="ts">
import { computed } from 'vue';

interface TagCloudItem {
  id: number
  name: string
  photoCount: number
}

const props = defineProps<{
  tags: TagCloudItem[]
}>()

const largestCount = computed(() =>
  Math.max(1, ...props.tags.map((tag) => tag.photoCount)),
)

function tagStyle(count: number) {
  const strength = count / largestCount.value
  return { fontSize: `${0.85 + strength * 0.55}rem` }
}
</script>

<style scoped>
.tag-cloud {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px;
}

.tag {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 8px 12px;
  border-radius: 999px;
  background: #dbeafe;
  color: #1d4ed8;
  font-weight: 700;
}

.tag small {
  padding: 2px 6px;
  border-radius: 999px;
  background: white;
}

.empty {
  color: #64748b;
}
</style>