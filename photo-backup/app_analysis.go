package main

import (
	"fmt"

	"photo-backup/service"
)

func (a *App) GetAnalysisStatus() (service.AnalysisSummary, error) {
	if a.service == nil {
		return service.AnalysisSummary{}, fmt.Errorf("database is not ready")
	}
	return a.service.AnalysisStatus()
}

func (a *App) RetryFailedAnalysis() int {
	if a.service == nil {
		return 0
	}
	return a.service.RetryFailedAnalysis()
}
