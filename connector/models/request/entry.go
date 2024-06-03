package request

//{
//token: 'e26ad95f109145af94ee0a2e4815ca5e755f897891ed9ec421942ec974c76fb538e47ef41a7ebd32fef68bbc5d9e215a',
//userInfo: { nickname: '码神710065', avatar: 'Common/head_icon_default', sex: 1 }
//}

type EntryReq struct {
	Token    string   `json:"token"`
	UserInfo UserInfo `json:"userInfo"`
}

type UserInfo struct {
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
	Sex      int    `json:"sex"`
}
