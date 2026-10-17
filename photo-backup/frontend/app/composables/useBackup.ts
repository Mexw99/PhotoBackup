import { ref, computed, onUnmounted } from "vue";
import {
  ListImageFiles,
  SelectSourceFolder,
  SelectDestFolder,
  StartBackup,
} from "~~/wailsjs/go/main/App";
import { EventsOn } from "~~/wailsjs/runtime/runtime";
import type { ImageFile, BackupSummary, BackupProgress } from "~/types/backup";

export function useBackup() {
  let stopProgress: (() => void) | undefined;
  onUnmounted(() => stopProgress?.());

  // ---------- State ----------
  const sourcePath = ref("");
  const destPath = ref("");
  const imageFiles = ref<ImageFile[]>([]);
  const loading = ref(false);
  const errorMessage = ref("");

  // Progress
  const backupRunning = ref(false);
  const backupDone = ref(false);
  const progressCurrent = ref(0);
  const progressTotal = ref(0);
  const progressName = ref("");
  const backupSummary = ref<BackupSummary>({
    total: 0,
    success: 0,
    failed: 0,
    results: [],
  });

  const progressPercent = computed(() => {
    if (progressTotal.value === 0) return 0;
    return Math.round((progressCurrent.value / progressTotal.value) * 100);
  });

  const successResults = computed(() =>
    backupSummary.value.results.filter((r) => r.success),
  );
  const failedResults = computed(() =>
    backupSummary.value.results.filter((r) => !r.success),
  );

  // ---------- เลือกโฟลเดอร์ต้นทาง ----------
  async function chooseSourceFolder() {
    errorMessage.value = "";
    loading.value = true;

    try {
      const selectedPath = await SelectSourceFolder();
      if (!selectedPath) return;

      sourcePath.value = selectedPath;
      imageFiles.value = await ListImageFiles(selectedPath);
    } catch (error) {
      errorMessage.value =
        error instanceof Error ? error.message : "ไม่สามารถอ่านโฟลเดอร์ได้";
      imageFiles.value = [];
    } finally {
      loading.value = false;
    }
  }

  function clearSourceFolder() {
    sourcePath.value = "";
    imageFiles.value = [];
    errorMessage.value = "";
    resetBackupState();
  }

  // ---------- เลือกโฟลเดอร์ปลายทาง ----------
  async function chooseDestFolder() {
    try {
      const selectedPath = await SelectDestFolder();
      if (selectedPath) {
        destPath.value = selectedPath;
      }
    } catch (error) {
      errorMessage.value =
        error instanceof Error ? error.message : "ไม่สามารถเลือกโฟลเดอร์ได้";
    }
  }

  function clearDestFolder() {
    destPath.value = "";
  }

  // ---------- เริ่ม Backup ----------
  async function startBackup() {
    if (!sourcePath.value || !destPath.value) return;

    resetBackupState();
    backupRunning.value = true;
    errorMessage.value = "";

    // ดักรับ event progress จาก Go backend
    stopProgress?.();
    stopProgress = EventsOn("backup:progress", (data: BackupProgress) => {
      progressCurrent.value = data.current;
      progressTotal.value = data.total;
      progressName.value = data.name;
    });

    try {
      const result = await StartBackup(sourcePath.value, destPath.value);
      backupSummary.value = result;
      backupDone.value = true;
      imageFiles.value = await ListImageFiles(sourcePath.value);
    } catch (error) {
      errorMessage.value =
        error instanceof Error ? error.message : "Backup ล้มเหลว";
    } finally {
      backupRunning.value = false;
      stopProgress?.();
      stopProgress = undefined;
    }
  }

  function resetBackupState() {
    backupDone.value = false;
    progressCurrent.value = 0;
    progressTotal.value = 0;
    progressName.value = "";
    backupSummary.value = { total: 0, success: 0, failed: 0, results: [] };
  }

  function formatBytes(size: number) {
    if (size < 1024) return `${size} B`;
    if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KB`;
    return `${(size / (1024 * 1024)).toFixed(1)} MB`;
  }

  return {
    sourcePath,
    destPath,
    imageFiles,
    loading,
    errorMessage,
    backupRunning,
    backupDone,
    progressCurrent,
    progressTotal,
    progressName,
    backupSummary,
    progressPercent,
    successResults,
    failedResults,
    chooseSourceFolder,
    clearSourceFolder,
    chooseDestFolder,
    clearDestFolder,
    startBackup,
    resetBackupState,
    formatBytes,
  };
}
