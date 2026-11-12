package main

import (
	"context"
	"log"

	"github.com/egoist/mygo"
	"github.com/stmtc233/sportsmeet/internal/bind"
	"github.com/stmtc233/sportsmeet/internal/service"
	"github.com/stmtc233/sportsmeet/internal/store"
)

func main() {
	// Bindings are registered before Run; the service is filled in once
	// the data directory is known (mygo generate never reaches it).
	core := &bind.Core{}
	mygo.Bind(bind.Meets{Core: core}, bind.Units{Core: core}, bind.Athletes{Core: core}, bind.Events{Core: core},
		bind.Entries{Core: core}, bind.Arrange{Core: core}, bind.Results{Core: core}, bind.Reports{Core: core},
		&bind.Exchange{Core: core}, bind.Marks{}, bind.App{})

	var st *store.Store
	mygo.App.WhenReady(func() {
		dir, err := mygo.App.Path(mygo.PathUserData)
		if err != nil {
			log.Fatal(err)
		}
		st, err = store.Open(context.Background(), dir)
		if err != nil {
			log.Fatal(err)
		}
		core.Svc = service.New(st, bind.Notify)

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
		if st != nil {
			st.Close()
		}
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
