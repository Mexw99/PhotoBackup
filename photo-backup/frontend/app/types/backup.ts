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
  | "error"
>;
export type BackupSummary = Pick<
  backup.BackupSummary,
  "total" | "success" | "failed"
> & {
  results: BackupResult[];
};
export interface BackupProgress {
  current: number;
  total: number;
  name: string;
}
