package main

import (
	"gofr.dev/pkg/gofr"

	"reports-app/handler"
	"reports-app/migrations"
	"reports-app/secondarydb"
	"reports-app/storage"
	"reports-app/store"
)

func main() {
	app := gofr.New()

	if err := secondarydb.Init(); err != nil {
		app.Logger().Fatalf("failed to connect to secondary db: %v", err)
	}

	if err := storage.Init(); err != nil {
		app.Logger().Fatalf("failed to init cloudinary storage: %v", err)
	}

	app.Migrate(migrations.All())

	h := handler.New(store.New())

	app.POST("/reports", h.Create)
	app.GET("/reports", h.List)
	app.GET("/reports/{id}", h.Get)

	app.Run()
}
