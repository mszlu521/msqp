package response

type UnionRecord struct {
	UnionID       int64  `json:"unionID"`
	UnionName     string `json:"unionName"`
	OwnerUid      string `json:"ownerUid"`
	OwnerAvatar   string `json:"ownerAvatar"`
	OwnerNickname string `json:"ownerNickname"`
	MemberCount   int32  `json:"memberCount"`
	OnlineCount   int32  `json:"onlineCount"`
}
type UnionListResp struct {
	RecordArr []UnionRecord `json:"recordArr"`
}
