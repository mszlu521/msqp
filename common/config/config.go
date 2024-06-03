package config

import (
	"fmt"
	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
	"log"
)

var Conf *Config

type Config struct {
	Log        LogConf                 `mapstructure:"log"`
	Port       int                     `mapstructure:"port"`
	WsPort     int                     `mapstructure:"wsPort"`
	MetricPort int                     `mapstructure:"metricPort"`
	HttpPort   int                     `mapstructure:"httpPort"`
	AppName    string                  `mapstructure:"appName"`
	Database   Database                `mapstructure:"db"`
	Jwt        JwtConf                 `mapstructure:"jwt"`
	Grpc       GrpcConf                `mapstructure:"grpc"`
	Etcd       EtcdConf                `mapstructure:"etcd"`
	Domain     map[string]Domain       `mapstructure:"domain"`
	Services   map[string]ServicesConf `mapstructure:"services"`
}

type ServicesConf struct {
	Id         string `mapstructure:"id"`
	ClientHost string `mapstructure:"clientHost"`
	ClientPort int    `mapstructure:"clientPort"`
}
type Domain struct {
	Name        string `mapstructure:"name"`
	LoadBalance bool   `mapstructure:"loadBalance"`
}
type JwtConf struct {
	Secret string `mapstructure:"secret"`
	Exp    int64  `mapstructure:"exp"`
}
type LogConf struct {
	Level string `mapstructure:"level"`
}

// Database 数据库配置
type Database struct {
	MongoConf MongoConf `mapstructure:"mongo"`
	RedisConf RedisConf `mapstructure:"redis"`
}
type MongoConf struct {
	Url         string `mapstructure:"url"`
	Db          string `mapstructure:"db"`
	UserName    string `mapstructure:"userName"`
	Password    string `mapstructure:"password"`
	MinPoolSize int    `mapstructure:"minPoolSize"`
	MaxPoolSize int    `mapstructure:"maxPoolSize"`
}
type RedisConf struct {
	Addr         string   `mapstructure:"addr"`
	ClusterAddrs []string `mapstructure:"clusterAddrs"`
	Password     string   `mapstructure:"password"`
	PoolSize     int      `mapstructure:"poolSize"`
	MinIdleConns int      `mapstructure:"minIdleConns"`
	Host         string   `mapstructure:"host"`
	Port         int      `mapstructure:"port"`
}
type EtcdConf struct {
	Addrs       []string       `mapstructure:"addrs"`
	RWTimeout   int            `mapstructure:"rwTimeout"`
	DialTimeout int            `mapstructure:"dialTimeout"`
	Register    RegisterServer `mapstructure:"register"`
}
type RegisterServer struct {
	Addr    string `mapstructure:"addr"`
	Name    string `mapstructure:"name"`
	Version string `mapstructure:"version"`
	Weight  int    `mapstructure:"weight"`
	Ttl     int64  `mapstructure:"ttl"` //租约时长
}
type GrpcConf struct {
	Addr string `mapstructure:"addr"`
}

// InitConfig 加载配置
func InitConfig(confFile string) {
	Conf = new(Config)
	v := viper.New()
	v.SetConfigFile(confFile)
	v.WatchConfig()
	v.OnConfigChange(func(in fsnotify.Event) {
		log.Println("配置文件被修改了")
		err := v.Unmarshal(&Conf)
		if err != nil {
			panic(fmt.Errorf("Unmarshal change config data,err:%v \n", err))
		}
	})
	err := v.ReadInConfig()
	if err != nil {
		panic(fmt.Errorf("读取配置文件出错,err:%v \n", err))
	}
	//解析
	err = v.Unmarshal(&Conf)
	if err != nil {
		panic(fmt.Errorf("Unmarshal config data,err:%v \n", err))
	}
}
