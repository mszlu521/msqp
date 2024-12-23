package enums

type ScoreChangeType int

const (
	ScoreChangeNone    ScoreChangeType = iota
	Give                               // 被赠送积分
	ModifyLow                          // 修改下级分数
	ModifyUp                           // 被上级修改分数
	GameWin                            // 游戏赢分
	GameStartUnionChou                 // 游戏开始联盟抽分
	GameWinChou                        // 游戏赢家抽分
	SafeBox                            // 保险柜操作
)

// RoomRentPayType 房间支付模式
type RoomRentPayType int

const (
	BigWin RoomRentPayType = 1
	AA                     = 2
)

type RoomPayType int

const (
	AAZhiFu  RoomPayType = 1
	WinZhiFu             = 2
	MyPay                = 3
)

type RebateType string

const (
	One RebateType = "one"
	All            = "all"
)

type GameType int

const (
	SZ   GameType = iota + 1 // 拼三张
	NN                       // 牛牛
	PDK                      // 跑得快
	SG                       // 三公
	ZNMJ                     // 扎鸟麻将
	SY                       // 水鱼
	DGN  = 8                 // 斗公牛
)

// RoomDismissReason 房间解散原因
type RoomDismissReason int

const (
	DismissNone       RoomDismissReason = 0 //未知原因
	BureauFinished                      = 1 //完成所有局
	UserDismiss                         = 2 //用户解散
	UnionOwnerDismiss                   = 3 //盟主解散
)

type CreatorType int

const (
	UserCreatorType  CreatorType = 1
	UnionCreatorType             = 2
)

type UserStatus int

const (
	UserStatusNone UserStatus = 0
	Ready                     = 1
	Playing                   = 2
	Offline                   = 4
	Dismiss                   = 8
)
