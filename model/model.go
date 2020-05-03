package model
import (
    "runtime"
    "sort"
)
//ACL mode
const (
	PublicMode = iota + 1
	TraceMode
	ProtectMode
)
/*
var RunInMain = make(chan func(),100)
func init() {
    runtime.LockOSThread()
}
func CallRunInMain(){   
    
    // push an ending func into RunInMain
    var runInMainCompleted = make(chan bool,1)
    RunInMain <- func(){
        runInMainCompleted <- true
    }
    close(RunInMain)
    
    loop:
    for{
        select {
            case  <-runInMainCompleted :
                // ending func was called
                break loop
            case f := <- RunInMain:
                f()
        }
    }
    // must put to defer, otherwise "segment fault happened"
    defer runtime.UnlockOSThread()   
}
*/


/*
The section implements a feature to call functions after init() in order with priority.
For example, python3.callWhenRunning is called lastest after all other python module has initialized
*/
type weightedFunc struct {
    p int //priority
    f func()
}
var runInMainQueue []weightedFunc
// RunInMain, smaller p run first, larger p run later
func RunInMain(p int, f func()){
    runInMainQueue = append(runInMainQueue, weightedFunc{p,f})
}
func init() {
    // restrict to main thread for calling "CallRunInMain" in main thread
    runtime.LockOSThread()
}
func CallRunInMain(){
    // in accent order
    sort.Slice(runInMainQueue,func(i,j int) bool {return runInMainQueue[i].p < runInMainQueue[j].p})
    for _,s := range runInMainQueue{
        s.f()
    }
    // release main thread
    runtime.UnlockOSThread()   
}
