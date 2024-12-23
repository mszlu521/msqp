package dao

import (
	"common/logs"
	"context"
	"core/repo"
	"errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type CommonDao struct {
	repo *repo.Manager
}

func (d *CommonDao) GetStatisticsInfo(ctx context.Context, tableName string, matchData mongo.Pipeline, list any) error {
	db := d.repo.Mongo.Db.Collection(tableName)
	cursor, err := db.Aggregate(ctx, matchData)
	if err != nil {
		return err
	}
	defer cursor.Close(ctx)
	err = cursor.All(ctx, list)
	return err
}

func (d *CommonDao) FindOneData(ctx context.Context, tableName string, matchData bson.M, data any) error {
	err := d.repo.Mongo.Db.Collection(tableName).FindOne(ctx, matchData).Decode(data)
	if err != nil && errors.Is(err, mongo.ErrNoDocuments) {
		return nil
	}
	return err
}

func (d *CommonDao) FindDataAndCount(ctx context.Context, tableName string, startIndex int, count int, sortData bson.M, matchData bson.M, list any) (int64, error) {
	collection := d.repo.Mongo.Db.Collection(tableName)
	cursor, err := collection.Find(ctx,
		matchData,
		options.Find().SetSort(sortData).SetSkip(int64(startIndex)).SetLimit(int64(count)))
	if err != nil {
		logs.Error("FindDataAndCount err:%v", err)
		return 0, err
	}
	defer cursor.Close(ctx)
	err = cursor.All(ctx, list)
	total, err := collection.CountDocuments(ctx, matchData)
	if err != nil {
		logs.Error("CountDocuments error: %v", err)
		return 0, err
	}
	return total, err
}

func NewCommonDao(m *repo.Manager) *CommonDao {
	return &CommonDao{
		repo: m,
	}
}
