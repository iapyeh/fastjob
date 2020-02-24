package tree

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os/exec"
	"sync"
	"unicode/utf8"
)

// ChatBranch would connect to tree
type ExecBranch struct {
	BaseBranch
}

func (eb *ExecBranch) BeReady(treeroot *TreeRoot) {
	eb.SetName("$exec")
	eb.InitBaseBranch()
	eb.Export(
		eb.Command,
		eb.BackgroundCommand,
	)
	treeroot.SureReady(eb)
}

var (
	NotAllowedError = errors.New("Not Allowed Command")
)

func GetAllowedCommand(cmdName string, args ...string) *exec.Cmd {
	if cmdName == "ls" {
		return exec.Command("/bin/ls", args...)
	} else if cmdName == "pwd" {
		return exec.Command("/bin/pwd")
	} else if cmdName == "top" {
		return exec.Command("/usr/bin/top")
	}
	return nil
}

/*
# $exec.Command
Runs a foreground command. Run in blocking style.

    Args:[
        command*: ls, (* = required)
        arg0: -l,
        arg1: ~/,
    ]
@command: command name, allows: ls, pwd
*/
func (sb *ExecBranch) Command(ctx *TreeCallCtx) {
	if len(ctx.Args) < 1 {
		ctx.Reject(304, errors.New("Name of room is missing"))
		return
	}
	cmdName := ctx.Args[0]
	fmt.Println("====calling command", cmdName)
	if cmd := GetAllowedCommand(cmdName, ctx.Args[1:]...); cmd != nil {
		out, err := cmd.Output()
		if err != nil {
			if stderr, err1 := cmd.StderrPipe(); err1 == nil {
				b, _ := ioutil.ReadAll(stderr)
				ctx.Reject(304, errors.New(string(b)))
				return
			}
			ctx.Reject(304, err)
			return
		}
		ctx.Resolve(string(out))
		return
	}
	ctx.Reject(304, NotAllowedError)
}

/*
# $exec.BackgroundCommand
    Args:[
        command: top,
        arg0,
        arg1,
    ]
Allows: top
*/
func (sb *ExecBranch) BackgroundCommand(ctx *TreeCallCtx) {
	if len(ctx.Args) < 1 {
		ctx.Reject(304, errors.New("Name of room is missing"))
		return
	}
	cmdName := ctx.Args[0]
	cmd := GetAllowedCommand(cmdName, ctx.Args[1:]...)
	if cmd == nil {
		ctx.Reject(304, NotAllowedError)
	}
	stdout, _ := cmd.StdoutPipe()
	cmd.Start()

	ctx.SetBackground(true)

	var mutex sync.Mutex
	stop := make(chan bool)
	outC := make(chan []byte)
	// copy the output in a separate goroutine so printing can't block indefinitely
	go func() {
		oneRune := make([]byte, utf8.UTFMax)
		for {
			mutex.Lock()
			count, err := stdout.Read(oneRune)
			mutex.Unlock()
			if err != nil {
				fmt.Println("error read stdout", err)
				break
			}
			outC <- oneRune[:count]
		}
		fmt.Println("---stop--")
		stop <- true
	}()
	ctx.On("Kill", func() {
		if err := cmd.Process.Kill(); err != nil {
			fmt.Println("failed to kill process: ", err)
		}
		stop <- true
	})

	var buf bytes.Buffer
loop:
	for {
		//newLine := []byte("\n")
		select {
		case s := <-outC:
			mutex.Lock()
			buf.Write(s)
			if buf.Len() > 1024 {
				ctx.Notify(buf.String())
				fmt.Println(buf.String())
				buf.Reset()
			}
			mutex.Unlock()
		case <-stop:
			break loop
		}
	}
	cmd.Wait()
	fmt.Println("---bye--")
	ctx.Resolve(1)
	return

}
