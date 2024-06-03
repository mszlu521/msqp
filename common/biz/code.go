package biz

import (
	"errors"
	"framework/msError"
)

const OK = 0

var (
	Fail                        = msError.NewError(1, errors.New("请求失败"))
	RequestDataError            = msError.NewError(2, errors.New("请求数据错误"))
	SqlError                    = msError.NewError(3, errors.New("数据库操作错误"))
	InvalidUsers                = msError.NewError(4, errors.New("无效用户"))
	PermissionNotEnough         = msError.NewError(6, errors.New("权限不足"))
	SmsCodeError                = msError.NewError(7, errors.New("短信验证码错误"))
	ImgCodeError                = msError.NewError(8, errors.New("图形验证码错误")) // 图形验证码错误
	SmsSendFailed               = msError.NewError(9, errors.New("短信发送失败"))
	ServerMaintenance           = msError.NewError(10, errors.New("服务器维护"))
	NotEnoughGold               = msError.NewError(11, errors.New("钻石不足"))
	UserDataLocked              = msError.NewError(12, errors.New("用户数据被锁定"))
	NotEnoughScore              = msError.NewError(13, errors.New("积分不足"))
	AccountOrPasswordError      = msError.NewError(101, errors.New("账号或密码错误"))
	GetHallServersFail          = msError.NewError(102, errors.New("获取大厅服务器失败"))
	AccountExist                = msError.NewError(103, errors.New("账号已存在"))
	AccountNotExist             = msError.NewError(104, errors.New("帐号不存在"))
	NotFindBindPhone            = msError.NewError(105, errors.New("该手机号未绑定"))
	PhoneAlreadyBind            = msError.NewError(106, errors.New("该手机号已被绑定，无法重复绑定"))
	NotFindUser                 = msError.NewError(107, errors.New("用户不存在"))
	TokenInfoError              = msError.NewError(201, errors.New("无效的token"))
	NotEnoughVipLevel           = msError.NewError(202, errors.New("vip等级不足"))
	BlockedAccount              = msError.NewError(203, errors.New("帐号已冻结"))
	AlreadyCreatedUnion         = msError.NewError(204, errors.New("已经创建过牌友圈，无法重复创建"))
	UnionNotExist               = msError.NewError(205, errors.New("联盟不存在"))
	UserInRoomDataLocked        = msError.NewError(206, errors.New("用户在房间中，无法操作数据"))
	NotInUnion                  = msError.NewError(207, errors.New("用户不在联盟中"))
	AlreadyInUnion              = msError.NewError(208, errors.New("用户已经在联盟中"))
	InviteIdError               = msError.NewError(209, errors.New("邀请码错误"))
	NotYourMember               = msError.NewError(210, errors.New("添加的用户不是你的下级成员"))
	ForbidGiveScore             = msError.NewError(211, errors.New("禁止赠送积分"))
	ForbidInviteScore           = msError.NewError(212, errors.New("禁止玩家或代理邀请玩家"))
	CanNotCreateNewHongBao      = msError.NewError(213, errors.New("暂时无法分发新的红包"))
	CanNotLeaveRoom             = msError.NewError(305, errors.New("正在游戏中无法离开房间"))
	RoomCountReachLimit         = msError.NewError(301, errors.New("房间数量到达上线"))
	LeaveRoomGoldNotEnoughLimit = msError.NewError(302, errors.New("金币不足，无法开始游戏"))
	LeaveRoomGoldExceedLimit    = msError.NewError(303, errors.New("金币超过最大限度，无法开始游戏"))
	NotInRoom                   = msError.NewError(306, errors.New("不在该房间中"))
	RoomPlayerCountFull         = msError.NewError(307, errors.New("房间玩家已满"))
	RoomNotExist                = msError.NewError(308, errors.New("房间不存在"))
	CanNotEnterNotLocation      = msError.NewError(309, errors.New("无法进入房间，获取定位信息失败"))
	CanNotEnterTooNear          = msError.NewError(310, errors.New("无法进入房间，与房间中的其他玩家太近"))
)
