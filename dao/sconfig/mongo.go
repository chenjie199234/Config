package sconfig

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readconcern"
)

//doesn't support sharding
//index has a unique key
//summary's index is 0
//config's index start from 1
type Summary struct {
	Index    uint64 `bson:"index"`
	CurIndex uint64 `bson:"cur_index"`
	MaxIndex uint64 `bson:"max_index"`
	OpNum    uint64 `bson:"op_num"`
}
type Config struct {
	Index        uint64 `bson:"index"`
	AppConfig    string `bson:"app_config"`
	SourceConfig string `bson:"source_config"`
}

func (d *Dao) MongoGetInfo(ctx context.Context, groupname, appname string) (*Summary, *Config, error) {
	summary := &Summary{}
	if e := d.mongo.Database("s_"+groupname).Collection(appname).FindOne(ctx, bson.M{"index": 0}).Decode(summary); e != nil {
		return nil, nil, e
	}
	config := &Config{}
	if e := d.mongo.Database("s_"+groupname).Collection(appname).FindOne(ctx, bson.M{"index": summary.CurIndex}).Decode(config); e != nil {
		return nil, nil, e
	}
	return summary, config, nil
}

func (d *Dao) MongoGetConfig(ctx context.Context, groupname, appname string, index uint64) (*Config, error) {
	config := &Config{}
	if e := d.mongo.Database("s_"+groupname).Collection(appname).FindOne(ctx, bson.M{"index": index}).Decode(config); e != nil {
		return nil, e
	}
	return config, nil
}

func (d *Dao) MongoSetConfig(ctx context.Context, groupname, appname, appconfig, sourceconfig string) (e error) {
	var s mongo.Session
	if s, e = d.mongo.StartSession(); e != nil {
		return
	}
	sctx := mongo.NewSessionContext(ctx, s)
	defer s.EndSession(sctx)
	if e = s.StartTransaction(); e != nil {
		return
	}
	defer func() {
		if e != nil {
			s.AbortTransaction(sctx)
		} else if e = s.CommitTransaction(sctx); e != nil {
			s.AbortTransaction(sctx)
		}
	}()
	filter := bson.M{"index": 0}
	update := bson.A{
		bson.M{"$inc": bson.M{"op_num": 1, "max_index:": 1}},
		bson.M{"$set": bson.M{"cur_index": "$max_index"}},
	}
	summary := &Summary{}
	r := d.mongo.Database("s_"+groupname).Collection(appname).FindOneAndUpdate(sctx, filter, update, options.FindOneAndUpdate().SetUpsert(true))
	if r.Err() != nil && r.Err() != mongo.ErrNoDocuments {
		e = r.Err()
		return
	} else if r.Err() == nil {
		if e = r.Decode(summary); e != nil {
			return
		}
	}
	_, e = d.mongo.Database("s_"+groupname).Collection(appname).UpdateOne(sctx, bson.M{"index": summary.CurIndex + 1}, bson.M{"app_config": appconfig, "source_config": sourceconfig}, options.Update().SetUpsert(true))
	return
}

func (d *Dao) MongoRollbackConfig(ctx context.Context, groupname, appname string, index uint64) error {
	filter := bson.M{"index": 0}
	update := bson.M{
		"$set": bson.M{"cur_index": index},
		"$inc": bson.M{"op_num": 1},
	}
	if _, e := d.mongo.Database("s_"+groupname).Collection(appname).UpdateOne(ctx, filter, update); e != nil {
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

func (d *Dao) MongoWatch(groupname, appname string, update func(*Config)) error {
	curop := uint64(0)

	pipeline := mongo.Pipeline{bson.D{bson.E{Key: "$match", Value: bson.M{"documentKey.index": 0}}}}
	c, e := d.mongo.Database("s_"+groupname, options.Database().SetReadConcern(readconcern.Majority())).Collection(appname).Watch(context.Background(), pipeline)
	if e != nil {
		return e
	}
	defer c.Close(context.Background())
	summary, config, e := d.MongoGetInfo(context.Background(), groupname, appname)
	if e != nil && e != mongo.ErrNoDocuments {
		return e
	} else if e == nil {
		update(config)
		curop = summary.OpNum
	} else {
		update(&Config{})
	}
	for c.Next(context.Background()) {
		var curindex uint64
		var opnum uint64
		switch c.Current.Lookup("operationType").StringValue() {
		case "insert":
			curindex = uint64(c.Current.Lookup("fullDocument").Document().Lookup("cur_index").AsInt64())
			opnum = uint64(c.Current.Lookup("fullDocument").Document().Lookup("op_num").AsInt64())
		case "update":
			curindex = uint64(c.Current.Lookup("updateDescription").Document().Lookup("updatedFields").Document().Lookup("cur_index").AsInt64())
			opnum = uint64(c.Current.Lookup("updateDescription").Document().Lookup("updatedFields").Document().Lookup("op_num").AsInt64())
		case "delete":
			curindex = 0
			opnum = 0
		}
		config := &Config{}
		if opnum == 0 {
			update(config)
			curop = 0
		} else if opnum > curop {
			if e := d.mongo.Database("s_"+groupname).Collection(appname).FindOne(context.Background(), bson.M{"index": curindex}).Decode(config); e != nil {
				return e
			}
			update(config)
			curop = opnum
		}
	}
	if c.Err() != nil {
		return c.Err()
	}
	return nil
}
