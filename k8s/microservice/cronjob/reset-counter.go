package main

import (
	"fmt"
	"github.com/go-redis/redis"
	"microservice/pkg/config"
)

var (
	cfg      config.Config
	redisCli *redis.ClusterClient
)

func main() {
	initConf()
	initRedisClient(cfg.RedisAddrs)
	result, err := redisCli.Del("counter").Result()
	fmt.Println(result, err)
}

func initConf() {
	var err error
	cfg, err = config.InitYamlConfig("/conf/all.yml")
	if err != nil {
		panic(err)
	}
	fmt.Printf("conf:%+v\n", cfg)
}
func initRedisClient(addrs []string) {
	redisCli = redis.NewClusterClient(&redis.ClusterOptions{
		Addrs: addrs,
	})
}
