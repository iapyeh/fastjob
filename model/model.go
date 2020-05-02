package model
import (
    _ "runtime"
)
//ACL mode
const (
	PublicMode = iota + 1
	TraceMode
	ProtectMode
)
/*
var RunInMainQueue = make(chan func(),100)
func init() {
    runtime.LockOSThread()
}
func RunInMain() {
    var runInMainCompleted = make(chan bool,1)
    RunInMainQueue <- func(){
        runInMainCompleted <- true
    }
    close(RunInMainQueue)
    loop:
    for{
        select {
            case  <-runInMainCompleted :
                break loop
            case f := <- RunInMainQueue:
                f()
        }
    }
    runtime.UnlockOSThread()   
}
*/