package protocol

// Guild-related protocol messages and structures

type GuildSummary struct {
	GuildID      string `json:"guildId"`
	Name         string `json:"name"`
	MembersCount int    `json:"membersCount"`
	Privacy      string `json:"privacy"` // public/private
	Region       string `json:"region"`
	Activity     int    `json:"activity"` // simple score for now
}

type GuildMember struct {
	PlayerID         int64  `json:"playerId"`
	Name             string `json:"name"`
	Role             string `json:"role"` // leader/officer/member
	LastOnlineMillis int64  `json:"lastOnline"`
	Contribution     int    `json:"contribution"`
	LeagueRank       string `json:"leagueRank"`
	Online           bool   `json:"online,omitempty"`
}

type GuildProfile struct {
	GuildID string        `json:"guildId"`
	Name    string        `json:"name"`
	Desc    string        `json:"desc"`
	Privacy string        `json:"privacy"`
	Region  string        `json:"region"`
	Leader  string        `json:"leader"`
	Members []GuildMember `json:"members"`
}

// C->S
type GetGuild struct{}
type CreateGuild struct{ Name, Desc, Privacy, Region string }
type JoinGuild struct{ GuildID string }
type LeaveGuild struct{}
type DisbandGuild struct{}
type ListGuilds struct{ Query string }
type GuildChatSend struct{ Text string }
type SetGuildDesc struct{ Desc string }

// Guild admin
type PromoteMember struct{ User string }
type DemoteMember struct{ User string }
type KickMember struct{ User string }
type TransferLeader struct{ To string }

// S->C
type GuildNone struct{}
type GuildInfo struct{ Profile GuildProfile }
type GuildList struct{ Items []GuildSummary }
type GuildChatMsg struct {
	From   string
	Text   string
	Ts     int64
	System bool
}
type GuildChatHistory struct{ Messages []GuildChatMsg }
