package model

import (
	"bytes"
	"crypto/md5"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	_ "path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	uuid "github.com/google/uuid"
	"github.com/valyala/fasthttp"
)

// RequestCtx extend fasthttp.RequestCtx with "User" for protected resource of GET, POST
type RequestCtx struct {
	Ctx *fasthttp.RequestCtx
	// be not nil for ProtectMode
	User User
	// be not nil for TraceMode
	UUID string
	Args *fasthttp.Args //pointer to  fasthttp's QueryArgs and PostArgs
}

func (ctx *RequestCtx) Peek(key string) []byte {
	return ctx.Args.Peek(key)
}
func (ctx *RequestCtx) RemoteAddr() net.Addr {
	return ctx.Ctx.RemoteAddr()
}

func (ctx *RequestCtx) Write(p []byte) (int, error) {
	return ctx.Ctx.Write(p)
}

func (ctx *RequestCtx) WriteString(s string) (int, error) {
	return ctx.Ctx.WriteString(s)
}

// RequestHandler handle ReqestCtx
type RequestHandler func(*RequestCtx)

//GenerateToken is used for generate token, which is server's duty
//UserManager takes the role of a "BaseAuthProvider subsystem"
//UserManager should respect server's rule about token.
type tokenGenerator func(salt string) string

var DefaultTokenGenerator tokenGenerator

func SetTokenGenerator(fn tokenGenerator) {
	DefaultTokenGenerator = fn
}

func SimpleTokenGenerator(salt string) string {
	t := time.Now()
	m := md5.New()
	io.WriteString(m, salt)
	io.WriteString(m, uuid.New().String())
	io.WriteString(m, t.Format("2006010215040599999"))
	return fmt.Sprintf("%x", m.Sum(nil))
}

type passworkHasher func(password string, salt string) []byte

var DefaultPasswordHasher passworkHasher

func SimplePasswordHasher(password string, salt string) []byte {
	m := md5.New()
	io.WriteString(m, password)
	io.WriteString(m, salt)
	io.WriteString(m, password)
	return m.Sum(nil)
}
func SetPasswordHasher(fn passworkHasher) {
	DefaultPasswordHasher = fn
}

//
// User
//
type User interface {
	Username() string //
	Password() []byte
	SetPassword(string)
	//metadata fields
	SetMetadata(string, string)
	GetMetadata(string) (string, bool)
	Metadata() map[string]string
	Activated() bool
	SetActivated(bool)
	Disabled() bool
	SetDisabled(bool)

	// Related to BaseAuthProvider, got value when this instance is cached in memory
	Token() string //in-memory cache only, keep for easy to delete token when logout
	SetToken(string)
	//for clean up
	LastTS() int64 //in-memory cache only
	Touch()

	//CheckPassword(password string, salt string) bool

}

//Avatar is an implementation of interface "User"
type Avatar struct {
	Username_ string //alias of Username
	Password_ []byte

	//metadata fields
	Activated_ bool
	Disabled_  bool

	// Related to BaseAuthProvider, got value when this instance is cached in memory
	token string `json:"-"` //in-memory cache only, keep for easy to delete token when logout

	//for clean up
	lastTS int64 `json:"-"` //in-memory cache only
    
    Metadata_ map[string]string

}

func (avatar *Avatar) Username() string {
	return avatar.Username_
}
func (avatar *Avatar) Password() []byte {
	return avatar.Password_
}
func (avatar *Avatar) Token() string {
	return avatar.token
}
func (avatar *Avatar) LastTS() int64 {
	return avatar.lastTS
}
func (avatar *Avatar) Activated() bool {
	return avatar.Activated_
}
func (avatar *Avatar) Disabled() bool {
	return avatar.Disabled_
}
func (avatar *Avatar) SetUsername(s string) {
	//Can be set only once
	if avatar.Username_ == "" {
		avatar.Username_ = s
	}
}
func (avatar *Avatar) SetPassword(s string) {
	v := DefaultPasswordHasher(s, avatar.Username())
	avatar.Password_ = v
}
func (avatar *Avatar) SetToken(v string) {
	avatar.token = v
}
func (avatar *Avatar) SetLastTS(v int64) {
	avatar.lastTS = v
}
func (avatar *Avatar) SetActivated(v bool) {
	avatar.Activated_ = v
}
func (avatar *Avatar) SetDisabled(v bool) {
	avatar.Disabled_ = v
}
func (avatar *Avatar) SetMetadata(key string, value string) {
    if avatar.Metadata_ == nil{
        avatar.Metadata_ = make(map[string]string)
    }
    avatar.Metadata_[key] = value
}
func (avatar *Avatar) GetMetadata(key string) (string, bool) {
    if avatar.Metadata_ == nil{
        return "", true
    }
    if value, ok := avatar.Metadata_[key]; ok{
        return value, true
    }else{
        return "", false
    }
}
func (avatar *Avatar) Metadata() map[string]string {
    if avatar.Metadata_ == nil{
        avatar.Metadata_ = make(map[string]string)
    }
	return avatar.Metadata_
}

/*
func (avatar *Avatar) CheckPassword(password string, salt string) bool {

	if bytes.Equal(avatar.Password_, DefaultPasswordHasher(password, salt)) {
		return true
	}
	return false
}
*/
func (avatar *Avatar) Touch() {
	avatar.lastTS = time.Now().Unix()
}

//WriteToCookie is a utility to set browser cookie
//if ＠value is "", cookie will be removed
func WriteToCookie(ctx *fasthttp.RequestCtx, cookieName string, value string) {
	cookie := fasthttp.AcquireCookie()
	cookie.SetKey(cookieName)
	cookie.SetPath("/")
	if len(value) > 0 {
		cookie.SetValue(value)
		cookie.SetHTTPOnly(true)
		cookie.SetSameSite(fasthttp.CookieSameSiteStrictMode)
	} else {
		ctx.Response.Header.DelCookie(cookieName)
		cookie.SetExpire(fasthttp.CookieExpireDelete)
	}
	ctx.Response.Header.SetCookie(cookie)
	fasthttp.ReleaseCookie(cookie)
}

//SetTraceCookie  will ensure there is uuid in cookie
func SetTraceCookie(ctx *fasthttp.RequestCtx) string {
	if uuidBytes := ctx.Request.Header.Cookie(UserUUIDName); len(uuidBytes) == 0 {
		uuidstr := uuid.New().String()
		WriteToCookie(ctx, UserUUIDName, uuidstr)
		return uuidstr
	} else {
		return B2S(uuidBytes)
	}
}

const (
	// UserUUIDName is cookies' key
	UserUUIDName = "uuid" //used by cookie
	// AuthTokenName is cookies' key
	AuthTokenName = "token" //used by cookie
)

var (
	errorWrongPassword   = errors.New("Wrong Password")
	errorWrongUsername   = errors.New("Wrong Username")
	errorUserDisabled    = errors.New("User Disabled")
	errorUserInactivated = errors.New("User Inactivated")
)

type PersitentAccountProvider interface {
	GetUser(string) User
}
type PasswordChecker interface {
	CheckPassword(user User, password string) bool
}
type AuthProvider interface {
	UserFromRequest(*fasthttp.RequestCtx) User
	Login(*RequestCtx) (user User, reason error) //success, error
	Logout(*RequestCtx)

	//AccountProvider is an instance which implement interface PersitentAccountProvider
	SetAccountProvider(interface{})
	//PasswordChecker is an instance which implement interface PasswordChecker
	SetPasswordChecker(PasswordChecker)
}

//system-wide singleton of AuthProvider
var AuthProvierSingleton AuthProvider

// Generic implementation of AuthProvider
type BaseAuthProvider struct {
	// This is in-memory table to get user object by uuid for authenticated browser.
	TokenCache map[string]User

	// This is a persistent table to verify a valid uuid+token pair, and recreate an
	// user object into tokenCache. (renew an expired BaseAuthProvider session)
	TokenToUsername Dict

	// Must be a pointer to sync.RWMutex, if not,
	// it can not actually be called in an instance of "inherited" class.
	// REF: https://stackoverflow.com/questions/45784722/golang-data-race-even-with-mutex-for-custom-concurrent-maps
	Mutex *sync.RWMutex

	mstopch         *chan bool
	AccountProvider PersitentAccountProvider
	PasswordChecker PasswordChecker
    // original is fuCheckPassword 
    FuCheckPassword func(user User, password string) bool
}

func (bap *BaseAuthProvider) Cache(user User) {
	bap.TokenCache[user.Username()] = user
}

func (bap *BaseAuthProvider) SetAccountProvider(obj interface{}) {
	if ap, ok := obj.(PersitentAccountProvider); ok {
		bap.AccountProvider = ap
	}
	if pc, ok := obj.(PasswordChecker); ok {
		bap.SetPasswordChecker(pc)
	}
}
func (bap *BaseAuthProvider) SetPasswordChecker(pc PasswordChecker) {
	bap.FuCheckPassword = pc.CheckPassword
}

func (bap *BaseAuthProvider) CheckPassword(user User, password2check string) bool {
	if bytes.Equal(user.Password(), DefaultPasswordHasher(password2check, user.Username())) {
		return true
	}
	return false
}

// @period: check interval in seconds
// @ttl: time to live in seconds
// stop and stopCh are chan to stop the interval, ex:
// stop <- true, *stopCh <- true
func (bap *BaseAuthProvider) KickOffMaintenance(period uint, ttl uint) {

	bap.mstopch = SetInterval(func(stopCh *chan bool) {
		allowedAfter := time.Now().Unix() - int64(ttl)
		expiredTokens := make([]string, 0)
		(*bap.Mutex).RLock()
		for token, user := range bap.TokenCache {
			if user.LastTS() > allowedAfter {
				continue
			}
			expiredTokens = append(expiredTokens, token)
		}
		fmt.Println("ma size", len(expiredTokens), "/", len(bap.TokenCache))
		(*bap.Mutex).RUnlock()
		if len(expiredTokens) > 0 {
			for _, token := range expiredTokens {
				(*bap.Mutex).Lock()
				// Same as what did in Logout()
				delete(bap.TokenCache, token)

				// If un-comment the line below, user have to login again. (so called: session expired)
				//bap.TokenToUsername.DelString(token)

				(*bap.Mutex).Unlock()
			}
		}
	}, int64(period)*1000)

}

// UserFromRequest will be called by every request! Should be of good performance
// @createIfAbsent, when true, if there is no valid cookie, create one for it
func (bap *BaseAuthProvider) UserFromRequest(ctx *fasthttp.RequestCtx) User {
	tokenBytes := ctx.Request.Header.Cookie(AuthTokenName)
	// try to retrive userobj from memory if uuid is presented in cookie
	if len(tokenBytes) == 0 {
		return nil
	}
	token := B2S(tokenBytes)
	//在memory裡面的userobj，應該維持是最新的狀態
	(*bap.Mutex).RLock()
	userincache, ok := bap.TokenCache[token]
	if ok {
		(*bap.Mutex).RUnlock()
		return userincache
	}
	// Verity the UUID + Token pair
	username_byte, err := bap.TokenToUsername.GetString(token)
	username := B2S(username_byte)
	(*bap.Mutex).RUnlock()
	if err == nil {
		// Recreate an user object into the tokenCache to improve performance in next time.
		//if userobjindb := bap.UserFromStorage(username); userobjindb != nil {
		if userobjindb := bap.AccountProvider.GetUser(username); userobjindb != nil {
			// This is the very beginning when an user object been created.
			// Let's make a record for sake of maintance of tokenCache

			(*bap.Mutex).Lock()
			userobjindb.Touch() //utilize bap's Lock, so that don't need to have individual lock for every user object.
			bap.TokenCache[token] = userobjindb
			(*bap.Mutex).Unlock()
			return userobjindb
		}
	}
	WriteToCookie(ctx, AuthTokenName, "")
	return nil
}

// Login will reuse browser side's UUID if it is presented.
// But will regenerate new token
func (bap *BaseAuthProvider) Login(ctx *RequestCtx) (user User, reason error) {
	// try to verify credential
	args := ctx.Args
	username := B2S(args.Peek("username"))
	password := B2S(args.Peek("password"))
	if len(username) == 0 && len(password) == 0 {
		//recover session from token
		user := bap.UserFromRequest(ctx.Ctx)
		if user != nil {
			return user, nil
		} else {
			fmt.Println("#1 errorWrongUsername")
			return nil, errorWrongUsername
		}
	} else if len(username) > 0 {
		// retrieve user's master data from db
		//log.Printf("Verify password by rich userobj in %T\n", bap.AccountProvider)
		var userInDB User
		userInDB = bap.AccountProvider.GetUser(username)
		if userInDB == nil {
			WriteToCookie(ctx.Ctx, AuthTokenName, "")
			fmt.Println("#2 errorWrongUsername")
			return nil, errorWrongUsername
		} else {
			if userInDB.Disabled() {
				return nil, errorUserDisabled
			} else if !userInDB.Activated() {
				return nil, errorUserInactivated
			}
			if ok := bap.FuCheckPassword(userInDB, password); ok {
				userInDB.SetToken(DefaultTokenGenerator(userInDB.Username()))
				(*bap.Mutex).Lock()
				userInDB.Touch()
				bap.TokenCache[userInDB.Token()] = userInDB
				bap.TokenToUsername.SetString(userInDB.Token(), []byte(userInDB.Username()))
				(*bap.Mutex).Unlock()

				WriteToCookie(ctx.Ctx, AuthTokenName, userInDB.Token())
				return userInDB, nil
			}
			WriteToCookie(ctx.Ctx, AuthTokenName, "")
			return nil, errorWrongPassword
		}
	}
	WriteToCookie(ctx.Ctx, AuthTokenName, "")
	fmt.Println("#3 errorWrongUsername")
	return nil, errorWrongUsername
}

// Logout will remove user object from tokenCache, which makes token be invalid.
// Logout will also remove the entry of map(uuid:token+username) from uuid2UsernameTokenDict.
// Since the token is invalid, that entry is invalid too.
func (bap *BaseAuthProvider) Logout(ctx *RequestCtx) {
	// When this is internally called (such as user reset password, or been disabled)
	// ctx might be nil
	if ctx.Ctx != nil {
		//delete token cookie, keep uuid cookie
		WriteToCookie(ctx.Ctx, AuthTokenName, "")
	}
	if ctx.User != nil {
		(*bap.Mutex).Lock()
		delete(bap.TokenCache, ctx.User.Token())
		bap.TokenToUsername.DelString(ctx.User.Token())
		(*bap.Mutex).Unlock()
		log.Println("Logout:", ctx.User.Username())
	}
}

func (bap *BaseAuthProvider) StopMaintenance() {
	*(bap.mstopch) <- true
}

/*
moved to auth/leveldb/leveldb.go
func NewBaseAuthProvider(folder string, accountProvider PersitentAccountProvider) BaseAuthProvider {
	path := filepath.Join(folder, "tokenbank")
	bap := BaseAuthProvider{
		TokenToUsername: NewLevelDbDict(path),
		TokenCache:      make(map[string]User),
		Mutex:           &sync.RWMutex{},
	}
	bap.fuCheckPassword = bap.CheckPassword

	if accountProvider == nil {
		panic("accountProvider is nil")
	}
	bap.SetAccountProvider(accountProvider)

	// 目前TTL設定不長，背景假設是會搭配heartbeat使用
	bap.KickOffMaintenance(uint(300), uint(3600))
	//bap.KickOffMaintenance(uint(3), uint(9))

	return bap
}
*/

//NormalizeUsername is helper function to filter abnormal username
// Rules:
//	1. lower case
//	2. Starts with a-z , if not return empty string
//	3. Accept:  a-z0-9._@ others are omitted
const (
	usernameMinLength = 4
)

var (
	//usernameReserved = []string{"admin", "guest"}
	replaceRe = regexp.MustCompile(`[^\da-z\._@]`)
	allowedRe = regexp.MustCompile(`^[a-z]`)
)

func NormalizeUsername(username string) string {
	//lower case
	lower := strings.ToLower(username)
	//only allow  a-z._@
	replaced := replaceRe.ReplaceAllString(lower, "")
	if pass := allowedRe.MatchString(replaced); pass {
		if len(replaced) < usernameMinLength {
			return ""
		} else {
			return replaced
		}
	} else {
		return ""
	}
}
