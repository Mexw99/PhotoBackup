package main

type TagSummary struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

type PhotoSummary struct {
	ID              uint   `json:"id"`
	OriginalName    string `json:"originalName"`
	BackupPath      string `json:"backupPath"`
	BackedUpAt      string `json:"backedUpAt"`
	Thumbnail       string `json:"thumbnail"`
	ColorTag        string `json:"colorTag"`
	Description     string `json:"description"`
	BackupAvailable bool   `json:"backupAvailable"`
	BackupStatus    string `json:"backupStatus"`
}

type TagCloudItem struct {
	ID         uint   `json:"id"`
	Name       string `json:"name"`
	PhotoCount int64  `json:"photoCount"`
}

type TaggedPhotoSummary struct {
	ID              uint         `json:"id"`
	OriginalName    string       `json:"originalName"`
	BackupPath      string       `json:"backupPath"`
	BackedUpAt      string       `json:"backedUpAt"`
	Thumbnail       string       `json:"thumbnail"`
	ColorTag        string       `json:"colorTag"`
	Description     string       `json:"description"`
	BackupAvailable bool         `json:"backupAvailable"`
	BackupStatus    string       `json:"backupStatus"`
	Tags            []TagSummary `json:"tags"`
}

type BackupSettings struct {
	SourcePath string `json:"sourcePath"`
	DestPath   string `json:"destPath"`
}
