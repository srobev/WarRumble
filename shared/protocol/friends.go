package protocol

// Friends / Direct Messages

type FriendInfo struct {
    Name   string `json:"name"`
    Online bool   `json:"online"`
}

// C->S
type GetFriends struct{}
type AddFriend struct{ Name string }
type RemoveFriend struct{ Name string }
type SendFriendDM struct{ To, Text string }
type GetFriendHistory struct{ With string; Limit int }

// S->C
type FriendsList struct{ Items []FriendInfo }
type FriendDM struct{ From, To, Text string; Ts int64 }
type FriendHistory struct{ With string; Items []FriendDM }
