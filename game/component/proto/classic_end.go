package proto

import "sort"

type ClassicEndResult struct {
	Uid      string `json:"uid"`
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
	Score    int    `json:"score"`
}

type ClassicEndCreator struct {
	Uid      string `json:"uid"`
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
}

// ClassicGameEndPushData matches the payload consumed by the legacy NN, SG,
// SZ and DGN result dialogs.
func ClassicGameEndPushData(messageType int, roomUsers map[string]*RoomUser, creatorInfo *RoomCreator, hongBaoList any, hasResult bool) any {
	ordered := make([]*RoomUser, 0, len(roomUsers))
	for _, user := range roomUsers {
		if user != nil && user.UserInfo != nil && user.ChairID >= 0 {
			ordered = append(ordered, user)
		}
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].ChairID < ordered[j].ChairID })

	result := make([]*ClassicEndResult, 0, len(ordered))
	var creator *ClassicEndCreator
	var winMost any
	var loseMost any
	if hasResult {
		for _, user := range ordered {
			item := &ClassicEndResult{
				Uid: user.UserInfo.Uid, Nickname: user.UserInfo.Nickname,
				Avatar: user.UserInfo.Avatar, Score: user.WinScore,
			}
			result = append(result, item)
			if creatorInfo != nil && item.Uid == creatorInfo.Uid {
				creator = &ClassicEndCreator{Uid: item.Uid, Nickname: item.Nickname, Avatar: item.Avatar}
			}
		}
		if len(result) > 0 {
			winner, loser := result[0], result[0]
			for _, item := range result[1:] {
				if item.Score > winner.Score {
					winner = item
				}
				if item.Score < loser.Score {
					loser = item
				}
			}
			winMost, loseMost = winner.Uid, loser.Uid
		}
	}

	return map[string]any{
		"type": messageType,
		"data": map[string]any{
			"result": result, "winMost": winMost, "loseMost": loseMost,
			"creater": creator, "hongBaoList": hongBaoList,
		},
		"pushRouter": "GameMessagePush",
	}
}
