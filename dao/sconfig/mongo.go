package sconfig

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readconcern"
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
	if e := d.mongo.Database("s_"+groupname).Collection(appname).FindOne(ctx, bson.M{"_id": 0}).Decode(summary); e != nil {
		return nil, nil, e
	}
	//version check:didn't change
	if summary.OpNum == op_num && op_num != 0 {
		return nil, nil, nil
	}
	config := &Config{}
	if e := d.mongo.Database("s_"+groupname).Collection(appname).FindOne(ctx, bson.M{"_id": summary.CurId}).Decode(config); e != nil {
		return nil, nil, e
	}
	return summary, config, nil
}

func (d *Dao) MongoGetConfig(ctx context.Context, groupname, appname, id string) (*Config, error) {
	config := &Config{}
	objid, e := primitive.ObjectIDFromHex(id)
	if e != nil {
		return nil, e
	}
	if e := d.mongo.Database("s_"+groupname).Collection(appname).FindOne(ctx, bson.M{"_id": objid}).Decode(config); e != nil {
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
		col := sctx.Client().Database("s_" + groupname).Collection(appname)
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
	if _, e = d.mongo.Database("s_"+groupname).Collection(appname).UpdateOne(ctx, filter, update); e != nil {
		return e
	}
	return nil
}

func (d *Dao) MongoGetGroups(ctx context.Context) ([]string, error) {
	result, e := d.mongo.ListDatabaseNames(ctx, bson.M{"name": bson.M{"$regex": "^s_"}})
	if e != nil {
		return nil, e
	}
	for i := range result {
		result[i] = result[i][2:]
	}
	return result, nil
}

func (d *Dao) MongoGetApps(ctx context.Context, groupname string) ([]string, error) {
	return d.mongo.Database("s_"+groupname).ListCollectionNames(ctx, bson.M{})
}

type WatchAddr struct {
	Username       string   `bson:"username"`
	Passwd         string   `bson:"passwd"`
	Addrs          []string `bson:"addrs"`
	ReplicaSetName string   `bson:"replica_set_name"`
}

func (d *Dao) MongoDelWatchAddr(ctx context.Context) error {
	_, e := d.mongo.Database("s_default").Collection("config").DeleteOne(ctx, bson.M{"_id": "watchaddr"})
	return e
}
func (d *Dao) MongoSetWatchAddr(ctx context.Context, wa *WatchAddr) error {
	filter := bson.M{"_id": "watchaddr"}
	update := bson.M{
		"$set": bson.M{"username": wa.Username, "passwd": wa.Passwd, "addrs": wa.Addrs, "replica_set_name": wa.ReplicaSetName},
	}
	_, e := d.mongo.Database("s_default").Collection("config").UpdateOne(ctx, filter, update, options.Update().SetUpsert(true))
	return e
}
func (d *Dao) MongoGetWatchAddr(ctx context.Context) (*WatchAddr, error) {
	wa := &WatchAddr{}
	if e := d.mongo.Database("s_default").Collection("config").FindOne(ctx, bson.M{"_id": "watchaddr"}).Decode(wa); e != nil {
		return nil, e
	}
	return wa, nil
}
func (d *Dao) MongoWatch(groupname, appname string, update func(int64, *Config)) error {
	pipeline := mongo.Pipeline{bson.D{bson.E{Key: "$match", Value: bson.M{"documentKey._id": 0}}}}
	c, e := d.mongo.Database("s_"+groupname, options.Database().SetReadConcern(readconcern.Majority())).Collection(appname).Watch(context.Background(), pipeline)
	if e != nil {
		return e
	}
	defer c.Close(context.Background())
	summary, config, e := d.MongoGetInfo(context.Background(), groupname, appname, 0)
	if e == nil {
		update(summary.OpNum, config)
	} else if e == mongo.ErrNoDocuments {
		update(0, nil)
	} else {
		return e
	}
	for c.Next(context.Background()) {
		var curid primitive.ObjectID
		var opnum int64
		switch c.Current.Lookup("operationType").StringValue() {
		case "insert":
			curid = c.Current.Lookup("fullDocument").Document().Lookup("cur_id").ObjectID()
			opnum = c.Current.Lookup("fullDocument").Document().Lookup("op_num").AsInt64()
		case "update":
			curid = c.Current.Lookup("updateDescription").Document().Lookup("updatedFields").Document().Lookup("cur_id").ObjectID()
			opnum = c.Current.Lookup("updateDescription").Document().Lookup("updatedFields").Document().Lookup("op_num").AsInt64()
		case "delete":
			update(0, nil)
			return nil
		}
		config := &Config{}
		if e := d.mongo.Database("s_"+groupname).Collection(appname).FindOne(context.Background(), bson.M{"_id": curid}).Decode(config); e != nil {
			return e
		}
		update(opnum, config)
	}
	if c.Err() != nil {
		return c.Err()
	}
	return nil
}
