package proto

import "core/models/entity"

type RoomCreator struct {
	Uid         string      `json:"uid"`
	CreatorType CreatorType `json:"creatorType"`
}

type CreatorType int

const (
	UserCreatorType  CreatorType = 1
	UnionCreatorType             = 2
)

type RoomUser struct {
	UserInfo   UserInfo   `json:"userInfo"`
	ChairID    int        `json:"chairID"`
	UserStatus UserStatus `json:"userStatus"`
	WinScore   int        `json:"winScore"`
}
type UserInfo struct {
	Uid          string `json:"uid"`
	Nickname     string `json:"nickname"`
	Avatar       string `json:"avatar"`
	Gold         int64  `json:"gold"`
	FrontendId   string `json:"frontendId"`
	Address      string `json:"address"`
	Location     string `json:"location"`
	LastLoginIP  string `json:"lastLoginIP"`
	Sex          int    `json:"sex"`
	Score        int    `json:"score"`
	SpreaderID   string `json:"spreaderID"` //推广ID
	ProhibitGame bool   `json:"prohibitGame"`
	RoomID       string `json:"roomID"`
}

type UserStatus int

const (
	None    UserStatus = 0
	Ready              = 1
	Playing            = 2
	Offline            = 4
	Dismiss            = 8
)

func ToRoomUser(data *entity.User, chairID int, connectorId string) *RoomUser {
	userInfo := UserInfo{
		Uid:        data.Uid,
		Nickname:   data.Nickname,
		Avatar:     data.Avatar,
		Gold:       data.Gold,
		Sex:        data.Sex,
		Address:    data.Address,
		FrontendId: connectorId,
	}
	return &RoomUser{
		UserInfo:   userInfo,
		ChairID:    chairID,
		UserStatus: None,
	}
}
