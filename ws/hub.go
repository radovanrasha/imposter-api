package ws

import (
	"context"
	"encoding/json"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Phase string

const (
	PhaseWaiting    Phase = "waiting"
	PhaseCardReveal Phase = "card_reveal"
	PhaseDiscussion Phase = "discussion"
	PhaseVoting     Phase = "voting"
	PhaseResult     Phase = "result"
)

const disconnectGrace = 30 * time.Second

type Player struct {
	ID              string
	Name            string
	IsHost          bool
	Conn            *websocket.Conn
	Role            string // "citizen" or "imposter"
	Word            string
	Hint            string
	Ready           bool
	Vote            string // voted player ID
	Connected       bool
	HasConnected    bool // true after first WS connect ever
	disconnectTimer *time.Timer
	mu              sync.Mutex
}

func (p *Player) Send(msg Message) {
	if p.Conn == nil {
		return
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Conn.WriteMessage(websocket.TextMessage, data) //nolint
}

type Room struct {
	Code               string
	Players            map[string]*Player
	Phase              Phase
	Word               string
	DiscussionTimeLeft int
	StartingPlayerName string
	LastGameResult     *GameResultPayload
	stopDiscussion     context.CancelFunc
	mu                 sync.Mutex
}

func (r *Room) Lock()   { r.mu.Lock() }
func (r *Room) Unlock() { r.mu.Unlock() }

func (r *Room) PlayerList() []PlayerInfo {
	list := make([]PlayerInfo, 0, len(r.Players))
	for _, p := range r.Players {
		list = append(list, PlayerInfo{ID: p.ID, Name: p.Name, IsHost: p.IsHost, Connected: p.Connected})
	}
	return list
}

// BroadcastMsg sends to all players. Must NOT be called while holding room.mu.
func (r *Room) BroadcastMsg(msg Message) {
	r.mu.Lock()
	players := make([]*Player, 0, len(r.Players))
	for _, p := range r.Players {
		players = append(players, p)
	}
	r.mu.Unlock()
	for _, p := range players {
		p.Send(msg)
	}
}

// broadcast sends to all players. Must be called while holding room.mu.
func (r *Room) broadcast(msg Message) {
	for _, p := range r.Players {
		p.Send(msg)
	}
}

func (r *Room) StartGame(word, hint string, impostersCount int) {
	r.Phase = PhaseCardReveal
	r.Word = word

	ids := make([]string, 0, len(r.Players))
	for id := range r.Players {
		ids = append(ids, id)
	}
	rand.Shuffle(len(ids), func(i, j int) { ids[i], ids[j] = ids[j], ids[i] })

	imposterSet := make(map[string]bool)
	for i := 0; i < impostersCount && i < len(ids); i++ {
		imposterSet[ids[i]] = true
	}

	for id, p := range r.Players {
		p.Ready = false
		p.Hint = hint
		if imposterSet[id] {
			p.Role = "imposter"
			p.Word = ""
			p.Send(Message{Type: MsgRoleAssigned, Payload: RoleAssignedPayload{
				Role: "imposter",
				Hint: hint,
			}})
		} else {
			p.Role = "citizen"
			p.Word = word
			p.Send(Message{Type: MsgRoleAssigned, Payload: RoleAssignedPayload{
				Role: "citizen",
				Word: word,
				Hint: hint,
			}})
		}
	}
}

func (r *Room) MarkReady(playerID string) {
	p, ok := r.Players[playerID]
	if !ok || r.Phase != PhaseCardReveal {
		return
	}
	p.Ready = true

	for _, pl := range r.Players {
		if !pl.Ready {
			return
		}
	}
	r.startDiscussion()
}

func (r *Room) startDiscussion() {
	r.Phase = PhaseDiscussion

	ids := make([]string, 0, len(r.Players))
	for id := range r.Players {
		ids = append(ids, id)
	}
	startName := r.Players[ids[rand.Intn(len(ids))]].Name
	r.StartingPlayerName = startName

	const duration = 300
	r.DiscussionTimeLeft = duration

	r.broadcast(Message{
		Type: MsgPhaseDiscussion,
		Payload: PhaseDiscussionPayload{
			StartingPlayerName: startName,
			Duration:           duration,
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	r.stopDiscussion = cancel

	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		timeLeft := duration
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				timeLeft--
				r.Lock()
				r.DiscussionTimeLeft = timeLeft
				r.Unlock()
				r.BroadcastMsg(Message{
					Type:    MsgDiscussionTimer,
					Payload: DiscussionTimerPayload{TimeLeft: timeLeft},
				})
				if timeLeft <= 0 {
					r.Lock()
					if r.Phase == PhaseDiscussion {
						r.startVoting()
					}
					r.Unlock()
					return
				}
			}
		}
	}()
}

func (r *Room) EndDiscussion(playerID string) {
	if p, ok := r.Players[playerID]; !ok || !p.IsHost {
		return
	}
	if r.Phase != PhaseDiscussion {
		return
	}
	if r.stopDiscussion != nil {
		r.stopDiscussion()
		r.stopDiscussion = nil
	}
	r.startVoting()
}

func (r *Room) startVoting() {
	if r.stopDiscussion != nil {
		r.stopDiscussion()
		r.stopDiscussion = nil
	}
	r.Phase = PhaseVoting
	for _, p := range r.Players {
		p.Vote = ""
		p.Ready = false
	}
	r.broadcast(Message{Type: MsgPhaseVoting})
}

func (r *Room) CastVote(voterID, votedID string) {
	voter, ok := r.Players[voterID]
	if !ok || r.Phase != PhaseVoting || voter.Vote != "" {
		return
	}
	voter.Vote = votedID

	votesCast := 0
	voteCounts := make(map[string]int)
	hasVoted := make([]string, 0)
	for _, p := range r.Players {
		if p.Vote != "" {
			votesCast++
			voteCounts[p.Vote]++
			hasVoted = append(hasVoted, p.ID)
		}
	}

	r.broadcast(Message{
		Type: MsgVoteUpdate,
		Payload: VoteUpdatePayload{
			VotesCast:    votesCast,
			TotalPlayers: len(r.Players),
			VoteCounts:   voteCounts,
			HasVoted:     hasVoted,
		},
	})

	if votesCast == len(r.Players) {
		r.determineResult()
	}
}

func (r *Room) determineResult() {
	r.Phase = PhaseResult

	voteCounts := make(map[string]int)
	for _, p := range r.Players {
		if p.Vote != "" {
			voteCounts[p.Vote]++
		}
	}

	maxVotes := -1
	votedOutID := ""
	tie := false
	for id, count := range voteCounts {
		if count > maxVotes {
			maxVotes = count
			votedOutID = id
			tie = false
		} else if count == maxVotes {
			tie = true
		}
	}

	votedOutName := "Niko (Izjednačeno)"
	wasImposter := false
	if !tie && votedOutID != "" {
		if p, ok := r.Players[votedOutID]; ok {
			votedOutName = p.Name
			wasImposter = p.Role == "imposter"
		}
	}

	imposters := make([]PlayerInfo, 0)
	for _, p := range r.Players {
		if p.Role == "imposter" {
			imposters = append(imposters, PlayerInfo{ID: p.ID, Name: p.Name, IsHost: p.IsHost, Connected: p.Connected})
		}
	}

	result := &GameResultPayload{
		VotedOutName: votedOutName,
		WasImposter:  wasImposter,
		Imposters:    imposters,
		Word:         r.Word,
	}
	r.LastGameResult = result
	r.broadcast(Message{Type: MsgGameResult, Payload: result})
}

func (r *Room) ResetGame(playerID string) {
	p, ok := r.Players[playerID]
	if !ok || !p.IsHost {
		return
	}
	if r.stopDiscussion != nil {
		r.stopDiscussion()
		r.stopDiscussion = nil
	}
	r.Phase = PhaseWaiting
	r.Word = ""
	r.StartingPlayerName = ""
	r.DiscussionTimeLeft = 0
	r.LastGameResult = nil
	for _, pl := range r.Players {
		pl.Role = ""
		pl.Word = ""
		pl.Hint = ""
		pl.Vote = ""
		pl.Ready = false
	}
	r.broadcast(Message{Type: MsgGameReset})
}

// checkPhaseAdvancement tries to move the game forward after a player is removed.
// Must be called while holding room.mu.
func (r *Room) checkPhaseAdvancement() {
	switch r.Phase {
	case PhaseCardReveal:
		for _, p := range r.Players {
			if !p.Ready {
				return
			}
		}
		r.startDiscussion()
	case PhaseVoting:
		votesCast := 0
		for _, p := range r.Players {
			if p.Vote != "" {
				votesCast++
			}
		}
		if len(r.Players) > 0 && votesCast == len(r.Players) {
			r.determineResult()
		}
	}
}

// buildReconnectState builds the reconnect payload for a player.
// Must be called while holding room.mu.
func (r *Room) buildReconnectState(playerID string) ReconnectStatePayload {
	p := r.Players[playerID]
	payload := ReconnectStatePayload{
		Phase:   r.Phase,
		Players: r.PlayerList(),
		Role:    p.Role,
		Word:    p.Word,
		Hint:    p.Hint,
	}
	switch r.Phase {
	case PhaseDiscussion:
		payload.StartingPlayerName = r.StartingPlayerName
		payload.DiscussionTimeLeft = r.DiscussionTimeLeft
	case PhaseVoting:
		votesCast := 0
		voteCounts := make(map[string]int)
		hasVoted := make([]string, 0)
		for _, pl := range r.Players {
			if pl.Vote != "" {
				votesCast++
				voteCounts[pl.Vote]++
				hasVoted = append(hasVoted, pl.ID)
			}
		}
		payload.VotesCast = votesCast
		payload.TotalPlayers = len(r.Players)
		payload.VoteCounts = voteCounts
		payload.HasVoted = hasVoted
		payload.MyVotedPlayerID = p.Vote
	case PhaseResult:
		payload.GameResult = r.LastGameResult
	}
	return payload
}

// PlayerDisconnected marks a player as disconnected and starts a grace period timer.
// Must NOT be called while holding room.mu.
func (r *Room) PlayerDisconnected(playerID string, hub *Hub) {
	r.Lock()
	p, ok := r.Players[playerID]
	if !ok {
		r.Unlock()
		return
	}
	if p.disconnectTimer != nil {
		p.disconnectTimer.Stop()
		p.disconnectTimer = nil
	}
	p.Connected = false
	p.Conn = nil
	players := r.PlayerList()
	r.Unlock()

	r.BroadcastMsg(Message{
		Type:    MsgPlayerLeft,
		Payload: PlayerLeftPayload{PlayerID: playerID, Players: players},
	})

	r.Lock()
	if _, exists := r.Players[playerID]; exists {
		r.Players[playerID].disconnectTimer = time.AfterFunc(disconnectGrace, func() {
			removed, remaining := hub.removeExpiredPlayer(r.Code, playerID)
			if !removed {
				return
			}
			r.Lock()
			r.checkPhaseAdvancement()
			r.Unlock()
			r.BroadcastMsg(Message{
				Type:    MsgPlayerLeft,
				Payload: PlayerLeftPayload{PlayerID: playerID, Players: remaining},
			})
		})
	}
	r.Unlock()
}

// PlayerReconnected restores a player's connection and sends them the current game state.
// Must NOT be called while holding room.mu.
func (r *Room) PlayerReconnected(playerID string, conn *websocket.Conn) {
	r.Lock()
	p, ok := r.Players[playerID]
	if !ok {
		r.Unlock()
		return
	}
	if p.disconnectTimer != nil {
		p.disconnectTimer.Stop()
		p.disconnectTimer = nil
	}
	p.Connected = true
	p.Conn = conn

	reconnectPayload := r.buildReconnectState(playerID)
	players := r.PlayerList()
	r.Unlock()

	p.Send(Message{Type: MsgReconnectState, Payload: reconnectPayload})

	r.BroadcastMsg(Message{
		Type:    MsgPlayerJoined,
		Payload: PlayerJoinedPayload{Players: players},
	})
}

// Hub manages all active rooms.
type Hub struct {
	rooms map[string]*Room
	mu    sync.Mutex
}

func NewHub() *Hub {
	return &Hub{rooms: make(map[string]*Room)}
}

func (h *Hub) CreateRoom(hostID, hostName string) *Room {
	h.mu.Lock()
	defer h.mu.Unlock()

	code := generateCode()
	for {
		if _, exists := h.rooms[code]; !exists {
			break
		}
		code = generateCode()
	}

	room := &Room{
		Code:    code,
		Players: map[string]*Player{hostID: {ID: hostID, Name: hostName, IsHost: true}},
		Phase:   PhaseWaiting,
	}
	h.rooms[code] = room
	return room
}

func (h *Hub) GetRoom(code string) (*Room, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	r, ok := h.rooms[strings.ToUpper(code)]
	return r, ok
}

func (h *Hub) AddPlayer(code, playerID, playerName string) (*Room, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	room, ok := h.rooms[strings.ToUpper(code)]
	if !ok {
		return nil, false
	}
	room.Players[playerID] = &Player{ID: playerID, Name: playerName}
	return room, true
}

func (h *Hub) RemovePlayer(code, playerID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	room, ok := h.rooms[code]
	if !ok {
		return
	}
	delete(room.Players, playerID)
	if len(room.Players) == 0 {
		delete(h.rooms, code)
	}
}

// removeExpiredPlayer removes a disconnected player after the grace period expires.
// Returns whether the player was removed and the updated player list.
func (h *Hub) removeExpiredPlayer(code, playerID string) (removed bool, players []PlayerInfo) {
	h.mu.Lock()
	defer h.mu.Unlock()

	room, ok := h.rooms[code]
	if !ok {
		return false, nil
	}
	p, exists := room.Players[playerID]
	if !exists || p.Connected {
		return false, nil // already reconnected or already removed
	}
	delete(room.Players, playerID)
	if len(room.Players) == 0 {
		delete(h.rooms, code)
		return false, nil // room gone, no broadcast needed
	}
	list := make([]PlayerInfo, 0, len(room.Players))
	for _, pl := range room.Players {
		list = append(list, PlayerInfo{ID: pl.ID, Name: pl.Name, IsHost: pl.IsHost, Connected: pl.Connected})
	}
	return true, list
}

func generateCode() string {
	const letters = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	b := make([]byte, 4)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
