import type { main } from "~~/wailsjs/go/models";

export type PhotoSummary = main.PhotoSummary;
export type PhotoItem = main.TaggedPhotoSummary;
export type TagItem = main.TagSummary;
export type TagCloudItem = main.TagCloudItem;
export type MatchMode = "AND" | "OR";
export type AvailableTag = TagItem & { photoCount?: number };
