// Implement "Dict" interface by LevelDB
package authleveldb
import (
    "encoding/json"
	"errors"
    "log"
    "path/filepath"
    "sync"
    "fmt"
    //fastjob "github.com/iapyeh/fastjob"
    model "github.com/iapyeh/fastjob/model"
	"github.com/syndtr/goleveldb/leveldb"

)
type User = model.User
type PersitentAccountProvider = model.PersitentAccountProvider
type BaseAuthProvider = model.BaseAuthProvider

var (
	KeyNotFoundError = errors.New("Key Not Found")
)

// LevelDbDict is a model.Dict implementation with LevelDB
type LevelDbDict struct {
	db *leveldb.DB
}

func NewLevelDbDict(dbpath string) *LevelDbDict {
	db, err := leveldb.OpenFile(dbpath, nil)
	if err != nil {
		log.Fatal(err)
	}
	dict := LevelDbDict{db: db}
	return &dict
}
func (self *LevelDbDict) Get(key []byte) ([]byte, error) {
	return self.db.Get(key, nil)
}
func (self *LevelDbDict) GetString(key string) ([]byte, error) {
	value, err := self.db.Get([]byte(key), nil)
	if err == nil {
		return value, nil
	} else {
		return nil, err
	}
}

/*
GetStringObject usage example:
	var user AppUser
	//self.accountDict is an instance of LevelDbDict
    if err:=self.accountDict.GetStringObject(username,&user); err == nil{
        return &user
    }
*/
// GetStringObject 會用db裡面的資料initialize obj
// 但如果沒有資料，會有 KeyNotFoundError
func (self *LevelDbDict) GetStringObject(key string, obj interface{}) error {
	if data, err := self.Get([]byte(key)); err == nil {
		err = json.Unmarshal(data, obj)
		if err != nil {
			log.Println("Deserialize Dict.GetStringObject error:", err)
			return err
		}
		return nil
	} else {
		return KeyNotFoundError
	}
}
func (self *LevelDbDict) Set(key []byte, value []byte) error {
	err := self.db.Put(key, value, nil)
	return err
}
func (self *LevelDbDict) SetString(key string, value []byte) error {
	err := self.db.Put([]byte(key), value, nil)
	return err
}

/*
SetStringObject usage example:
	func (self *DrawUserTable) Put(obj *DrawUser) error {
		// self.Entries is an instance of LevelDbDict
        // obj will be serialized by JSON
		return self.Entries.SetStringObject(u.Username, obj)
	}
*/
func (self *LevelDbDict) SetStringObject(key string, obj interface{}) error {
	data, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	return self.SetString(key, data)
}
func (self *LevelDbDict) Del(key []byte) error {
	err := self.db.Delete(key, nil)
	return err
}
func (self *LevelDbDict) DelString(key string) error {
	err := self.db.Delete([]byte(key), nil)
	return err
}
func (self *LevelDbDict) Close() {
	self.db.Close()
}


/*
    2019-11-20T08:59:46+00:00
    暫時取消，目前還沒實際使用


// DictEntriesTable 為通用的功能型base class, "table of dictionary-like entries"的意思
type DictEntry interface {
	Key() string
}
type DictEntriesTable struct {
	Entries      *LevelDbDict
	cacheEnabled bool
	cache        map[string]interface{} //for reducing database query
}

func (self *DictEntriesTable) EnableCache(yes bool) {
	self.cacheEnabled = yes
	if yes && self.cache == nil {
		self.cache = make(map[string]interface{})
	}
}

// GetCached returns an instance of given key in cache
func (self *DictEntriesTable) CacheGet(key string) interface{} {
	//pending: search cache
	if self.cacheEnabled {
		if obj, ok := self.cache[key]; ok {
			return obj
		}
	}
	return nil
}

// Get returns an instance of AppUser of given username
func (self *DictEntriesTable) Get(key string, entry DictEntry) error {
	return self.Entries.GetStringObject(key, entry)
}

// Put store an instance of AppUser to be persistent
func (self *DictEntriesTable) CachePut(key string, entry interface{}) {
	self.cache[key] = entry
}

// Put store an instance of AppUser to be persistent
func (self *DictEntriesTable) Put(entry DictEntry) error {
	if self.cacheEnabled {
		self.CachePut(entry.Key(), entry)
	}
	return self.Entries.SetStringObject(entry.Key(), entry)
}

// PutWithKey store an instance (not implementing DictEntry) to be persistent
func (self *DictEntriesTable) PutWithKey(key string, entry interface{}) error {
	if self.cacheEnabled {
		self.CachePut(key, entry)
	}
	return self.Entries.SetStringObject(key, entry)
}



func NewDictEntriesTable(dictpath string) DictEntriesTable {
	return DictEntriesTable{
		Entries: NewLevelDbDict(dictpath),
	}
}
*/
func NewBaseAuthProvider(folder string, accountProvider PersitentAccountProvider) BaseAuthProvider {
	path := filepath.Join(folder, "tokenbank")
	bap := BaseAuthProvider{
		TokenToUsername: NewLevelDbDict(path),
		TokenCache:      make(map[string]User),
		Mutex:           &sync.RWMutex{},
	}
	bap.FuCheckPassword = bap.CheckPassword

	if accountProvider == nil {
		panic("accountProvider is nil")
	}
	bap.SetAccountProvider(accountProvider)

	// 目前TTL設定不長，背景假設是會搭配heartbeat使用
	bap.KickOffMaintenance(uint(300), uint(3600))
	//bap.KickOffMaintenance(uint(3), uint(9))

	return bap
}


// DictAccountProvider is LevelDB-based persistent account database.
// It implements the model.PersistentAccountStorage interface
type DictAccountProvider struct {
	accountDict *LevelDbDict
}

// GetUser is required by fastjob authentication
// 這個函式用於認證
func (self *DictAccountProvider) GetUser(username string) model.User {
	// Caution:
	// The return value of self.GetUnitTestUser(username)
	// Must be tested by if u != nil, If it returns directly,
	// It will become non-nil even it is nil
	if u := self.GetAppUser(username); u != nil {
		return u
	}
	return nil
}

// GetUnitTestUser is for account manipulation in project
func (self *DictAccountProvider) GetAppUser(username string) *AppUser {
    var user AppUser
	if err := self.accountDict.GetStringObject(username, &user); err == nil {
		return &user
	}
	return nil
}

func (self *DictAccountProvider) CreateAppUser(username string, password string) (*AppUser, error) {
	if NormalizeUsername(username) != username {
		return nil, errors.New(fmt.Sprintf("Invalid Username:%v", username))
	}

	if existed := self.GetUser(username); existed != nil {
		return nil, errors.New(fmt.Sprintf("Username Occupied by %s", existed))
	}

	user := NewUser(username) //{Avatar: Avatar{Username: username}}
	user.SetActivated(true)
	if len(password) > 0 {
		user.SetPassword(password)
	}
	self.Serialize(&user)
	return &user, nil
}

func (self *DictAccountProvider) Deserialize(data []byte) (*AppUser, error) {
	var user *AppUser
	err := json.Unmarshal(data, &user)
	if err != nil {
		log.Println("Deserialize user error:", err)
		return nil, err
	}
	return user, nil
}
func (self *DictAccountProvider) Serialize(user *AppUser) error {
	data, err := json.Marshal(&user)
	if err != nil {
		return err
	}
	key := []byte(user.Username())
	self.accountDict.Set(key, data)
	return nil
}

func (self *DictAccountProvider) ChangePassword(username string, password string) error {

    var user *AppUser
    if  user = self.GetAppUser(username); user == nil {
		return errors.New(fmt.Sprintf("No user of given name"))
	}

	if len(password) > 0 {
		user.SetPassword(password)
	}
	self.Serialize(user)
	return nil
}

// AccountProvider is globally accessible in project for account maintencance.
// AccountProvider is the singleton of DictAccountProvider
// 2019-11-20T09:30:30+00:00
// Deprecated, use DictUserManager.GetAccountProvider() instead
//var AccountProvider *DictAccountProvider

// DictUserManager is LevelDB-based User Manager
// It inherits "BaseAuthProvider" and
// It has a persistant AccountProvider wich implements PersistantAccountStorage.
type DictUserManager struct {
    AccountProvider *DictAccountProvider
	model.BaseAuthProvider
}
func (self *DictUserManager) GetAccountProvider() *DictAccountProvider{
    return self.AccountProvider
}

var DictUserManagerSingleton *DictUserManager;
// NewDictUserManager creates an instance of DictUserManager
// It is an implementation of the  AuthHandler interface.
// It is based on BaseAuthProvider, so it implements the AccountProvider interface
func NewDictUserManager(dbpath string) *DictUserManager {
    
    if (DictUserManagerSingleton != nil) {return DictUserManagerSingleton}
    
    AccountProvider := &DictAccountProvider{
		accountDict: NewLevelDbDict(filepath.Join(dbpath, "account")), //username: user data in json
	}
	manager := DictUserManager{
        AccountProvider: AccountProvider,
		BaseAuthProvider: NewBaseAuthProvider(
			dbpath,
			AccountProvider,
		),
	}
	// BaseAuthProvider will auto starts a maintenance job,
	// If you know what you are doing, you can stop it by calling:
	//manager.StopMaintenance()

    DictUserManagerSingleton = &manager
	return DictUserManagerSingleton
}

