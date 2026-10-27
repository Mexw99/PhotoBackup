package main

import (
	"embed"

	"photo-backup/config"
	"photo-backup/database"
	"photo-backup/repository"
	"photo-backup/service"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	if err := config.LoadEnv(); err != nil {
		println("Warning: could not load .env:", err.Error())
	}

	app := NewApp()
	db, path, dbErr := database.OpenDefault()
	app.database, app.dbPath, app.dbErr = db, path, dbErr
	if dbErr == nil {
		app.service = service.New(repository.New(db.GORM()))
	}

	err := wails.Run(&options.App{
		Title:  "photo-backup",
		Width:  1024,
		Height: 768,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
