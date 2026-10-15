<template>
  <div class="app-shell">
    <AppHeader />

    <main class="page-container">
      <NuxtPage />
    </main>
  </div>
</template>

<script setup lang="ts">
import AppHeader from "../components/layout/AppHeader.vue";
</script>

<style>
* {
  box-sizing: border-box;
}

body {
  margin: 0;
  background: #f4f7fb;
  color: #172033;
  font-family: Arial, sans-serif;
}

.app-shell {
  min-height: 100vh;
}

.page-container {
  margin: 0 auto;
  max-width: 1100px;
  padding: 12px 24px 32px;
}
</style>
