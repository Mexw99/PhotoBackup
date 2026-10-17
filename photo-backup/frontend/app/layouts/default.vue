<template>
  <div class="app-shell">
    <AppHeader />
    <main class="page-container">
      <slot />
    </main>
  </div>
</template>

<script setup lang="ts">
import AppHeader from "~/components/layout/app-header.vue";
</script>

<style scoped>
.app-shell {
  min-height: 100vh;
}

.page-container {
  margin: 0 auto;
  max-width: 1100px;
  padding: 12px 24px 32px;
}
</style>
