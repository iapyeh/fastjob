// 希望基於unittest為sample開發的app
// 其他leveldb.go 可以不動,只要修改這個user.go檔案就可以
// 希望能達成這個目標
package authleveldb

import (
	model "github.com/iapyeh/fastjob/model"
)

type Avatar = model.Avatar

var NormalizeUsername = model.NormalizeUsername

// AppUser 繼承Avatar，因此可以用於fastjob的登入認證
type AppUser struct {
	Avatar `json:"avatar"`
	//Add extra fields of what you want beyond authentication required fields(avatar)
	//Role        string //such as "admin"
	//Email       string //maybe for account activation and password recovery
	//DisplayName string
}

// NewUser is a helper to create User instance
// @username: empty string "" will set the Username to be the same as user.Uuid
func NewUser(username string) AppUser {
	user := AppUser{}
	user.SetUsername(username)
	return user
}
