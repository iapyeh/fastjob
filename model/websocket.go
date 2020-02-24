package model

import (
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"github.com/dgrr/fastws"
	proto "github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/any"
	"github.com/valyala/fasthttp"
)

type WebsocketHandler = func(*WebsocketCtx)
type CloseEventHandler = func()
type MessageHandler = func(string)
type BinaryMessageHandler = func([]byte)
type ProtoBufMessageHandler = func(proto.Message, error)

//pedding: Args should be Args(), for being compatible with RequestCtx
type WebsocketCtx struct {
	Args                    *fasthttp.Args
	User                    User   //available in ProtectMode
	UUID                    string //available in TraceMode
	Conn                    *fastws.Conn
	mode                    fastws.Mode
	Closed                  bool
	closeListener           map[string]CloseEventHandler
	messageBinaryListener   map[string]BinaryMessageHandler
	messageListener         map[string]MessageHandler
	messageProtoBufListener map[string]ProtoBufMessageHandler
	mutex                   sync.RWMutex
}

func NewWebsocketCtx(user User, UUID string, args *fasthttp.Args, conn *fastws.Conn) *WebsocketCtx {
	wsCtx := WebsocketCtx{
		Args:                    args,
		User:                    nil,
		Conn:                    conn,
		mode:                    fastws.ModeText,
		Closed:                  false,
		closeListener:           make(map[string]CloseEventHandler, 0),
		messageListener:         make(map[string]MessageHandler, 0),
		messageBinaryListener:   make(map[string]BinaryMessageHandler, 0),
		messageProtoBufListener: make(map[string]ProtoBufMessageHandler, 0),
	}
	// user and UUID in arguments are mutual exclusive
	if user != nil {
		wsCtx.User = user
	} else if len(UUID) > 0 {
		wsCtx.UUID = UUID
	}
	return &wsCtx
}
func (self *WebsocketCtx) Close() {
	self.mutex.Lock()
	if self.Closed {
		log.Println("Warn:websocket close called twice")
        self.mutex.Unlock()
		return
	}
    log.Println("Websocket closed -------")
	self.Closed = true
	self.mutex.Unlock()

	self.mutex.RLock()
	for token, fn := range self.closeListener {
		log.Println("call close handler", token)
		fn()
	}
	self.mutex.RUnlock()

	// 在Server side先關閉時，要送這行讓自己聽訊息那裡能中斷跳出來（只有Close不會中斷跳出來）
	self.Conn.SetDeadline(time.Now())
	self.Conn.Close()
	self.closeListener = nil
	self.messageListener = nil
	self.messageBinaryListener = nil
	self.messageProtoBufListener = nil
	self.Conn = nil
	self.User = nil
}
func (self *WebsocketCtx) SetBinary(yes bool) {
	if yes {
		self.mode = fastws.ModeBinary
	} else {
		self.mode = fastws.ModeText
	}
}
func (self *WebsocketCtx) GetUser() User {
	return self.User
}

//GenID ，用途 此websocket加入chat room時，作為識別
func (self *WebsocketCtx) GenID() string {
	m := md5.New()
	io.WriteString(m, self.Conn.RemoteAddr().String())
	return fmt.Sprintf("%x", m.Sum(nil))
}

func (self *WebsocketCtx) On(evtName string, token string, fn interface{}, args ...interface{}) bool {
	//evtname := strings.ToLower(evtName)
	if self.IsClosed() {
		log.Printf("listen on closed websocket event %s with token=%s \n", evtName, token)
		return false
	}
	//log.Printf("listen on %s with token=%s\n", evtName, token)
	self.mutex.Lock()
	switch evtName {
	case "Close":
		if _, ok := self.closeListener[token]; ok {
			self.mutex.Unlock()
			//panic(fmt.Sprintf("listen on close failed, due to token %s has registered", token))
			fmt.Sprintf("listen on close failed, due to token %s has registered", token)
			return false
		}
		self.closeListener[token] = fn.(CloseEventHandler)
	case "Message":
		if _, ok := self.messageListener[token]; ok {
			self.mutex.Unlock()
			panic(fmt.Sprintf("listen on message failed, due to token %s has registered", token))
		}
		self.messageListener[token] = fn.(MessageHandler)
	case "BinaryMessage":
		if _, ok := self.messageBinaryListener[token]; ok {
			self.mutex.Unlock()
			panic(fmt.Sprintf("listen on binary message failed, due to token %s has registered", token))
		}
		self.messageBinaryListener[token] = fn.(BinaryMessageHandler)
	case "Protobuf":
		if _, ok := self.messageProtoBufListener[token]; ok {
			self.mutex.Unlock()
			panic(fmt.Sprintf("listen on protobuf message failed, due to token %s has registered", token))
		}
		self.messageProtoBufListener[token] = fn.(ProtoBufMessageHandler)
	default:
		panic("Websocket has not event:" + evtName + ", accept Close, Message, BinaryMessage, ProtoBuf")
	}

	self.mutex.Unlock()
	return true
}
func (self *WebsocketCtx) Off(evtName string, token string) bool {
	if self.IsClosed() {
		log.Printf("listen off closed websocket event %s with token=%s is ignored \n", evtName, token)
		return false
	}
	//log.Printf("listen on %s with token=%s is off\n", evtName, token)
	self.mutex.Lock()
	//evtname := strings.ToLower(evtName)
	found := false
	switch evtName {
	case "Close":
		if _, ok := self.closeListener[token]; ok {
			delete(self.closeListener, token)
			found = true
		}
	case "Message":
		if _, ok := self.messageListener[token]; ok {
			delete(self.messageListener, token)
			found = true
		}
	case "BinaryMessage":
		if _, ok := self.messageBinaryListener[token]; ok {
			delete(self.messageBinaryListener, token)
			found = true
		}
	case "Protobuf":
		if _, ok := self.messageProtoBufListener[token]; ok {
			delete(self.messageProtoBufListener, token)
			found = true
		}
	default:
		panic("Websocket has not event:" + evtName + ", accept Close, Message, BinaryMessage, ProtoBuf")
	}
	self.mutex.Unlock()
	return found
}

/*
func (self *WebsocketCtx) OnClose(fn func(), token string) {
	self.mutex.Lock()
	self.closeListener[token] = fn
	self.mutex.Unlock()
}
func (self *WebsocketCtx) OnMessage(fn func(string)) {
	self.messageListener = append(self.messageListener, fn)
}
func (self *WebsocketCtx) OnMessageBinary(fn func([]byte)) {
	self.messageBinaryListener = append(self.messageBinaryListener, fn)
}
func (self *WebsocketCtx) OnProtobufMessage(fn func(proto.Message, error)) {
	self.messageProtoBufListener = append(self.messageProtoBufListener, fn)
}
*/
func (self *WebsocketCtx) Write(data []byte) (int, error) {
	return self.Send(string(data))
}
func (self *WebsocketCtx) IsClosed() bool {
	self.mutex.RLock()
	c := self.Closed
	self.mutex.RUnlock()
	return c
}
func (self *WebsocketCtx) errorOnIO(err error) {
	log.Printf("ws IO error: %s\n", err)
	if !self.IsClosed() {self.Close()}
}
func (self *WebsocketCtx) Send(data string) (int, error) {
	if self.IsClosed() {
		return 0, errors.New("disconnected")
	}
	size, err := self.Conn.Write([]byte(data))
	if err != nil {
		self.errorOnIO(err)
		return 0, err
	}
	return size, nil
}
func (self *WebsocketCtx) SendBinary(data []byte) (int, error) {
	if self.IsClosed() {
		return 0, errors.New("disconnected")
	}
	size, err := self.Conn.WriteMessage(fastws.ModeBinary, data)
	if err != nil {
		self.errorOnIO(err)
		return 0, err
	}
	return size, nil
}

func (self *WebsocketCtx) SendProtobuf(obj proto.Message) (int, error) {
	//send raw proto.Message object
	binaryBytes, err := proto.Marshal(obj)
	if err == nil {
		return self.SendBinary(binaryBytes)
	} else {
		return 0, err
	}
}
func (self *WebsocketCtx) SendProtobufMessage(obj proto.Message) (int, error) {
	//send proto.Message object encaptured as any.Any
	// 會被包在Any當中傳送，browser端需使用 objshsdk.js 解開此message
	data, err := proto.Marshal(obj)
	if err != nil {
		return 0, err
	}
	anymsg := any.Any{
		TypeUrl: proto.MessageName(obj),
		Value:   data,
	}
	return self.SendProtobuf(&anymsg)
}

func (self *WebsocketCtx) SendTreeCallReturn(ret *TreeCallReturn) (int, error) {
	result := Result{
		Id:      ret.CmdID,
		Retcode: ret.Retcode,
	}
	if ret.Retcode <= 0 { //0 (success) -1 (in progress), -2 (in progress of background task)
		jsonstring, err := json.Marshal(ret.Stdout)
		if err != nil {
			fmt.Println("Convert ret.stdout error", err)
		}
		result.Stdout = jsonstring
	} else {
		result.Stderr = ret.Stderr.Error()
	}
	return self.SendProtobufMessage(&result)

}
func (self *WebsocketCtx) Handle() {
	self.Conn.ReadTimeout = time.Second * 3600
	var msg []byte
	var err error
	defer self.Close()
	for !self.IsClosed() {
		_, msg, err = self.Conn.ReadMessage(msg[:0])
		if err != nil {
			if err != fastws.EOF {
				//log.Printf("error reading message: %s", err)
				self.errorOnIO(err)
			}
			break
		}
		for _, listener := range self.messageListener {
			listener(string(msg))
		}
		for _, blistener := range self.messageBinaryListener {
			blistener(msg)
		}
		if len(self.messageProtoBufListener) > 0 {
			anyMsg := any.Any{}
			var err error
			var obj proto.Message
			if err = proto.Unmarshal(msg, &anyMsg); err == nil {
				if objInAny, err2 := ptypes.Empty(&anyMsg); err2 == nil {
					if err = proto.Unmarshal(anyMsg.Value, objInAny); err == nil {
						obj = objInAny
					}
				}
			}
			for _, pblistener := range self.messageProtoBufListener {
				pblistener(obj, err)
			}
		}
	}
}
