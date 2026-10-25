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
  | "message"
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

export interface IntegrityResult {
  filename: string;
  path: string;
  status: string;
  message: string;
  expectedSize: number;
  actualSize: number;
}
