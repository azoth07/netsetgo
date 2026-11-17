package main

import (
	"log"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func main() {
	state := newDesktop()
	mygo.App.WhenReady(func() {
		mygo.App.SetLocale("zh-CN")
		state.window = mygo.NewWindow(mygo.WindowOptions{
			Title: "NetSetGo · 网络方案",
			Width: 1280, Height: 900, MinWidth: 1000, MinHeight: 700,
			Content: ui.View(state.render),
		})
		state.initialize()
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
