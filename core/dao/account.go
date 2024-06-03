package dao

import (
	"context"
	"core/models/entity"
	"core/repo"
	"errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type AccountDao struct {
	repo *repo.Manager
}

func (d *AccountDao) SaveAccount(ctx context.Context, ac *entity.Account) error {
	table := d.repo.Mongo.Db.Collection("account")
	_, err := table.InsertOne(ctx, ac)
	if err != nil {
		return err
	}
	return nil
}

func (d *AccountDao) FindAccountByAccount(ctx context.Context, account string) (*entity.Account, error) {
	table := d.repo.Mongo.Db.Collection("account")
	result := table.FindOne(ctx, bson.D{
		{"phoneAccount", account},
	})
	ac := new(entity.Account)
	err := result.Decode(ac)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, err
	}
	return ac, nil
}

func (d *AccountDao) FindAccountByPhone(ctx context.Context, phone string) (*entity.Account, error) {
	table := d.repo.Mongo.Db.Collection("account")
	result := table.FindOne(ctx, bson.D{
		{"phoneAccount", phone},
	})
	ac := new(entity.Account)
	err := result.Decode(ac)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, err
	}
	return ac, nil
}

func (d *AccountDao) UpdatePhone(ctx context.Context, uid string, phone string) error {
	db := d.repo.Mongo.Db.Collection("account")
	_, err := db.UpdateOne(ctx, bson.M{
		"uid": uid,
	}, bson.M{
		"$set": bson.M{
			"phoneAccount": phone,
		},
	})
	return err
}

func NewAccountDao(m *repo.Manager) *AccountDao {
	return &AccountDao{
		repo: m,
	}
}
