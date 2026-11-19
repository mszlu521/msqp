package handler

import (
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// Both rank dialogs consume flat records, rather than User.UnionInfo or
// UserGameRecord.UserList. Keep the aggregate result in that same shape.
type unionRankRecord struct {
	Uid          string `bson:"uid" json:"uid"`
	Nickname     string `bson:"nickname" json:"nickname"`
	Avatar       string `bson:"avatar" json:"avatar"`
	Score        int64  `bson:"score" json:"score"`
	YesterdayWin int64  `bson:"yesterdayWin" json:"yesterdayWin"`
	TodayDraw    int64  `bson:"todayDraw" json:"todayDraw"`
	WeekDraw     int64  `bson:"weekDraw" json:"weekDraw"`
	TotalDraw    int64  `bson:"totalDraw" json:"totalDraw"`
}

type unionRankPage struct {
	Records []unionRankRecord `bson:"records"`
	Totals  []struct {
		Count int64 `bson:"count"`
	} `bson:"totals"`
}

func unionRankPayload(pages []unionRankPage) map[string]any {
	records := []unionRankRecord{}
	var total int64
	if len(pages) > 0 {
		if pages[0].Records != nil {
			records = pages[0].Records
		}
		if len(pages[0].Totals) > 0 {
			total = pages[0].Totals[0].Count
		}
	}
	return map[string]any{"recordArr": records, "totalCount": total}
}

func unionRankPagination(sort bson.M, start, count int, projection bson.M) bson.D {
	return bson.D{{Key: "$facet", Value: bson.M{
		"records": mongo.Pipeline{
			bson.D{{Key: "$sort", Value: sort}},
			bson.D{{Key: "$skip", Value: start}},
			bson.D{{Key: "$limit", Value: count}},
			bson.D{{Key: "$project", Value: projection}},
		},
		"totals": mongo.Pipeline{bson.D{{Key: "$count", Value: "count"}}},
	}}}
}

func unionRankPipeline(unionID int64, match, sort bson.M, start, count int) mongo.Pipeline {
	if len(sort) == 0 {
		sort = bson.M{"unionInfo.todayDraw": -1}
	}
	return mongo.Pipeline{
		bson.D{{Key: "$match", Value: mergeBsonMatch(match, bson.M{"unionInfo.unionID": unionID})}},
		bson.D{{Key: "$unwind", Value: "$unionInfo"}},
		bson.D{{Key: "$match", Value: bson.M{"unionInfo.unionID": unionID}}},
		unionRankPagination(sort, start, count, bson.M{
			"_id": 0, "uid": 1, "nickname": 1, "avatar": 1,
			"score": "$unionInfo.score", "yesterdayWin": "$unionInfo.yesterdayWin",
			"todayDraw": "$unionInfo.todayDraw", "weekDraw": "$unionInfo.weekDraw", "totalDraw": "$unionInfo.totalDraw",
		}),
	}
}

func unionSingleRankPipeline(unionID, todayStart int64, match, sort bson.M, start, count int) mongo.Pipeline {
	if len(sort) == 0 {
		sort = bson.M{"score": -1}
	}
	rankMatch := mergeBsonMatch(match, bson.M{"unionID": unionID, "createTime": bson.M{"$gte": todayStart}})
	op := "$max"
	if direction := sort["score"]; direction == 1 || direction == float64(1) || direction == int32(1) || direction == int64(1) {
		op = "$min"
	}
	return mongo.Pipeline{
		bson.D{{Key: "$match", Value: rankMatch}},
		bson.D{{Key: "$unwind", Value: "$userList"}},
		bson.D{{Key: "$match", Value: rankMatch}},
		bson.D{{Key: "$group", Value: bson.M{
			"_id":      "$userList.uid",
			"nickname": bson.M{"$first": "$userList.nickname"},
			"avatar":   bson.M{"$first": "$userList.avatar"},
			"score":    bson.M{op: "$userList.score"},
		}}},
		unionRankPagination(sort, start, count, bson.M{"_id": 0, "uid": "$_id", "nickname": 1, "avatar": 1, "score": 1}),
	}
}
