package dao

import (
	"context"
	"core/models/entity"
	"core/repo"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
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
	err := db.FindOneAndUpdate(ctx, matchData, saveData).Decode(&user)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func NewUserDao(m *repo.Manager) *UserDao {
	return &UserDao{
		repo: m,
	}
}
