package proto

import (
	"core/models/entity"
	"core/models/enums"
)

type RoomCreator struct {
	Uid         string            `json:"uid"`
	UnionID     int64             `json:"unionID"`
	CreatorType enums.CreatorType `json:"creatorType"`
}

type RoomUser struct {
	UserInfo   *UserInfo        `json:"userInfo"`
	ChairID    int              `json:"chairID"`
	UserStatus enums.UserStatus `json:"userStatus"`
	WinScore   int              `json:"winScore"`
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
type UserRoomData struct {
	Uid        string `json:"uid"`
	Nickname   string `json:"nickname"`
	Avatar     string `json:"avatar"`
	Score      int    `json:"score"`
	SpreaderID string `json:"spreaderID"`
	WinScore   int    `json:"winScore"`
}

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
		UserInfo:   &userInfo,
		ChairID:    chairID,
		UserStatus: enums.UserStatusNone,
	}
}

func BuildGameRoomUserInfoWithUnion(data *entity.User, unionID int64, connectorId string) *UserInfo {
	userInfo := &UserInfo{
		Uid:         data.Uid,
		Nickname:    data.Nickname,
		Avatar:      data.Avatar,
		Gold:        data.Gold,
		Sex:         data.Sex,
		Address:     data.Address,
		FrontendId:  connectorId,
		Location:    data.Location,
		LastLoginIP: data.LastLoginIp,
	}
	if unionID == 1 {
		userInfo.Score = 0
		userInfo.SpreaderID = ""
	} else {
		if data.UnionInfo != nil {
			unionItem := data.GetUnionItem(unionID)
			if unionItem != nil {
				userInfo.Score = unionItem.Score
				userInfo.SpreaderID = unionItem.SpreaderID
				userInfo.ProhibitGame = unionItem.ProhibitGame
			} else {
				userInfo.Score = 0
				userInfo.SpreaderID = ""
				userInfo.ProhibitGame = false
			}
		}
	}
	return userInfo
}
