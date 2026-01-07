package assets

import (
	_ "embed"
)

//go:embed cfst
var CFSTBinary []byte

//go:embed ip.txt
var IPList []byte

//go:embed ipv6.txt
var IPv6List []byte

// [新增] 嵌入默认配置文件模板
// 请确保 assets 目录下存在 config-example.yml 文件

//go:embed config-example.yml
var DefaultConfig []byte
