package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/EternisAI/silo-proxy/internal/api/http"
	"github.com/EternisAI/silo-proxy/internal/auth"
	"github.com/EternisAI/silo-proxy/internal/db"
	"github.com/joho/godotenv"
	"github.com/lwlee2608/adder"
)

type Config struct {
	Log       LogConfig
	Http      http.Config
	Grpc      GrpcConfig
	DB        db.Config        `mapstructure:"db"`
	JWT       auth.Config      `mapstructure:"jwt"`
	Provision ProvisionConfig  `mapstructure:"provision"`
}

type ProvisionConfig struct {
	Enabled                bool `mapstructure:"enabled"`
	KeyTTLHours            int  `mapstructure:"key_ttl_hours"`
	CleanupIntervalMinutes int  `mapstructure:"cleanup_interval_minutes"`
}

type GrpcConfig struct {
	Port int       `mapstructure:"port"`
	TLS  TLSConfig `mapstructure:"tls"`
}

type TLSConfig struct {
	Enabled      bool   `mapstructure:"enabled"`
	CertFile     string `mapstructure:"cert_file"`
	KeyFile      string `mapstructure:"key_file"`
	CAFile       string `mapstructure:"ca_file"`
	CAKeyFile    string `mapstructure:"ca_key_file"`
	ClientAuth   string `mapstructure:"client_auth"`
	DomainNames  string `mapstructure:"domain_names"`
	IPAddresses  string `mapstructure:"ip_addresses"`
	AgentCertDir string `mapstructure:"agent_cert_dir"`
}

var config Config

func InitConfig() {
	var err error

	_ = godotenv.Load()

	adder.SetConfigName("application")
	adder.AddConfigPath(".")
	adder.AddConfigPath("./cmd/silo-proxy-server")
	adder.SetConfigType("yaml")
	adder.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	adder.AutomaticEnv()

	_ = adder.BindEnv("telegram.token", "TELEGRAM_TOKEN")
	_ = adder.BindEnv("openrouter.apiKey", "OPENROUTER_API_KEY")

	if err := adder.ReadInConfig(); err != nil {
		panic(err)
	}

	err = adder.Unmarshal(&config)
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
