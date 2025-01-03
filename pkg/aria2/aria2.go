package aria2

import (
	"context"
	"fmt"
	"net/url"
	"sync"
	"time"

	model "gitee.com/jiangjiali/cloudreve/models"
	"gitee.com/jiangjiali/cloudreve/pkg/aria2/common"
	"gitee.com/jiangjiali/cloudreve/pkg/aria2/monitor"
	"gitee.com/jiangjiali/cloudreve/pkg/aria2/rpc"
	"gitee.com/jiangjiali/cloudreve/pkg/balancer"
	"gitee.com/jiangjiali/cloudreve/pkg/cluster"
	"gitee.com/jiangjiali/cloudreve/pkg/mq"
)

// Instance 默认使用的Aria2处理实例
var Instance common.Aria2 = &common.DummyAria2{}

// LB 获取 Aria2 节点的负载均衡器
var LB balancer.Balancer

// Lock Instance的读写锁
var Lock sync.RWMutex

// GetLoadBalancer 返回供Aria2使用的负载均衡器
func GetLoadBalancer() balancer.Balancer {
	Lock.RLock()
	defer Lock.RUnlock()
	return LB
}

// Init 初始化
func Init(isReload bool, pool cluster.Pool, mqClient mq.MQ) {
	Lock.Lock()
	LB = balancer.NewBalancer("RoundRobin")
	Lock.Unlock()

	if !isReload {
		// 从数据库中读取未完成任务，创建监控
		unfinished := model.GetDownloadsByStatus(common.Ready, common.Paused, common.Downloading, common.Seeding)

		for i := 0; i < len(unfinished); i++ {
			// 创建任务监控
			monitor.NewMonitor(&unfinished[i], pool, mqClient)
		}
	}
}

// TestRPCConnection 发送测试用的 RPC 请求，测试服务连通性
func TestRPCConnection(server, secret string, timeout int) (rpc.VersionInfo, error) {
	// 解析RPC服务地址
	rpcServer, err := url.Parse(server)
	if err != nil {
		return rpc.VersionInfo{}, fmt.Errorf("cannot parse RPC server: %w", err)
	}

	rpcServer.Path = "/jsonrpc"
	caller, err := rpc.New(context.Background(), rpcServer.String(), secret, time.Duration(timeout)*time.Second, nil)
	if err != nil {
		return rpc.VersionInfo{}, fmt.Errorf("cannot initialize rpc connection: %w", err)
	}

	return caller.GetVersion()
}
