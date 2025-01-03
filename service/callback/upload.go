package callback

import (
	"context"
	model "gitee.com/jiangjiali/cloudreve/models"
	"gitee.com/jiangjiali/cloudreve/pkg/filesystem"
	"gitee.com/jiangjiali/cloudreve/pkg/filesystem/fsctx"
	"gitee.com/jiangjiali/cloudreve/pkg/serializer"
	"github.com/gin-gonic/gin"
)

// CallbackProcessService 上传请求回调正文接口
type CallbackProcessService interface {
	GetBody() serializer.UploadCallback
}

// RemoteUploadCallbackService 远程存储上传回调请求服务
type RemoteUploadCallbackService struct {
	Data serializer.UploadCallback `json:"data" binding:"required"`
}

// GetBody 返回回调正文
func (service RemoteUploadCallbackService) GetBody() serializer.UploadCallback {
	return service.Data
}

// ProcessCallback 处理上传结果回调
func ProcessCallback(service CallbackProcessService, c *gin.Context) serializer.Response {
	callbackBody := service.GetBody()

	// 创建文件系统
	fs, err := filesystem.NewFileSystemFromCallback(c)
	if err != nil {
		return serializer.Err(serializer.CodeCreateFSError, err.Error(), err)
	}
	defer fs.Recycle()

	// 获取上传会话
	uploadSession := c.MustGet(filesystem.UploadSessionCtx).(*serializer.UploadSession)

	// 查找上传会话创建的占位文件
	file, err := model.GetFilesByUploadSession(uploadSession.Key, fs.User.ID)
	if err != nil {
		return serializer.Err(serializer.CodeUploadSessionExpired, "LocalUpload session file placeholder not exist", err)
	}

	fileData := fsctx.FileStream{
		Size:         uploadSession.Size,
		Name:         uploadSession.Name,
		VirtualPath:  uploadSession.VirtualPath,
		SavePath:     uploadSession.SavePath,
		Mode:         fsctx.Nop,
		Model:        file,
		LastModified: uploadSession.LastModified,
	}

	// 占位符未扣除容量需要校验和扣除
	if !fs.Policy.IsUploadPlaceholderWithSize() {
		fs.Use("AfterUpload", filesystem.HookValidateCapacity)
		fs.Use("AfterUpload", filesystem.HookChunkUploaded)
	}

	fs.Use("AfterUpload", filesystem.HookPopPlaceholderToFile(callbackBody.PicInfo))
	fs.Use("AfterValidateFailed", filesystem.HookDeleteTempFile)
	err = fs.Upload(context.Background(), &fileData)
	if err != nil {
		return serializer.Err(serializer.CodeUploadFailed, err.Error(), err)
	}

	return serializer.Response{}
}
