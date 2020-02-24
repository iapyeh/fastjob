package model

import (
	"errors"
	"fmt"
	"log"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"

	"github.com/buaazp/fasthttprouter"
	"github.com/dgrr/fastws"

	//model "github.com/iapyeh/fastjob/model"
	"github.com/valyala/fasthttp"
)

/*
認證很貴。測試的結果，增加讀取cookie找出是誰的這一段功能，效能下降到1／3（B）.
為了避免牽連無辜，本系統採取的作法是，分成三類：
 A: 公開模式＝使用者沒有登入時，不放cookie,所有人都一樣不知道是誰
 B: 追蹤模式＝放cookie（uuid)，在記憶體中建立臨時性User物件
 C: 保護模式＝放cookie (uuid, token)，建立永久性物件，在記憶體中做cache
使用者在指定每一個route時，需指定所要使用的模式。
*/
//ACL mode
/*
const (
	PublicMode  = model.PublicMode
	TraceMode   = model.TraceMode
	ProtectMode = model.ProtectMode
)
*/
func LoginFailedDoRedirect(ctx *fasthttp.RequestCtx) {
	ctx.Response.Header.DelClientCookie(AuthTokenName)
	ctx.Redirect("/", 307)
}

func LoginFailedResponseJson(ctx *fasthttp.RequestCtx) {
	ctx.Response.Header.DelClientCookie(AuthTokenName)
	fmt.Fprintf(ctx, "{\"login\":0}")
}

type RouteRegister struct {
	Router  *fasthttprouter.Router
	Handler func(*fasthttp.RequestCtx)
	//fasthttproute does not have api to know if a path has been used 2019-06-31T07:25:21+00:00
	RegisteredPaths []string
}

func NewRouteRegister() *RouteRegister {
	r := fasthttprouter.New()
	register := RouteRegister{
		Router:          r,
		Handler:         r.Handler,
		RegisteredPaths: make([]string, 0),
	}
	return &register
}
func (self *RouteRegister) HasRegistered(urlPath string) bool {
	for _, p := range self.RegisteredPaths {
		if p == urlPath {
			return true
		}
	}
	return false
}
func (routeRegister *RouteRegister) File(urlPath string, fsPath string, acl int) {

	if _, err := os.Stat(fsPath); os.IsNotExist(err) {
		log.Fatalf("%v not found", fsPath)
	}
	if routeRegister.HasRegistered(urlPath) {
		log.Println("Warn: Double registering existing route: " + urlPath)
		return
	}
	routeRegister.RegisteredPaths = append(routeRegister.RegisteredPaths, urlPath)

	if urlPath == "/" {
		// 暫時這樣
		// do nothing but enforce / not to be protected
		// TODO: allow other acl mode
		if acl == ProtectMode {
			panic("Currently, root path / is enforced to be public accessible")
		}
		acl = PublicMode
	} else if urlPath[len(urlPath)-1:] == "/" {
		urlPath = urlPath + "*filepath"
	} else {
		urlPath = urlPath + "/*filepath"
	}
	switch acl {
	case PublicMode:
		if urlPath == "/" {
			routeRegister.Router.NotFound = fasthttp.FSHandler(fsPath, 0)
		} else {
			routeRegister.Router.ServeFiles(urlPath, fsPath)
		}
	case TraceMode:
		prefix := urlPath[:len(urlPath)-10]
		fileHandler := fasthttp.FSHandler(fsPath, strings.Count(prefix, "/"))
		routeRegister.Router.Handle("GET", urlPath, func(ctx *fasthttp.RequestCtx) {
			SetTraceCookie(ctx)
			fileHandler(ctx)
		})
	case ProtectMode:
		prefix := urlPath[:len(urlPath)-10]
		fileHandler := fasthttp.FSHandler(fsPath, strings.Count(prefix, "/"))
		routeRegister.Router.Handle("GET", urlPath, func(ctx *fasthttp.RequestCtx) {
			user := AuthProvierSingleton.UserFromRequest(ctx)
			if user == nil {
				fmt.Println("probidden to access user folder", urlPath, "prefix=", prefix)
				LoginFailedDoRedirect(ctx)
				return
			}
			fileHandler(ctx)
		})
	}
}

// Get register a http GET request handler
func (routeRegister *RouteRegister) Get(urlPath string, handler RequestHandler, acl int) {
	if routeRegister.HasRegistered(urlPath) {
		log.Println("Warn: Double registering existing get route: " + urlPath)
		return
	}
	routeRegister.RegisteredPaths = append(routeRegister.RegisteredPaths, urlPath)

	switch acl {
	case PublicMode:
		routeRegister.Router.Handle("GET", urlPath, func(ctx *fasthttp.RequestCtx) {
			pCtx := &RequestCtx{
				Ctx:  ctx,
				Args: ctx.QueryArgs(),
			}
			handler(pCtx)
		})
	case TraceMode:
		//與publicMode效能相比，有cookie時降低5％，沒cookie時降低16%
		routeRegister.Router.Handle("GET", urlPath, func(ctx *fasthttp.RequestCtx) {
			pCtx := &RequestCtx{
				Ctx:  ctx,
				Args: ctx.QueryArgs(),
				UUID: SetTraceCookie(ctx),
			}
			handler(pCtx)
		})
	case ProtectMode:
		routeRegister.Router.Handle("GET", urlPath, func(ctx *fasthttp.RequestCtx) {
			user := AuthProvierSingleton.UserFromRequest(ctx)
			if user == nil {
				LoginFailedDoRedirect(ctx)
				return
			}
			//user.Touch()
			pCtx := &RequestCtx{
				Ctx:  ctx,
				Args: ctx.QueryArgs(),
				User: user,
			}
			handler(pCtx)
		})
	}
}

//Post registers a Post request.
func (routeRegister *RouteRegister) Post(urlPath string, handler RequestHandler, acl int) {
	if routeRegister.HasRegistered(urlPath) {
		log.Println("Warn: Double registering existing post route: " + urlPath)
		return
	}
	routeRegister.RegisteredPaths = append(routeRegister.RegisteredPaths, urlPath)

	switch acl {
	case PublicMode:
		routeRegister.Router.Handle("POST", urlPath, func(ctx *fasthttp.RequestCtx) {
			pCtx := &RequestCtx{
				Ctx:  ctx,
				Args: ctx.PostArgs(),
			}
			handler(pCtx)
		})
	case TraceMode:
		//與publicMode效能相比，有cookie時降低5％，沒cookie時降低16%
		routeRegister.Router.Handle("POST", urlPath, func(ctx *fasthttp.RequestCtx) {
			pCtx := &RequestCtx{
				Ctx:  ctx,
				Args: ctx.PostArgs(),
				UUID: SetTraceCookie(ctx),
			}
			handler(pCtx)
		})
	case ProtectMode:
		routeRegister.Router.Handle("POST", urlPath, func(ctx *fasthttp.RequestCtx) {
			user := AuthProvierSingleton.UserFromRequest(ctx)
			if user == nil {
				LoginFailedDoRedirect(ctx)
				return
			}
			//user.Touch()
			pCtx := &RequestCtx{
				Ctx:  ctx,
				Args: ctx.PostArgs(),
				User: user,
			}
			handler(pCtx)
		})
	}
}

type WebsocketOptions struct{
    MaxPayloadSize uint64
}
// WebsocketWithOption registers a websocket handler
func (routeRegister *RouteRegister) WebsocketWithOptions(urlPath string, reqHandler WebsocketHandler, acl int,options *WebsocketOptions) {
	if routeRegister.HasRegistered(urlPath) {
		log.Println("Warn: Double registering existing websocket route: " + urlPath)
		return
	}
	routeRegister.RegisteredPaths = append(routeRegister.RegisteredPaths, urlPath)
	switch acl {
	case PublicMode:
		routeRegister.Router.Handle("GET", urlPath, func(ctx *fasthttp.RequestCtx) {
			//fasthttp will reset args to reuse it, so we got to make a copy
			args := ctx.QueryArgs()
			newargs := fasthttp.Args{}
			args.CopyTo(&newargs)
			upgr := fastws.Upgrader{
				Handler: func(conn *fastws.Conn) {
					wsCtx := NewWebsocketCtx(nil, "", &newargs, conn)
					reqHandler(wsCtx)
					wsCtx.Handle()
				},
				Compress: true,
			}
			upgr.Upgrade(ctx)
		})
	case TraceMode:
		routeRegister.Router.Handle("GET", urlPath, func(ctx *fasthttp.RequestCtx) {
			UUID := SetTraceCookie(ctx)
			//fasthttp will reset args to reuse it, so we got to make a copy
			args := ctx.QueryArgs()
			newargs := fasthttp.Args{}
			args.CopyTo(&newargs)
			upgr := fastws.Upgrader{
				Handler: func(conn *fastws.Conn) {
					wsCtx := NewWebsocketCtx(nil, UUID, &newargs, conn)
					reqHandler(wsCtx)
					wsCtx.Handle()
				},
				Compress: true,
			}
			upgr.Upgrade(ctx)
		})
	case ProtectMode:
		routeRegister.Router.Handle("GET", urlPath, func(ctx *fasthttp.RequestCtx) {
			user := AuthProvierSingleton.UserFromRequest(ctx)
			if user == nil {
				fmt.Println("probidden to access protected websocket", urlPath)
				LoginFailedResponseJson(ctx)
				return
			}

			//fasthttp will reset args to reuse it, so we got to make a copy
			args := ctx.QueryArgs()
			newargs := fasthttp.Args{}
			args.CopyTo(&newargs)
			upgr := fastws.Upgrader{
				Handler: func(conn *fastws.Conn) {
                    if options != nil && options.MaxPayloadSize > 0 {
                        conn.MaxPayloadSize = options.MaxPayloadSize
                    }
					wsCtx := NewWebsocketCtx(user, "", &newargs, conn)
					reqHandler(wsCtx)
					wsCtx.Handle()
				},
				Compress: true,
			}
			upgr.Upgrade(ctx)
		})
	}
}
// Websocket is shortcut of WebsocketWithOption
func (routeRegister *RouteRegister) Websocket(urlPath string, reqHandler WebsocketHandler, acl int) {
    routeRegister.WebsocketWithOptions(urlPath, reqHandler , acl , nil)
}

//只支援上傳一個檔案

type FileUploadCtx struct {
	RequestCtx
	/*
		User       model.User
		UUID       string
		Ctx        *fasthttp.RequestCtx
	*/
	Filesize   int64
	Filename   string
	FileHeader *multipart.FileHeader
}
type UploadHandler func(*FileUploadCtx)

/*
func (fuCtx *FileUploadCtx) Peek(key string) []byte {
	return fuCtx.Args.Peek(key)
}

func (fuCtx *FileUploadCtx) Write(p []byte) (int, error) {
	return fuCtx.Ctx.Write(p)
}
*/
func (fuCtx *FileUploadCtx) SaveTo(path string) error {
	if fuCtx.Filename == "" {
		return errors.New("No file uploaded")
	}
	var abspath string
	var pathToSave string
	if string(path[0]) != "/" {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		abspath = filepath.Join(cwd, path) //, fileHeader.Filename)
	} else {
		abspath = path
	}
	fi, err := os.Stat(abspath)
	if err == nil {
		switch mode := fi.Mode(); {
		case mode.IsDir():
			// save to this folder
			pathToSave = filepath.Join(abspath, fuCtx.Filename)
		default:
			// overwrite existing file
			pathToSave = abspath
		}
	} else {
		pathToSave = abspath
	}
	if err := fasthttp.SaveMultipartFile(fuCtx.FileHeader, pathToSave); err != nil {
		return err
	}
	return nil
}

// FileUpload is not commented yet
func (routeRegister *RouteRegister) FileUpload(urlPath string, uploadHandler UploadHandler, acl int) {
	if routeRegister.HasRegistered(urlPath) {
		log.Println("Warn: Double registering existing fileupload route: " + urlPath)
		return
	}
	// Don't need to put to RegisteredPaths, because "Post" will do it laster.
	//routeRegister.RegisteredPaths = append(routeRegister.RegisteredPaths, urlPath)

	handle := func(ctx *RequestCtx) (size int64, filename string, fileheader *multipart.FileHeader) {
		mf, err := ctx.Ctx.MultipartForm()
		var inputname string
		if err != nil {
			//let self.Filesize = 0
		} else if mf.File == nil {
			//let self.Filesize = 0
		} else {
			type Sizer interface {
				Size() int64
			}
			//only support 1 uploaded file
			for name := range mf.File {
				inputname = name
				break
			}
			fh, err := ctx.Ctx.FormFile(inputname)
			if err == nil {
				size = fh.Size
				filename = fh.Filename
				fileheader = fh
			}
		}

		// Append other parameters in formdata to unified "ctx.Args"
		for key, values := range mf.Value {
			for _, v := range values {
				ctx.Args.Add(key, v)
			}
		}
		return size, filename, fileheader
	}
	routeRegister.Post(urlPath, func(ctx *RequestCtx) {
		size, filename, fileheader := handle(ctx)
		uploadCtx := FileUploadCtx{
			RequestCtx: *ctx,
			Filesize:   size,
			Filename:   filename,
			FileHeader: fileheader,
		}
		uploadHandler(&uploadCtx)
	}, acl)

	/*
		switch acl {
		case PublicMode:
			routeRegister.Post(urlPath, func(ctx *RequestCtx) {
				size, filename, fileheader := handle(ctx.Ctx)
				uploadCtx := FileUploadCtx{
					User:       nil,
					Ctx:        ctx,
					Filesize:   size,
					Filename:   filename,
					FileHeader: fileheader,
				}
				uploadHandler(&uploadCtx)
			}, acl)
		case TraceMode:
			routeRegister.Router.Handle("POST", urlPath, func(ctx *fasthttp.RequestCtx) {
				size, filename, fileheader := handle(ctx)
				uploadCtx := FileUploadCtx{
					User:       nil,
					UUID:       model.SetTraceCookie(ctx),
					Ctx:        ctx,
					Filesize:   size,
					Filename:   filename,
					FileHeader: fileheader,
				}
				uploadHandler(&uploadCtx)
			})
		case ProtectMode:
			routeRegister.Router.Handle("POST", urlPath, func(ctx *fasthttp.RequestCtx) {
				user := authProvider.UserFromRequest(ctx)
				if user == nil {
					log.Println("probidden to access protected websocket", urlPath)
					LoginFailedResponseJson(ctx)
					return
				}
				size, filename, fileheader := handle(ctx)
				uploadCtx := FileUploadCtx{
					User:       user,
					Ctx:        ctx,
					Filesize:   size,
					Filename:   filename,
					FileHeader: fileheader,
				}
				uploadHandler(&uploadCtx)
			})
		}
	*/
}

/*
這個是系統預設登錄用的CGI.它有okback跟errback兩種參數.
可以指定在登陸成功或者失敗之後下一步的網址.
如果都沒有指定,在登錄成功的情況下，他會回傳JSON字串{username:$username}，
如果是失敗他會回傳JSON字串{}，讓瀏覽器那一端的JS知道登入成功或者失敗
*/
func LoginHandler(ctx *RequestCtx) {
	user, err := AuthProvierSingleton.Login(ctx)

	args := ctx.Args
	if err != nil {
		log.Println("Login failed, reason:", err)
		errnext := args.Peek("errnext")
		if len(errnext) > 0 {
			ctx.Ctx.Redirect(string(errnext), 307)
		} else {
			fmt.Fprint(ctx, "{}")
		}
		return
	}
	ctx.User = user
	oknext := args.Peek("oknext")
	if len(oknext) > 0 {
		ctx.Ctx.Redirect(string(oknext), 307)
	} else {
		fmt.Fprintf(ctx, "{\"username\":\"%v\"}", ctx.User.Username())
	}
	log.Println("Login OK:", ctx.User.Username(), "<<<<")
}

//LogoutHandler must be set to ProtectMode
func LogoutHandler(ctx *RequestCtx) {
	AuthProvierSingleton.Logout(ctx)
	args := ctx.Args
	oknext := args.Peek("oknext")
	if len(oknext) > 0 {
		ctx.Ctx.Redirect(string(oknext), 307)
	} else {
		fmt.Fprint(ctx, "1")
	}
}

// Router is not commented yet
var Router = NewRouteRegister()
