import type { backup } from "~~/wailsjs/go/models";

// Pick only serialized data fields, excluding generated class helper methods.
export type ImageFile = Pick<backup.ImageFile, "name" | "path" | "size">;
export type BackupResult = Pick<
  backup.BackupResult,
  | "originalName"
  | "originalPath"
  | "backupPath"
  | "fileSize"
  | "success"
  | "skipped"
  | "error"
>;
export type BackupSummary = Pick<
  backup.BackupSummary,
  "total" | "success" | "skipped" | "failed"
> & {
  results: BackupResult[];
};
export interface BackupProgress {
  current: number;
  total: number;
  name: string;
}

export interface AIQueueStatus {
  queued: number;
  processing: number;
  completed: number;
  failed: number;
  retryWait: number;
}

export interface AIProviderStatus {
  provider: string;
  model: string;
  state: string;
  message: string;
}
