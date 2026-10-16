package main

type TagSummary struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

type PhotoSummary struct {
	ID           uint   `json:"id"`
	OriginalName string `json:"originalName"`
	BackupPath   string `json:"backupPath"`
	BackedUpAt   string `json:"backedUpAt"`
	Thumbnail    string `json:"thumbnail"`
}

type TagCloudItem struct {
	ID         uint   `json:"id"`
	Name       string `json:"name"`
	PhotoCount int64  `json:"photoCount"`
}

type TaggedPhotoSummary struct {
	ID           uint         `json:"id"`
	OriginalName string       `json:"originalName"`
	BackupPath   string       `json:"backupPath"`
	BackedUpAt   string       `json:"backedUpAt"`
	Thumbnail    string       `json:"thumbnail"`
	Tags         []TagSummary `json:"tags"`
}
