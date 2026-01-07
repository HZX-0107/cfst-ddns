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
			// IPv6 文件不存在通常是可以接受的
			log.Printf("IPv6 list file not found (%s), skipping IPv6 test.", ipFile)
			return "", nil
		}
		return "", fmt.Errorf("IP file not found: %s", ipFile)
	}

	// 3. 检查并清理旧的结果文件
	if _, err := os.Stat(outputFile); err == nil {
		if err := os.Remove(outputFile); err != nil {
			log.Printf("Warning: Failed to delete old result file %s: %v", outputFile, err)
		}
	}

	log.Printf("Running speedtest for %s...", func() string {
		if isIPv6 {
			return "IPv6"
		}
		return "IPv4"
	}())

	// 4. 构造命令
	// 基础参数
	args := []string{
		"-f", ipFile,
		"-n", strconv.Itoa(r.Cfg.TestCount),
		"-o", outputFile,
		"-tl", strconv.Itoa(r.Cfg.MaxPing),
		"-dn", "10", // 限制参与下载测速的数量为 10 个，避免时间过长
	}

	// [新增] 关键逻辑：如果配置了自定义下载地址，则通过 -url 参数传递
	if r.Cfg.DownloadURL != "" {
		args = append(args, "-url", r.Cfg.DownloadURL)
	}

	// 打印调试信息，方便查看实际使用了什么参数
	// log.Printf("Executing command: %s %v", r.Cfg.BinPath, args)

	cmd := exec.Command(r.Cfg.BinPath, args...)

	// 5. 执行命令
	// [修改] 使用 CombinedOutput 替代 Run，捕获并打印 cfst 的完整输出到日志
	output, err := cmd.CombinedOutput()

	// 打印输出内容，方便调试测速过程（如查看具体的下载错误信息）
	if len(output) > 0 {
		label := "IPv4"
		if isIPv6 {
			label = "IPv6"
		}
		log.Printf("--- CFST Output (%s) ---\n%s\n--- End Output ---", label, string(output))
	}

	if err != nil {
		return "", fmt.Errorf("failed to execute speedtest binary: %w", err)
	}

	// 6. 解析结果
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
	defer f.Close()

	reader := csv.NewReader(f)
	// 读取所有记录
	records, err := reader.ReadAll()
	if err != nil {
		return "", err
	}

	// CloudflareSpeedTest 的 CSV 格式：
	// 第一行是表头
	// 第二行开始是数据
	if len(records) < 2 {
		return "", fmt.Errorf("no results found in csv file")
	}

	// CSV 结构通常是: IP地址, 已发送, 已接收, 丢包率, 平均延迟, 下载速度, ...
	// records[1] 是第一条最佳数据
	bestIP := records[1][0]
	downloadSpeed := records[1][5] // 第 6 列是下载速度 (MB/s)

	// [新增] 智能检测：如果速度是 0.00，发出警告
	if downloadSpeed == "0.00" {
		log.Printf("[Warning] The best IP (%s) has 0.00 MB/s download speed.", bestIP)
		log.Printf("[Tip] This usually means the default test URL is blocked or timed out.")
		log.Printf("[Tip] Please try setting 'download_url' in config.yaml to a different CDN URL.")
	}

	if bestIP == "" {
		return "", fmt.Errorf("parsed IP is empty")
	}

	return bestIP, nil
}
