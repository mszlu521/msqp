package stream

type PushUser struct {
	Uid         string `json:"uid"`
	ConnectorId string `json:"connectorId"`
}
type PushData struct {
	Data   []byte `json:"data"`
	Router string `json:"router"`
}

type PushMessage struct {
	PushData PushData   `json:"pushData"`
	Users    []PushUser `json:"users"`
	Msg      *Msg       `json:"msg"`
}
