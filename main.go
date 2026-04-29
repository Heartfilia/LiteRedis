package main

import (
	"embed"

	"LiteRedis/backend/config"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

const minAppWidth = 1220
const minAppHeight = 720

func main() {
	// Create an instance of the app structure
	app := NewApp()

	// 加载上次保存的窗口状态
	ws := config.LoadWindowState()
	app.initWindowState = ws
	if ws.Width < minAppWidth {
		ws.Width = minAppWidth
	}
	if ws.Height < minAppHeight {
		ws.Height = minAppHeight
	}

	// Create application with options
	err := wails.Run(&options.App{
		Title:     "LiteRedis",
		Width:     ws.Width,
		Height:    ws.Height,
		MinWidth:  minAppWidth,
		MinHeight: minAppHeight,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 255, G: 255, B: 255, A: 1},
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
