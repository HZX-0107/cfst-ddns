package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"

	"cfst-ddns/assets" // 引入 assets 包以访问嵌入的二进制数据
	"cfst-ddns/internal/config"
	"cfst-ddns/internal/dns"
	"cfst-ddns/internal/speedtest"
)

// main 是程序的唯一入口
func main() {
	// 1. 初始化日志系统
	setupLogger()

	log.Println("Starting Cloudflare DNS Updater...")

	// 2. 加载配置
	cfg, err := config.Load("configs")
	if err != nil {
		log.Fatalf("Fatal: Failed to load configuration: %v", err)
	}

	// [新增] 自动释放默认 IP 库文件
	// 解决 Docker 挂载空目录导致文件丢失的问题，实现“自我初始化”
	if err := initAssetFile(cfg.SpeedTest.IPFile, assets.IPList); err != nil {
		log.Printf("Warning: Failed to init default IPv4 list: %v", err)
	}
	if cfg.SpeedTest.IPv6File != "" {
		if err := initAssetFile(cfg.SpeedTest.IPv6File, assets.IPv6List); err != nil {
			log.Printf("Warning: Failed to init default IPv6 list: %v", err)
		}
	}

	// 检查并释放嵌入的 cfst 二进制文件
	if len(assets.CFSTBinary) > 0 {
		log.Println("Detected embedded cfst binary, extracting...")
		tempBinPath, err := extractBinary(assets.CFSTBinary)
		if err != nil {
			log.Fatalf("Fatal: Failed to extract embedded binary: %v", err)
		}
		// 覆盖配置文件中的路径，指向临时释放的文件
		cfg.SpeedTest.BinPath = tempBinPath
		// 程序退出时清理临时文件
		defer os.Remove(tempBinPath)
		log.Printf("Using temporary binary at: %s", tempBinPath)
	}

	if cfg.App.Debug {
		printDebugInfo(cfg)
	}

	// 3. 初始化模块
	// 3.1 初始化腾讯云 DNS 客户端
	dnsClient, err := dns.NewTencentClient(&cfg.Tencent)
	if err != nil {
		log.Fatalf("Fatal: Failed to init DNS client: %v", err)
	}

	// 3.2 初始化测速运行器
	stRunner := speedtest.NewRunner(&cfg.SpeedTest)

	// 4. 执行业务逻辑
	log.Println("------------------------------------------------")

	// --- 处理 IPv4 ---
	processIP(cfg, stRunner, dnsClient, false)

	// --- 处理 IPv6 ---
	if cfg.SpeedTest.IPv6File != "" {
		log.Println("------------------------------------------------")
		processIP(cfg, stRunner, dnsClient, true)
	}

	log.Println("------------------------------------------------")
	log.Println("All tasks completed.")
}

// [新增] initAssetFile 检查文件是否存在，不存在则从嵌入资源中写入
func initAssetFile(targetPath string, data []byte) error {
	// 如果没有嵌入数据，直接跳过
	if len(data) == 0 {
		return nil
	}

	// 检查文件是否存在
	if _, err := os.Stat(targetPath); err == nil {
		// 文件存在，不做任何操作（保留用户修改过的内容）
		return nil
	}

	// 确保目录存在
	dir := filepath.Dir(targetPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}

	// 写入文件
	log.Printf("Initializing missing asset file: %s", targetPath)
	if err := os.WriteFile(targetPath, data, 0644); err != nil {
		return fmt.Errorf("write asset: %w", err)
	}
	return nil
}

// extractBinary 将内存中的二进制数据写入临时文件
func extractBinary(data []byte) (string, error) {
	// 确定文件名（Windows下需要.exe后缀）
	binName := "cfst_runner"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}

	// 创建临时文件
	tmpFile, err := os.CreateTemp("", binName)
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	defer tmpFile.Close()

	// 写入数据
	if _, err := tmpFile.Write(data); err != nil {
		return "", fmt.Errorf("write binary data: %w", err)
	}

	// 赋予执行权限 (Linux/macOS 必需，Windows 会忽略但无害)
	if err := tmpFile.Chmod(0755); err != nil {
		return "", fmt.Errorf("chmod binary: %w", err)
	}

	return tmpFile.Name(), nil
}

// processIP 封装测速和更新逻辑
func processIP(cfg *config.Config, runner *speedtest.Runner, client *dns.TencentClient, isIPv6 bool) {
	ipVersion := "IPv4"
	recordType := "A"
	if isIPv6 {
		ipVersion = "IPv6"
		recordType = "AAAA"
	}

	log.Printf("[%s] Starting process...", ipVersion)

	// A. 运行测速
	bestIP, err := runner.Run(isIPv6)
	if err != nil {
		log.Printf("[%s] SpeedTest failed or skipped: %v", ipVersion, err)
		return
	}

	if bestIP == "" {
		log.Printf("[%s] No valid IP found, skipping update.", ipVersion)
		return
	}

	log.Printf("[%s] Best IP found: %s", ipVersion, bestIP)

	// B. 更新 DNS
	err = client.UpdateRecord(cfg.Domain.MainDomain, cfg.Domain.SubDomain, recordType, bestIP)
	if err != nil {
		log.Printf("[%s] DNS update failed: %v", ipVersion, err)
		return
	}

	log.Printf("[%s] Process finished successfully.", ipVersion)
}

func setupLogger() {
	logFile, err := os.OpenFile("app.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		fmt.Println("Failed to open log file:", err)
		return
	}
	multiWriter := io.MultiWriter(os.Stdout, logFile)
	log.SetOutput(multiWriter)
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}

func printDebugInfo(cfg *config.Config) {
	log.Printf("\n[Debug Configuration]\n")
	log.Printf("Target Domain: %s.%s\n", cfg.Domain.SubDomain, cfg.Domain.MainDomain)
	log.Printf("Outputs:\n  v4: %s\n  v6: %s\n", cfg.SpeedTest.OutputCSV4, cfg.SpeedTest.OutputCSV6)
	log.Printf("SpeedTest Bin: %s\n", cfg.SpeedTest.BinPath)
}
