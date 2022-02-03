package config

type Config struct {
	GateWayHttpAddr string `yaml:"gateway-http-addr"`
	TimeGrpcAddr    string `yaml:"time-grpc-addr"`
}
