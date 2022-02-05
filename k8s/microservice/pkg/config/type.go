package config

type Config struct {
	GateWayHttpAddr string   `yaml:"gateway-http-addr"`
	TimeGrpcAddr    string   `yaml:"time-grpc-addr"`
	TimeSvcAddr     string   `yaml:"time-svc-addr"`
	IPHttpAddr      string   `yaml:"ip-http-addr"`
	IPSvcAddr       string   `yaml:"ip-svc-addr"`
	RedisAddrs      []string `yaml:"redis-addrs"`
}
