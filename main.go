package main

import (
	"embed"
	"log"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"shop/internal/api"
	"shop/service"
)

// Wails 通过 Go 的 embed 包将 frontend/dist 中的前端文件嵌入二进制。
//
//go:embed all:frontend/dist
var assets embed.FS

func init() {
	application.RegisterEvent[string]("time")
}

func main() {
	go api.Start()

	app := application.New(application.Options{
		Name:        "shop",
		Description: "A shop application",
		Services: []application.Service{
			application.NewService(&service.GreetService{}),
			application.NewService(service.NewQRService()),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:  "Wails",
		Width:  1000,
		Height: 618,
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		BackgroundColour: application.NewRGB(6, 7, 15),
		URL:              "/",
	})

	go func() {
		for {
			now := time.Now().Format(time.RFC1123)
			app.Event.Emit("time", now)
			time.Sleep(time.Second)
		}
	}()

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
