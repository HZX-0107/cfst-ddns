package dns

import (
	"errors"
	"fmt"
	"log"

	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	tcerr "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/errors" // 关键：给 SDK 的 errors 包起别名 tcerr
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	dnspod "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/dnspod/v20210323"

	"cfst-ddns/internal/config"
)

// TencentClient 封装腾讯云 DNSPod 客户端
type TencentClient struct {
	api *dnspod.Client
}

// NewTencentClient 初始化腾讯云客户端
func NewTencentClient(cfg *config.TencentConfig) (*TencentClient, error) {
	credential := common.NewCredential(cfg.SecretID, cfg.SecretKey)

	// 配置 API 接入点 (使用 API 3.0)
	cpf := profile.NewClientProfile()
	cpf.HttpProfile.Endpoint = "dnspod.tencentcloudapi.com"

	client, err := dnspod.NewClient(credential, "", cpf)
	if err != nil {
		return nil, fmt.Errorf("failed to init tencent client: %w", err)
	}

	return &TencentClient{api: client}, nil
}

// UpdateRecord 智能更新 DNS 记录 (自动判断是创建还是修改)
// domain: 主域名 (例如 hzx17.xyz)
// subDomain: 子域名 (例如 cf)
// recordType: 记录类型 (A 或 AAAA)
// value: IP 地址
func (c *TencentClient) UpdateRecord(domain, subDomain, recordType, value string) error {
	log.Printf("[Tencent] Checking DNS record for %s.%s (%s)...", subDomain, domain, recordType)

	// 1. 查询现有的记录 ID
	recordId, err := c.getRecordID(domain, subDomain, recordType)
	if err != nil {
		return err
	}

	if recordId != nil {
		// 2a. 如果记录存在，则更新
		log.Printf("[Tencent] Record found (ID: %d), updating to %s...", *recordId, value)
		return c.modifyRecord(domain, subDomain, recordType, value, *recordId)
	}

	// 2b. 如果记录不存在，则创建
	log.Printf("[Tencent] Record not found, creating new record %s -> %s...", subDomain, value)
	return c.createRecord(domain, subDomain, recordType, value)
}

// getRecordID 获取指定记录的 ID，如果不存在返回 nil
func (c *TencentClient) getRecordID(domain, subDomain, recordType string) (*uint64, error) {
	req := dnspod.NewDescribeRecordListRequest()
	req.Domain = common.StringPtr(domain)
	req.Subdomain = common.StringPtr(subDomain)
	req.RecordType = common.StringPtr(recordType)

	resp, err := c.api.DescribeRecordList(req)
	if err != nil {
		return nil, fmt.Errorf("api error (describe): %w", err)
	}

	if len(resp.Response.RecordList) > 0 {
		return resp.Response.RecordList[0].RecordId, nil
	}
	return nil, nil
}

// createRecord 创建新记录
func (c *TencentClient) createRecord(domain, subDomain, recordType, value string) error {
	req := dnspod.NewCreateRecordRequest()
	req.Domain = common.StringPtr(domain)
	req.SubDomain = common.StringPtr(subDomain)
	req.RecordType = common.StringPtr(recordType)
	req.RecordLine = common.StringPtr("默认")
	req.Value = common.StringPtr(value)

	_, err := c.api.CreateRecord(req)
	if err != nil {
		return fmt.Errorf("api error (create): %w", err)
	}
	log.Println("[Tencent] DNS record created successfully.")
	return nil
}

// modifyRecord 修改现有记录
func (c *TencentClient) modifyRecord(domain, subDomain, recordType, value string, recordId uint64) error {
	req := dnspod.NewModifyRecordRequest()
	req.Domain = common.StringPtr(domain)
	req.SubDomain = common.StringPtr(subDomain)
	req.RecordType = common.StringPtr(recordType)
	req.RecordLine = common.StringPtr("默认")
	req.Value = common.StringPtr(value)
	req.RecordId = common.Uint64Ptr(recordId)

	_, err := c.api.ModifyRecord(req)
	if err != nil {
		// 忽略 "记录内容没有变化" 的错误
		// 忽略 "记录内容没有变化" 的错误
		// 使用 errors.As 进行更稳健的错误解包和类型断言
		var sdkErr *tcerr.TencentCloudSDKError
		if errors.As(err, &sdkErr) && sdkErr.Code == "InvalidParameter.RecordValueInvalid" {
			log.Println("[Tencent] DNS record value is already up to date.")
			return nil
		}
		return fmt.Errorf("api error (modify): %w", err)
	}
	log.Println("[Tencent] DNS record updated successfully.")
	return nil
}
