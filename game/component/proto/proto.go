package proto

import (
	"core/models/enums"
	"framework/msError"
)

type GameRule struct {
	AddScores         []int          `json:"addScores"`      //加注分
	BaseScore         int            `json:"baseScore"`      //底分 sz hz
	Bureau            int            `json:"bureau"`         //局数 sz hz
	CanEnter          bool           `json:"canEnter"`       //中途进人 sz hz
	CanTrust          bool           `json:"canTrust"`       //允许托管 sz hz
	Chunniunai        bool           `json:"chunniunai"`     //是否允许搓牛  hz
	CanWatch          bool           `json:"canWatch"`       //允许观战
	Cuopai            bool           `json:"cuopai"`         //高级 是否允许搓牌
	GameFrameType     int            `json:"gameFrameType"`  //游戏模式  sz hz
	GameType          enums.GameType `json:"gameType"`       //游戏类型 牛牛 三公等  sz hz
	Ma                int            `json:"ma"`             //扎码 hz
	MaxPlayerCount    int            `json:"maxPlayerCount"` //最大人数 sz hz
	MinPlayerCount    int            `json:"minPlayerCount"` //最小人数 sz hz
	PayDiamond        int            `json:"payDiamond"`     //房费 sz hz
	PayType           int            `json:"payType"`        //支付方式 1 AA支付 2 赢家支付 3 我支付 sz hz
	Qidui             bool           `json:"qidui"`          //七对 一种胡牌方式 hz
	RoomType          int            `json:"roomType"`       // 1 正常房间 2 持续房间 3 百人房间 hz
	Yuyin             bool           `json:"yuyin"`          //语音 sz hz
	TrustTm           int            `json:"trustTm"`        //托管时长 hz
	Fangzuobi         bool           `json:"fangzuobi"`      //防作弊 sz
	MaxScore          int            `json:"maxScore"`       //最大加注分 sz
	RoundType         int            `json:"roundType"`      //轮数 sz
	ScoreLowLimit     int            `json:"scoreLowLimit"`  //最低分限制
	ScoreDismissLimit int            `json:"scoreDismissLimit"`
	RoomPayRule       RoomPayRule    `json:"roomPayRule"`
}

type RoomPayRule struct {
	PayMode              enums.RoomRentPayType `json:"payMode"`
	BigWinCount          int                   `json:"bigWinCount"`
	RebateType           enums.RebateType      `json:"rebateType"`
	IsAvg                bool                  `json:"isAvg"`
	EveryFixedScore      int                   `json:"everyFixedScore"`
	EveryAgentFixedScore int                   `json:"everyAgentFixedScore"`
	FixedScore           int                   `json:"fixedScore"`
	FixedMinWinScore     int                   `json:"fixedMinWinScore"`
	PercentScore         int                   `json:"percentScore"`
	PercentMinWinScore   int                   `json:"percentMinWinScore"`
}

type SendCardType int
type GameFrameType int
type ScaleType int
type RoomPayType int

const (
	AA    RoomPayType = 1
	Win               = 2
	MyPay             = 3
)

type RoomMessageType int

const (
	UserReadyNotify             RoomMessageType = 301 // 用户准备的通知
	UserReadyPush                               = 401 // 用户准备的推送
	UserLeaveRoomNotify                         = 303 // 用户离开房间的通知
	UserLeaveRoomResponse                       = 403 //用户离开房间的回复
	UserLeaveRoomPush                           = 404 //用户离开房间的推送
	OtherUserEntryRoomPush                      = 402 // 用户进入房间的推送
	RoomDismissPush                             = 405 //房间解散的推送
	UserInfoChangePush                          = 406 //房间用户信息变化的推送
	UserChatNotify                              = 307 // 用户聊天通知
	UserChatPush                                = 407 // 用户聊天推送
	UserOffLinePush                             = 408 //用户掉线的推送
	DrawFinishedPush                            = 409 //开设的房间局数用完推送
	UserReconnectNotify                         = 312 //玩家断线重连
	UserReconnectPush                           = 412 //
	AskForDismissNotify                         = 313 //玩家请求解散房间
	AskForDismissPush                           = 413 //
	EndPush                                     = 414 //最终结果推送
	AskForDismissStatusNotify                   = 316
	AskForDismissStatusPush                     = 416
	GetRoomShowUserInfoNotify                   = 317 // 获取房间需要显示的玩家信息通知
	GetRoomShowUserInfoPush                     = 417 // 获取房间需要显示的玩家信息推送
	GetRoomSceneInfoNotify                      = 318 // 获取房间场景信息的通知
	GetRoomSceneInfoPush                        = 418 // 获取房间场景信息的推送
	GetRoomOnlineUserInfoNotify                 = 319 // 获取房间在线用户信息的通知
	GetRoomOnlineUserInfoPush                   = 419
	UserChangeSeatNotify                        = 320 //换座通知
	UserChangeSeatPush                          = 420
)

const (
	ExitWaitSecond = 30
)

type EndData struct {
	Uid   string `json:"uid"`
	Score int    `json:"score"`
}

func RoomDismissPushData(reason enums.RoomDismissReason) any {
	pushMsg := map[string]any{
		"type": RoomDismissPush,
		"data": map[string]any{
			"reason": reason,
		},
		"pushRouter": "RoomMessagePush",
	}
	return pushMsg
}

func UserChatPushData(fromChairID int, toChairID int, content string) any {
	pushMsg := map[string]any{
		"type": UserChatPush,
		"data": map[string]any{
			"fromChairID": fromChairID,
			"toChairID":   toChairID,
			"msg":         content,
		},
		"pushRouter": "RoomMessagePush",
	}
	return pushMsg
}
func UserOffLinePushData(chairId int) any {
	pushMsg := map[string]any{
		"type": UserOffLinePush,
		"data": map[string]any{
			"chairID": chairId,
		},
		"pushRouter": "RoomMessagePush",
	}
	return pushMsg
}
func UpdateUserInfoPush(pushMsg map[string]any) any {
	//pushMsg := map[string]any{
	//	"roomID":     roomId,
	//	"pushRouter": "UpdateUserInfoPush",
	//}
	pushMsg["pushRouter"] = "UpdateUserInfoPush"
	return pushMsg
}
func UserLeaveRoomResponsePushData(chairId int) any {
	pushMsg := map[string]any{
		"type": UserLeaveRoomResponse,
		"data": map[string]any{
			"chairID": chairId,
		},
		"pushRouter": "RoomMessagePush",
	}
	return pushMsg
}
func UserLeaveRoomPushData(roomUserInfo *RoomUser) any {
	pushMsg := map[string]any{
		"type": UserLeaveRoomPush,
		"data": map[string]any{
			"roomUserInfo": roomUserInfo,
		},
		"pushRouter": "RoomMessagePush",
	}
	return pushMsg
}
func UserReadyPushData(chairID int) any {
	pushMsg := map[string]any{
		"type": UserReadyPush,
		"data": map[string]any{
			"chairID": chairID,
		},
		"pushRouter": "RoomMessagePush",
	}
	return pushMsg
}

func OtherUserEntryRoomPushData(roomUserInfo *RoomUser) any {
	pushMsg := map[string]any{
		"type": OtherUserEntryRoomPush,
		"data": map[string]any{
			"roomUserInfo": roomUserInfo,
		},
		"pushRouter": "RoomMessagePush",
	}
	return pushMsg
}

type DismissPushData struct {
	NameArr    []string `json:"nameArr"`
	ChairIDArr []any    `json:"chairIDArr"` //如果对方是第一次弹出解散框 any==nil
	AvatarArr  []string `json:"avatarArr"`
	OnlineArr  []bool   `json:"onlineArr"`
	AskChairId int      `json:"askChairId"`
	Tm         int      `json:"tm"`
	ScoreArr   []int    `json:"scoreArr"`
}

func AskForDismissPushData(data *DismissPushData) any {
	pushMsg := map[string]any{
		"type":       AskForDismissPush,
		"data":       data,
		"pushRouter": "RoomMessagePush",
	}
	return pushMsg
}
func GetUserChangeSeatPush(fromChairID, toChairID int, uid string) any {
	pushMsg := map[string]any{
		"type": UserChangeSeatPush,
		"data": map[string]any{
			"fromChairID": fromChairID,
			"toChairID":   toChairID,
			"uid":         uid,
		},
		"pushRouter": "RoomMessagePush",
	}
	return pushMsg
}

func PopDialogContentPushData(code *msError.Error) any {
	pushMsg := map[string]any{
		"code":       code.Code,
		"content":    code.Error(),
		"pushRouter": "PopDialogContentPush",
	}
	return pushMsg
}

func UserInfoChangePushData(userInfo *UserInfo) any {
	pushMsg := map[string]any{
		"type": UserInfoChangePush,
		"data": map[string]any{
			"changeInfo": userInfo,
		},
		"pushRouter": "RoomMessagePush",
	}
	return pushMsg
}
