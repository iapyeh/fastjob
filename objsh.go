package objsh

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	model "github.com/iapyeh/fastjob/model"
	tree "github.com/iapyeh/fastjob/tree"
	_ "github.com/valyala/fasthttp"
)

var Router = model.Router
var LoginHandler = model.LoginHandler
var LogoutHandler = model.LogoutHandler

const (
	ProtectMode = model.ProtectMode
	PublicMode  = model.PublicMode
	TraceMode   = model.TraceMode
)
func UseTreeWithOptions(name string, urlPath string, acl int, options *model.TreeOptions) *TreeRoot {
	if Router.HasRegistered(urlPath) {
		panic("Can not use tree at " + urlPath + ", because it has been occupied")
		return nil
	}
	treeRoot := model.NewTreeRootWithName(name)
	//所有tree都需要的branch(?資安風險？)
	treeRoot.AddBranchWithName(&tree.DefaultBranch{},"$")

	treeCallHandler := tree.TreeCallHandler{Root: treeRoot}

	Router.WebsocketWithOptions(urlPath, treeCallHandler.Handler, acl,options.WebsocketOptions)

	return treeRoot
}
func UseTree(name string, urlPath string, acl int) *TreeRoot {
    return UseTreeWithOptions(name,urlPath,acl,&model.TreeOptions{})
}
func AddSystemBranches(treeRoot *TreeRoot) {
	treeRoot.AddBranch(&tree.ChatBranch{})
	treeRoot.AddBranch(&tree.ExecBranch{})

	// 2019-11-12T13:31:02+00:00
	// PythonBranch has moved to fastjob-python
	// Todo: add it back
	//treeRoot.AddBranch(&tree.PythonBranch{})
}

/*

Args:[objshStaicFolder]

@objshStaicFolder:
    objshStaicFolder is the folder which links to fastjob/static.
    It has "playground" and "file" subfolders.
*/
func UsePlayground(objshStaicFolder string) {

	if _, err := os.Stat(objshStaicFolder); os.IsNotExist(err) {
		fmt.Errorf("fastjob's static file is not existed:%v", objshStaicFolder)
	}

	Router.Post("/playground/login", LoginHandler, PublicMode)
	Router.Get("/playground/login", LoginHandler, PublicMode)
	Router.Get("/playground/logout", LogoutHandler, ProtectMode)
	Router.File("/playground/file", filepath.Join(objshStaicFolder, "playground"), PublicMode)
	Router.File("/playgroun/fastjob", filepath.Join(objshStaicFolder, "file"), PublicMode)
	log.Println("[Debug] Playground url: /playground/file/index.html")

	playgroundTree := UseTree("Playground", "/playground/tree", model.ProtectMode)
	AddSystemBranches(playgroundTree)

	//注意：需動態產生playground.js可以存取的 tree ，此功能還沒實作

	//Python3
	//Py3 := objshpy.NewPy3()
	//Py3.AddTree(playgroundTree)
	//Py3.ImportModule("pytest/pybranches.py")

	playgroundTree.BeReady()
	playgroundTree.Dump()
}

var authProvider model.AuthProvider

func UseAuthentication(userManager model.AuthProvider) {
	if model.DefaultTokenGenerator == nil {
		model.SetTokenGenerator(model.SimpleTokenGenerator)
	}
	if model.DefaultPasswordHasher == nil {
		model.SetPasswordHasher(model.SimplePasswordHasher)
	}
	authProvider = userManager.(model.AuthProvider)
	model.AuthProvierSingleton = authProvider
}

// Expose stuffs in subpackage for user
// 拉到objsh來，這樣使用objsh的專案
// 不需要import其他像是 tree, model之類的
type BaseBranch = model.BaseBranch
type TreeRoot = model.TreeRoot
type TreeCallCtx = model.TreeCallCtx
type Exportable = model.Exportable
type BaseAuthProvider = model.BaseAuthProvider
type Dict = model.Dict
type User = model.User
type WebsocketCtx = model.WebsocketCtx

var NormalizeUsername = model.NormalizeUsername

//Utilities
var SetTimeout = model.SetTimeout
var SetInterval = model.SetInterval
