import { ref, computed, onMounted, onUnmounted } from "vue";
import {
  GetBackupSettings,
  SaveBackupSettings,
  ListImageFiles,
  SelectSourceFolder,
  SelectDestFolder,
  StartBackup,
  CheckBackup,
} from "~~/wailsjs/go/main/App";
import { EventsOn } from "~~/wailsjs/runtime/runtime";
import type {
  ImageFile,
  BackupSummary,
  BackupProgress,
  IntegrityResult,
} from "~/types/backup";

export function useBackup() {
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

  let stopProgress: (() => void) | undefined;
  onUnmounted(() => stopProgress?.());

  const progressPercent = computed(() => {
    if (progressTotal.value === 0) return 0;
    return Math.round((progressCurrent.value / progressTotal.value) * 100);
  });
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
      if (selectedPath) destPath.value = selectedPath;
    } catch (error) {
      errorMessage.value = String(error || "ไม่สามารถเลือกโฟลเดอร์ได้");
    }
  }

  function clearDestFolder() {
    destPath.value = "";
    settingsMessage.value = "";
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

    try {
      if (!(await saveSettings())) return;

      stopProgress?.();
      stopProgress = EventsOn("backup:progress", (data: BackupProgress) => {
        progressCurrent.value = data.current;
        progressTotal.value = data.total;
        progressName.value = data.name;
      });

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
      stopProgress?.();
      stopProgress = undefined;
    }
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

  function resetBackupState() {
    backupDone.value = false;
    progressCurrent.value = 0;
    progressTotal.value = 0;
    progressName.value = "";
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
    successResults,
    skippedResults,
    failedResults,
    readSourceFiles,
    chooseSourceFolder,
    clearSourceFolder,
    chooseDestFolder,
    clearDestFolder,
    saveSettings,
    startBackup,
    checkDestination,
    resetBackupState,
    formatBytes,
  };
}
