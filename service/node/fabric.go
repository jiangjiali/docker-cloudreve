package node

import (
	"encoding/gob"
	"gitee.com/jiangjiali/cloudreve/pkg/cluster"
	"gitee.com/jiangjiali/cloudreve/pkg/mq"
	"gitee.com/jiangjiali/cloudreve/pkg/serializer"
	"github.com/gin-gonic/gin"
)

type SlaveNotificationService struct {
	Subject string `uri:"subject" binding:"required"`
}

type OauthCredentialService struct {
	PolicyID uint `uri:"id" binding:"required"`
}

func HandleMasterHeartbeat(req *serializer.NodePingReq) serializer.Response {
	res, err := cluster.DefaultController.HandleHeartBeat(req)
	if err != nil {
		return serializer.Err(serializer.CodeInternalSetting, "Cannot initialize slave controller", err)
	}

	return serializer.Response{
		Code: 0,
		Data: res,
	}
}

// HandleSlaveNotificationPush 转发从机的消息通知到本机消息队列
func (s *SlaveNotificationService) HandleSlaveNotificationPush(c *gin.Context) serializer.Response {
	var msg mq.Message
	dec := gob.NewDecoder(c.Request.Body)
	if err := dec.Decode(&msg); err != nil {
		return serializer.ParamErr("Cannot parse notification message", err)
	}

	mq.GlobalMQ.Publish(s.Subject, msg)
	return serializer.Response{}
}
