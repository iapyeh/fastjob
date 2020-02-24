/*
Mostly used in playground
*/

package tree

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

type DefaultBranch struct {
	BaseBranch
	treeRoot *TreeRoot
}

var emptyArray = make([]int8, 0)

func (db *DefaultBranch) BeReady(treeroot *TreeRoot) {
	db.treeRoot = treeroot
	db.InitBaseBranch()
	db.Export(
		db.Layout,
		db.RescanAPIInfo,
		db.ListUserTasks,
		db.Hook,
		db.Unhook,
	)
	treeroot.SureReady(db)
}

func (db *DefaultBranch) Layout(callCtx *TreeCallCtx) {
	layout := db.treeRoot.Layout()
	ret := make(map[string](map[string](map[string]string)))
	rootName := db.treeRoot.Name
	for branchName, exportableNames := range layout {
		key := rootName + "." + branchName
		ret[key] = make(map[string](map[string]string))
		for _, apiinfo := range exportableNames {
			key2 := (*apiinfo).Name
			ret[key][key2] = make(map[string]string)
			ret[key][key2]["Comment"] = (*apiinfo).Comment
		}
	}
	callCtx.Resolve(ret)
}

// RescanAPIInfo is called by Playground to update docstring of some exported API
func (db *DefaultBranch) RescanAPIInfo(callCtx *TreeCallCtx) {
	if len(callCtx.Args) < 1 {
		callCtx.Reject(304, errors.New("not enought arguments"))
		return
	}
	//ex. Member.$chat.Talk
	callpaths := strings.Split(callCtx.Args[0], ".")
	if db.treeRoot.Name == callpaths[0] {
		apiName := strings.Join(callpaths[1:], ".")
		ret := make(map[string]map[string]string)
		if changed := db.treeRoot.RescanAPIInfo(apiName); changed != nil {
			for apiName, docitemAddr := range *changed {
				ret[apiName] = make(map[string]string)
				ret[apiName]["Comment"] = (*docitemAddr).Comment
			}
			callCtx.Resolve(ret)
		} else {
			callCtx.Resolve(ret)
		}
		return
	}
	callCtx.Reject(304, errors.New(callCtx.Args[0]+" not found"))
}

//ListUserTasks is called by Playground to restore background tasks if any.
func (db *DefaultBranch) ListUserTasks(tcCtx *TreeCallCtx) {
	//Should be accessed after login
	user := tcCtx.WsCtx.GetUser()
	if user == nil {
		tcCtx.Resolve(emptyArray)
	}
	if tcCtxs, err := db.treeRoot.Bank.ListUser(user); err == nil {
		ret := make([]string, len(tcCtxs))
		for i, tcCtx := range tcCtxs {
			ret[i] = fmt.Sprintf("%v\t%v\t%v\t%v\t%v", tcCtx.CmdID, tcCtx.CmdPath, strings.Join(tcCtx.Args, ", "), tcCtx.Kw.String(), tcCtx.Ctime)
		}
		tcCtx.Resolve(ret)
	} else {
		tcCtx.Resolve(emptyArray) //empty array
	}
}

//Hook is called by Playground to hook-up output of background tasks if any.
func (db *DefaultBranch) Hook(tcCtx *TreeCallCtx) {
	user := tcCtx.WsCtx.GetUser()
	if user == nil {
		tcCtx.Resolve(emptyArray)
	}
	if len(tcCtx.Args) < 1 {
		// no cmdID is given
		tcCtx.Reject(304, errors.New("not enought arguments"))
		return
	}
	cmdID, err := strconv.ParseInt(tcCtx.Args[0], 10, 64)
	if err != nil {
		// given cmdID is not legal cmdID
		tcCtx.Reject(304, err)
		return
	}
	ctx := db.treeRoot.Bank.Get(int32(cmdID))
	if ctx == nil {
		// this cmdID is not a background task
		tcCtx.Reject(304, err)
		return
	}
	if err := tcCtx.HookTo(ctx); err == nil {
		defer tcCtx.Resolve(1)
	} else {
		defer tcCtx.Reject(304, err)
	}
}
func (db *DefaultBranch) Unhook(tcCtx *TreeCallCtx) {
	user := tcCtx.WsCtx.GetUser()
	if user == nil {
		tcCtx.Resolve(emptyArray)
	}
	if len(tcCtx.Args) < 1 {
		// no cmdID is given
		tcCtx.Reject(304, errors.New("not enought arguments"))
		return
	}
	cmdID, err := strconv.ParseInt(tcCtx.Args[0], 10, 64)
	if err != nil {
		// given cmdID is not legal cmdID
		tcCtx.Reject(304, err)
		return
	}
	ctx := db.treeRoot.Bank.Get(int32(cmdID))
	if ctx == nil {
		// this cmdID is not a background task
		tcCtx.Reject(304, err)
		return
	}
	if err := tcCtx.UnHookFrom(ctx); err == nil {
		defer tcCtx.Resolve(1)
	} else {
		defer tcCtx.Reject(304, err)
	}
}
