package room

import (
	"errors"
	"fmt"
	"math"

	"core/models/enums"
	"framework/remote"
	"game/component/base"
	"game/component/dgn"
	"game/component/mj"
	"game/component/nn"
	"game/component/pdk"
	"game/component/proto"
	"game/component/sg"
	"game/component/sz"
)

type GameFrame interface {
	GetEnterGameData(session *remote.Session) any
	GameMessageHandle(user *proto.RoomUser, session *remote.Session, msg []byte)
	IsUserEnableLeave(chairID int) bool
	OnEventUserOffLine(user *proto.RoomUser, session *remote.Session)
	OnEventUserEntry(user *proto.RoomUser, session *remote.Session)
	OnEventGameStart(user *proto.RoomUser, session *remote.Session)
	OnEventRoomDismiss(reason enums.RoomDismissReason, session *remote.Session)
	GetGameVideoData() any
	GetGameBureauData() any
}

func NewGameFrame(rule proto.GameRule, r base.RoomFrame, session *remote.Session) (GameFrame, error) {
	if err := validateGameRule(rule); err != nil {
		return nil, err
	}
	if err := validatePourRule(rule); err != nil {
		return nil, err
	}
	if rule.GameType == enums.SZ {
		return sz.NewGameFrame(rule, r, session), nil
	}
	if rule.GameType == enums.ZNMJ {
		return mj.NewGameFrame(rule, r, session), nil
	}
	if rule.GameType == enums.PDK {
		return pdk.NewGameFrame(rule, r, session), nil
	}
	if rule.GameType == enums.NN {
		return nn.NewGameFrame(rule, r, session), nil
	}
	if rule.GameType == enums.SG {
		return sg.NewGameFrame(rule, r, session), nil
	}
	if rule.GameType == enums.DGN {
		return dgn.NewGameFrame(rule, r, session), nil
	}
	return nil, errors.New("no gameType")
}

func validateGameRule(rule proto.GameRule) error {
	minPlayers, maxPlayers := playerCountRange(rule.GameType)
	if minPlayers > 0 {
		if rule.MaxPlayerCount > 0 && (rule.MaxPlayerCount < minPlayers || rule.MaxPlayerCount > maxPlayers) {
			return fmt.Errorf("invalid maxPlayerCount %d for gameType %d, want %d-%d", rule.MaxPlayerCount, rule.GameType, minPlayers, maxPlayers)
		}
		if rule.MinPlayerCount > 0 && (rule.MinPlayerCount < minPlayers || rule.MinPlayerCount > maxPlayers) {
			return fmt.Errorf("invalid minPlayerCount %d for gameType %d, want %d-%d", rule.MinPlayerCount, rule.GameType, minPlayers, maxPlayers)
		}
	}
	switch rule.GameType {
	case enums.SZ:
		if rule.GameFrameType < 0 || rule.GameFrameType > 3 {
			return fmt.Errorf("invalid gameFrameType %d for SZ", rule.GameFrameType)
		}
		for _, score := range rule.AddScores {
			if score <= 0 {
				return fmt.Errorf("invalid addScores value %d", score)
			}
		}
		if len(rule.AddScores) == 0 {
			return errors.New("addScores is required for SZ")
		}
	case enums.NN:
		if rule.GameFrameType != 0 && rule.GameFrameType != 1 && rule.GameFrameType != 3 && rule.GameFrameType != 5 && rule.GameFrameType != 6 {
			return fmt.Errorf("invalid gameFrameType %d for NN", rule.GameFrameType)
		}
	case enums.PDK:
		if rule.GameFrameType != 0 && (rule.GameFrameType < 1 || rule.GameFrameType > 3) {
			return fmt.Errorf("invalid gameFrameType %d for PDK", rule.GameFrameType)
		}
	case enums.SG:
		if rule.GameFrameType != 0 && rule.GameFrameType != 1 && rule.GameFrameType != 2 && rule.GameFrameType != 4 {
			return fmt.Errorf("unsupported gameFrameType %d for SG", rule.GameFrameType)
		}
	case enums.ZNMJ:
		if rule.GameFrameType != 0 && rule.GameFrameType != 1 && rule.GameFrameType != 2 {
			return fmt.Errorf("invalid gameFrameType %d for ZNMJ", rule.GameFrameType)
		}
	case enums.DGN:
		if rule.GameFrameType < 0 || rule.GameFrameType > 2 {
			return fmt.Errorf("invalid gameFrameType %d for DGN", rule.GameFrameType)
		}
	default:
		return fmt.Errorf("unsupported gameType %d", rule.GameType)
	}
	return nil
}

func playerCountRange(gameType enums.GameType) (int, int) {
	switch gameType {
	case enums.SZ:
		return 2, 6
	case enums.PDK:
		return 2, 3
	case enums.ZNMJ:
		return 2, 4
	case enums.NN, enums.SG, enums.DGN:
		return 2, 10
	default:
		return 0, 0
	}
}

func validatePourRule(rule proto.GameRule) error {
	if rule.GameType == enums.DGN {
		return validateDGNRule(rule)
	}
	if rule.GameType != enums.NN && rule.GameType != enums.SG {
		return nil
	}
	minimum := 0
	for _, score := range rule.CanPourScores {
		if score <= 0 {
			return fmt.Errorf("invalid canPourScores value %d", score)
		}
		if minimum == 0 || score < minimum {
			minimum = score
		}
	}
	if rule.GameType == enums.NN {
		for _, scale := range rule.TuiScale {
			if scale <= 0 {
				return fmt.Errorf("invalid tuiScale value %d", scale)
			}
		}
	}
	if rule.GameType == enums.SG && rule.MaxCanPourGold > 0 && minimum > 0 && rule.MaxCanPourGold < minimum {
		return fmt.Errorf("maxCanPourGold %d is less than minimum pour score %d", rule.MaxCanPourGold, minimum)
	}
	return nil
}

func validateDGNRule(rule proto.GameRule) error {
	if rule.Shouzhuang <= 0 {
		return fmt.Errorf("invalid shouzhuang %d", rule.Shouzhuang)
	}
	if rule.Xiazhuangfen <= 0 {
		return fmt.Errorf("invalid xiazhuangfen %d", rule.Xiazhuangfen)
	}
	if rule.LianzhuangType != 1 && rule.LianzhuangType != 2 {
		return fmt.Errorf("invalid lianzhuangType %d", rule.LianzhuangType)
	}
	if rule.LianzhuangCount != 1 && rule.LianzhuangCount != 2 {
		return fmt.Errorf("invalid lianzhuangCount %d", rule.LianzhuangCount)
	}
	maximum := rule.Shouzhuang / 3
	for _, value := range []struct {
		name string
		rate float64
	}{
		{name: "firstBureauRate", rate: rule.FirstBureauRate},
		{name: "bureauRate", rate: rule.BureauRate},
	} {
		if value.rate <= 0 || math.IsNaN(value.rate) || math.IsInf(value.rate, 0) {
			return fmt.Errorf("invalid %s %g", value.name, value.rate)
		}
		minimum := int(float64(rule.Shouzhuang) * value.rate)
		if minimum < 1 {
			minimum = 1
		}
		if minimum > maximum {
			return fmt.Errorf("%s minimum pour score %d exceeds maximum %d", value.name, minimum, maximum)
		}
	}
	return nil
}
