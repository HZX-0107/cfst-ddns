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
	Cfg   *config.SpeedTestConfig
	Debug bool // [新增] 是否开启调试模式
}

// NewRunner 创建一个测速运行器
// [修改] 增加 debug 参数，用于接收全局调试配置
func NewRunner(cfg *config.SpeedTestConfig, debug bool) *Runner {
	return &Runner{
		Cfg:   cfg,
		Debug: debug,
	}
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

	// 关键逻辑：如果配置了自定义下载地址，则通过 -url 参数传递
	if r.Cfg.DownloadURL != "" {
		args = append(args, "-url", r.Cfg.DownloadURL)
	}

	// [新增] 如果开启了调试模式，添加 -debug 参数
	// 这会让 cfst 输出更多关于 HTTPing 和下载测速失败的详细原因
	if r.Debug {
		args = append(args, "-debug")
	}

	// 打印调试信息，方便查看实际使用了什么参数
	log.Printf("Executing command: %s %v", r.Cfg.BinPath, args)

	cmd := exec.Command(r.Cfg.BinPath, args...)

	// 5. 执行命令
	// 使用 CombinedOutput 替代 Run，捕获并打印 cfst 的完整输出到日志
	output, err := cmd.CombinedOutput()

	// [优化] 日志降噪：仅在 Debug 模式开启 或 执行出错时打印完整输出
	// 避免正常运行时日志文件过大
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

	// 6. 解析结果
	bestIP, err := r.parseBestIP(outputFile)
	if err != nil {
		return "", fmt.Errorf("failed to parse result csv: %w", err)
	}

	return bestIP, nil
}

// parseBestIP 读取 CSV 文件并返回最佳 IP
// [优化] 智能选优：遍历 CSV 寻找第一个下载速度 > 0 的 IP，而不是盲目取第一条
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

	// CSV 结构通常是: IP地址(0), 已发送(1), 已接收(2), 丢包率(3), 平均延迟(4), 下载速度(5)

	// 遍历寻找第一个有效的 IP (下载速度 > 0)
	for i, row := range records {
		if i == 0 {
			continue
		} // 跳过表头
		if len(row) < 6 {
			continue
		} // 防止数据不完整越界

		ip := row[0]
		speedStr := row[5]

		// 尝试解析速度
		speed, _ := strconv.ParseFloat(speedStr, 64)

		if speed > 0 {
			// 找到了一个下载速度正常的 IP
			if i > 1 {
				// 如果不是第一条，说明跳过了前面速度为 0 的 IP
				log.Printf("Smart Select: Skipped %d IP(s) with 0.00 speed. Selected: %s (Speed: %s MB/s, Latency: %s ms)",
					i-1, ip, speedStr, row[4])
			}
			return ip, nil
		}
	}

	// 如果遍历完所有结果，下载速度全都是 0.00
	// 降级策略：返回列表中的第一个 IP，并打印严重警告
	bestIP := records[1][0]
	log.Printf("[Warning] All tested IPs have 0.00 MB/s download speed. Fallback to the first one: %s", bestIP)
	log.Printf("[Tip] This usually means the default test URL is blocked or timed out.")
	log.Printf("[Tip] Please try setting 'download_url' in config.yaml to a different CDN URL.")

	if bestIP == "" {
		return "", fmt.Errorf("parsed IP is empty")
	}

	return bestIP, nil
}
