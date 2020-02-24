package tree

import (
	"errors"
	"fmt"
	"sync"
)

// Every websocket can join every rooms once
//
type Member struct {
	ID   string
	Name string
	Ctx  *TreeCallCtx
}

// 因為同一個帳號可能建立好幾個連線，所以不使用username，
// 而是依據websocket（連線）產生ID
func getMemberID(ctx *TreeCallCtx) string {
	return ctx.WsCtx.GenID()
}

//ChatMessage's property should be exportable for JSON to encode it.
type ChatMessage struct {
	Kind    string
	Payload interface{}
	From    string
}

type ChatRoom struct {
	Name   string
	Member map[string]*Member //key is TreeCallCtx.CmdID
	Mutex  sync.RWMutex
}

func (cr *ChatRoom) join(mID string, name string, ctx *TreeCallCtx) bool {
	cr.Mutex.Lock()
	_, ok := cr.Member[mID]
	cr.Mutex.Unlock()
	if !ok {
		cr.Member[mID] = &Member{
			ID:   mID,
			Name: name,
			Ctx:  ctx,
		}
		fmt.Println(string(mID) + " joined to " + cr.Name)
		return true
	}
	return false
}
func (cr *ChatRoom) exit(mID string) bool {
	cr.Mutex.Lock()
	m, ok := cr.Member[mID]
	cr.Mutex.Unlock()
	if ok {
		delete(cr.Member, mID)
		fmt.Println(m.Name + " exit " + cr.Name)
		return true
	}
	fmt.Println(string(mID[:]) + " not joined in " + cr.Name)
	return false
}

func (cr *ChatRoom) talk(fromMID string, content interface{}) {
	cr.Mutex.RLock()
	for mID, member := range cr.Member {
		// It seems that we can compare MemberID simpley by "="
		//if bytes.Equal(mID[:], byMID[:]) {
		if mID != fromMID {
			member.Ctx.Notify(&ChatMessage{
				Kind:    "Talk",
				Payload: content,
				From:    fromMID,
			})
		}
	}
	cr.Mutex.RUnlock()
}
func (cr *ChatRoom) broadcast(mesg *ChatMessage) {
	cr.Mutex.RLock()
	for _, member := range cr.Member {
		member.Ctx.Notify(mesg)
	}
	cr.Mutex.RUnlock()
}

//List returns a map(memerID:memberName)
func (cr *ChatRoom) list() *map[string]string {
	list := make(map[string]string)
	cr.Mutex.RLock()
	for mID, member := range cr.Member {
		list[mID] = member.Name
	}
	cr.Mutex.RUnlock()
	return &list
}

// ChatBranch would connect to tree
type ChatBranch struct {
	BaseBranch
	room  map[string]*ChatRoom
	Mutex sync.RWMutex
	//trace rooms which a member has join. Used when a user disconnected
	memberInRooms map[string][]*ChatRoom
}

func (cb *ChatBranch) BeReady(treeroot *TreeRoot) {
	cb.SetName("$chat")
	cb.InitBaseBranch()
	cb.room = make(map[string]*ChatRoom)
	cb.memberInRooms = make(map[string][]*ChatRoom)
	cb.Export(cb.Join, cb.Exit, cb.Talk)
	treeroot.SureReady(cb)
}

/*
Join to a chat room
Args: [roomName:unittest, myame:your name]
Kw: {
    key1: value1,
    key2: value2
}
Returns:
Notify: {Kind: "List", Payload: {
        "memberID" : "member Name"
}}
Reject: 304, Name of room is missing
Reject:
*/
func (cb *ChatBranch) Join(ctx *TreeCallCtx) {
	if len(ctx.Args) < 2 {
		ctx.Reject(304, errors.New("Name of room is missing"))
		return
	}
	mID := getMemberID(ctx)
	roomName := ctx.Args[0]
	var room *ChatRoom
	var ok bool
	cb.Mutex.Lock()
	if room, ok = cb.room[roomName]; ok {
		//room.Join(mID, ctx)
	} else {
		fmt.Println("Create room ", roomName)
		room = &ChatRoom{
			Name:   roomName,
			Member: make(map[string]*Member),
		}
		cb.room[roomName] = room
	}
	cb.Mutex.Unlock()
	myName := ctx.Args[1]
	if room.join(mID, myName, ctx) {
		var rooms []*ChatRoom
		if rooms, ok = cb.memberInRooms[mID]; ok {
			cb.memberInRooms[mID] = append(rooms, room)
		} else {
			cb.memberInRooms[mID] = make([]*ChatRoom, 1)
			cb.memberInRooms[mID][0] = room
			// Listen on websocket close event at fisttime
			ctx.WsCtx.On("Close", "cj"+string(mID), func() {
				// purge ctx.Args to enforce cb.Exit exits from all rooms
				ctx.Args = make([]string, 0)
				cb.Exit(ctx)
			})
		}
	}
	room.broadcast(&ChatMessage{Kind: "Join", Payload: mID + "\t" + myName})
	ctx.Notify(&ChatMessage{Kind: "List", Payload: room.list()})
}
func (cb *ChatBranch) Exit(ctx *TreeCallCtx) {
	mID := getMemberID(ctx)
	cb.Mutex.RLock()
	rooms, ok := cb.memberInRooms[mID]
	cb.Mutex.RUnlock()
	if ok {
		if len(ctx.Args) > 0 {
			//exit from a room
			fmt.Println("exiting", ctx.Args[0])
			cb.Mutex.RLock()
			room, ok := cb.room[ctx.Args[0]]
			cb.Mutex.RUnlock()
			if ok {
				if room.exit(mID) {
					//no more join in any room
					if len(rooms) == 1 {
						fmt.Println("memberInRoom purged")
						delete(cb.memberInRooms, mID)
					} else {
						//update cb.memberInRooms[mID]
						idx := -1
						for i, r := range rooms {
							if r == room {
								idx = i
								break
							}
						}
						if idx >= 0 {
							rooms[idx] = rooms[len(rooms)-1]
							rooms = rooms[:len(rooms)-1]
							cb.Mutex.Lock()
							cb.memberInRooms[mID] = rooms
							cb.Mutex.Unlock()
						}
					}

					if len(room.Member) == 0 {
						// Remove this room if there is nobody in it
						fmt.Println("delete room ", room.Name)
						cb.Mutex.Lock()
						delete(cb.room, ctx.Args[0])
						cb.Mutex.Unlock()
					} else {
						//Broadcast to nofity member exit
						room.broadcast(&ChatMessage{Kind: "Exit", Payload: mID})
					}
				}
			}
		} else {
			//exit from all room
			for _, room := range rooms {
				room.exit(mID)
				if len(room.Member) == 0 {
					// Remove this room if there is nobody in it
					fmt.Println("delete room ", room.Name)
					cb.Mutex.Lock()
					delete(cb.room, room.Name)
					cb.Mutex.Unlock()
				} else {
					//Broadcast to nofity member exit
					room.broadcast(&ChatMessage{Kind: "Exit", Payload: mID})
				}
			}
			//no more join in any room
			cb.Mutex.Lock()
			delete(cb.memberInRooms, mID)
			cb.Mutex.Unlock()
		}
	}

	if !ctx.WsCtx.IsClosed() {
		//Exit is called by user, not because of lost connection
		ctx.Resolve(1)
	}
}

/*

$chat.Talk()
=============

Talk is doing broadcasting to all members in the room.
Member who talks will not receive echo message.

Parameters:
-----------

Args:[   RoomName,    Message ]
@RoomName: the room to talk
@Message: the message of your talk

Kw: {
    key1: value1,
    key2: value2
}

Returns:
---------

 Resolve: 1
 Reject: 304 , Room not found

*/
func (cb *ChatBranch) Talk(ctx *TreeCallCtx) {
	if len(ctx.Args) < 2 {
		ctx.Reject(304, errors.New("Name of room is missing"))
		return
	}
	roomName := ctx.Args[0]
	var room *ChatRoom
	var ok bool
	cb.Mutex.RLock()
	room, ok = cb.room[roomName]
	cb.Mutex.RUnlock()
	if ok {
		mID := getMemberID(ctx)
		room.talk(mID, ctx.Args[1])
		ctx.Resolve(1)
	} else {
		ctx.Reject(304, errors.New("Room name is not found"))
	}
}
