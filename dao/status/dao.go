package status

import (
	csql "database/sql"

	credis "github.com/chenjie199234/Corelib/redis"
	cmongo "go.mongodb.org/mongo-driver/mongo"
)

//Dao this is a data operation layer to operate status service's data
type Dao struct {
	sql   *csql.DB
	redis *credis.Pool
	mongo *cmongo.Client
}

//NewDao Dao is only a data operation layer
//don't write business logic in this package
//business logic should be written in service package
func NewDao(sql *csql.DB, redis *credis.Pool, mongo *cmongo.Client) *Dao {
	return &Dao{
		sql:   sql,
		redis: redis,
		mongo: mongo,
	}
}