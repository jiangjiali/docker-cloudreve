package filesystem

import (
	"errors"
	"fmt"
	model "gitee.com/jiangjiali/cloudreve/models"
	"gitee.com/jiangjiali/cloudreve/pkg/cluster"
	"gitee.com/jiangjiali/cloudreve/pkg/conf"
	"gitee.com/jiangjiali/cloudreve/pkg/filesystem/driver"
	"gitee.com/jiangjiali/cloudreve/pkg/filesystem/driver/local"
	"gitee.com/jiangjiali/cloudreve/pkg/filesystem/driver/remote"
	"gitee.com/jiangjiali/cloudreve/pkg/filesystem/driver/shadow/masterinslave"
	"gitee.com/jiangjiali/cloudreve/pkg/filesystem/driver/shadow/slaveinmaster"
	"gitee.com/jiangjiali/cloudreve/pkg/serializer"
	"github.com/gin-gonic/gin"
	"sync"
)

// FSPool 文件系统资源池
var FSPool = sync.Pool{
	New: func() interface{} {
		return &FileSystem{}
	},
}

// FileSystem 管理文件的文件系统
type FileSystem struct {
	// 文件系统所有者
	User *model.User
	// 操作文件使用的存储策略
	Policy *model.Policy
	// 当前正在处理的文件对象
	FileTarget []model.File
	// 当前正在处理的目录对象
	DirTarget []model.Folder
	// 相对根目录
	Root *model.Folder
	// 互斥锁
	Lock sync.Mutex

	/*
	   钩子函数
	*/
	Hooks map[string][]Hook

	/*
	   文件系统处理适配器
	*/
	Handler driver.Handler

	// 回收锁
	recycleLock sync.Mutex
}

// getEmptyFS 从pool中获取新的FileSystem
func getEmptyFS() *FileSystem {
	fs := FSPool.Get().(*FileSystem)
	return fs
}

// Recycle 回收FileSystem资源
func (fs *FileSystem) Recycle() {
	fs.recycleLock.Lock()
	fs.reset()
	FSPool.Put(fs)
}

// reset 重设文件系统，以便回收使用
func (fs *FileSystem) reset() {
	fs.User = nil
	fs.CleanTargets()
	fs.Policy = nil
	fs.Hooks = nil
	fs.Handler = nil
	fs.Root = nil
	fs.Lock = sync.Mutex{}
	fs.recycleLock = sync.Mutex{}
}

// NewFileSystem 初始化一个文件系统
func NewFileSystem(user *model.User) (*FileSystem, error) {
	fs := getEmptyFS()
	fs.User = user
	fs.Policy = &fs.User.Policy

	// 分配存储策略适配器
	err := fs.DispatchHandler()

	return fs, err
}

// NewAnonymousFileSystem 初始化匿名文件系统
func NewAnonymousFileSystem() (*FileSystem, error) {
	fs := getEmptyFS()
	fs.User = &model.User{}

	// 如果是主机模式下，则为匿名文件系统分配游客用户组
	if conf.SystemConfig.Mode == "master" {
		anonymousGroup, err := model.GetGroupByID(3)
		if err != nil {
			return nil, err
		}
		fs.User.Group = anonymousGroup
	} else {
		// 从机模式下，分配本地策略处理器
		fs.Handler = local.Driver{}
	}

	return fs, nil
}

// DispatchHandler 根据存储策略分配文件适配器
func (fs *FileSystem) DispatchHandler() error {
	if fs.Policy == nil {
		return errors.New("未设置存储策略")
	}
	policyType := fs.Policy.Type
	currentPolicy := fs.Policy

	switch policyType {
	case "mock", "anonymous":
		return nil
	case "local":
		fs.Handler = local.Driver{
			Policy: currentPolicy,
		}
		return nil
	case "remote":
		handler, err := remote.NewDriver(currentPolicy)
		if err != nil {
			return err
		}
		fs.Handler = handler
	default:
		return ErrUnknownPolicyType
	}

	return nil
}

// NewFileSystemFromContext 从gin.Context创建文件系统
func NewFileSystemFromContext(c *gin.Context) (*FileSystem, error) {
	user, exist := c.Get("user")
	if !exist {
		return NewAnonymousFileSystem()
	}
	fs, err := NewFileSystem(user.(*model.User))
	return fs, err
}

// NewFileSystemFromCallback 从gin.Context创建回调用文件系统
func NewFileSystemFromCallback(c *gin.Context) (*FileSystem, error) {
	fs, err := NewFileSystemFromContext(c)
	if err != nil {
		return nil, err
	}

	// 获取回调会话
	callbackSessionRaw, ok := c.Get(UploadSessionCtx)
	if !ok {
		return nil, errors.New("upload session not exist")
	}
	callbackSession := callbackSessionRaw.(*serializer.UploadSession)

	// 重新指向上传策略
	fs.Policy = &callbackSession.Policy
	err = fs.DispatchHandler()

	return fs, err
}

// SwitchToSlaveHandler 将负责上传的 Handler 切换为从机节点
func (fs *FileSystem) SwitchToSlaveHandler(node cluster.Node) {
	fs.Handler = slaveinmaster.NewDriver(node, fs.Handler, fs.Policy)
}

// SwitchToShadowHandler 将负责上传的 Handler 切换为从机节点转存使用的影子处理器
func (fs *FileSystem) SwitchToShadowHandler(master cluster.Node, masterURL, masterID string) {
	switch fs.Policy.Type {
	case "local":
		fs.Policy.Type = "remote"
		fs.Policy.Server = masterURL
		fs.Policy.AccessKey = fmt.Sprintf("%d", master.ID())
		fs.Policy.SecretKey = master.DBModel().MasterKey
		fs.DispatchHandler()
	}

	fs.Handler = masterinslave.NewDriver(master, fs.Handler, fs.Policy)
}

// SetTargetFile 设置当前处理的目标文件
func (fs *FileSystem) SetTargetFile(files *[]model.File) {
	if len(fs.FileTarget) == 0 {
		fs.FileTarget = *files
	} else {
		fs.FileTarget = append(fs.FileTarget, *files...)
	}

}

// SetTargetDir 设置当前处理的目标目录
func (fs *FileSystem) SetTargetDir(dirs *[]model.Folder) {
	if len(fs.DirTarget) == 0 {
		fs.DirTarget = *dirs
	} else {
		fs.DirTarget = append(fs.DirTarget, *dirs...)
	}

}

// SetTargetFileByIDs 根据文件ID设置目标文件，忽略用户ID
func (fs *FileSystem) SetTargetFileByIDs(ids []uint) error {
	files, err := model.GetFilesByIDs(ids, 0)
	if err != nil || len(files) == 0 {
		return ErrFileExisted.WithError(err)
	}
	fs.SetTargetFile(&files)
	return nil
}

// SetTargetByInterface 根据 model.File 或者 model.Folder 设置目标对象
// TODO 测试
func (fs *FileSystem) SetTargetByInterface(target interface{}) error {
	if file, ok := target.(*model.File); ok {
		fs.SetTargetFile(&[]model.File{*file})
		return nil
	}
	if folder, ok := target.(*model.Folder); ok {
		fs.SetTargetDir(&[]model.Folder{*folder})
		return nil
	}

	return ErrObjectNotExist
}

// CleanTargets 清空目标
func (fs *FileSystem) CleanTargets() {
	fs.FileTarget = fs.FileTarget[:0]
	fs.DirTarget = fs.DirTarget[:0]
}
