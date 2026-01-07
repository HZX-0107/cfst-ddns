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
