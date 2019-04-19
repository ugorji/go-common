/*
WEB ROUTER

Intelligently dispatches requests to handlers.

It figures out what handler to dispatch a request to, by
analysing any/all attributes of the request (headers and url). It also supports the
ability to recreate a request (URL + Headers) based on the characteristics of a Route.

A Route is Matched if all its Routes or Route Expressions match.

It works as follows:
  - A Route is a node in a tree. It can have children, and also have matchExpr to determine
    whether to proceed walking down the tree or not.
  - At runtime, the package looks for the deepest Route which can handle a Request,
    in sequence. This means that a branch is checked, and if it matches, then its children
    are checked. All this recursively. If none of it's children can handle the request, then
    the branch handles it.
  - You can also reverse-create a URL for a route, from the parameters of the route. For example,
    if a route has Host: {hostname}.mydomain.com, and Path: /show/{id}, you should be able to
    reconstruct the URL for that host, passing appropriate parameters for hostname and id.

An application will define functions with the signature:
  (w http.ResponseWriter, r *http.Request) (int, os.Error)

These functions will handle requests, and return the appropriate http status code, and an error
The application wrapper can then decide whether to do anything further based on these e.g.
show a custom error view, etc

In addition, during a Dispatch, when Paths are matched, it will look for named variables and
store them in a request-scoped store. This way, they are not parsed again, and can be used for:
  - during request handling, to get variables
  - during URL generation of a named route

An application using the router will have pseudo-code like:
   -----------------------------------------------------
   func main() {
     go logging.Run()
     root = web.NewRoot("Root")
     web.NewRoute(root, "p1", p1).Path("/p1/${key}")
     ...
     http.HandleFunc("/", MyTopLevelHandler)
   }

   func MyTopLevelHandler(w http.ResponseWriter, r *http.Request) {
     ... some filtering work
     statusCode, err = web.Dispatch(ctx, root, w, r)
     ... some more filtering and error handling
   }
   -----------------------------------------------------

*/
package app

import (
	"net/http"
	"net/url"

	"github.com/ugorji/go-common/logging"
	"github.com/ugorji/go-common/safestore"
	"github.com/ugorji/go-common/util"
	"github.com/ugorji/go-common/zerror"
)

const (
	VarsKey   = "router_vars"
	LogTarget = "router"
)

// This interface will serve http request, and return a status code and an error
// This allows the wrapping function to do filtering and error handling
type Handler interface {
	HandleHttp(Context, http.ResponseWriter, *http.Request) error
}

type HandlerFunc func(Context, http.ResponseWriter, *http.Request) error

func (hf HandlerFunc) HandleHttp(c Context, w http.ResponseWriter, r *http.Request) error {
	return hf(c, w, r)
}

type matchExpr func(safestore.I, *http.Request) (bool, error)

type Route struct {
	// either the children or the handler is nil
	Name     string
	Parent   *Route
	Children []*Route
	Matchers []matchExpr
	Handler  Handler
	url      *url.URL // store info for reconstructing a url
}

// Any wrapping function can call Dispatch, and overwrite TopLevelHandler
func Dispatch(ctx Context, root *Route, w http.ResponseWriter, r *http.Request) error {
	rt := root.Match(ctx.Store(), r)
	logging.Trace(ctx, "rt: %v", rt.Name)
	return rt.Handler.HandleHttp(ctx, w, r)
}

// This is the method that the root handler runs by default. The
// root handler is called if nothing matches. It just returns a "404" error.
func NoMatchFoundHandler(c Context, w http.ResponseWriter, r *http.Request) error {
	// s := "No Match Found"
	// //w.Header().Set("Content-Type", "text/plain")
	// //w.Write([]byte(s))
	// return Error(s, errors.New(s), http.StatusNotFound)
	return PageNotFoundError("no match found")
}

func NewRoot(name string) (root *Route) {
	root = NewRouteFunc(nil, name, NoMatchFoundHandler)
	root.Matchers = append(root.Matchers, TrueExpr)
	return
}

//Creates a new Route and adds it to the route tree.
func NewRoute(parent *Route, name string, handler Handler) *Route {
	r := &Route{
		Name:     name,
		Children: make([]*Route, 0, 4),
		Matchers: make([]matchExpr, 0, 4),
		Handler:  handler,
		url:      new(url.URL),
	}
	if parent != nil {
		logging.Trace(nil, "Created Route: %v with parent: %v", name, parent.Name)
		r.Parent = parent
		parent.Children = append(parent.Children, r)
	} else {
		logging.Trace(nil, "Created Route: %v", name)
	}
	return r
}

//Creates a new Route off a route function and adds it to the route tree.
func NewRouteFunc(parent *Route, name string, handler HandlerFunc) *Route {
	return NewRoute(parent, name, handler)
}

//Find a Route under the tree at this root.
func (rt *Route) FindByName(name string) *Route {
	if name == rt.Name {
		return rt
	}
	for _, rt0 := range rt.Children {
		if rt1 := rt0.FindByName(name); rt1 != nil {
			return rt1
		}
	}
	return nil
}

// check if the route matches. If so, check it's children. If any doesn't
// match, then return the current one.
func (rt *Route) Match(store safestore.I, req *http.Request) *Route {
	//check if match first
	b := false
	for _, x := range rt.Matchers {
		if b1, err1 := x(store, req); err1 == nil {
			b = b1
			if !b {
				break
			}
		}
	}
	if !b {
		return nil
	}
	// if matched, then check children to see first one that matches recursively
	for _, rt1 := range rt.Children {
		if rt2 := rt1.Match(store, req); rt2 != nil {
			return rt2
		}
	}
	return rt
}

//Sister function to ToURL.
func (rt *Route) ToURLX(params map[string]interface{}) (u *url.URL, err error) {
	defer zerror.OnErrorf(1, &err, nil)
	//populate Scheme, Path, Host, RawUserinfo, RawQuery and call String() method
	u = new(url.URL)
	for rt2 := rt; rt2 != nil; rt2 = rt2.Parent {
		if rt2.url.Path != "" {
			u.Path = rt2.url.Path
			break
		}
	}
	for rt2 := rt; rt2 != nil; rt2 = rt2.Parent {
		if rt2.url.Scheme != "" {
			u.Scheme = rt2.url.Scheme
			break
		}
	}
	for rt2 := rt; rt2 != nil; rt2 = rt2.Parent {
		if rt2.url.Host != "" {
			u.Host = rt2.url.Host
			break
		}
	}
	for rt2 := rt; rt2 != nil; rt2 = rt2.Parent {
		if rt2.url.User != nil {
			u.User = rt2.url.User
			break
		}
	}
	for rt2 := rt; rt2 != nil; rt2 = rt2.Parent {
		if rt2.url.RawQuery != "" {
			u.RawQuery = rt2.url.RawQuery
			break
		}
	}
	//do substitution on Path and Host if necessary
	//for i := 0; i < len(params);  { // where params is a []string
	//	u.Path = strings.Replace(u.Path, util.InterpolatePrefix + params[i] + util.InterpolatePostfix, params[i+1], -1)
	//	u.Host = strings.Replace(u.Host, util.InterpolatePrefix + params[i] + util.InterpolatePostfix, params[i+1], -1)
	//	i += 2
	//}
	u.Path = util.Interpolate(u.Path, params)
	u.Host = util.Interpolate(u.Host, params)
	if u.RawQuery != "" {
		u.RawQuery = util.Interpolate(u.RawQuery, params)
	}
	return
}

func (rt *Route) ToURL(params ...string) (u *url.URL, err error) {
	defer zerror.OnErrorf(1, &err, nil)
	m := make(map[string]interface{})
	for i := 0; i < len(params); {
		m[params[i]] = params[i+1]
		i += 2
	}
	return rt.ToURLX(m)
}

// This adds a Host matchExpr to this router.
func (rt *Route) Host(hostRegexp string) *Route {
	logging.Trace(nil, "Adding Host Match: %v to Route: %v", hostRegexp, rt.Name)
	re, sclean, keys, _ := util.ParseRegexTemplate(hostRegexp)
	rt.url.Host = sclean
	x := func(store safestore.I, req *http.Request) (bool, error) {
		if res := re.FindStringSubmatch(req.URL.Host); res != nil {
			storeVars(store, keys, res)
			return true, nil
		}
		return false, nil
	}
	rt.Matchers = append(rt.Matchers, x)
	return rt
}

// This adds a Path matchExpr to this router. It supports exact matches,
// as well as matches of regexp.
func (rt *Route) Path(pathRegexp string) *Route {
	logging.Trace(nil, "Adding Path Match: %v to Route: %v", pathRegexp, rt.Name)
	re, sclean, keys, err := util.ParseRegexTemplate(pathRegexp)
	if err != nil {
		panic(err)
	}
	rt.url.Path = sclean
	x := func(store safestore.I, req *http.Request) (bool, error) {
		// logging.Trace(nil, "req.URL.Path: (against route: %v): %v", rt.Name, req.URL.Path)
		if len(keys) == 0 { // exact match
			if sclean == req.URL.Path {
				return true, nil
			}
		} else {
			if res := re.FindStringSubmatch(req.URL.Path); res != nil {
				storeVars(store, keys, res)
				return true, nil
			}
		}
		return false, nil
	}
	rt.Matchers = append(rt.Matchers, x)
	return rt
}

func (rt *Route) Param(param string) *Route {
	logging.Trace(nil, "Adding Param Match: %v to Route: %v", param, rt.Name)
	x := func(store safestore.I, req *http.Request) (bool, error) {
		_ = req.FormValue("")
		_, ok := req.Form[param]
		return ok, nil
	}
	rt.Matchers = append(rt.Matchers, x)
	return rt
}

//A Matcher which always returns true.
func TrueExpr(store safestore.I, req *http.Request) (bool, error) {
	return true, nil
}

// update the vars for this request.
func storeVars(sf safestore.I, keys []string, res []string) {
	vars := Vars(sf)
	slen := len(res) - 1 // first match is for full
	if slen > len(keys) {
		slen = len(keys)
	}
	for i := 0; i < slen; i++ {
		vars[keys[i]] = res[i+1]
	}
}

// Return the vars stored for this request. An application can request it
// and then update values in here.
func Vars(sf safestore.I) (vars map[string]string) {
	ivars := sf.Get(VarsKey)
	if ivars == nil {
		vars = make(map[string]string)
		sf.Put(VarsKey, vars, 0)
	} else {
		vars = ivars.(map[string]string)
	}
	return
}
