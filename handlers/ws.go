package handlers

import (
	"encoding/json"
	"imposter-api/ws"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	gorillaws "github.com/gorilla/websocket"
)

var upgrader = gorillaws.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func HandleWS(hub *ws.Hub) gin.HandlerFunc {
	return func(c *gin.Context) {
		code := c.Param("code")
		playerID := c.Query("player_id")

		room, ok := hub.GetRoom(code)
		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"error": "room not found"})
			return
		}

		room.Lock()
		player, ok := room.Players[playerID]
		room.Unlock()
		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"error": "player not found"})
			return
		}

		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Println("ws upgrade error:", err)
			return
		}

		room.Lock()
		isReconnect := player.HasConnected && !player.Connected
		room.Unlock()

		if isReconnect {
			room.PlayerReconnected(playerID, conn)
		} else {
			room.Lock()
			player.Conn = conn
			player.Connected = true
			player.HasConnected = true
			players := room.PlayerList()
			room.Unlock()

			room.BroadcastMsg(ws.Message{
				Type:    ws.MsgPlayerJoined,
				Payload: ws.PlayerJoinedPayload{Players: players},
			})
		}

		defer func() {
			conn.Close()
			room.PlayerDisconnected(playerID, hub)
		}()

		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				break
			}

			var msg ws.Message
			if err := json.Unmarshal(data, &msg); err != nil {
				continue
			}

			room.Lock()
			switch msg.Type {
			case ws.MsgStartGame:
				if player.IsHost {
					raw, _ := json.Marshal(msg.Payload)
					var payload ws.StartGamePayload
					if err := json.Unmarshal(raw, &payload); err == nil {
						room.StartGame(payload.Word, payload.Hint, payload.ImpostersCount)
					}
				}
			case ws.MsgReady:
				room.MarkReady(playerID)
			case ws.MsgEndDiscussion:
				room.EndDiscussion(playerID)
			case ws.MsgVote:
				raw, _ := json.Marshal(msg.Payload)
				var payload ws.VotePayload
				if err := json.Unmarshal(raw, &payload); err == nil {
					room.CastVote(playerID, payload.VotedPlayerID)
				}
			case ws.MsgResetGame:
				room.ResetGame(playerID)
			}
			room.Unlock()
		}
	}
}
