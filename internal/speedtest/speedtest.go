package speedtest

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"cfst-ddns/internal/config"
)

// Runner 负责执行测速任务
type Runner struct {
	Cfg   *config.SpeedTestConfig
	Debug bool
}

// NewRunner 创建一个测速运行器
func NewRunner(cfg *config.SpeedTestConfig, debug bool) *Runner {
	return &Runner{
		Cfg:   cfg,
		Debug: debug,
	}
}

// Run 执行测速并返回最佳 IP
func (r *Runner) Run(isIPv6 bool) (string, error) {
	ipFile := r.Cfg.IPFile
	outputFile := r.Cfg.OutputCSV4

	if isIPv6 {
		ipFile = r.Cfg.IPv6File
		outputFile = r.Cfg.OutputCSV6
	}

	// 检查 IP 库文件是否存在
	if _, err := os.Stat(ipFile); os.IsNotExist(err) {
		if isIPv6 {
			log.Printf("IPv6 list file not found (%s), skipping IPv6 test.", ipFile)
			return "", nil
		}
		return "", fmt.Errorf("IP file not found: %s", ipFile)
	}

	// 检查并清理旧的结果文件
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

	// [新增] 处理下载测速数量的默认值
	dnCount := r.Cfg.DownloadTestCount
	if dnCount <= 0 {
		dnCount = 10 // 默认值保持 10
	}

	// 构造命令
	args := []string{
		"-f", ipFile,
		"-n", strconv.Itoa(r.Cfg.TestCount),
		"-o", outputFile,
		"-tl", strconv.Itoa(r.Cfg.MaxPing),
		"-dn", strconv.Itoa(dnCount), // [修改] 使用配置的数量
	}

	if r.Cfg.DownloadURL != "" {
		args = append(args, "-url", r.Cfg.DownloadURL)
	}

	if r.Debug {
		args = append(args, "-debug")
	}

	cmd := exec.Command(r.Cfg.BinPath, args...)

	output, err := cmd.CombinedOutput()

	if (r.Debug || err != nil) && len(output) > 0 {
		label := "IPv4"
		if isIPv6 {
			label = "IPv6"
		}
		log.Printf("--- CFST Output (%s) ---\n%s\n--- End Output ---", label, string(output))
	}

	if err != nil {
		return "", fmt.Errorf("failed to execute speedtest binary: %w", err)
	}

	bestIP, err := r.parseBestIP(outputFile)
	if err != nil {
		return "", fmt.Errorf("failed to parse result csv: %w", err)
	}

	return bestIP, nil
}

// parseBestIP 读取 CSV 文件并返回最佳 IP
func (r *Runner) parseBestIP(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	reader := csv.NewReader(f)
	reader.FieldsPerRecord = -1

	records, err := reader.ReadAll()
	if err != nil {
		return "", err
	}

	if len(records) < 2 {
		return "", fmt.Errorf("no results found in csv file")
	}

	// 遍历寻找第一个有效的 IP (下载速度 > 0)
	for i, row := range records {
		if i == 0 {
			continue
		}
		if len(row) < 6 {
			continue
		}

		ip := row[0]
		speedStr := strings.TrimSpace(row[5])

		speed, _ := strconv.ParseFloat(speedStr, 64)

		if speed > 0 {
			if i > 1 {
				latency := strings.TrimSpace(row[4])
				log.Printf("Smart Select: Skipped %d IP(s) with 0.00 speed. Selected: %s (Speed: %s MB/s, Latency: %s ms)",
					i-1, ip, speedStr, latency)
			}
			return ip, nil
		}
	}

	bestIP := records[1][0]
	log.Printf("[Warning] All tested IPs have 0.00 MB/s download speed. Fallback to the first one: %s", bestIP)
	log.Printf("[Tip] This usually means the default test URL is blocked or timed out.")
	log.Printf("[Tip] Please try setting 'download_url' in config.yaml to a different CDN URL.")

	if bestIP == "" {
		return "", fmt.Errorf("parsed IP is empty")
	}

	return bestIP, nil
}
