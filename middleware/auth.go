package middleware

import (
	"gitee.com/jiangjiali/cloudreve/pkg/filesystem"
	"net/http"

	model "gitee.com/jiangjiali/cloudreve/models"
	"gitee.com/jiangjiali/cloudreve/pkg/auth"
	"gitee.com/jiangjiali/cloudreve/pkg/cache"
	"gitee.com/jiangjiali/cloudreve/pkg/serializer"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

const (
	CallbackFailedStatusCode = http.StatusUnauthorized
)

// SignRequired 验证请求签名
func SignRequired(authInstance auth.Auth) gin.HandlerFunc {
	return func(c *gin.Context) {
		var err error
		switch c.Request.Method {
		case "PUT", "POST", "PATCH":
			err = auth.CheckRequest(authInstance, c.Request)
		default:
			err = auth.CheckURI(authInstance, c.Request.URL)
		}

		if err != nil {
			c.JSON(200, serializer.Err(serializer.CodeCredentialInvalid, err.Error(), err))
			c.Abort()
			return
		}

		c.Next()
	}
}

// CurrentUser 获取登录用户
func CurrentUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)
		uid := session.Get("user_id")
		if uid != nil {
			user, err := model.GetActiveUserByID(uid)
			if err == nil {
				c.Set("user", &user)
			}
		}
		c.Next()
	}
}

// AuthRequired 需要登录
func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		if user, _ := c.Get("user"); user != nil {
			if _, ok := user.(*model.User); ok {
				c.Next()
				return
			}
		}

		c.JSON(200, serializer.CheckLogin())
		c.Abort()
	}
}

// WebDAVAuth 验证WebDAV登录及权限
func WebDAVAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// OPTIONS 请求不需要鉴权，否则Windows10下无法保存文档
		if c.Request.Method == "OPTIONS" {
			c.Next()
			return
		}

		username, password, ok := c.Request.BasicAuth()
		if !ok {
			c.Writer.Header()["WWW-Authenticate"] = []string{`Basic realm="cloudreve"`}
			c.Status(http.StatusUnauthorized)
			c.Abort()
			return
		}

		expectedUser, err := model.GetActiveUserByEmail(username)
		if err != nil {
			c.Status(http.StatusUnauthorized)
			c.Abort()
			return
		}

		// 密码正确？
		webdav, err := model.GetWebdavByPassword(password, expectedUser.ID)
		if err != nil {
			c.Status(http.StatusUnauthorized)
			c.Abort()
			return
		}

		// 用户组已启用WebDAV？
		if !expectedUser.Group.WebDAVEnabled {
			c.Status(http.StatusForbidden)
			c.Abort()
			return
		}

		// 用户组已启用WebDAV代理？
		if !expectedUser.Group.OptionsSerialized.WebDAVProxy {
			webdav.UseProxy = false
		}

		c.Set("user", &expectedUser)
		c.Set("webdav", webdav)
		c.Next()
	}
}

// UseUploadSession 对上传会话进行验证
func UseUploadSession(policyType string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 验证key并查找用户
		resp := uploadCallbackCheck(c, policyType)
		if resp.Code != 0 {
			c.JSON(CallbackFailedStatusCode, resp)
			c.Abort()
			return
		}

		c.Next()
	}
}

// uploadCallbackCheck 对上传回调请求的 callback key 进行验证，如果成功则返回上传用户
func uploadCallbackCheck(c *gin.Context, policyType string) serializer.Response {
	// 验证 Callback Key
	sessionID := c.Param("sessionID")
	if sessionID == "" {
		return serializer.ParamErr("Session ID cannot be empty", nil)
	}

	callbackSessionRaw, exist := cache.Get(filesystem.UploadSessionCachePrefix + sessionID)
	if !exist {
		return serializer.Err(serializer.CodeUploadSessionExpired, "上传会话不存在或已过期", nil)
	}

	callbackSession := callbackSessionRaw.(serializer.UploadSession)
	c.Set(filesystem.UploadSessionCtx, &callbackSession)
	if callbackSession.Policy.Type != policyType {
		return serializer.Err(serializer.CodePolicyNotAllowed, "", nil)
	}

	// 清理回调会话
	_ = cache.Deletes([]string{sessionID}, filesystem.UploadSessionCachePrefix)

	// 查找用户
	user, err := model.GetActiveUserByID(callbackSession.UID)
	if err != nil {
		return serializer.Err(serializer.CodeUserNotFound, "", err)
	}
	c.Set(filesystem.UserCtx, &user)
	return serializer.Response{}
}

// RemoteCallbackAuth 远程回调签名验证
func RemoteCallbackAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 验证签名
		session := c.MustGet(filesystem.UploadSessionCtx).(*serializer.UploadSession)
		authInstance := auth.HMACAuth{SecretKey: []byte(session.Policy.SecretKey)}
		if err := auth.CheckRequest(authInstance, c.Request); err != nil {
			c.JSON(CallbackFailedStatusCode, serializer.Err(serializer.CodeCredentialInvalid, err.Error(), err))
			c.Abort()
			return
		}

		c.Next()

	}
}

// IsAdmin 必须为管理员用户组
func IsAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, _ := c.Get("user")
		if user.(*model.User).Group.ID != 1 && user.(*model.User).ID != 1 {
			c.JSON(200, serializer.Err(serializer.CodeNoPermissionErr, "", nil))
			c.Abort()
			return
		}

		c.Next()
	}
}
