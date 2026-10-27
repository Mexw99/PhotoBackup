import { ref, computed, onMounted } from "vue";
import {
  GetBackupSettings,
  SaveBackupSettings,
  ListImageFiles,
  SelectSourceFolder,
  SelectDestFolder,
  StartBackup,
  CheckBackup,
  GetAIQueueStatus,
  GetAIProviderStatus,
} from "~~/wailsjs/go/main/App";
import { EventsOn } from "~~/wailsjs/runtime/runtime";
import type { ImageFile, BackupSummary, BackupProgress, AIQueueStatus, AIProviderStatus, IntegrityResult } from "~/types/backup";

export function useBackup() {
  ensureProgressListener();

  const successResults = computed(() =>
    backupSummary.value.results.filter(
      (result) => result.success && !result.skipped,
    ),
  );
  const skippedResults = computed(() =>
    backupSummary.value.results.filter((result) => result.skipped),
  );
  const failedResults = computed(() =>
    backupSummary.value.results.filter(
      (result) => !result.success && !result.skipped,
    ),
  );

  async function readSourceFiles() {
    loading.value = true;
    errorMessage.value = "";

    try {
      imageFiles.value = (await ListImageFiles(sourcePath.value)) ?? [];
    } catch (error) {
      imageFiles.value = [];
      errorMessage.value = `${String(error || "อ่านโฟลเดอร์ไม่สำเร็จ")} กรุณาต่อ Flash Drive หรือเลือกโฟลเดอร์ต้นทางใหม่`;
    } finally {
      loading.value = false;
    }
  }

  // โหลดโฟลเดอร์ที่บันทึกใน SQLite เมื่อเปิดหน้านี้
  onMounted(async () => {
    loading.value = true;

    try {
      await refreshAIQueueStatus();
      await refreshAIProviderStatus();
      const settings = await GetBackupSettings();
      sourcePath.value = settings.sourcePath;
      destPath.value = settings.destPath;
      if (sourcePath.value) await readSourceFiles();
    } catch (error) {
      errorMessage.value = String(error || "โหลดโฟลเดอร์ที่บันทึกไว้ไม่สำเร็จ");
    } finally {
      loading.value = false;
    }
  });

  async function refreshAIQueueStatus() {
    try {
      const status = await GetAIQueueStatus();
      if (status) updateAIQueueStatus(status);
    } catch {
      // Queue status is supplementary; it must not block Backup UI.
    }
  }

  async function refreshAIProviderStatus() {
    try {
      const status = await GetAIProviderStatus();
      if (status) aiProviderStatus.value = status;
    } catch {
      // Provider status is supplementary; it must not block Backup UI.
    }
  }

  async function chooseSourceFolder() {
    errorMessage.value = "";
    settingsMessage.value = "";

    try {
      const selectedPath = await SelectSourceFolder();
      if (!selectedPath) return;
      sourcePath.value = selectedPath;
      resetBackupState();
      await readSourceFiles();
    } catch (error) {
      errorMessage.value = String(error || "ไม่สามารถเลือกโฟลเดอร์ได้");
    }
  }

  function clearSourceFolder() {
    sourcePath.value = "";
    imageFiles.value = [];
    errorMessage.value = "";
    settingsMessage.value = "";
    resetBackupState();
  }

  async function chooseDestFolder() {
    errorMessage.value = "";
    settingsMessage.value = "";

    try {
      const selectedPath = await SelectDestFolder();
      if (selectedPath) {
        destPath.value = selectedPath;
        integrityDone.value = false;
        integrityResults.value = [];
      }
    } catch (error) {
      errorMessage.value = String(error || "ไม่สามารถเลือกโฟลเดอร์ได้");
    }
  }

  function clearDestFolder() {
    destPath.value = "";
    settingsMessage.value = "";
    integrityDone.value = false;
    integrityResults.value = [];
  }

  async function checkDestination() {
    if (!destPath.value) {
      errorMessage.value = "กรุณาเลือกโฟลเดอร์ปลายทางก่อนตรวจสอบ";
      return;
    }
    errorMessage.value = "";
    integrityLoading.value = true;
    integrityDone.value = false;
    try {
      integrityResults.value = (await CheckBackup(destPath.value)) ?? [];
      integrityDone.value = true;
    } catch (error) {
      errorMessage.value = String(error || "ตรวจสอบไฟล์ปลายทางไม่สำเร็จ");
    } finally {
      integrityLoading.value = false;
    }
  }

  async function saveSettings() {
    errorMessage.value = "";
    settingsMessage.value = "";
    if (!sourcePath.value || !destPath.value) {
      errorMessage.value = "กรุณาเลือกโฟลเดอร์ต้นทางและปลายทางให้ครบ";
      return false;
    }

    settingsSaving.value = true;
    try {
      await SaveBackupSettings(sourcePath.value, destPath.value);
      settingsMessage.value = "บันทึกโฟลเดอร์แล้ว ครั้งถัดไปจะใช้ค่าที่บันทึกไว้";
      return true;
    } catch (error) {
      errorMessage.value = String(error || "บันทึกโฟลเดอร์ไม่สำเร็จ");
      return false;
    } finally {
      settingsSaving.value = false;
    }
  }

  async function startBackup() {
    if (backupRunning.value || settingsSaving.value) return;
    if (!imageFiles.value.length) {
      errorMessage.value = "ไม่พบรูปภาพ กรุณาเลือกโฟลเดอร์ที่มีไฟล์ .jpg, .jpeg หรือ .png";
      return;
    }

    resetBackupState();
    backupRunning.value = true;
    backupStartedAt.value = Date.now();
    elapsedMs.value = 0;
    progressTotal.value = imageFiles.value.length;
    startElapsedTimer();

    try {
      if (!(await saveSettings())) return;

      const result = await StartBackup(sourcePath.value, destPath.value);
      backupSummary.value = result;
      progressCurrent.value = result.total;
      progressTotal.value = result.total;
      progressName.value = "";
      backupDone.value = true;
      await readSourceFiles();
    } catch (error) {
      errorMessage.value = String(error || "Backup ล้มเหลว");
    } finally {
      backupRunning.value = false;
      stopElapsedTimer();
    }
  }

  function resetBackupState() {
    backupDone.value = false;
    progressCurrent.value = 0;
    progressTotal.value = 0;
    progressName.value = "";
    backupStartedAt.value = null;
    elapsedMs.value = 0;
    integrityDone.value = false;
    integrityResults.value = [];
    backupSummary.value = {
      total: 0,
      success: 0,
      skipped: 0,
      failed: 0,
      results: [],
    };
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
    settingsMessage,
    settingsSaving,
    backupRunning,
    backupDone,
    progressCurrent,
    progressTotal,
    progressName,
    backupSummary,
    integrityLoading,
    integrityDone,
    integrityResults,
    progressPercent,
    elapsedMs,
    elapsedLabel,
    aiElapsedLabel,
    successResults,
    skippedResults,
    failedResults,
    aiQueueStatus,
    aiProviderStatus,
    refreshAIQueueStatus,
    refreshAIProviderStatus,
    readSourceFiles,
    chooseSourceFolder,
    clearSourceFolder,
    chooseDestFolder,
    clearDestFolder,
    checkDestination,
    saveSettings,
    startBackup,
    resetBackupState,
    formatBytes,
  };
}

export function useBackupStatus() {
  ensureProgressListener();
  return {
    backupRunning,
    backupDone,
    progressCurrent,
    progressTotal,
    progressName,
    progressPercent,
    elapsedLabel,
    aiElapsedLabel,
  };
}

// State อยู่ระดับ module จึงไม่ถูกสร้างใหม่เมื่อเปลี่ยนหน้า
const sourcePath = ref("");
const destPath = ref("");
const imageFiles = ref<ImageFile[]>([]);
const loading = ref(false);
const errorMessage = ref("");
const settingsMessage = ref("");
const settingsSaving = ref(false);
const backupRunning = ref(false);
const backupDone = ref(false);
const progressCurrent = ref(0);
const progressTotal = ref(0);
const progressName = ref("");
const backupStartedAt = ref<number | null>(null);
const elapsedMs = ref(0);
const progressPercent = computed(() => {
  if (progressTotal.value === 0) return 0;
  return Math.round((progressCurrent.value / progressTotal.value) * 100);
});
const elapsedLabel = computed(() => formatElapsed(elapsedMs.value));
const backupSummary = ref<BackupSummary>({
  total: 0,
  success: 0,
  skipped: 0,
  failed: 0,
  results: [],
});
const integrityLoading = ref(false);
const integrityDone = ref(false);
const integrityResults = ref<IntegrityResult[]>([]);
const aiQueueStatus = ref<AIQueueStatus>({
  queued: 0,
  processing: 0,
  completed: 0,
  failed: 0,
  retryWait: 0,
});
const aiProviderStatus = ref<AIProviderStatus>({
  provider: "",
  model: "",
  state: "disabled",
  message: "ยังไม่ได้ตั้งค่า AI",
});
const aiStartedAt = ref<number | null>(null);
const aiElapsedMs = ref(0);
const aiElapsedLabel = computed(() => formatElapsed(aiElapsedMs.value));
let aiWasActive = false;
let aiElapsedTimer: ReturnType<typeof setInterval> | undefined;

let progressListenerReady = false;
let elapsedTimer: ReturnType<typeof setInterval> | undefined;

function ensureProgressListener() {
  if (progressListenerReady) return;
  progressListenerReady = true;
  EventsOn("backup:progress", (data: BackupProgress) => {
    progressCurrent.value = data.current;
    progressTotal.value = data.total;
    progressName.value = data.name;
  });
  EventsOn("ai:progress", (data: AIQueueStatus) => {
    if (data) updateAIQueueStatus(data);
  });
}

function updateAIQueueStatus(status: AIQueueStatus) {
  aiQueueStatus.value = status;
  const active = status.queued > 0 || status.processing > 0 || status.retryWait > 0;
  if (active && !aiWasActive) {
    aiStartedAt.value = Date.now();
    aiElapsedMs.value = 0;
    startAIElapsedTimer();
  }
  if (!active && aiWasActive) {
    stopAIElapsedTimer();
  }
  aiWasActive = active;
}

function startAIElapsedTimer() {
  if (aiElapsedTimer) clearInterval(aiElapsedTimer);
  aiElapsedTimer = setInterval(() => {
    if (aiStartedAt.value !== null && aiWasActive) {
      aiElapsedMs.value = Date.now() - aiStartedAt.value;
    }
  }, 1000);
}

function stopAIElapsedTimer() {
  if (aiElapsedTimer) clearInterval(aiElapsedTimer);
  aiElapsedTimer = undefined;
  if (aiStartedAt.value !== null) {
    aiElapsedMs.value = Date.now() - aiStartedAt.value;
  }
}

function startElapsedTimer() {
  if (elapsedTimer) clearInterval(elapsedTimer);
  elapsedTimer = setInterval(() => {
    if (backupStartedAt.value !== null && backupRunning.value) {
      elapsedMs.value = Date.now() - backupStartedAt.value;
    }
  }, 1000);
}

function stopElapsedTimer() {
  if (elapsedTimer) clearInterval(elapsedTimer);
  elapsedTimer = undefined;
  if (backupStartedAt.value !== null) {
    elapsedMs.value = Date.now() - backupStartedAt.value;
  }
}

function formatElapsed(milliseconds: number) {
  const totalSeconds = Math.max(0, Math.floor(milliseconds / 1000));
  const hours = Math.floor(totalSeconds / 3600);
  const minutes = Math.floor((totalSeconds % 3600) / 60);
  const seconds = totalSeconds % 60;
  return [hours, minutes, seconds].map((value) => String(value).padStart(2, "0")).join(":");
}
