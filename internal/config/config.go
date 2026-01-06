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
	BinPath    string `mapstructure:"bin_path"`
	IPFile     string `mapstructure:"ip_file"`
	IPv6File   string `mapstructure:"ipv6_file"`
	OutputCSV4 string `mapstructure:"output_csv_v4"` // IPv4 结果输出路径
	OutputCSV6 string `mapstructure:"output_csv_v6"` // IPv6 结果输出路径	MaxPing   int
	MaxPing    int    `mapstructure:"max_ping"`
	TestCount  int    `mapstructure:"test_count"`
}

// Load 读取并解析配置文件
// path: 配置文件所在的目录 (例如 "./configs")
func Load(path string) (*Config, error) {
	viper.AddConfigPath(path)     // 设置配置文件的搜索路径
	viper.SetConfigName("config") // 设置配置文件名 (不带后缀)
	viper.SetConfigType("yaml")   // 设置配置文件类型

	// 读取配置文件
	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// 将配置映射到结构体
	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	if cfg.App.Debug {
		log.Println("Debug mode enabled. Config loaded successfully.")
	}

	return &cfg, nil
}
