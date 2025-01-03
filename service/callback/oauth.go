package callback

// OauthService OAuth 存储策略授权回调服务
type OauthService struct {
	Code     string `form:"code"`
	Error    string `form:"error"`
	ErrorMsg string `form:"error_description"`
	Scope    string `form:"scope"`
}
