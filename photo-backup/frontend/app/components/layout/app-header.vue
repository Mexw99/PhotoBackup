<template>
  <header class="header">
    <NuxtLink class="brand" to="/"> PhotoBackup </NuxtLink>

    <nav class="menu">
      <NuxtLink to="/">Dashboard</NuxtLink>
      <NuxtLink to="/backup">Backup</NuxtLink>
      <NuxtLink to="/tags">Tag</NuxtLink>
      <NuxtLink to="/search">Search</NuxtLink>
    </nav>

    <NuxtLink v-if="backupRunning" class="backup-status" to="/backup">
      <span class="status-dot"></span>
      Backup {{ progressPercent }}% · {{ progressCurrent }}/{{ progressTotal }} · {{ elapsedLabel }}
    </NuxtLink>
    <NuxtLink v-else-if="backupDone" class="backup-status done" to="/backup">
      ✓ Backup เสร็จแล้ว · {{ elapsedLabel }}
    </NuxtLink>
  </header>
</template>

<script setup lang="ts">
import { useBackupStatus } from "~/composables/useBackup";

const {
  backupRunning,
  backupDone,
  progressCurrent,
  progressTotal,
  progressPercent,
  elapsedLabel,
} = useBackupStatus();
</script>

<style scoped>
.header {
  align-items: center;
  background: #ffffff;
  border-bottom: 1px solid #e5e7eb;
  display: flex;
  justify-content: space-between;
  padding: 18px 32px;
}

.brand {
  color: #2563eb;
  font-size: 1.25rem;
  font-weight: bold;
  text-decoration: none;
}

.menu {
  display: flex;
  gap: 10px;
}

.menu a {
  border-radius: 8px;
  color: #475569;
  padding: 8px 12px;
  text-decoration: none;
}

.menu a:hover,
.menu a.router-link-active {
  background: #eff6ff;
  color: #1d4ed8;
}

.backup-status {
  align-items: center;
  background: #eff6ff;
  border: 1px solid #bfdbfe;
  border-radius: 999px;
  color: #1d4ed8;
  display: inline-flex;
  font-size: 0.8rem;
  font-weight: 700;
  gap: 7px;
  padding: 7px 11px;
  text-decoration: none;
  white-space: nowrap;
}

.backup-status.done {
  background: #f0fdf4;
  border-color: #bbf7d0;
  color: #15803d;
}

.status-dot {
  animation: pulse 1.2s infinite;
  background: #2563eb;
  border-radius: 50%;
  height: 8px;
  width: 8px;
}

@keyframes pulse {
  50% { opacity: 0.35; }
}

@media (max-width: 700px) {
  .header {
    align-items: flex-start;
    flex-direction: column;
    gap: 16px;
  }

  .menu {
    flex-wrap: wrap;
  }

  .backup-status {
    white-space: normal;
  }
}
</style>
