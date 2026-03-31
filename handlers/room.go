package handlers

import (
	"imposter-api/ws"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func CreateRoom(hub *ws.Hub) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			HostName string `json:"host_name" binding:"required"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		playerID := uuid.NewString()
		room := hub.CreateRoom(playerID, body.HostName)

		c.JSON(http.StatusCreated, gin.H{
			"room_code": room.Code,
			"player_id": playerID,
		})
	}
}

func JoinRoom(hub *ws.Hub) gin.HandlerFunc {
	return func(c *gin.Context) {
		code := c.Param("code")

		var body struct {
			Name string `json:"name" binding:"required"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		playerID := uuid.NewString()
		_, ok := hub.AddPlayer(code, playerID, body.Name)
		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"error": "room not found"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"player_id": playerID})
	}
}
