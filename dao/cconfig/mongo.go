package cconfig

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func (d *Dao) MongoGetInfo(ctx context.Context, groupname, appname string) (string, string, []string, int64, error) {
	var summary struct {
		CurId  primitive.ObjectID   `bson:"cur_id"`
		AllIds []primitive.ObjectID `bson:"all_ids"`
		OpNum  int64                `bson:"op_num"`
	}
	if e := d.mongo.Database("c_"+groupname).Collection(appname).FindOne(ctx, bson.M{"_id": 0}).Decode(&summary); e != nil {
		return "", "", nil, 0, e
	}
	var infotemp struct {
		Data string `bson:"data"`
	}
	if e := d.mongo.Database("c_"+groupname).Collection(appname).FindOne(ctx, bson.M{"_id": summary.CurId}).Decode(&infotemp); e != nil {
		return "", "", nil, 0, e
	}
	temp := make([]string, 0, len(summary.AllIds))
	for _, id := range summary.AllIds {
		temp = append(temp, id.Hex())
	}
	return summary.CurId.Hex(), infotemp.Data, temp, summary.OpNum, nil
}

func (d *Dao) MongoGetConfig(ctx context.Context, groupname, appname, id string) (string, error) {
	var temp struct {
		Data string `bson:"data"`
	}
	objid, e := primitive.ObjectIDFromHex(id)
	if e != nil {
		return "", e
	}
	if e := d.mongo.Database("c_"+groupname).Collection(appname).FindOne(ctx, bson.M{"_id": objid}).Decode(&temp); e != nil {
		return "", e
	}
	return temp.Data, nil
}

func (d *Dao) MongoSetConfig(ctx context.Context, groupname, appname, config string) error {
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
		r, e = col.InsertOne(sctx, bson.M{"data": config})
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
