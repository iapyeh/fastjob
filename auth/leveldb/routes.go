/*
 Web-APIs for account management
 */
package authleveldb
import (
    fastjob "github.com/iapyeh/fastjob"
    model "github.com/iapyeh/fastjob/model"
    "fmt"
)

// ListAllUsers extends original DictAccountProvider class
func (self *DictAccountProvider) ListAllUsers() []*AppUser{
    fmt.Println("list all users called")
    return nil
}

func UserListHandler(ctx *model.RequestCtx){
    DictUserManagerSingleton.GetAccountProvider().ListAllUsers()
    fmt.Fprint(ctx.Ctx,"fuck you")
}

func UseRouteWithPrefix(prefix string){
    if prefix[0] != '/' { prefix = "/" + prefix}
    fastjob.Router.Get(prefix + "/list", UserListHandler, fastjob.ProtectMode)
}