package config

import (
	"fmt"
	"log"

	"github.com/spf13/viper"
)

// Config 聚合所有配置项
type Config struct {
	App       AppConfig       `mapstructure:"app"`
	Tencent   TencentConfig   `mapstructure:"tencent"`
	Domain    DomainConfig    `mapstructure:"domain"`
	SpeedTest SpeedTestConfig `mapstructure:"speedtest"`
}

// AppConfig 应用程序通用配置
type AppConfig struct {
	Debug bool   `mapstructure:"debug"`
	Cron  string `mapstructure:"cron"`
}

// TencentConfig 腾讯云 API 配置
type TencentConfig struct {
	SecretID  string `mapstructure:"secret_id"`
	SecretKey string `mapstructure:"secret_key"`
}

// DomainConfig 域名配置
type DomainConfig struct {
	MainDomain string `mapstructure:"main_domain"`
	SubDomain  string `mapstructure:"sub_domain"`
}

// SpeedTestConfig 测速工具配置
type SpeedTestConfig struct {
	BinPath     string `mapstructure:"bin_path"`
	IPFile      string `mapstructure:"ip_file"`
	IPv6File    string `mapstructure:"ipv6_file"`
	OutputCSV4  string `mapstructure:"output_csv_v4"`
	OutputCSV6  string `mapstructure:"output_csv_v6"`
	MaxPing     int    `mapstructure:"max_ping"`
	TestCount   int    `mapstructure:"test_count"`
	DownloadURL string `mapstructure:"download_url"`

	// [新增] 参与下载测速的数量
	// 默认 cfst 是 10，建议设为 20-50 以提高命中率
	DownloadTestCount int `mapstructure:"download_test_count"`
}

// Load 读取并解析配置文件
func Load(path string) (*Config, error) {
	viper.AddConfigPath(path)
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	if cfg.App.Debug {
		log.Println("Debug mode enabled. Config loaded successfully.")
	}

	return &cfg, nil
}
