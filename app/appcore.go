/*
This package contains features that apply across the app.

CONTEXT

We do not depend on information stored per request. Instead, we pass all the information
that is needed for each function via arguments.

All interaction in the application may require a Context. The Context includes (for example):
 - app-engine.Context
 - tx (is this in a transaction?)
 - util.SafeStore (contains all information that normally goes into request attributes)

Handlers should take an app.Context. The top-level handler should create it and pass it down
the function chain. WebRouter supports this natively with a Context object.

DRIVER

This is the interface between the services and the backend system.
It is exposed by an app.Context.

Some typical things the Driver will support:
  - SendMail(...)
  - Give me an appropriate *http.Client
  - Store some entities in the backend
  - Load an entity from backend given its id, or some of its properties
  - ...

MISC

Some misc info:
  - To allow for rpc, testing, and other uses, we provide a header called
    "Z-App-Json-Response-On-Error" (app.UseJsonOnErrHttpHeaderKey).
    If set, then we return errors as a json string, as opposed to showing the
    user friendly, and browser friendly, error view page.
    RPC, Testing, etc will set this on their requests.

*/
package app

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"
	"github.com/ugorji/go-common/safestore"
	"github.com/ugorji/go-common/vfs"
)

const (
	//CtxKey = "app_context_key"

	//if this header is set, and there's an error, we will return response in json format.
	//this way, testing, rpc, etc do not have to deal with the error view for browsers.
	UseJsonOnErrHttpHeaderKey = "Z-App-Json-Response-On-Error"

	//Used for shared cache contents, etc
	//SharedNsPfx = "_shared::"
)

var (
	appdrivers = make(map[string]Driver)
	appmu      = new(sync.RWMutex)
)

func RegisterAppDriver(appname string, driver Driver) {
	appmu.Lock()
	appdrivers[appname] = driver
	appmu.Unlock()
}

func AppDriver(appname string) (dr Driver) {
	appmu.RLock()
	dr = appdrivers[appname]
	appmu.RUnlock()
	return
}

// Context is the context passed by an app into a request.
// Note: It is in the util package so it can be shared by both app and web packages.
type Context interface {
	Id() string
	Store() safestore.I
	AppUUID() string
}


// var (
// 	ErrEntityNotFoundMsg = "<App_Entity_Not_Found>"
// 	//Error returned by Load calls where no entity is found
// 	ErrEntityNotFound = util.WrappedError{ NotFound: true, Cause: util.ErrorString(ErrEntityNotFoundMsg) }
// )

type Tier int32

const (
	DEVELOPMENT Tier = iota + 1
	STAGING
	PRODUCTION
) 

type QueryFilterOp int

const (
	_ QueryFilterOp = iota
	EQ
	GTE
	GT
	LTE
	LT
)

func (q QueryFilterOp) String() string {
	switch q {
	case EQ:
		return "="
	case GT:
		return ">"
	case GTE:
		return ">="
	case LT:
		return "<"
	case LTE:
		return "<="
	}
	return ""
}

func ToQueryFilterOp(op string) QueryFilterOp {
	switch op {
	case "=":
		return EQ
	case ">":
		return GT
	case ">=":
		return GTE
	case "<":
		return LT
	case "<=":
		return LTE
	}
	return 0
}

type Key interface {
	// Encode will encode into a string.
	// Encode() string
	// Incomplete denotes true if this key cannot be stored in datastore as is.
	Incomplete() bool
	// Id returns the value of the Id variable of the entity that this key represents.
	EntityId() int64
}

// The most fundamental entity is the User. It is basic, and only supports basic things,
// like email, login providers/credentials, and tags.
//
// We don't support passwords in here since authentication is handled by external providers.
type User struct {
	_struct bool `db:"keyf=Id,kind=U,kindid=101,auto"`
	Id      int64
	Name    string            `db:"dbname=n"`
	Email   string            `db:"dbname=em"`
	Atn     map[string]string `db:"dbname=a,ftype=expando"`
}

type BlobReader interface {
	io.Reader
	io.ReaderAt
	io.Seeker
	io.Closer
}

type BlobWriter interface {
	io.Writer
	Finish() (key string, err error)
}

type BlobInfo struct {
	Key          string
	ContentType  string
	CreationTime time.Time
	Filename     string
	Size         int64
}

type AppInfo struct {
	Tier   Tier     //= PRODUCTION
	ResVfs *vfs.Vfs // This must always be here (anyone may want resources)
	UUID   string   //this is "typically" a 4-digit string between 1001 and 9999 (identify the instance)
}

// Filter encapsulates filters passed to a query.
type QueryFilter struct {
	Name  string
	Op    QueryFilterOp
	Value interface{}
}

type QueryOpts struct {
	Shape       string
	Limit       int
	Offset      int
	StartCursor string
	EndCursor   string
	Order       string
	//FullEntity bool
	//LastFilterOp string
	//SortDesc bool
}

// This is safe for copying, and embedding as a value.
type BasicContext struct {
	// should implement logging.HasHostRequestId, logging.HasId

	SeqNum     uint64
	TheAppUUID string
	SafeStore  *safestore.T
}

type Cache interface {
	CacheGet(ctx Context, items ...*safestore.Item) (err error)
	CachePut(ctx Context, items ...*safestore.Item) (err error)
	CacheIncr(ctx Context, key interface{}, delta int64, initVal uint64) (newVal uint64, err error)
	CacheDelete(ctx Context, keys ...interface{}) (err error)
}

type Driver interface {
	Render(ctx Context, view string, data map[string]interface{}, wr io.Writer) error
	LandingPageURL(ctx Context, includeHost bool) (string, error)
	Info() *AppInfo
	LowLevelDriver
}

type LowLevelDriver interface {
	DriverName() string
	IndexesOnlyInProps() bool
	InstanceCache() Cache
	SharedCache(returnInstanceCacheIfNil bool) Cache

	NewContext(r *http.Request, appUUID string, seqnum uint64) (Context, error)

	//Check to see whether to use the cache or not for the current operation
	UseCache(ctx Context, preferred bool) bool

	// Return a http client which is applicable for the runtime environment
	// For example, some environments will want you to pass through a proxy, etc
	HttpClient(ctx Context) (*http.Client, error)

	// Return the Host for this app (e.g. mydomain.net:8080, mydomain.net, etc)
	Host(ctx Context) (string, error)

	ParentKey(ctx Context, key Key) Key
	EncodeKey(ctx Context, key Key) string

	GetInfoFromKey(ctx Context, key Key) (kind string, shape string, intId int64, err error)
	DecodeKey(ctx Context, s string) (Key, error)

	// NewKey returns a New Key representing the kind, shape, intId and parent key passed.
	// If intId < 0, allocate a new intId from the backend datastore.
	NewKey(ctx Context, kind string, shape string, intId int64, pkey Key) (key Key, err error)

	DatastoreGet(ctx Context, keys []Key, dst []interface{}) (err error)
	DatastorePut(ctx Context, keys []Key, dst []interface{}, dprops []interface{}) (keys2 []Key, err error)
	DatastoreDelete(ctx Context, keys []Key) (err error)

	// Query for some entities, given pairs of filters which are AND'ed together.
	// The kind and shape limit the returns.
	Query(ctx Context, parent Key, kind string, opts *QueryOpts, filters ...*QueryFilter) (
		res []Key, endCursor string, err error)

	BlobWriter(ctx Context, contentType string) (BlobWriter, error)
	BlobReader(ctx Context, key string) (BlobReader, error)
	BlobInfo(ctx Context, key string) (*BlobInfo, error)
	BlobServe(ctx Context, key string, response http.ResponseWriter) error

	//BlobUpload is tricky and involves request hijacking and rewriting. So don't support it.
	//Also, GAE-GO doesn't even fully support it right now.
	//BlobUploadURL(c Context, successPath string) (*url.URL, error)
}

func (c *BasicContext) Store() safestore.I {
	return c.SafeStore
}

func (c *BasicContext) Id() string {
	return fmt.Sprintf("%s %d", c.TheAppUUID, c.SeqNum)
}

func (c *BasicContext) AppUUID() string {
	return c.TheAppUUID
}

func (c *BasicContext) HostId() string {
	return c.TheAppUUID
}

func (c *BasicContext) RequestId() string {
	return strconv.FormatUint(c.SeqNum, 10)
}
