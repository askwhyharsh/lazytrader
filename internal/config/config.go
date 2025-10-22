// internal/config/config.go
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	// Database
	DatabasePath string `yaml:"database_path"`

	// Polymarket
	TopTradersCount     int     `yaml:"top_traders_count"`
	MinProfitThreshold  float64 `yaml:"min_profit_threshold"`
	CopyTradeMultiplier float64 `yaml:"copy_trade_multiplier"`

	// Telegram
	TelegramBotToken string  `yaml:"telegram_bot_token"`
	TelegramChatID   int64   `yaml:"telegram_chat_id"`

	// Wallet
	PrivateKey      string `yaml:"private_key"`
	WalletAddress   string `yaml:"wallet_address"`
	PolygonRPCURL   string `yaml:"polygon_rpc_url"`

	// // Proxy Settings (NEW)
	// ProxyEnabled    bool   `yaml:"proxy_enabled"`
	// ProxyURL        string `yaml:"proxy_url"`
	// ProxyType       string `yaml:"proxy_type"` // "socks5", "http", "https"

	// Feature Flags
	DryRun          bool   `yaml:"dry_run"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Set defaults
	if cfg.DatabasePath == "" {
		cfg.DatabasePath = "./data/lazytrader.db"
	}
	if cfg.TopTradersCount == 0 {
		cfg.TopTradersCount = 10
	}
	if cfg.MinProfitThreshold == 0 {
		cfg.MinProfitThreshold = 1000.0
	}
	if cfg.CopyTradeMultiplier == 0 {
		cfg.CopyTradeMultiplier = 0.1
	}
	if cfg.PolygonRPCURL == "" {
		cfg.PolygonRPCURL = "https://polygon-rpc.com"
	}

	return &cfg, nil
}

func (c *Config) Validate() error {
	if c.TelegramBotToken == "" {
		return fmt.Errorf("telegram_bot_token is required")
	}
	if c.TelegramChatID == 0 {
		return fmt.Errorf("telegram_chat_id is required")
	}
	if c.PrivateKey == "" {
		return fmt.Errorf("private_key is required")
	}
	if c.WalletAddress == "" {
		return fmt.Errorf("wallet_address is required")
	}

	// // Validate proxy settings if enabled
	// if c.ProxyEnabled {
	// 	if c.ProxyURL == "" {
	// 		return fmt.Errorf("proxy_url is required when proxy is enabled")
	// 	}
	// 	if c.ProxyType != "socks5" && c.ProxyType != "http" && c.ProxyType != "https" {
	// 		return fmt.Errorf("proxy_type must be 'socks5', 'http', or 'https'")
	// 	}
	// }

	return nil
}