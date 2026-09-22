package util

import (
	"time"

	"github.com/spf13/viper"
)

// Config 保存應用程式所有設定，由 viper 從設定檔或環境變數讀取。
type Config struct {
	Environment       string        `mapstructure:"ENVIRONMENT"`
	DBDriver          string        `mapstructure:"DB_DRIVER"`
	DBSource          string        `mapstructure:"DB_SOURCE"`
	HTTPServerAddress string        `mapstructure:"HTTP_SERVER_ADDRESS"`
	ShutdownTimeout   time.Duration `mapstructure:"SHUTDOWN_TIMEOUT"`
}

// LoadConfig 從指定路徑讀取 app.env，環境變數可覆蓋設定檔的值。
func LoadConfig(path string) (config Config, err error) {
	viper.AddConfigPath(path)
	viper.SetConfigName("app")
	viper.SetConfigType("env")

	viper.AutomaticEnv()

	err = viper.ReadInConfig()
	if err != nil {
		return
	}

	err = viper.Unmarshal(&config)
	return
}
