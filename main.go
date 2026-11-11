package main

import (
	"embed"
	"runtime"

	"LiteRedis/backend/config"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:frontend/dist
var assets embed.FS

var AppVersion = "dev"

const minAppWidth = 1220
const minAppHeight = 720

func main() {
	// 开发版默认换一个端口，避免和同时运行的正式版抢 64201
	config.SetDevBuild(AppVersion == "dev")

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
	var appMenu *menu.Menu
	if runtime.GOOS == "darwin" {
		// Keep native Quit (Cmd+Q) routed through OnBeforeClose. Cmd+W closes
		// the selected key instead of allowing the WebView to close its window.
		appMenu = menu.NewMenuFromItems(menu.AppMenu())
		fileMenu := appMenu.AddSubmenu("File")
		fileMenu.AddText("Close Key", keys.CmdOrCtrl("w"), func(_ *menu.CallbackData) {
			if app.ctx != nil {
				wailsruntime.EventsEmit(app.ctx, "app:close-key")
			}
		})
		appMenu.Append(menu.EditMenu())
		appMenu.Append(menu.WindowMenu())
	}
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
		OnBeforeClose:    app.beforeClose,
		Menu:             appMenu,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
