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

type UnionDao struct {
	repo *repo.Manager
}

func (d *UnionDao) FindUnionListByIds(ctx context.Context, unionIds []int64) ([]*entity.Union, error) {
	collection := d.repo.Mongo.Db.Collection("union")
	var list []*entity.Union
	cur, err := collection.Find(ctx, bson.D{
		{"unionID", bson.D{{"$in", unionIds}}},
	})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	for cur.Next(ctx) {
		var union entity.Union
		err := cur.Decode(&union)
		if err != nil {
			return nil, err
		}
		list = append(list, &union)
	}
	if err := cur.Err(); err != nil {
		return nil, err
	}
	return list, nil
}

func (d *UnionDao) Insert(ctx context.Context, union *entity.Union) (any, error) {
	collection := d.repo.Mongo.Db.Collection("union")
	res, err := collection.InsertOne(ctx, union)
	return res.InsertedID, err
}

func (d *UnionDao) FindUnionListByUId(ctx context.Context, uid string) (*entity.Union, error) {
	collection := d.repo.Mongo.Db.Collection("union")
	singleResult := collection.FindOne(ctx, bson.D{
		{"ownerUid", uid},
	})
	union := new(entity.Union)
	err := singleResult.Decode(union)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, err
	}
	return union, nil
}

func (d *UnionDao) FindUnionByUnionID(ctx context.Context, unionID int64) (*entity.Union, error) {
	collection := d.repo.Mongo.Db.Collection("union")
	singleResult := collection.FindOne(ctx, bson.D{
		{"unionID", unionID},
	})
	union := new(entity.Union)
	err := singleResult.Decode(union)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, err
	}
	return union, nil
}

func (d *UnionDao) FindAndUpdate(ctx context.Context, matchData bson.M, saveData bson.M) (*entity.Union, error) {
	collection := d.repo.Mongo.Db.Collection("union")
	var union entity.Union
	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)
	err := collection.FindOneAndUpdate(ctx, matchData, saveData, opts).Decode(&union)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			logs.Error("FindAndUpdate ErrNoDocuments err : %v", err)
			return nil, nil
		}
		logs.Error("FindAndUpdate err : %v", err)
		return nil, err
	}
	return &union, nil
}

func NewUnionDao(manager *repo.Manager) *UnionDao {
	return &UnionDao{repo: manager}
}
