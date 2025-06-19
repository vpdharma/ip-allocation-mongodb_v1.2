package config

import (
	"log"

	"github.com/spf13/viper"
)

type Config struct {
	DBUri      string `mapstructure:"DB_URI"`
	DBName     string `mapstructure:"DB_NAME"`
	ServerHost string `mapstructure:"SERVER_HOST"`
	ServerPort string `mapstructure:"SERVER_PORT"`
	Env        string `mapstructure:"ENV"`
}

func LoadConfig(path string) (config Config, err error) {
	viper.AddConfigPath(path)
	viper.SetConfigName("app")
	viper.SetConfigType("env")

	viper.AutomaticEnv()

	err = viper.ReadInConfig()
	if err != nil {
		log.Printf("Warning: Config file not found, using environment variables: %v", err)
	}

	err = viper.Unmarshal(&config)
	return
}
