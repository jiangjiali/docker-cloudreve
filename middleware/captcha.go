package middleware

import (
	"bytes"
	"encoding/json"
	model "gitee.com/jiangjiali/cloudreve/models"
	"gitee.com/jiangjiali/cloudreve/pkg/recaptcha"
	"gitee.com/jiangjiali/cloudreve/pkg/serializer"
	"gitee.com/jiangjiali/cloudreve/pkg/util"
	"github.com/gin-gonic/gin"
	"github.com/mojocn/base64Captcha"
	"io"
	"io/ioutil"
	"time"
)

type req struct {
	CaptchaCode string `json:"captchaCode"`
	Ticket      string `json:"ticket"`
	Randstr     string `json:"randstr"`
}

const (
	captchaNotMatch = "CAPTCHA not match."
	captchaRefresh  = "Verification failed, please refresh the page and retry."
)

// CaptchaRequired 验证请求签名
func CaptchaRequired(configName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 相关设定
		options := model.GetSettingByNames(configName,
			"captcha_type",
			"captcha_ReCaptchaSecret")
		// 检查验证码
		isCaptchaRequired := model.IsTrueVal(options[configName])

		if isCaptchaRequired {
			var service req
			bodyCopy := new(bytes.Buffer)
			_, err := io.Copy(bodyCopy, c.Request.Body)
			if err != nil {
				c.JSON(200, serializer.Err(serializer.CodeCaptchaError, captchaNotMatch, err))
				c.Abort()
				return
			}

			bodyData := bodyCopy.Bytes()
			err = json.Unmarshal(bodyData, &service)
			if err != nil {
				c.JSON(200, serializer.Err(serializer.CodeCaptchaError, captchaNotMatch, err))
				c.Abort()
				return
			}

			c.Request.Body = ioutil.NopCloser(bytes.NewReader(bodyData))
			switch options["captcha_type"] {
			case "normal":
				captchaID := util.GetSession(c, "captchaID")
				util.DeleteSession(c, "captchaID")
				if captchaID == nil || !base64Captcha.VerifyCaptcha(captchaID.(string), service.CaptchaCode) {
					c.JSON(200, serializer.Err(serializer.CodeCaptchaError, captchaNotMatch, err))
					c.Abort()
					return
				}

				break
			case "recaptcha":
				reCAPTCHA, err := recaptcha.NewReCAPTCHA(options["captcha_ReCaptchaSecret"], recaptcha.V2, 10*time.Second)
				if err != nil {
					util.Log().Warning("reCAPTCHA verification failed, %s", err)
					c.Abort()
					break
				}

				err = reCAPTCHA.Verify(service.CaptchaCode)
				if err != nil {
					util.Log().Warning("reCAPTCHA verification failed, %s", err)
					c.JSON(200, serializer.Err(serializer.CodeCaptchaRefreshNeeded, captchaRefresh, nil))
					c.Abort()
					return
				}

				break
			}
		}
		c.Next()
	}
}
