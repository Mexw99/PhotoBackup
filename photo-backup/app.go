package main

import (
	"context"
	"fmt"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"photo-backup/database"
	"photo-backup/service"
)

// App is the Wails presentation layer. Business operations live in service.
type App struct {
	ctx      context.Context
	database *database.Database
	service  *service.Service
	dbPath   string
	dbErr    error
}

func NewApp() *App { return &App{} }

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	if a.service != nil {
		a.service.SetContext(ctx)
		a.service.SetAnalysisNotifier(func(progress service.AnalysisProgress) {
			runtime.EventsEmit(ctx, "photo:analysis", progress)
		})
	}
}

func (a *App) shutdown(_ context.Context) {
	if a.service != nil {
		a.service.Close()
	}
	if a.database != nil {
		_ = a.database.Close()
	}
}

func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's show time!", name)
}

func (a *App) GetDatabaseStatus() string {
	if a.dbErr != nil || a.database == nil || a.service == nil {
		return "Disconnected"
	}
	return "Connected"
}

func (a *App) GetDatabasePath() string { return a.dbPath }
