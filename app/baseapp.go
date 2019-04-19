/*
 This is the base of an actual application. It sets up everything and handles requests.
 By design, it fully implements app.AppDriver.

 It sets up the following specifically:
   - logging (calling logging.RunAsync if desired)
   - app Driver (setup app.Svc)
   - Initialize template sets for all the different views.
     Including creating a function map for the templates and defining an appropriate Render method
   - Setup Routing logic ie how to route requests
   - Setup OauthHandles for all our supported oauth providers
   - map all requests to its builtin dispatcher
     (which wraps router.Dispatch and does pre and post things)

 Why we did things a certain way
   - Wrapping ResponseWriter:
     So we can know if the headers have been written (ie response committed)

 We need to differentiate code for dev environment from prod environment:
   - Tests should not be shipped on prod
   - LoadInit, other dev things should not even run on prod

 This package expects the following:
   - Define a route called "landing" (which is typically mapped to Path: /)
     so we can route to the landing page or show the link to the landing page
   - Define views called "error", "notfound" so we can show something when either is encountered from code
   - Also define view called "apperror", and we just show its content when there's an error
     without inheriting or depending on anyone else.
*/
package app

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"reflect"
	"regexp"
	// "runtime"
	"strconv"
	"sync"
	"sync/atomic" //"runtime/debug"
	"time"
	//"text/template"
	"github.com/ugorji/go-common/logging"
	"github.com/ugorji/go-common/safestore"
	"github.com/ugorji/go-common/util"
	"github.com/ugorji/go-web"

	// "crypto/rand"
	// "math/big"
	"github.com/ugorji/go-common/vfs"
	"github.com/ugorji/go-common/zerror"
)

const (
//Used for shared cache contents, etc
//SharedNsPfx = "_shared::"
//defaultMaxMemory = 4 << 20
)

type BaseApp struct {
	BaseDriver
	AppDriver Driver
	//SecureProvNames []string
	HostFn       func(Context) (string, error)
	HttpClientFn func(Context) (*http.Client, error)

	DumpRequestAtStartup bool
	DumpRequestOnError   bool

	reqSeq      uint64
	onceInitErr error
	initMu      sync.Mutex
	inited      uint32
	//minLogLevel = logging.INFO
	//firstRequestHost string
}

type PageNotFoundError string

func (e PageNotFoundError) Error() string {
	return string(e)
}

type BaseDriver struct {
	AppInfo
	Views       *web.Views // = web.NewViews()
	PreRenderFn func(ctx Context, view string, data map[string]interface{}) error
	Root        *Route
}

type SafeStoreCache struct {
	*safestore.T
}

type HTTPHandler struct {
	App       *BaseApp
	InitFn    func(c Context, w http.ResponseWriter, r *http.Request) (err error)
	StartFn   func(c Context, w http.ResponseWriter, r *http.Request) (haltOnErr bool, err error)
	OnErrorFn func(err interface{}, c Context, w http.ResponseWriter, r *http.Request)
}

// type LoggingHandler logging.DefHandler

type myResponseError struct {
	Code    int
	Path    string
	Message string
	// Trace   string
}

func NewApp(devServer bool, uuid string, viewsCfgPath string, lld LowLevelDriver) (gapp *BaseApp, err error) {
	defer zerror.OnErrorf(1, &err, nil)
	type tlld struct {
		*BaseDriver
		LowLevelDriver
	}
	gapp = new(BaseApp)
	gapp.AppDriver = tlld{&gapp.BaseDriver, lld}
	gapp.UUID = uuid
	gapp.Tier = PRODUCTION
	if devServer {
		gapp.Tier = DEVELOPMENT
	}

	gapp.Views = web.NewViews()
	gapp.Root = NewRoot("Root")
	// uuid, err = util.Uuid(16)
	// anId, err := rand.Int(rand.Reader, big.NewInt(8998))
	// gapp.UUID = strconv.FormatInt(anId.Int64()+1001, 10)
	gapp.ResVfs = new(vfs.Vfs)
	if err = gapp.ResVfs.Adds(false, "resources.zip", "resources"); err != nil {
		return
	}

	// load templates
	tmplVfs := new(vfs.Vfs)
	defer tmplVfs.Close()

	//if err = tmplVfs.AddIfExist("templates.zip"); err != nil { return err }
	if err = tmplVfs.Adds(false, "templates.zip", "templates"); err != nil {
		return
	}
	vcn := new(web.ViewConfigNode)

	//f, err := os.Open(viewsCfgPath)
	f, _, err := gapp.ResVfs.Find(viewsCfgPath)
	if err != nil {
		return
	}
	defer f.Close()
	if err = json.NewDecoder(f).Decode(vcn); err != nil {
		return
	}
	// vcfg := web.NodeToMap(vcn)
	// logging.Trace(nil, "VCN: %v =======> VCFG: %v", vcn, vcfg)

	toUrlLink := func(ctx Context, route string, params ...interface{}) (string, error) {
		logging.Trace(ctx, "Getting Link for: route: %v, params: %v", route, params)
		sparams := make([]string, len(params))
		for i, p := range params {
			sparams[i] = fmt.Sprint(p)
		}
		// logging.Trace(ctx, "Calling Route: %v, with params: %v", route, params)
		url0, err := gapp.Root.FindByName(route).ToURL(sparams...)
		if err == nil {
			return url0.String(), nil
		}
		return "", err
	}
	gapp.Views.FnMap["Link"] = toUrlLink
	gapp.Views.FnMap["Eq"] = reflect.DeepEqual

	re, err := regexp.Compile(`.*\.thtml`)
	if err != nil {
		return
	}
	if err = gapp.Views.AddTemplates(tmplVfs, re); err != nil {
		return
	}
	if err = gapp.Views.Load(vcn); err != nil {
		return
	}
	RegisterAppDriver(gapp.UUID, gapp.AppDriver)
	return
}

func (gapp *BaseApp) newContext(r *http.Request) (c Context, err error) {
	i := atomic.AddUint64(&gapp.reqSeq, 1)
	//Inject GC during request. Doesn't help.
	// if i % 1000 == 0 {
	// 	runtime.GC()
	// }
	return gapp.AppDriver.NewContext(r, gapp.UUID, i)
}

func (gapp *BaseDriver) Info() *AppInfo {
	return &gapp.AppInfo
}

//App Render method, which adds some variables to the data for the templates.
//Note: All added keys start with Z.
//(so application code should not add keys which start with Z).
func (gapp *BaseDriver) Render(ctx Context, view string, data map[string]interface{}, wr io.Writer) (err error) {
	defer zerror.OnErrorf(1, &err, nil)
	v, ok := gapp.Views.Views[view]
	if !ok {
		//emsg := fmt.Sprintf("No View found for: %s", view)
		//err = web.Error(emsg, errors.New(emsg), http.StatusNotFound)
		err = PageNotFoundError(fmt.Sprintf("No View found for: %s", view))
		return
	}

	// data["Zdrivername"] = ctx.DriverName()
	// data["Zdriver_" + ctx.DriverName()] = true
	data["Zcontext"] = ctx

	if gapp.PreRenderFn != nil {
		if err = gapp.PreRenderFn(ctx, view, data); err != nil {
			return
		}
	}
	tmpl := v.Lookup("main")
	if tmpl == nil {
		err = fmt.Errorf("No main template defined for view: %v", view)
		return
	}
	if err = tmpl.Execute(wr, data); err != nil {
		return
	}
	return nil
}

func (gapp *BaseDriver) LandingPageURL(ctx Context, includeHost bool) (s string, err error) {
	defer zerror.OnErrorf(1, &err, nil)
	u, err := gapp.Root.FindByName("landing").ToURL()
	if err != nil {
		return
	}
	if includeHost {
		dr := AppDriver(ctx.AppUUID())
		if u.Host, err = dr.Host(ctx); err != nil {
			return
		}
	}
	s = u.String()
	return
}

func (h HTTPHandler) ServeHTTP(w0 http.ResponseWriter, r *http.Request) {
	//NOTE: Do not use asynchronous logging (as we pass mutable objects in call to logging)
	//go logging.Run()
	var (
		gapp = h.App
		c    Context
		err  error
		w    web.ResponseWriter
	)
	if gapp.Tier == DEVELOPMENT {
		time0 := time.Now()
		defer func() {
			logging.Always(c, "Request Completed in: %v", time.Now().Sub(time0))
		}()
	}
	defer func() {
		// // println("gapp.Tier is DEV: ", gapp.Tier == DEVELOPMENT)
		// if gapp.Tier == DEVELOPMENT {
		// 	return // don't recover - just crash, and see full trace
		// }
		if x := recover(); x != nil {
			gapp.derr(x, c, w, r, h.OnErrorFn)
		}
	}()
	if c, err = gapp.newContext(r); err != nil {
		//don't panic here, since w and c are nil
		http.Error(w0, err.Error(), 500)
		return
	}
	//defer func() { logging.Debug(nil, "w.headerWritten: %v", w.headerWritten) }()
	// w = web.ToResponseWriter(w0, r, web.GzipTypes)
	// defer w.Finish()
	w = web.AsResponseWriter(w0)
	defer w.Flush()
	//var c99 app.app.Context = c
	//logging.Info(nil, "XXXXXXX: As App.Context: %v", reflect.TypeOf(c99

	//Do not parse here. Instead, anyone needing req.Form[] should call FormValue first to ensure parsed.
	//if err = r.ParseMultipartForm(defaultMaxMemory); err != nil {
	//	derr(err, c, w, r, h.OnErrorFn)
	//	return
	//}
	if gapp.DumpRequestAtStartup {
		DumpRequest(c, r)
	}
	if h.InitFn != nil {
		// we don't want to create a closure each time, when once.Do is actually never run except first time
		// gapp.once.Do(func() {
		// 	gapp.onceInitErr = h.InitFn(c, w, r)
		// })
		//
		// Instead, simulate once.Do functionality
		// (so closure is only created once, and once.Do is not constantly called).
		// Note:
		// The atomic.Compare ensures that all previous values e.g. onceInitErr are committed (full barrier).
		// The atomic.Load ensures that the Compare has been called, meaning onceInitErr is already committed.
		if atomic.LoadUint32(&gapp.inited) == 0 {
			// it's possible that multiple goroutines go into this block, but only 1 calls InitFn
			func() {
				gapp.initMu.Lock()
				defer gapp.initMu.Unlock()
				if gapp.inited == 0 {
					gapp.onceInitErr = h.InitFn(c, w, r)
					atomic.StoreUint32(&gapp.inited, 1)
					// only log information the first time when onceInitErr is set
					if gapp.onceInitErr != nil {
						if gapp.Tier == DEVELOPMENT {
							logging.Severe(c, "Initialization Error: %v\n%s", gapp.onceInitErr, util.Stack(nil, false))
							//debug.PrintStack()
							if gapp.DumpRequestOnError {
								DumpRequest(c, r)
							}
						} else {
							logging.Severe(c, "Initialization Error: %v", gapp.onceInitErr)
						}
					}
				}
			}()
		}
		if gapp.onceInitErr != nil {
			http.Error(w, gapp.onceInitErr.Error(), http.StatusInternalServerError)
			return
		}
	}
	if h.StartFn != nil {
		var haltOnError bool
		if haltOnError, err = h.StartFn(c, w, r); err != nil {
			if haltOnError {
				return
			}
			gapp.derr(err, c, w, r, h.OnErrorFn)
			return
		}
	}
	logging.Trace(c, "Dispatching: URL: %v", r.RequestURI)
	if err = Dispatch(c, gapp.Root, w, r); err != nil {
		gapp.derr(err, c, w, r, h.OnErrorFn)
		return
	}
	// okbuf := make([]byte, 4096)
	// for i := 0; i < 4096; i++ { okbuf[i] = '-' }
	// okbuf[4095] = '\n'
	// fnHealthChk := func(w0 http.ResponseWriter, r *http.Request) {
	// 	w0.Write(okbuf)
	// }
	// http.HandleFunc("/_ugorji_dot_net_health_check", fnHealthChk)
}

func (ch SafeStoreCache) CacheGet(ctx Context, items ...*safestore.Item) (err error) {
	ch.Gets2(items...)
	//((*util.SafeStore)(ch)).GetAllStored(items...)
	return
}

func (ch SafeStoreCache) CachePut(ctx Context, items ...*safestore.Item) (err error) {
	ch.Puts(items...)
	//((*util.SafeStore)(ch)).PutAllStored(items...)
	return
}

func (ch SafeStoreCache) CacheIncr(ctx Context, key interface{}, delta int64,
	initVal uint64) (newVal uint64, err error) {
	newVal = ch.Incr(key, delta, initVal)
	//newVal = ((*util.SafeStore)(ch)).Incr(key, delta, initVal)
	return
}

func (ch SafeStoreCache) CacheDelete(ctx Context, keys ...interface{}) (err error) {
	ch.Removes(keys...)
	//((*util.SafeStore)(ch)).PutAll(keys, nil, nil)
	return
}

//Handles error from a dispatch request.
//err is an interface{} so we share same method with panic/recover from dispatch.
//It will show the error or notfound view based on the type of the error.
//This is done if/only if the response is not committed (ie headers not written).
//
//derr function expects w, r and c to be non-nil
func (gapp *BaseApp) derr(
	err interface{},
	c Context,
	w web.ResponseWriter,
	r *http.Request,
	onRequestError func(interface{}, Context, http.ResponseWriter, *http.Request),
) {
	if gapp.Tier == DEVELOPMENT {
		//debug.PrintStack()
		logging.Error(c, "Error handling request: %v\nStackTrace ... \n%s", err, util.Stack(nil, false))
		if gapp.DumpRequestOnError {
			DumpRequest(c, r)
		}
	} else {
		logging.Error(c, "Error handling request: %v", err)
	}
	if onRequestError != nil {
		onRequestError(err, c, w, r)
	}
	if w.IsHeaderWritten() {
		return
	}
	useJsonOnErr, _ := strconv.ParseBool(r.Header.Get(UseJsonOnErrHttpHeaderKey))
	errTmpl := gapp.Views.Views["apperror"]
	if errTmpl != nil {
		errTmpl = errTmpl.Lookup("content")
	}
	useJsonOnErr = useJsonOnErr || errTmpl == nil
	logging.Trace(c, "&&&&&&&&&&&&&&&&: useJsonOnErr: %v", useJsonOnErr)
	var (
		data     map[string]interface{}
		jsondata *myResponseError
		err2     error
	)
	fnErr := func(view string, code int) {
		logging.Trace(c, "fnErr: view: %v, code: %v", view, code)
		w.WriteHeader(code)
		if useJsonOnErr {
			jsondata.Code = code
			var buf2 []byte
			buf2, err2 = json.MarshalIndent(jsondata, "  ", "  ")
			if err2 == nil {
				//fmt.Printf("(%s)\n", string(buf2))
				_, err2 = w.Write(buf2)
			}
		} else {
			//Shouldn't call Render fullstack when Render just threw a nasty exception.
			//Use a simple template execute instead.
			//err2 = c.Render(c, view, data, w)
			err2 = errTmpl.Execute(w, data)
		}
	}
	//http.Error(w, fmt.Sprintf("Error: %v", err), http.StatusInternalServerError)
	logging.Trace(c, "err: %v", err)
	if useJsonOnErr {
		w.Header().Set("Content-Type", "text/plain")
		jsondata = new(myResponseError)
		if err != nil {
			jsondata.Message = fmt.Sprintf("%v", err)
			//jsondata.Trace = errTrace
		}
		jsondata.Path = r.URL.Path
	} else {
		//Don't set Content-Type, so that browser auto-discovers it based on content.
		//This helps support our development mode error reporting (just plain text file)
		//w.Header().Set("Content-Type", "text/html")
		data = make(map[string]interface{})
		data["Error"] = fmt.Sprintf("%v", err)
		data["LandingPageURL"], _ = gapp.AppDriver.LandingPageURL(c, false)
		data["Path"] = r.URL.Path
		//data["Trace"] = errTrace
	}
	//check if a wrapped util.Error, since we need the real error for disambiguation

	if _, ok9 := err.(PageNotFoundError); ok9 {
		fnErr("notfound", http.StatusNotFound)
	} else {
		fnErr("error", http.StatusInternalServerError)
	}

	if err2 != nil && err != err2 {
		logging.Error2(c, err2, "Error while handling error: %v", err)
	}
}

func DumpRequest(c Context, r *http.Request) (err error) {
	dump, err := httputil.DumpRequest(r, true)
	if err != nil {
		logging.Error2(c, err, "Error dumping request")
	}
	if len(dump) > 0 {
		logging.Debug(c, "REQUEST DUMP: \n%v", string(dump))
	}
	return
}

// func runtimeStack() (v []byte) {
// 	return util.Stack(nil, false)
// }
