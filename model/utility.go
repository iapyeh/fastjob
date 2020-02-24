package model

import (
	fmt "fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
    "unsafe"
    "os/exec"
    "bufio"
    "errors"
    "sync"
)

/*
//
// Example Usage of SetTimeout and SetInterval
//
// There are 3 methods to stop an interval when lost connection.
// All 3 methods can avoid data-race problem
//
func (self *UnitTest) Clock(callCtx *TreeCallCtx) {

	//method 1
	objsh.SetInterval(func(stop *chan bool) {
		if callCtx.IsClosed() {
			*stop <- true
			return
		}
		currentTime := time.Now().Format("15:04:05")
		fmt.Println(currentTime)
		callCtx.Notify(&currentTime)
	}, 100)

	// Method 2
	stop := objsh.SetInterval(func(stopCh *chan bool) {
		currentTime := time.Now().Format("15:04:05")
		fmt.Println(currentTime)
		callCtx.Notify(&currentTime)
	}, 100)
	callCtx.WsCtx.On("Close", "TokenToRemove", func() {
		// should "off" this callback
		defer callCtx.WsCtx.Off("Close", "TokenToRemove")
		log.Println("Clock() called but connection closed")
		*stop <- true
	})

	//Method 3 by SetTimeout
	token := "123444"
	var stop *chan bool
	var issue func()
	m := sync.Mutex{}
	issue = func() {
		m.Lock()
		stop = objsh.SetTimeout(func() {
			currentTime := time.Now().Format("15:04:05")
			fmt.Println(currentTime)
			callCtx.Notify(&currentTime)
			issue()
		}, 100)
		m.Unlock()
	}
	callCtx.WsCtx.On("Close", token, func() {
		defer callCtx.WsCtx.Off("Close", token)
		log.Println("Clock() called but connection closed")
		m.Lock()
		*stop <- true
		m.Unlock()
	})
	issue()
}
*/

func SetInterval(someFunc func(*chan bool), mseconds int64) *chan bool {
	ticker := time.NewTicker(time.Duration(mseconds) * time.Millisecond)
	stop := make(chan bool)
	go func() {
		for {
			select {
			case <-ticker.C:
				someFunc(&stop)
			case <-stop:
				return
			}
		}
	}()
	return &stop
}
func SetTimeout(someFunc func(), mseconds uint) *chan bool {
	timeout := time.Duration(mseconds) * time.Millisecond
	// This spawns a goroutine and therefore does not block
	stop := make(chan bool)
	timer := time.AfterFunc(timeout, someFunc)
	go func() {
		for {
			select {
			case <-stop:
				timer.Stop()
				return
			}
		}
	}()
	return &stop
}

//
// Borrowed from fasthttp
//
// b2s converts byte slice to a string without memory allocation.
// See https://groups.google.com/forum/#!msg/Golang-Nuts/ENgbUzYvCuU/90yGx7GUAgAJ .
//
// Note it may break if string and/or slice header will change
// in the future go versions.
func B2S(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

/*
	Test the memory useage of UserManager.tokenCache.
	When there are 3,000,000 entries in tokenCache, the numbers are:
		size of userManager.tokenCache= 3000001 size= 8
		Alloc = 1598 MiB	TotalAlloc = 2910 MiB	Sys = 1968 MiB	NumGC = 17
		（2019-08-04T03:00:35+00:00）
	Snap of Testing:(Injected in Login)
		for i := 0; i < 3000000; i++ {
			UUID := uuid.New().String()
			u := User{
				UUID:        UUID,
				Username:    UUID,
				Password:    []byte(UUID + UUID),
				Role:        UUID,
				DisplayName: UUID + UUID,
				Email:       UUID + UUID,
				Disabled:    true,
				Activated:   true,
				LastTS:      time.Now().Unix(),
			}
			u.GenerateToken()
			userManager.mutex.Lock()
			userManager.tokenCache[u.Token] = &u
			userManager.mutex.Unlock()
			if i < 5 {
				fmt.Println("size of user:", unsafe.Sizeof(u))
			}
		}
		fmt.Println("size of userManager.tokenCache=", len(userManager.tokenCache))
		PrintMemUsage()

*/

// PrintMemUsage is borrowed from: https://golangcode.com/print-the-current-memory-usage/
func PrintMemUsage() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	// For info on each, see: https://golang.org/pkg/runtime/#MemStats
	fmt.Printf("Alloc = %v MiB", bToMb(m.Alloc))
	fmt.Printf("\tTotalAlloc = %v MiB", bToMb(m.TotalAlloc))
	fmt.Printf("\tSys = %v MiB", bToMb(m.Sys))
	fmt.Printf("\tNumGC = %v\n", m.NumGC)
}

func bToMb(b uint64) uint64 {
	return b / 1024 / 1024
}

//GetGoDocs scan exported funcs' docstring from source
func GetGoDocs(abspath string) (map[string]string, error) {
	fi, err := os.Stat(abspath)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	var paths []string
	if fi.Mode().IsRegular() {
		paths = make([]string, 1)
		paths[0] = abspath
	} else if fi.Mode().IsDir() {
		files, err := ioutil.ReadDir(abspath)
		if err != nil {
			log.Println(err)
			return nil, err
		}
		for _, f := range files {
			if filepath.Ext(f.Name()) == ".go" {
				paths = append(paths, filepath.Join(abspath, f.Name()))
			}
		}
	}
	ret := make(map[string]string)
	for _, path := range paths {
		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			log.Fatal(err)
		}
		findFunc(node, ret)
	}
	return ret, nil
}
func findFunc(node *ast.File, ret map[string]string) {
	for _, f := range node.Decls {
		fn, ok := f.(*ast.FuncDecl)
		if !ok {
			continue
		}
		names := make([]string, 2)
		names[1] = fn.Name.Name
		if fn.Recv.NumFields() > 0 {
			for _, v := range fn.Recv.List {
				switch xv := v.Type.(type) {
				case *ast.StarExpr:
					if si, ok := xv.X.(*ast.Ident); ok {
						names[0] = si.Name
					}
				case *ast.Ident:
					names[0] = xv.Name
				}
			}
		}
		ret[strings.Join(names, ".")] = fn.Doc.Text()
	}
}

// 2019-11-22T07:49:35+00:00
// Use twisted terminology to tell from "promise" in tree.go
type Deferred struct {
    okCallback func(interface{})
    errCallback func(error)
    progressCallback func(interface{})
    signal chan int
    success interface{} //success value
    failure error //failure value
}
func NewDeferred() *Deferred{
    d := &Deferred{
        signal: make(chan int,1),
    }
    return d
}
func (self *Deferred) Done(callback func(interface{})){
    if (self.success != nil) {
        // already resolved
        defer callback(self.success)
    }else{
        self.okCallback = callback
    }
}
func (self *Deferred) Progress(callback func(interface{})){
    if (self.success != nil) {
        // already resolved
        defer callback(self.success)
    }else{
        self.progressCallback = callback
    }
}
func (self *Deferred) Fail(errback func(error)){
    if (self.failure != nil){
        // already rejected
        defer errback(self.failure)
    }else{
        self.errCallback = errback
    }
}
func (self *Deferred) Resolve(s interface{}){
    if (self.okCallback != nil) {self.okCallback(s)}
    self.success = s
}
func (self *Deferred) Reject(err error){
    if (self.errCallback != nil) {self.errCallback(err)}
    self.failure = err
}
func (self *Deferred) Notify(s interface{}){
    if (self.progressCallback != nil) {self.progressCallback(s)}
}
func (self *Deferred) Kill(sig int){
    self.signal <- sig
}

// Subprocess run cmd (exec.Cmd)
// @cmd :  exec.Command("python3","/Users/iap/Dropbox/workspace/ObjectiveShell/src/unittest/subprocess_test.py")
// @flag:
//      0 , stdout, stderr return all at once at Resolve and Reject
//      1 (01), stderr return line by line by Notify, stderr return all at once
//      2 (10), stdout return line by line by Notify, stdout return all at once
//      3 (11), stdout and stderr return line by line by Notify
func Subprocess(cmd *exec.Cmd, flag int) *Deferred{

    deferred := NewDeferred()
       
    stdout, err := cmd.StdoutPipe()

    if err != nil {
        deferred.Reject(err)
        return deferred
	}    

    stderr, err := cmd.StderrPipe()

    if err != nil {
        deferred.Reject(err)
        return deferred
	}    

    err = cmd.Start()
    if err != nil {
        deferred.Reject(err)
        return deferred
    }

    go func(){

        var wg sync.WaitGroup
        wg.Add(1)
    
        go func(){
            loop:
                for{
                    select{
                    case <- deferred.signal:
                        //Kill process, 
                        cmd.Process.Kill()
                        break loop;
                    }
                }
        }()

        stdoutAtOnce := flag & 2 == 0
        rejected := false
        var stdoutBody string
        go func(){
            // send stdout line by line
            if stdoutAtOnce{
                stdoutRawBody,errerr := ioutil.ReadAll(stdout)
                if errerr != nil {
                    deferred.Reject(errerr)
                    rejected = true
                }else if len(stdoutRawBody) > 0{
                    // send stderr all at once
                    stdoutBody = B2S(stdoutRawBody)
                }                        
            }else{
                scanner := bufio.NewScanner(stdout)
                for scanner.Scan() {
                    line := scanner.Text()
                    deferred.Notify(line)
                }    
            }
            wg.Done()
        }()


        stderrAtOnce := flag & 1 == 0
        if stderrAtOnce{
            b, errerr := ioutil.ReadAll(stderr)
            if errerr != nil {
                deferred.Reject(errerr)
                rejected = true
            }else if len(b) > 0{
                // send stderr all at once
                errstr := B2S(b)
                deferred.Reject(errors.New(errstr))
                rejected = true
            }    
        }else{
            // send stdout line by line
            scanner := bufio.NewScanner(stderr)
            for scanner.Scan() {
                line := scanner.Text()
                deferred.Notify(line)
            }
        }
        wg.Wait()

        err = cmd.Wait()
        if err != nil {
            // May also be killed by another command from browser, in that case,
            // this Reject() will be ignored, because that will trigger a Reject() call.
            deferred.Reject(err)
        } else if !rejected {
            deferred.Resolve(stdoutBody)
        }
    }()
    return deferred
}


func OldSubprocess(cmd *exec.Cmd) *Deferred{
    //cmd := exec.Command("python3","/Users/iap/Dropbox/workspace/ObjectiveShell/src/unittest/subprocess_test.py")

    deferred := NewDeferred()
    done := make(chan error, 1)
    stop := deferred.signal
    eof := make(chan error,1)


    stdout, err := cmd.StdoutPipe()

    if err != nil {
        deferred.Reject(err)
        return deferred
	}    

    stderr, err := cmd.StderrPipe()

    if err != nil {
        deferred.Reject(err)
        return deferred
	}    

    err = cmd.Start()
    if err != nil {
        deferred.Reject(err)
        return deferred
    }

    go func() {
        done <- cmd.Wait()

        b, errerr := ioutil.ReadAll(stderr)
        if errerr != nil {
            log.Println(errerr)
        }else{
            errstr := B2S(b)
            fmt.Println("error:",errstr)
            deferred.Resolve("OK")
        }        
    }()
    


    go func(){
        /*
        go func(){
            // If the last line does not end up with "\n", 
            // it will be received after process exited sometimes.
            // for ensuring that we do not lost it, we have to watch EOF
            // before resolve. 
            scanner := bufio.NewScanner(stderr)
            length := 5
            lines := make([]string,length)
            cursor := 0
            for scanner.Scan() {
                line := scanner.Text()
                //fmt.Println("ERR:",line)
                if (cursor < length){
                    lines[cursor] = line
                    cursor += 1
                }else{
                    lines = append(lines, line)
                }
            }
            if err := scanner.Err(); err != nil {
                eof <- err
            } else if (cursor == 0){
                eof <- nil
            }else{
                eof <- errors.New(strings.Join(lines,"\n"))
            }
        }()
        */
        //stderrReader := bufio.NewReader(stderr)
    
        go func(){
            // If the last line does not end up with "\n", 
            // it will be received after process exited sometimes.
            // for ensuring that we do not lost it, we have to watch EOF
            // before resolve. 
            scanner := bufio.NewScanner(stdout)
            for scanner.Scan() {
                line := scanner.Text()
                //fmt.Println(">>",line)
                deferred.Notify(line)
            }


            

            if err := scanner.Err(); err != nil {
                eof <- err
            }else{
                eof <- nil
            }
        }()
        

        hasEOF := false
        hasDone := false
        var hasError error
        loop:
        for{
            select {
            case <- stop:
                if err := cmd.Process.Kill(); err != nil {
                    deferred.Reject(err)
                }else{
                    deferred.Reject(errors.New("killed"))
                }
                break loop;
            case err := <- eof:
                hasEOF = true
                //errSize := stderrReader.Size()
                //if errSize > 0 {
                
                _ = hasDone
                _ = err
                /*
                }else{
                    if hasDone {
                        if err != nil {
                            // EOF's error takes priority
                            deferred.Reject(err)
                        }else if (hasError != nil){
                            deferred.Reject(hasError)
                        }else{
                            deferred.Resolve("OK")
                        }
                        break loop
                    }else if (err != nil){
                        hasError = err
                    }    
                }*/
            case err := <-done:
                hasDone = true
                if hasEOF{
                    if (hasError != nil){
                        // EOF's error takes priority
                        deferred.Reject(hasError)
                    }else if err != nil {
                        deferred.Reject(err)
                    }else{
                        deferred.Resolve(1)
                    }
                    break loop
                }else if (err != nil){
                    hasError = err
                }
            }            
        }
    }()
    return deferred
}