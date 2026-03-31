package ws

// Client → Server
const (
	MsgStartGame     = "start_game"
	MsgReady         = "ready"
	MsgEndDiscussion = "end_discussion"
	MsgVote          = "vote"
	MsgResetGame     = "reset_game"
)

// Server → Client
const (
	MsgPlayerJoined    = "player_joined"
	MsgPlayerLeft      = "player_left"
	MsgRoleAssigned    = "role_assigned"
	MsgPhaseDiscussion = "phase_discussion"
	MsgDiscussionTimer = "discussion_timer"
	MsgPhaseVoting     = "phase_voting"
	MsgVoteUpdate      = "vote_update"
	MsgGameResult      = "game_result"
	MsgReconnectState  = "reconnect_state"
	MsgGameReset       = "game_reset"
	MsgError           = "error"
)

type Message struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload,omitempty"`
}

// Incoming payloads
type StartGamePayload struct {
	Word           string `json:"word"`
	Hint           string `json:"hint"`
	ImpostersCount int    `json:"imposters_count"`
}

type VotePayload struct {
	VotedPlayerID string `json:"voted_player_id"`
}

// Outgoing payloads
type PlayerInfo struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	IsHost    bool   `json:"is_host"`
	Connected bool   `json:"connected"`
}

type PlayerJoinedPayload struct {
	Players []PlayerInfo `json:"players"`
}

type PlayerLeftPayload struct {
	PlayerID string       `json:"player_id"`
	Players  []PlayerInfo `json:"players"`
}

type RoleAssignedPayload struct {
	Role string `json:"role"` // "citizen" or "imposter"
	Word string `json:"word"` // empty for imposter
	Hint string `json:"hint"`
}

type PhaseDiscussionPayload struct {
	StartingPlayerName string `json:"starting_player_name"`
	Duration           int    `json:"duration"`
}

type DiscussionTimerPayload struct {
	TimeLeft int `json:"time_left"`
}

type VoteUpdatePayload struct {
	VotesCast    int            `json:"votes_cast"`
	TotalPlayers int            `json:"total_players"`
	VoteCounts   map[string]int `json:"vote_counts"`
	HasVoted     []string       `json:"has_voted"` // IDs of players who have cast a vote
}

type GameResultPayload struct {
	VotedOutName string       `json:"voted_out_name"`
	WasImposter  bool         `json:"was_imposter"`
	Imposters    []PlayerInfo `json:"imposters"`
	Word         string       `json:"word"`
}

type ReconnectStatePayload struct {
	Phase              Phase              `json:"phase"`
	Players            []PlayerInfo       `json:"players"`
	Role               string             `json:"role,omitempty"`
	Word               string             `json:"word,omitempty"`
	Hint               string             `json:"hint,omitempty"`
	StartingPlayerName string             `json:"starting_player_name,omitempty"`
	DiscussionTimeLeft int                `json:"discussion_time_left,omitempty"`
	VotesCast          int                `json:"votes_cast,omitempty"`
	TotalPlayers       int                `json:"total_players,omitempty"`
	VoteCounts         map[string]int     `json:"vote_counts,omitempty"`
	HasVoted           []string           `json:"has_voted,omitempty"`
	MyVotedPlayerID    string             `json:"my_voted_player_id,omitempty"`
	GameResult         *GameResultPayload `json:"game_result,omitempty"`
}
