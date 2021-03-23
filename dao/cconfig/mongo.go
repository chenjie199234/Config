package cconfig

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Summary struct {
	CurId  primitive.ObjectID   `bson:"cur_id"`
	AllIds []primitive.ObjectID `bson:"all_ids"`
	OpNum  int64                `bson:"op_num"`
}
type Config struct {
	AppConfig    string `bson:"app_config"`
	SourceConfig string `bson:"source_config"`
}

func (d *Dao) MongoGetInfo(ctx context.Context, groupname, appname string, op_num int64) (*Summary, *Config, error) {
	summary := &Summary{}
	if e := d.mongo.Database("c_"+groupname).Collection(appname).FindOne(ctx, bson.M{"_id": 0}).Decode(&summary); e != nil {
		return nil, nil, e
	}
	//version check:didn't change
	if summary.OpNum == op_num && op_num != 0 {
		return nil, nil, nil
	}
	config := &Config{}
	if e := d.mongo.Database("c_"+groupname).Collection(appname).FindOne(ctx, bson.M{"_id": summary.CurId}).Decode(config); e != nil {
		return nil, nil, e
	}
	return summary, config, nil
}

func (d *Dao) MongoGetConfig(ctx context.Context, groupname, appname, id string) (*Config, error) {
	objid, e := primitive.ObjectIDFromHex(id)
	if e != nil {
		return nil, e
	}
	config := &Config{}
	if e := d.mongo.Database("c_"+groupname).Collection(appname).FindOne(ctx, bson.M{"_id": objid}).Decode(config); e != nil {
		return nil, e
	}
	return config, nil
}

func (d *Dao) MongoSetConfig(ctx context.Context, groupname, appname, appconfig, sourceconfig string) error {
	s, e := d.mongo.StartSession()
	if e != nil {
		return e
	}
	defer s.EndSession(ctx)
	if e = d.mongo.UseSession(ctx, func(sctx mongo.SessionContext) (e error) {
		sctx.StartTransaction()
		defer func() {
			if e != nil {
				sctx.AbortTransaction(sctx)
			} else if ee := sctx.CommitTransaction(sctx); ee != nil {
				e = ee
			}
		}()
		var r *mongo.InsertOneResult
		col := sctx.Client().Database("c_" + groupname).Collection(appname)
		r, e = col.InsertOne(sctx, bson.M{"app_config": appconfig, "source_config": sourceconfig})
		if e != nil {
			return e
		}
		filter := bson.M{"_id": 0}
		update := bson.M{
			"$set":  bson.M{"_id": 0, "cur_id": r.InsertedID},
			"$push": bson.M{"all_ids": r.InsertedID},
			"$inc":  bson.M{"op_num": 1},
		}
		_, e = col.UpdateOne(sctx, filter, update, options.Update().SetUpsert(true))
		if e != nil {
			return e
		}
		return nil
	}); e != nil {
		return e
	}
	return nil
}

func (d *Dao) MongoRollbackConfig(ctx context.Context, groupname, appname, id string) error {
	objid, e := primitive.ObjectIDFromHex(id)
	if e != nil {
		return e
	}
	filter := bson.M{"_id": 0}
	update := bson.M{
		"$set": bson.M{"cur_id": objid},
		"$inc": bson.M{"op_num": 1},
	}
	if _, e = d.mongo.Database("c_"+groupname).Collection(appname).UpdateOne(ctx, filter, update); e != nil {
		return e
	}
	return nil
}

func (d *Dao) MongoGetGroups(ctx context.Context) ([]string, error) {
	result, e := d.mongo.ListDatabaseNames(ctx, bson.M{"name": bson.M{"$regex": "^c_"}})
	if e != nil {
		return nil, e
	}
	for i := range result {
		result[i] = result[i][2:]
	}
	return result, nil
}

func (d *Dao) MongoGetApps(ctx context.Context, groupname string) ([]string, error) {
	return d.mongo.Database("c_"+groupname).ListCollectionNames(ctx, bson.M{})
}
