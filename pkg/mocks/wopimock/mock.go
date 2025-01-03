package wopimock

import (
	model "gitee.com/jiangjiali/cloudreve/models"
	"gitee.com/jiangjiali/cloudreve/pkg/wopi"
	"github.com/stretchr/testify/mock"
)

type WopiClientMock struct {
	mock.Mock
}

func (w *WopiClientMock) NewSession(user uint, file *model.File, action wopi.ActonType) (*wopi.Session, error) {
	args := w.Called(user, file, action)
	return args.Get(0).(*wopi.Session), args.Error(1)
}

func (w *WopiClientMock) AvailableExts() []string {
	args := w.Called()
	return args.Get(0).([]string)
}
