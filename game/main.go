package main

import (
	"common/config"
	"common/metrics"
	"context"
	"fmt"
	"framework/game"
	"game/app"
	"github.com/spf13/cobra"
	"log"
	"os"
)

var rootCmd = &cobra.Command{
	Use:   "game",
	Short: "game 游戏 比如拼三张 麻将等的具体处理",
	Long:  `game 游戏 比如拼三张 麻将等的具体处理`,
	Run: func(cmd *cobra.Command, args []string) {
	},
	PostRun: func(cmd *cobra.Command, args []string) {
	},
}

var (
	configFile    string
	gameConfigDir string
	serverId      string
)

//var configFile = flag.String("config", "application.yml", "config file")

func init() {
	rootCmd.Flags().StringVar(&configFile, "config", "application.yml", "app config yml file")
	rootCmd.Flags().StringVar(&gameConfigDir, "gameDir", "../config", "game config dir")
	rootCmd.Flags().StringVar(&serverId, "serverId", "", "app server id， required")
	_ = rootCmd.MarkFlagRequired("serverId")
}

func main() {
	//1.加载配置
	if err := rootCmd.Execute(); err != nil {
		log.Println(err)
		os.Exit(1)
	}
	config.InitConfig(configFile)
	game.InitConfig(gameConfigDir)
	//2.启动监控
	go func() {
		err := metrics.Serve(fmt.Sprintf("0.0.0.0:%d", config.Conf.MetricPort))
		if err != nil {
			panic(err)
		}
	}()
	//3.连接nats服务 并进行订阅
	err := app.Run(context.Background(), serverId)
	if err != nil {
		log.Println(err)
		os.Exit(-1)
	}
}
