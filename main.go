package main

import (
	"imposter-api/db"
	"imposter-api/routes"
	"os"

	"github.com/gin-gonic/gin"
)

func main() {
	db.Init()
	defer db.DB.Close()

	r := gin.Default()
	routes.Register(r)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	r.Run(":" + port)
}
