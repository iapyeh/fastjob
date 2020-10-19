/*
Tree Call系統有意義：
一、他提供階層式的方式，組織API呼叫。易於管理、累積（使用相同的API）以及讓前台工程師理解。
二、每一個呼叫都是一個Task. 與RESTful相比，RESTful是放置資料的概念，而Task的概念是執行Job的觀念。
Task的特性是：
	Task的生命週期與web request的週期相關，相對於process的生命週期與OS開機相關。
	- Task的應用廣，執行時間可能很長，讀寫資料只是其中一種Task
	- Task 要能被取消。
	- Task 在離線時，要能自動取消。
	- Task 在離線時，要能繼續執行。
	- Task 在重新上線時，要能取得執行結果。
	- Task 執行失敗或取消時，可能需要比較多的清理（Task需更依賴於伺服器狀態）
	- Task 有中途狀態的輸出，不只是progress bar的比例而已，task的步驟是比較複雜的。
	- Task 可以執行一個無限循環的函式，並且取消，或者在連線中斷時自動取消。RESTful API不行。

Tree Call 系統的策略
	- 所有task在離線時，一律自動關閉。task author需在OnClose事件處理相關清理。
	- 如果task要繼續執行，需向task manager註冊。

Task vs gRPC
	- Task is web-based, it bridges two kinds of person: backend and frontend engineers
		gPRC is totally backend. gPRC's client is app, Task's client is Browser.
	- gPRC's server is stateless. But Task's server is web server, which enriched with
		document, images, authentication, .....
	- gPRC has no idea of communities, but Task is based on the community of a web server.
		gPRC has no idea of "user" and "inter-users" feature, which Task/Web take for granted.
*/

package model

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	proto "github.com/golang/protobuf/proto"
	"github.com/valyala/fasthttp"
)

type TreeOptions struct{
    WebsocketOptions *WebsocketOptions
}

type TreeCallReturn struct {
	CmdID   int32
	Retcode int32
	Stderr  error
	Stdout  interface{}
}

// TreeCallCtxBank temporary stores TreeCallCtx before it is termincalted.
// It makes a TreeCall be cancelable.
type TreeCallCtxBank struct {
	storeByID   map[int32]*TreeCallCtx
	storeByUser map[string][]int32 //username : []tcCtx.CmdID
	Mutex       sync.RWMutex
}

func (bank *TreeCallCtxBank) Put(tcCtx *TreeCallCtx) error {
	bank.Mutex.Lock()
	defer bank.Mutex.Unlock()
	bank.storeByID[tcCtx.CmdID] = tcCtx
    //log.Println("put to bank",tcCtx.CmdID)
    user := tcCtx.WsCtx.GetUser();
    if user != nil {
		username := user.Username()
		if cmdIDs, ok := bank.storeByUser[username]; ok {
			bank.storeByUser[username] = append(cmdIDs, tcCtx.CmdID)
		} else {
			bank.storeByUser[username] = []int32{tcCtx.CmdID}
		}
    }
    return nil
}
func (bank *TreeCallCtxBank) Get(CmdID int32) *TreeCallCtx {
	bank.Mutex.RLock()
	defer bank.Mutex.RUnlock()
	if callCtx, ok := bank.storeByID[CmdID]; ok {
		return callCtx
    }
    
    // Maybe this job has been resolved or rejectd
	//fmt.Println("missed: get from store, size", len(bank.storeByID))    
    return nil
}
func (bank *TreeCallCtxBank) Del(CmdID int32) *TreeCallCtx {
    //fmt.Println("bank deleting",CmdID)
	bank.Mutex.Lock()
    defer bank.Mutex.Unlock()
	if tcCtx, ok := bank.storeByID[CmdID]; ok {
		delete(bank.storeByID, CmdID)
		var username string
		user := tcCtx.WsCtx.GetUser()
		if user == nil {
			// In case of background task, User is nil if original user kill this task
			// after he has closed browser and open again
			username = strings.Split(tcCtx.CmdPath, "\t")[1]
		} else {
			username = user.Username()
		}
		if cmdIDs, ok := bank.storeByUser[username]; ok {
			var idx = int(-1)
			for i, cmdID := range cmdIDs {
				if cmdID == CmdID {
					idx = i
					break
				}
			}
			if idx >= 0 {
				if len(cmdIDs) == 1 {
					delete(bank.storeByUser, username)
				} else {
					bank.storeByUser[username] = append(cmdIDs[:idx], cmdIDs[idx+1:]...)
				}
			}
		}
		//log.Println("Bank after del size:", len(bank.storeByID), len(bank.storeByUser))
		return tcCtx
	}
	return nil
}
func (bank *TreeCallCtxBank) DelByUser(username string) bool {
	//not implemented yet
	return false
}

//ListUser return  bank content of a user
func (bank *TreeCallCtxBank) ListUser(user User) ([]*TreeCallCtx, error) {
	bank.Mutex.RLock()
	defer bank.Mutex.RUnlock()
	if cmdIDs, ok := bank.storeByUser[user.Username()]; ok {
		ret := make([]*TreeCallCtx, len(cmdIDs))
		for i, cmdID := range cmdIDs {
			if tcCtx, ok := bank.storeByID[cmdID]; ok {
				ret[i] = tcCtx
			}
		}
		return ret, nil
	}
	return nil, errors.New("None")
}
func NewTreeCallCtxBank() *TreeCallCtxBank {
	return &TreeCallCtxBank{
		storeByID:   make(map[int32]*TreeCallCtx),
		storeByUser: make(map[string][]int32),
	}
}

// PromiseStateListener is an abstract of WebsocketCtx
// 為了可以internally 互相呼叫，因此把WebsocketCtx升級為interface
type PromiseStateListener interface {
	// 這幾個都是在管理狀態，適時結束task
	On(string, string, interface{}, ...interface{}) bool
	Off(string, string) bool
	IsClosed() bool
	//為此call而生的task，做出一個unique value；例如用於加入與退出 Chatting room
	GenID() string
	// 回傳 (int, error) 是有點怪，這是因為之前是websocket的緣故，以後再改（2019-09-09T00:21:43+00:00)
	SendTreeCallReturn(*TreeCallReturn) (int, error)
	// 直接送結果，適用於已經encode成json的資料，例如python的branch結果（已經在C中encode為json)
	SendProtobufMessage(proto.Message) (int, error) //proto.Message is an interface
	GetUser() User
}

// SimplePromiseStateListener is an implementation of PromiseStateListener
// for internal call or inter-branch call
type SimplePromiseStateListenerCallback func(*TreeCallReturn)
type SimplePromiseStateListener struct {
	user     User
	id       string
	callback SimplePromiseStateListenerCallback
}

func (listener *SimplePromiseStateListener) GetUser() User {
	return listener.user
}
func (listener *SimplePromiseStateListener) GenID() string {
	return listener.id
}
func (listener *SimplePromiseStateListener) IsClosed() bool {
	return false
}
func (listener *SimplePromiseStateListener) On(evtname string, token2remove string, dummy interface{}, alsoDummy ...interface{}) bool {
	return true
}
func (listener *SimplePromiseStateListener) Off(evtname string, token2remove string) bool {
	return true
}
func (listener *SimplePromiseStateListener) SendTreeCallReturn(ret *TreeCallReturn) (int, error) {
	if listener.callback != nil {
		listener.callback(ret)
	}
	return 0, nil
}
func (listener *SimplePromiseStateListener) SendProtobufMessage(mesg proto.Message) (int, error) {
	return 0, nil
}
func NewInternalCallPromiseListener(user User, id string, callback SimplePromiseStateListenerCallback) PromiseStateListener {
	listener := &SimplePromiseStateListener{user, id, callback}
	return listener
}

// Promise bridges the task and the browser
// For a background task, it might associated to differnt Promises.
// This is a helper for TreeCallCtx
// Promise只是借用js的名字，取得結果不是 promise.resolve(callback)的方式，
// 而是要implement PromiseStateListener。 因為需要從 PromiseStateListener 的 On, Off, IsClosed
// 取得task是否要繼續執行或中斷的資訊。 執行結果的處理在 SendTreeCallReturn 內implement，
// 因為task結束之後會呼叫它。

type Promise struct {
	CmdID int32 //CmdID of the TreeCallCtx which the promise belongs
	// major websocket, creator's websocket,
	// this value is nil, when websocket is closed
	stateListener PromiseStateListener //*WebsocketCtx
	onAndOffID    string
	// listener's websocket (added by hooking up)
	hookedStateListeners []PromiseStateListener //*WebsocketCtx
	mutex                sync.RWMutex
}

func NewPromise(cmdID int32, wsCtx PromiseStateListener) *Promise {
	onAndOffID := "_ps" + strconv.FormatInt(int64(cmdID), 10)
	p := Promise{
		CmdID:         cmdID,
		stateListener: wsCtx,
		onAndOffID:    onAndOffID,
	}
	wsCtx.On("Close", onAndOffID, func() {
		p.mutex.Lock()
		p.stateListener = nil
		p.mutex.Unlock()
	})
	return &p
}

//clean is called when task resolved or rejected.
func (p *Promise) clean() {
    //fmt.Println("promise of ",p.CmdID," clean() been called")
	if p.stateListener != nil {
		p.stateListener.Off("Close", p.onAndOffID)
		p.mutex.Lock()
		p.stateListener = nil
		p.mutex.Unlock()
	}
	if p.hookedStateListeners == nil {
		return
	}
	p.mutex.RLock()
	for _, stateListener := range p.hookedStateListeners {
		stateListener.Off("Close", p.onAndOffID)
	}
	p.mutex.RUnlock()
	p.mutex.Lock()
	p.hookedStateListeners = nil
	p.mutex.Unlock()
}
// Put will hook state listener to this promise
func (p *Promise) Put(wsCtx PromiseStateListener) error {
	if wsCtx.IsClosed() {
		return errors.New("Put a dead websocket is unnormal")
	}
	p.mutex.Lock()
	if p.hookedStateListeners == nil {
		//p.wsCtxes = make([]*WebsocketCtx, 1)
		p.hookedStateListeners = make([]PromiseStateListener, 1)
		p.hookedStateListeners[0] = wsCtx
	} else {
		for _, w := range p.hookedStateListeners {
			if w == wsCtx {
				//already in list
				p.mutex.Unlock()
				return errors.New("duplication put")
			}
		}
		p.hookedStateListeners = append(p.hookedStateListeners, wsCtx)
	}
	p.mutex.Unlock()
	wsCtx.On("Close", p.onAndOffID, func() {
		fmt.Println("removing ", wsCtx)
		p.Del(wsCtx)
	})
	return nil
}

// Del will remove state listener from hookedStateListeners
func (p *Promise) Del(wsCtx PromiseStateListener) error {
	if p.hookedStateListeners == nil { //Not found
		return errors.New("not found")
	}
	idx := -1
	p.mutex.RLock()
	for i, ws := range p.hookedStateListeners {
		if ws == wsCtx {
			idx = i
			break
		}
	}
	p.mutex.RUnlock()
	fmt.Println("found idx=", idx)
	if idx == -1 { //Not found
		log.Println("promiseStateListener is not found to delete")
		return errors.New("promiseStateListener is not found to delete")
	}
	wsCtx.Off("Close", p.onAndOffID)
	p.mutex.Lock()
	if len(p.hookedStateListeners) == 1 {
		p.hookedStateListeners = nil
		log.Println("p.wsCtxes is empty now")
	} else {
		p.hookedStateListeners = append(p.hookedStateListeners[:idx], p.hookedStateListeners[idx+1:]...)
		log.Println("p.wsCtxes after delete is of size:", len(p.hookedStateListeners))
	}
	p.mutex.Unlock()
	return nil
}

// ResolveResult send protobuffer "Result" message directly
func (p *Promise) DirectResult(result *Result) {
	p.mutex.RLock()
	if p.stateListener == nil {
		//skip
	} else if p.stateListener.IsClosed() {
		log.Println("[warn] DirectResult called but connection closed")
	} else {
		p.stateListener.SendProtobufMessage(result)
	}
    //log.Println("directresult of length====>:",len(result.Stdout))
	// 不必測試 "&& len(p.hookedStateListeners) > 0 ",
	// 因為hookedStateListeners是動態建立起來的，如果hookedStateListeners是空，會被清掉
	if p.hookedStateListeners != nil {
		for _, stateListener := range p.hookedStateListeners {
			stateListener.SendProtobufMessage(result)
		}
	}
	p.mutex.RUnlock()
}

// Resolve (retcode==0) is also Notify(retcode==-1)
func (p *Promise) Resolve(stdout interface{}, retcode int32) {
	if retcode == 0 { //Resolve
		defer p.clean()
	}
	ret := TreeCallReturn{
		CmdID:   p.CmdID,
		Retcode: retcode,
		Stdout:  stdout,
	}
	p.mutex.RLock()
	if p.stateListener == nil {
		//skip
	} else if p.stateListener.IsClosed() {
		log.Println("Resolve(", retcode, ") called but connection closed")
	} else {
		p.stateListener.SendTreeCallReturn(&ret)
	}
	// 不必測試 "&& len(p.hookedStateListeners) > 0 ",
	// 因為hookedStateListeners是動態建立起來的，如果hookedStateListeners是空，會被清掉
	if p.hookedStateListeners != nil {
		for _, stateListener := range p.hookedStateListeners {
			stateListener.SendTreeCallReturn(&ret)
		}
	}
	p.mutex.RUnlock()
}
func (p *Promise) Reject(retcode int32, err error) {
	defer p.clean()
	ret := TreeCallReturn{
		CmdID:   p.CmdID,
		Retcode: retcode,
		Stderr:  err,
	}

	p.mutex.RLock()

	if p.stateListener == nil {
        //skip
        log.Println("Reject() is skipped, retcode=",retcode )
	} else if p.stateListener.IsClosed() {
		log.Println("Reject() called but connection closed")
	} else {
		p.stateListener.SendTreeCallReturn(&ret)
	}
	// 不必測試 "&& len(p.hookedStateListeners) > 0 ",
	// 因為hookedStateListeners是動態建立起來的，如果hookedStateListeners是空，會被清掉
	if p.hookedStateListeners != nil {
		for _, stateListener := range p.hookedStateListeners {
			stateListener.SendTreeCallReturn(&ret)
		}
	}

	p.mutex.RUnlock()
}

// TreeCallCtx encapturelate a call context
// PromiseStateListener 不是很好的命名，但一時也沒更好的名字（本來是WsCtx)
type TreeCallCtx struct {
	// Metadata *map[string]interface{} //if presented, would be *model.User
	CmdID int32
	Root  *TreeRoot
	// The Websocket through which user triggers this api call
	WsCtx PromiseStateListener

	// below 3 are parameters to call
	Args    []string
	Kw      *fasthttp.Args //a dict-like instance
	Message *proto.Message

	// for efficiency, use two kind of promise
	promise *Promise //creator's promise

	background bool
	// -1 for regular task, -2 for background task, browse check to see if task is in background
	RetcodeOfNotify int32
	// listeners when task is killed
	killListener []func()
	// timestamp of creation
	Ctime uint32
	// Call's path and username, <TreeName.BranchName,FuncName>\t<username>
	// username is for checking before killing this task when it goes to background
	CmdPath string
}

// SetBackground
// @yes: if true, this call will not auto cancelled even websocket disconnected.
//       if false, when websocket disconnected, Kill() is called. This is default.
//
func (tcCtx *TreeCallCtx) SetBackground(yes bool) {
    // Do nothing if nothing changed or the promise has been resolved or rejected
	if yes == tcCtx.background || tcCtx.promise.stateListener==nil{
		return
	}

	tcCtx.background = yes

	onAndOffID := "cc" + strconv.FormatInt(int64(tcCtx.CmdID), 10)
	if yes {
		tcCtx.RetcodeOfNotify = int32(-2)
		// remove initially setup tcCtx.Kill by default
		tcCtx.WsCtx.Off("Close", onAndOffID)
	} else {
		tcCtx.RetcodeOfNotify = int32(-1)
		tcCtx.WsCtx.On("Close", onAndOffID, tcCtx.Kill)
	}
}

func (tcCtx *TreeCallCtx) HookTo(ctx *TreeCallCtx) error {
	return ctx.promise.Put(tcCtx.WsCtx)
}
func (tcCtx *TreeCallCtx) UnHookFrom(ctx *TreeCallCtx) error {
	return ctx.promise.Del(tcCtx.WsCtx)
}

/*
The call-style (or "routing" rules) of a call from browser are: (ObjshSDK.Tree.call)
// call(branchName,[arg])
// call(branchName,proto.Message)
// call(branchName,{k:v})
// call(branchName,[arg],{k:v})
// call(branchName,{k:v},proto.Message)
// call(branchName,[arg],proto.Message)
// call(branchName,[arg],{k:v},proto.Message)
*/

func (tcCtx *TreeCallCtx) Resolve(stdout interface{}) {
    fmt.Println("ctx resolved",tcCtx.CmdID)
	tcCtx.promise.Resolve(stdout, 0)
	tcCtx.clean()
}
func (tcCtx *TreeCallCtx) Notify(stdout interface{}) {
	tcCtx.promise.Resolve(stdout, tcCtx.RetcodeOfNotify)
}
func (tcCtx *TreeCallCtx) Reject(retcode int32, err error) {
	tcCtx.promise.Reject(retcode, err)
	tcCtx.clean()
}

// DirectResult is called to avoid stdout, stderr been jsonize again.
// For exmaple: python branch's call result.
// "clean" should be true, for resolve and reject
func (tcCtx *TreeCallCtx) DirectResult(result *Result, clean bool) {
	tcCtx.promise.DirectResult(result)
	if clean {
		tcCtx.clean()
	}
}

/*

   //Caution:
   //    Do not listen on TreeCallCtx.On("Kill") and call TreeCallCtx.Resolve() like the code block below.
   //    Because the listener callback will be removed by calling Resolve()
   //    On("Kill") can only be used by Progress-style API.

   callCtx.On("Kill", func() {
       log.Println(fmt.Sprintf("chat exit by %v ", id))
       cb.Exit(callCtx)
   })
   callCtx.Resolve(1)

*/
func (tcCtx *TreeCallCtx) On(evtName string, callback func()) error{
	if evtName == "Kill" {
        tcCtx.promise.mutex.Lock()
		if len(tcCtx.killListener) == 0 {
            error := tcCtx.Root.Bank.Put(tcCtx); 
            if error != nil{
                panic("On(Kill) Error")
            }
		}
        tcCtx.killListener = append(tcCtx.killListener, callback)
        tcCtx.promise.mutex.Unlock()
	} else {
		panic("TreeCallCtx has no event named: " + evtName)
    }
    return nil
}

//Kill is called by user or auto called when websocket closed (for foreground task)
func (tcCtx *TreeCallCtx) Kill() {
	// When WebSocket is closed, Kill() of all TreeCallCtx will be called.
	// But if one of TreeCallCtx.Kill() is called, TreeCallCtx is not necessary Closed
	if tcCtx.promise != nil {
        // tcCtx.Reject() will call "tcCtx.clean()", which will
        // call "tcCtx.promise.clean()" 
		// and " tcCtx.Root.Bank.Del(tcCtx.CmdID) "
        tcCtx.Reject(500,errors.New("job killed"))
	}else{
        //目前，只有當On(Kill)有listeners時才會放到bank內
        tcCtx.Root.Bank.Del(tcCtx.CmdID)

    }

	if tcCtx.killListener != nil {
		if len(tcCtx.killListener) == 0 {
			return
		}
		for _, fn := range tcCtx.killListener {
			fn()
		}
	}
	tcCtx.killListener = nil
}

//KillPeer kill other existing TreeCallCtx
// usually, this kill is oriented from browser
func (tcCtx *TreeCallCtx) KillPeer(cmdID int32) error {
	// When WebSocket is closed, Kill() of all TreeCallCtx will be called.
	// But if one of TreeCallCtx.Kill() is called, TreeCallCtx is not necessary Closed
	//目前，只有當On(Kill)有listeners時才會放到bank內
	if ctx := tcCtx.Root.Bank.Get(cmdID); ctx != nil {
		ctx.Kill()
		return nil
    }
    return errors.New("not found")
}

// clean is called when task is resolved or rejected
// (not been killed or foreground task's websocket was suddenly closed )
func (tcCtx *TreeCallCtx) clean() {
    onAndOffID := "cc" + strconv.FormatInt(int64(tcCtx.CmdID), 10)
    
    // 2019-11-22T04:54:11+00:00
    //  Since this job has been resovled or reject,
    //  It should be safe to remove it from bank (bank is for killing a job)
    tcCtx.Root.Bank.Del(tcCtx.CmdID)
    
    // remove initially setup tcCtx.Kill by default
    tcCtx.WsCtx.Off("Close", onAndOffID)
    
    /*
    if tcCtx.background{
        fmt.Println("background task ",onAndOffID," closed")
    }else{
        fmt.Println("foreground task ",onAndOffID," closed")
    }
    */
}

func NewTreeCallCtx(root *TreeRoot, CmdID int32, wsCtx PromiseStateListener, args []string, kw *map[string]string, message *proto.Message) *TreeCallCtx {
	fhArgs := fasthttp.Args{}
	if kw != nil {
		for k, v := range *kw {
			fhArgs.Add(k, v)
		}
	}
	tcCtx := TreeCallCtx{
		CmdID:           CmdID,
		WsCtx:           wsCtx,
		Args:            args,
		Kw:              &fhArgs,
		killListener:    make([]func(), 0),
		Ctime:           uint32(time.Now().Unix()),
		promise:         NewPromise(CmdID, wsCtx),
		RetcodeOfNotify: int32(-1),
	}
	if message != nil {
		tcCtx.Message = message
	}
	if root != nil {
		tcCtx.Root = root
	}

	tcCtx.background = true //enforce SetBackground(true) to work
	tcCtx.SetBackground(false)

	return &tcCtx
}

func NewSimpleTreeCallCtx(root *TreeRoot, CmdID int32, wsCtx PromiseStateListener) *TreeCallCtx {
	tcCtx := TreeCallCtx{
		CmdID: CmdID,
		WsCtx: wsCtx,
		promise: &Promise{
			CmdID:         CmdID,
			stateListener: wsCtx,
		},
	}
	if root != nil {
		tcCtx.Root = root
	}
	return &tcCtx
}

//root tree
type DocItem struct {
	srcpath string
	//相對於apiName=BranchName.FuncName， innerName=TypeName.FuncName
	innerName string
	Comment   string
	mtime     int64
}

func NewDocItem(srcpath string, innerName string, comment string, mtime int64) DocItem {
	return DocItem{
		srcpath, innerName, comment, mtime,
	}
}

type TreeRoot struct {
	Name     string
	Branches map[string]Branch
	Ready    chan bool //signal ready to caller
	Wg       sync.WaitGroup
	Bank     *TreeCallCtxBank
	Docs     map[string]*DocItem // "branch-name.func-name" to DocItem
	IsReady  bool
}

func NewTreeRoot() *TreeRoot {
	return NewTreeRootWithName("Tree")
	/*
		rootTree := TreeRoot{
			Name:     "Tree",
			Branches: make(map[string]Branch),
			Ready:    make(chan bool),
			Wg:       sync.WaitGroup{},
			Bank:     NewTreeCallCtxBank(),
			Docs:     make(map[string]*DocItem),
		}
		return &rootTree
	*/
}
func NewTreeRootWithName(name string) *TreeRoot {
	rootTree := TreeRoot{
		Name:     name,
		Branches: make(map[string]Branch),
		Ready:    make(chan bool),
		Wg:       sync.WaitGroup{},
		Bank:     NewTreeCallCtxBank(),
		Docs:     make(map[string]*DocItem),
	}
	return &rootTree
}

func (self *TreeRoot) SureReady(branch interface{}) {
	if b, ok := branch.(Branch); ok {
		log.Printf("Tree %s's branch \"%s\" is ready\n", self.Name, b.Name())

		// 如果是python reload module,會有在IsReady之後，還呼叫 SureReady
		// 的情況，此時不要呼叫 self.Wg.Done()，不然會crash。 目前 python reload module
		// 時，Tree IsReady的情況不會改變。
		if !self.IsReady {
			self.Wg.Done()
		}
		return
	}
	panic(fmt.Sprintf("%T is not a branch for tree", branch))
}

func (self *TreeRoot) BeReady() {
	if len(self.Branches) == 0 {
		self.Ready <- true
		return
	}
    log.Println("** Waiting branches to be ready of number",len(self.Branches))
	if len(self.Branches) > 0 {
		self.Wg.Add(len(self.Branches))
		for _, branch := range self.Branches {
			go branch.BeReady(self)
		}
		self.Wg.Wait()
	}
	self.IsReady = true
	// All nodes on the tree are ready
	// find the "class name" of this branch
	// Purpose:
	// 1. build a map(exposed-api-name to inner-api-name) for getting func's comments (documents)
	// 2. a branch might set its exporting name (path) at "BeReady()",
	//    it might be differnt from default name (ex. ChatBranch vs $chat),
	//    so here we re-correct the mapping key to their new branch name.
	changedBranchNames := make(map[string]string)
	for bname, branch := range self.Branches {

		if bname != branch.Name() {
			changedBranchNames[bname] = branch.Name()
		}
	}
	// re-correct the mappig key for name-changed branches
	for oldname, newname := range changedBranchNames {
		self.Branches[newname] = self.Branches[oldname]
		delete(self.Branches, oldname)
	}

	self.ScanAllAPIInfo()

}

func (self *TreeRoot) ScanAllAPIInfo() {

	// All nodes on the tree are ready
	// find the "class name" of this branch
	// Purpose:
	// 1. build a map(exposed-api-name to inner-api-name) for getting func's comments (documents)
	// 2. a branch might set its exporting name (path) at "BeReady()",
	//    it might be differnt from default name (ex. ChatBranch vs $chat),
	//    so here we re-correct the mapping key to their new branch name.

	// search for source files in these folders
	searchingPaths := make([]string, 0)
	if cwd, err := os.Getwd(); err == nil {
		// suffix "/" to hint ScanAPIInfo() that this is a folder
		searchingPaths = append(searchingPaths, cwd+"/")
	}
	if os.Getenv("HOME") != "" {
		searchingPaths = append(searchingPaths, filepath.Join(os.Getenv("HOME"), "go", "src"))
	}
	for _, gopath := range strings.Split(os.Getenv("GOPATH"), ":") {
		searchingPaths = append(searchingPaths, filepath.Join(gopath, "src"))
	}
	for _, spath := range searchingPaths {
		log.Printf("searchingPath=%v\n", spath)
	}
	uniqueSourcePaths := make(map[string][][2]string) // source file/folder to exported func
	for bname, branch := range self.Branches {
		var branchname string
		if t := reflect.TypeOf(branch); t.Kind() == reflect.Ptr {
			branchname = t.Elem().Name()
		} else {
			branchname = t.Name()
		}
		tp := reflect.TypeOf(branch).Elem()
		var srcpath string
		if pkgpath := tp.PkgPath(); pkgpath != "" {
			for _, spath := range searchingPaths {
				path := filepath.Join(spath, tp.PkgPath()+".go")
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					if pkgpath == "main" {
						srcpath = spath
					} else {
						srcpath = path
					}
					break
				}
				folder := filepath.Join(spath, tp.PkgPath())
				if _, err := os.Stat(folder); !os.IsNotExist(err) {
					srcpath = folder + "/"
					break
				}
			}
		}
		if srcpath == "" {
			log.Println("Caution: source file not found for " + tp.PkgPath() + ".go")
		} else {
			log.Println("Source file:" + srcpath)
			var funcs [][2]string
			var ok bool
			funcs, ok = uniqueSourcePaths[srcpath]
			if !ok {
				funcs = make([][2]string, 0)
			}
			for _, apiname := range branch.GetExportableNames(self) {
				log.Println("it exports", branchname+"."+apiname, "as", bname+"."+apiname)
				funcs = append(funcs, [...]string{branchname + "." + apiname, bname + "." + apiname})
			}
			uniqueSourcePaths[srcpath] = funcs
		}
	}
	for srcpath, funcs := range uniqueSourcePaths {
		finfo, _ := os.Stat(srcpath)
		ret, err := GetGoDocs(srcpath)
		if err != nil {
			fmt.Println(err)
			continue
		}
		for _, item := range funcs {
			innerName, apiName := item[0], item[1]
			//fmt.Println("Add api--->", apiName, "innername=", innerName)
			if comment, ok := ret[innerName]; ok {
				self.Docs[apiName] = &DocItem{srcpath, innerName, comment, finfo.ModTime().Unix()}
			} else {
				//有可能是定義在 Python Script
				//self.Docs[apiName] = &DocItem{srcpath: srcpath, innerName: innerName, mtime: 0}
				//log.Println("docstring of ", innerName, " is not found in ", srcpath)
			}
		}
	}
}

//RescanAPIInfo 掃描golang的 branch 有export的method的docstring
func (self *TreeRoot) RescanAPIInfo(apiName string) *map[string]*DocItem {
	docItem, ok := self.Docs[apiName]
	if !ok {
		log.Println("RescanAPIInfo: " + apiName + " is invalid")
		return nil
	}
	finfo, err := os.Stat(docItem.srcpath)
	if os.IsNotExist(err) {
		log.Println("RescanAPIInfo: " + docItem.srcpath + " is not found")
		return nil
	}

	// check mtime if srcpath is regular file.
	// enforce rescan if srcpath is a folder
	if !strings.HasSuffix(docItem.srcpath, "/") {
		if finfo.ModTime().Unix() == docItem.mtime {
			//file not changed
			log.Println("RescanAPIInfo: " + docItem.srcpath + " is the same")
			return nil
		}
	}

	ret, err := GetGoDocs(docItem.srcpath)
	if err != nil {
		log.Println(fmt.Sprintf("RescanAPIInfo: %v", err))
		return nil
	}
	funcs := make([][2]string, 0)
	for apiname, docitem := range self.Docs {
		if docitem.srcpath == docItem.srcpath {
			funcs = append(funcs, [2]string{docitem.innerName, apiname})
		}
	}
	changed := make(map[string]*DocItem)
	for _, item := range funcs {
		var docitem *DocItem
		innerName, apiName := item[0], item[1]
		if comment, ok := ret[innerName]; ok {
			docitem = &DocItem{docItem.srcpath, innerName, comment, finfo.ModTime().Unix()}
		} else {
			docitem = &DocItem{srcpath: docItem.srcpath, innerName: innerName, mtime: finfo.ModTime().Unix()}
			log.Println("docstring of ", innerName, " is not found in ", docItem.srcpath)
		}
		self.Docs[apiName] = docitem
		changed[apiName] = docitem
	}
	return &changed
}

//SetAPIInfo 是用來設定與管理Python的branch的API Info
func (self *TreeRoot) SetAPIInfo(apiName string, docitem *DocItem) {
	if docitem == nil {
		//Remove this item
		_, ok := self.Docs[apiName]
		if !ok {
			log.Println("SetAPIInfo remove " + apiName + " is invalid")
			return
		}
		delete(self.Docs, apiName)
		return
	}
	//docitem = DocItem{docItem.srcpath, innerName, comment, finfo.ModTime().Unix()}
	self.Docs[apiName] = docitem
}

// key 不限制是一層，也可以是兩層，
// 例如： sys.network
// tree.sys.network.SetInterface 解析的時候，會這樣解析：
// (tree).(key=第二個～倒數第二個).(最後一個), ex
// (tree).(sys.network).(SetInterface)
func (self *TreeRoot) AddBranchWithName(branch Branch,name string) {
	if branch.Name() != name {
		branch.SetName(name)
	}
	self.Branches[name] = branch

	//如果是reload module的情況，會有此情況發生
	if self.IsReady {
		branch.BeReady(self)
	}
}
func (self *TreeRoot) AddBranch(branch Branch) {
	var name string
	if branch.Name() == "" {
		nameFull := fmt.Sprintf("%T", branch)
		name = strings.TrimPrefix(filepath.Ext(nameFull), ".")
	} else {
		name = branch.Name()
	}
	self.AddBranchWithName(branch,name)
}

func (self *TreeRoot) Dump() {
	fmt.Printf("Tree (Root:%v) Layout Dump\n", self.Name)
	layout := self.Layout()
	for name, exportableNames := range layout {
		fmt.Printf("\t%v.%s\n", self.Name, name)
		for _, apiinfo := range exportableNames {
			fmt.Printf("\t\t%s.%s.%s\n", self.Name, name, apiinfo.Name)
		}
	}
}

type APIInfo struct {
	Name    string
	Comment string
}

func (ai *APIInfo) String() string {
	return ai.Name
}

func (self *TreeRoot) Layout() map[string][]*APIInfo {
	layout := make(map[string][]*APIInfo)
	for name, n := range self.Branches {
		apinames := n.GetExportableNames(self)
		layout[name] = make([]*APIInfo, len(apinames))
		for i, funcName := range apinames {
			layout[name][i] = &APIInfo{Name: funcName}
			if docitem, ok := self.Docs[name+"."+funcName]; ok {
				layout[name][i].Comment = docitem.Comment
			}
		}
	}
	return layout
}

func (self *TreeRoot) Call(nodePath string, ctx *TreeCallCtx) {
	paths := strings.Split(nodePath, ".")
	if paths[0] != self.Name {
		ctx.Reject(1, errors.New(nodePath+" Not Found"))
	} else if n, ok := self.Branches[paths[1]]; ok {
		var username string
		user := ctx.WsCtx.GetUser()
		if user != nil {
			username = user.Username()
		}
		ctx.CmdPath = nodePath + "\t" + username
		n.Call(paths[2], ctx)
	} else {
		ctx.Reject(1, errors.New(nodePath+" Not Found"))
	}
}

type Branch interface {
	BeReady(*TreeRoot) //chances to initialize this node
	GetExportableNames(*TreeRoot) []string
	Call(string, *TreeCallCtx)
	Name() string
	SetName(string)
}
type Exportable = func(*TreeCallCtx)

// BaseBranch is an implementation of interface Branch
type BaseBranch struct {
	name            string
	Ready           bool
	Exportables     map[string]Exportable
	ExportableNames []string
}

// InitBaseBranch is an example implementation of a generic node
func (bb *BaseBranch) InitBaseBranch(names ...string) {
	bb.Exportables = make(map[string]Exportable)
	if len(names) > 0 {
		bb.SetName(names[0])
	}
}
func (bb *BaseBranch) Name() string {
	return bb.name
}
func (bb *BaseBranch) SetName(name string) {
	bb.name = name
}
func (bb *BaseBranch) Export(callables ...Exportable) {
	//miso
	for _, callable := range callables {
		nameFull := runtime.FuncForPC(reflect.ValueOf(callable).Pointer()).Name()
		nameEnd := strings.TrimPrefix(filepath.Ext(nameFull), ".")
		name := strings.TrimSuffix(nameEnd, "-fm")
		bb.Exportables[name] = callable
	}
}
func (bb *BaseBranch) collectExportableNames() {
	keys := reflect.ValueOf(bb.Exportables).MapKeys()
	bb.ExportableNames = make([]string, len(keys))
	for i := 0; i < len(keys); i++ {
		bb.ExportableNames[i] = keys[i].String()
	}
}
func (bb *BaseBranch) GetExportableNames(tree *TreeRoot) []string {
	if bb.ExportableNames == nil {
		bb.collectExportableNames()
	}
	return bb.ExportableNames
}
func (bb *BaseBranch) Call(apiName string, ctx *TreeCallCtx) {
	if api, ok := bb.Exportables[apiName]; ok {
		api(ctx)
	} else {
		ctx.Reject(1, errors.New(apiName+" not found"))
	}
}
