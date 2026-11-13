package main

import (
	"context"
	"log"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/updater"
	"github.com/stmtc233/sportsmeet/internal/bind"
	"github.com/stmtc233/sportsmeet/internal/mcp"
	"github.com/stmtc233/sportsmeet/internal/service"
	"github.com/stmtc233/sportsmeet/internal/store"
)

func main() {
	// Bindings are registered before Run; the service is filled in once
	// the data directory is known (mygo generate never reaches it).
	core := &bind.Core{}
	mygo.Bind(bind.Meets{Core: core}, bind.Units{Core: core}, bind.Athletes{Core: core}, bind.Events{Core: core},
		bind.Combined{Core: core}, bind.Entries{Core: core}, bind.Arrange{Core: core}, bind.Results{Core: core}, bind.Reports{Core: core},
		&bind.Exchange{Core: core}, bind.Marks{}, bind.App{}, bind.Settings{Core: core})

	// The update window checks in the background and offers new versions;
	// the app's interface is Chinese whatever the system language.
	mygo.Use(updater.New(updater.Options{Language: "zh-Hans"}))

	var st *store.Store
	mygo.App.WhenReady(func() {
		// The packaged app's version comes from mygo.config.ts.
		if v := mygo.App.Version(); v != "" {
			service.Version = v
		}
		dir, err := mygo.App.Path(mygo.PathUserData)
		if err == nil {
			st, err = store.Open(context.Background(), dir)
		}
		if err != nil {
			// Without its data the app has nothing to show; say why
			// instead of vanishing.
			log.Print(err)
			mygo.Dialog.Error("无法打开数据", "数据未改动，请关闭其他 SportsMeet 窗口后重试。\n\n"+dir+"\n"+err.Error())
			mygo.App.Exit(1)
			return
		}
		core.Svc = service.New(st, bind.Notify)
		core.MCP = mcp.New(core.Svc)
		if err := core.MCP.Start(context.Background()); err != nil {
			log.Printf("MCP: %v", err)
		}

		mygo.NewWindow(mygo.WindowOptions{
			Title:           "SportsMeet",
			Width:           1280,
			Height:          800,
			MinWidth:        800,
			MinHeight:       560,
			BackgroundColor: "light-dark(#f5f6f8, #0f1115)", // --st-bg in src/design/tokens.css
			StateKey:        "main",
			URL:             "/",
		})
	})
	mygo.App.OnQuit(func() {
		if core.MCP != nil {
			_ = core.MCP.Close()
		}
		if st != nil {
			st.Close()
		}
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
