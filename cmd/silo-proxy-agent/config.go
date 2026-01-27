package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/EternisAI/silo-proxy/internal/api/http"
	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

type Config struct {
	Log   LogConfig
	Http  http.Config
	Grpc  GrpcConfig
	Local LocalConfig
}

type GrpcConfig struct {
	ServerAddress string    `mapstructure:"server_address"`
	AgentID       string    `mapstructure:"agent_id"`
	TLS           TLSConfig `mapstructure:"tls"`
}

type TLSConfig struct {
	Enabled            bool   `mapstructure:"enabled"`
	CertFile           string `mapstructure:"cert_file"`
	KeyFile            string `mapstructure:"key_file"`
	CAFile             string `mapstructure:"ca_file"`
	ServerNameOverride string `mapstructure:"server_name_override"`
}

type LocalConfig struct {
	ServiceURL string `mapstructure:"service_url"`
}

var config Config

func InitConfig() {
	var err error

	_ = godotenv.Load()

	viper.SetConfigName("application")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./cmd/silo-proxy-agent")
	viper.SetConfigType("yaml")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	_ = viper.BindEnv("telegram.token", "TELEGRAM_TOKEN")
	_ = viper.BindEnv("openrouter.apiKey", "OPENROUTER_API_KEY")

	if err := viper.ReadInConfig(); err != nil {
		panic(err)
	}

	err = viper.Unmarshal(&config)
	if err != nil {
		panic(err)
	}

	// Initialize logger with configured log level
	initLogger(config.Log.Level)

	// Pretty print config as JSON (only at DEBUG level)
	if strings.ToUpper(config.Log.Level) == LOG_LEVEL_DEBUG {
		configJSON, err := json.MarshalIndent(config, "", "  ")
		if err == nil {
			fmt.Println("Config loaded:")
			fmt.Println(string(configJSON))
		}
	}
}
