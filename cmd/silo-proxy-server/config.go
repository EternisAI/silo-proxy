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
	Log  LogConfig
	Http http.Config
	Grpc GrpcConfig
}

type GrpcConfig struct {
	Port int       `mapstructure:"port"`
	TLS  TLSConfig `mapstructure:"tls"`
}

type TLSConfig struct {
	Enabled     bool   `mapstructure:"enabled"`
	CertFile    string `mapstructure:"cert_file"`
	KeyFile     string `mapstructure:"key_file"`
	CAFile      string `mapstructure:"ca_file"`
	CAKeyFile   string `mapstructure:"ca_key_file"`
	ClientAuth  string `mapstructure:"client_auth"`
	DomainNames string `mapstructure:"domain_names"`
	IPAddresses string `mapstructure:"ip_addresses"`
}

var config Config

func ParseCommaSeparated(input string) []string {
	if input == "" {
		return nil
	}
	parts := strings.Split(input, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func InitConfig() {
	var err error

	_ = godotenv.Load()

	viper.SetConfigName("application")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./cmd/silo-proxy-server")
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
