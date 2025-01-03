package bootstrap

import (
	"context"
	"gitee.com/jiangjiali/cloudreve/models/scripts/invoker"
	"gitee.com/jiangjiali/cloudreve/pkg/util"
)

func RunScript(name string) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := invoker.RunDBScript(name, ctx); err != nil {
		util.Log().Error("Failed to execute database script: %s", err)
		return
	}

	util.Log().Info("Finish executing database script %q.", name)
}
