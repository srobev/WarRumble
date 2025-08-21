package protocol

type SetAvatar struct {
	Avatar string `json:"avatar"` // filename only, e.g. "knight.png"
}
