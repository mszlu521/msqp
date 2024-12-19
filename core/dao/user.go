package dao

import (
	"common/logs"
	"context"
	"core/models/entity"
	"core/repo"
	"errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type UserDao struct {
	repo *repo.Manager
}

func (d *UserDao) FindUserByUid(ctx context.Context, uid string) (*entity.User, error) {
	db := d.repo.Mongo.Db.Collection("user")
	singleResult := db.FindOne(ctx, bson.D{
		{"uid", uid},
	})
	user := new(entity.User)
	err := singleResult.Decode(user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return user, nil
}

func (d *UserDao) Insert(ctx context.Context, user *entity.User) error {
	db := d.repo.Mongo.Db.Collection("user")
	_, err := db.InsertOne(ctx, user)
	return err
}

func (d *UserDao) UpdateUserAddressByUid(ctx context.Context, user *entity.User) error {
	db := d.repo.Mongo.Db.Collection("user")
	_, err := db.UpdateOne(ctx, bson.M{
		"uid": user.Uid,
	}, bson.M{
		"$set": bson.M{
			"address":  user.Address,
			"location": user.Location,
		},
	})
	return err
}

func (d *UserDao) UpdatePhone(ctx context.Context, uid string, phone string) error {
	db := d.repo.Mongo.Db.Collection("user")
	_, err := db.UpdateOne(ctx, bson.M{
		"uid": uid,
	}, bson.M{
		"$set": bson.M{
			"mobilePhone": phone,
		},
	})
	return err
}

func (d *UserDao) UpdateUserRealName(ctx context.Context, uid string, info string) error {
	db := d.repo.Mongo.Db.Collection("user")
	_, err := db.UpdateOne(ctx, bson.M{
		"uid": uid,
	}, bson.M{
		"$set": bson.M{
			"realName": info,
		},
	})
	return err
}

func (d *UserDao) FindUserByPhone(ctx context.Context, phone string) (*entity.User, error) {
	db := d.repo.Mongo.Db.Collection("user")
	singleResult := db.FindOne(ctx, bson.D{
		{"mobilePhone", phone},
	})
	user := new(entity.User)
	err := singleResult.Decode(user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return user, nil
}

func (d *UserDao) UpdateUserRoomId(ctx context.Context, uid string, roomId string) error {
	db := d.repo.Mongo.Db.Collection("user")
	_, err := db.UpdateOne(ctx, bson.M{
		"uid": uid,
	}, bson.M{
		"$set": bson.M{
			"roomID": roomId,
		},
	})
	return err
}

func (d *UserDao) FindAndUpdate(ctx context.Context, matchData bson.M, saveData bson.M) (*entity.User, error) {
	db := d.repo.Mongo.Db.Collection("user")
	var user entity.User
	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)
	err := db.FindOneAndUpdate(ctx, matchData, saveData, opts).Decode(&user)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (d *UserDao) FindUserByInviteID(ctx context.Context, inviteID int64) (*entity.User, error) {
	db := d.repo.Mongo.Db.Collection("user")
	var user entity.User
	singleResult := db.FindOne(ctx, bson.D{
		{"unionInfo.inviteID", inviteID},
	})
	err := singleResult.Decode(&user)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, err
	}
	return &user, err
}

func (d *UserDao) FindUserPage(ctx context.Context, index int, count int, sortData bson.M, matchData bson.M) (list []*entity.User, total int64, err error) {
	db := d.repo.Mongo.Db.Collection("user")
	pipeline := mongo.Pipeline{
		{{"$match", matchData}},    // 匹配条件
		{{"$sort", sortData}},      // 排序
		{{"$skip", int64(index)}},  // 跳过的文档数量
		{{"$limit", int64(count)}}, // 返回的文档数量
	}
	cursor, err := db.Aggregate(ctx, pipeline)
	//cursor, err := db.Find(ctx, matchData, options.Find().SetSort(sortData).SetSkip(int64(index)).SetLimit(int64(count)))
	if err != nil {
		logs.Error("FindUserPage err:%v", err)
		if errors.Is(err, mongo.ErrNoDocuments) {
			return []*entity.User{}, 0, nil
		}
		return nil, 0, err
	}
	defer cursor.Close(ctx)
	for cursor.Next(ctx) {
		var user entity.User
		err := cursor.Decode(&user)
		if err != nil {
			return nil, 0, err
		}
		list = append(list, &user)
	}
	total, err = db.CountDocuments(ctx, matchData)
	return list, total, err
}

func (d *UserDao) FindUserByMatchData(ctx context.Context, matchData bson.M) (*entity.User, error) {
	db := d.repo.Mongo.Db.Collection("user")
	var user entity.User
	singleResult := db.FindOne(ctx, matchData)
	err := singleResult.Decode(&user)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

func (d *UserDao) UpdateAllData(ctx context.Context, matchData bson.M, saveData bson.M) error {
	db := d.repo.Mongo.Db.Collection("user")
	_, err := db.UpdateMany(ctx, matchData, saveData)
	return err
}

func (d *UserDao) FindUserAggregateByMatchData(ctx context.Context, matchData mongo.Pipeline) ([]*entity.StatisticsResult, error) {
	db := d.repo.Mongo.Db.Collection("user")
	var list []*entity.StatisticsResult
	cursor, err := db.Aggregate(ctx, matchData)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	err = cursor.All(ctx, &list)
	return list, err
}

func (d *UserDao) GetUnlockUserDataAndLock(ctx context.Context, uid string) (*entity.User, error) {
	db := d.repo.Mongo.Db.Collection("user")
	//先检查是否有syncLock字段 没有则添加
	var result bson.M
	err := db.FindOne(ctx, bson.M{"uid": uid}).Decode(&result)
	if err != nil {
		return nil, err
	} else {
		if _, ok := result["syncLock"]; !ok {
			_, err = db.UpdateOne(ctx, bson.M{"uid": uid}, bson.M{"$set": bson.M{"syncLock": 0}})
			if err != nil {
				return nil, err
			}
		}
	}
	var user entity.User
	err = db.FindOneAndUpdate(ctx,
		bson.M{
			"uid":      uid,
			"syncLock": 0,
		}, bson.M{"$set": bson.M{"syncLock": 1}}).Decode(&user)
	if err != nil {
		logs.Error("GetUnlockUserDataAndLock err:%v", err)
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

func (d *UserDao) UnLockUserData(ctx context.Context, uid string) error {
	db := d.repo.Mongo.Db.Collection("user")
	_, err := db.UpdateOne(ctx, bson.M{
		"uid": uid,
	}, bson.M{"$set": bson.M{"syncLock": 0}})
	return err
}

func NewUserDao(m *repo.Manager) *UserDao {
	return &UserDao{
		repo: m,
	}
}
