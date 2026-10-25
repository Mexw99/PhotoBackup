package main

type TagSummary struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

type PhotoSummary struct {
	ID             uint   `json:"id"`
	OriginalName   string `json:"originalName"`
	BackupPath     string `json:"backupPath"`
	BackedUpAt     string `json:"backedUpAt"`
	Thumbnail      string `json:"thumbnail"`
	Description    string `json:"description"`
	AnalysisStatus string `json:"analysisStatus"`
	AnalysisError  string `json:"analysisError"`
	AnalyzedAt     string `json:"analyzedAt"`
}

type TagCloudItem struct {
	ID         uint   `json:"id"`
	Name       string `json:"name"`
	PhotoCount int64  `json:"photoCount"`
}

type TaggedPhotoSummary struct {
	ID             uint         `json:"id"`
	OriginalName   string       `json:"originalName"`
	BackupPath     string       `json:"backupPath"`
	BackedUpAt     string       `json:"backedUpAt"`
	Thumbnail      string       `json:"thumbnail"`
	Description    string       `json:"description"`
	AnalysisStatus string       `json:"analysisStatus"`
	AnalysisError  string       `json:"analysisError"`
	AnalyzedAt     string       `json:"analyzedAt"`
	Tags           []TagSummary `json:"tags"`
}

type BackupSettings struct {
	SourcePath string `json:"sourcePath"`
	DestPath   string `json:"destPath"`
}
