package routes

import (
	"imposter-api/handlers"
	"imposter-api/ws"
	"net/http"

	"github.com/gin-gonic/gin"
)

func Register(r *gin.Engine, hub *ws.Hub) {
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "pong"})
	})

	r.POST("/reviews", handlers.CreateReview)

	// Online game
	r.POST("/rooms", handlers.CreateRoom(hub))
	r.POST("/rooms/:code/join", handlers.JoinRoom(hub))
	r.GET("/ws/:code", handlers.HandleWS(hub))
}
