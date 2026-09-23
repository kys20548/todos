package util

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

// 可用的環境名稱，對應 config/app.<env>.env
const (
	EnvDev  = "dev"
	EnvQA   = "qa"
	EnvProd = "prod"
)

// Config 保存應用程式所有設定，由 viper 從設定檔或環境變數讀取。
type Config struct {
	// Environment 不從設定檔讀，由 LoadConfig 的 env 參數決定——
	// 避免「用 qa 啟動、檔案裡卻寫著另一個環境」兩邊對不上
	Environment       string        `mapstructure:"-"`
	DBSource          string        `mapstructure:"DB_SOURCE"`
	HTTPServerAddress string        `mapstructure:"HTTP_SERVER_ADDRESS"`
	ShutdownTimeout   time.Duration `mapstructure:"SHUTDOWN_TIMEOUT"`
}

// LoadConfig 從 path 目錄讀取 app.<env>.env，環境變數可覆蓋設定檔的值。
func LoadConfig(path, env string) (config Config, err error) {
	switch env {
	case EnvDev, EnvQA, EnvProd:
	default:
		return config, fmt.Errorf("unknown env %q, must be one of: %s, %s, %s", env, EnvDev, EnvQA, EnvProd)
	}

	viper.AddConfigPath(path)
	viper.SetConfigName("app." + env)
	viper.SetConfigType("env")

	viper.AutomaticEnv()

	err = viper.ReadInConfig()
	if err != nil {
		return
	}

	err = viper.Unmarshal(&config)
	config.Environment = env
	return
}
