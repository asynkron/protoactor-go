package cache

import (
	"log"
	"time"

	"github.com/go-redis/redis"
)

var rds *redis.Client

func init() {
	// var err error
	addr := "127.0.0.1:6379"
	rds = redis.NewClient(&redis.Options{
		Addr:        addr,
		DialTimeout: 1 * time.Second,
	})
	if err := rds.Ping().Err(); err != nil {
		log.Printf("no redis err=%v", err)
	}
}

func GetCountor(key string) int64 {
	v, err := rds.Get(key).Int64()
	if err != nil && err != redis.Nil {
		panic(err)
	}
	return v
}

func SetCountor(key string, val int64) {
	err := rds.Set(key, val, time.Hour).Err()
	if err != nil {
		panic(err)
	}
}
