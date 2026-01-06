package speedtest

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"

	"cfst-ddns/internal/config"
)

// Runner 负责执行测速任务
type Runner struct {
	Cfg *config.SpeedTestConfig
}

// NewRunner 创建一个测速运行器
func NewRunner(cfg *config.SpeedTestConfig) *Runner {
	return &Runner{Cfg: cfg}
}

// Run 执行测速并返回最佳 IP
// isIPv6: true 表示测速 IPv6, false 表示 IPv4
// 返回值: (最佳IP, 错误)
func (r *Runner) Run(isIPv6 bool) (string, error) {
	// 1. 根据 IPv4/IPv6 选择对应的输入和输出文件
	ipFile := r.Cfg.IPFile
	outputFile := r.Cfg.OutputCSV4

	if isIPv6 {
		ipFile = r.Cfg.IPv6File
		outputFile = r.Cfg.OutputCSV6
	}

	// 2. 检查 IP 库文件是否存在
	if _, err := os.Stat(ipFile); os.IsNotExist(err) {
		if isIPv6 {
			// IPv6 文件不存在通常是可以接受的（用户可能不需要 IPv6）
			log.Printf("IPv6 list file not found (%s), skipping IPv6 test.", ipFile)
			return "", nil
		}
		return "", fmt.Errorf("IP file not found: %s", ipFile)
	}

	// [新增] 3. 检查并清理旧的结果文件
	// 虽然 cfst 工具通常会覆盖文件，但手动清理能确保万无一失
	if _, err := os.Stat(outputFile); err == nil {
		if err := os.Remove(outputFile); err != nil {
			log.Printf("Warning: Failed to delete old result file %s: %v", outputFile, err)
		} else {
			log.Printf("Cleaned up old result file: %s", outputFile)
		}
	}

	log.Printf("Running speedtest for %s...", func() string {
		if isIPv6 {
			return "IPv6"
		}
		return "IPv4"
	}())

	// 3. 构造命令
	// 对应命令: ./cfst -f ip.txt -n 500 -o result.csv -tl 9999
	// 注意：这里假设 bin_path 是可执行文件路径
	cmd := exec.Command(r.Cfg.BinPath,
		"-f", ipFile,
		"-n", strconv.Itoa(r.Cfg.TestCount),
		"-o", outputFile,
		"-tl", strconv.Itoa(r.Cfg.MaxPing),
	)

	// 4. 执行命令
	// 如果你想在控制台看到 cfst 原本的彩色输出，可以将 Stdout 对接
	// cmd.Stdout = os.Stdout
	// 这里为了保持日志整洁，我们只捕获错误，或者根据 Debug 开关决定

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to execute speedtest binary: %w", err)
	}

	// 5. 解析结果
	bestIP, err := r.parseBestIP(outputFile)
	if err != nil {
		return "", fmt.Errorf("failed to parse result csv: %w", err)
	}

	return bestIP, nil
}

// parseBestIP 读取 CSV 文件并返回第一行的 IP
func (r *Runner) parseBestIP(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	// 仅在严格需要记录错误日志时才这样写，通常没必要
	defer func() {
		if err := f.Close(); err != nil {
			log.Printf("Error closing file: %v", err)
		}
	}()

	reader := csv.NewReader(f)
	// 读取所有记录
	records, err := reader.ReadAll()
	if err != nil {
		return "", err
	}

	// CloudflareSpeedTest 的 CSV 格式：
	// 第一行是表头 (IP地址, 速度, 延迟...)
	// 第二行开始是数据
	if len(records) < 2 {
		return "", fmt.Errorf("no results found in csv file")
	}

	// 获取第二行第一列 (最佳 IP)
	bestIP := records[1][0]
	if bestIP == "" {
		return "", fmt.Errorf("parsed IP is empty")
	}

	return bestIP, nil
}
