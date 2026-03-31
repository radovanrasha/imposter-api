package main

import (
	"imposter-api/routes"
	"imposter-api/ws"
	"os"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	// db.Init()
	// defer db.DB.Close()

	hub := ws.NewHub()

	r := gin.Default()
	r.Use(cors.Default())
	routes.Register(r, hub)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	r.Run(":" + port)
}
