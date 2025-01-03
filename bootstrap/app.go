package bootstrap

import (
	"fmt"

	"gitee.com/jiangjiali/cloudreve/pkg/conf"
)

// InitApplication 初始化应用常量
func InitApplication() {
	fmt.Print(`云盘系统 V` + conf.BackendVersion + `
`)
}
