package tree

import (
	"errors"
	"fmt"
	"log"
	"strings"
    "strconv"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	model "github.com/iapyeh/fastjob/model"
)

type User = model.User
type Command = model.Command
type Result = model.Result
type TreeCallCtx = model.TreeCallCtx
type TreeCallCtxBank = model.TreeCallCtxBank
type Branch = model.Branch
type TreeRoot = model.TreeRoot
type BaseBranch = model.BaseBranch

/*
type ArgPatten struct {
	Type string
	Name string //optional
}
type ParameterPatten struct {
	Args []*ArgPatten
}

func (self *ParameterPatten) AddArgPatten(argtype string, name string) {
	self.Args = append(self.Args, &ArgPatten{Type: argtype, Name: name})
}
func (self *ParameterPatten) String() string {
	stuff := make([]string, len(self.Args))
	for i, arg := range self.Args {
		s := arg.Type
		if arg.Name != "" {
			s = s + " " + arg.Name
		}
		stuff[i] = s
	}
	return "(" + strings.Join(stuff, ", ") + ")"
}
*/

/*
TreeCallHandler is the default handler of a tree.
It is usually mounted to a websocket route, say /objsh/tree like this:
  Router.Websocket("/objsh/tree", node.TreeCallHandler, true)
*/
type TreeCallHandler struct {
	Root *TreeRoot
}

// Handler is the callback for websocket when it is connected
func (self *TreeCallHandler) Handler(wsCtx *model.WebsocketCtx) {
	who := "guest"
	if wsCtx.User != nil {
		who = wsCtx.User.Username()
	}
	log.Println("Tree connected by ", who)
	/*
		layout := wsCtx.Args.Peek("layout")
		if len(layout) > 0 {
			i, err := strconv.ParseInt(string(layout), 10, 32)
			if err == nil {
				layoutMap := Tree.Layout()
				layoutByte, err := json.Marshal(layoutMap)
				if err != nil {
					fmt.Println("treeApiHandler err#3", err)
				}
				ret := Result{
					Retcode: 0,
					Id:      int32(i),
					Stdout:  layoutByte,
				}
				_, err = wsCtx.SendProtobufMessage(&ret)
				if err != nil {
					log.Println("tree send layout error", err)
				}
			}
		}
	*/

	/*
			wsCtx.On("Close", "_", func() {
				log.Println("Tree connection closed by ", who)
		    })
	*/

	wsCtx.On("Protobuf", "_", func(message proto.Message, err error) {

		if err != nil {
			log.Println("err=", err)
			return
		}
		//var callCtx *model.TreeCallCtx
		switch typeName := proto.MessageName(message); typeName {
		case "objsh.Command":
			obj := message.(*Command)

			
			//decode obj.Message (Any Message)
			if obj.Kill {
                callCtx := model.NewSimpleTreeCallCtx(self.Root, obj.Id, wsCtx)
                if idToKill, err := strconv.ParseInt(obj.Name,10,64); err == nil{
                    if err := callCtx.KillPeer(int32(idToKill)); err == nil{
                        callCtx.Resolve("job killing completed")
                    }else{
                        callCtx.Reject(500, err)
                    }    
                }else{
                    callCtx.Reject(400, err)
                }
            } else if !strings.HasPrefix(obj.Name, self.Root.Name) {
                log.Println("Accept ", self.Root.Name+".* only, not ", obj.Name)
                wsCtx.SendTreeCallReturn(&model.TreeCallReturn{
                    CmdID:   obj.Id,
                    Retcode: -404,
                    Stderr:  errors.New(obj.Name + " not found"),
                })
                return
            } else {
				// create TreeCallCtx
				var pbMsg proto.Message
				if obj.Message != nil {
					if objInAny, err2 := ptypes.Empty(obj.Message); err2 == nil {
						if err = proto.Unmarshal(obj.Message.Value, objInAny); err == nil {
							pbMsg = objInAny
						} else {
							log.Println(err)
						}
					} else {
						fmt.Println(err2)
					}
				}
                callCtx := model.NewTreeCallCtx(self.Root, obj.Id, wsCtx, obj.Args, &obj.Kw, &pbMsg)
                
                // 2019-11-21T11:00:02+00:00
                // if not been put to subroute , ex "self.Root.Call(obj.Name, callCtx)"
                // a blocking call will blocks all
				go self.Root.Call(obj.Name, callCtx)
			}
			//default:
			//	//go self.Root.Call(typeName, callCtx)
		}
	})
}
