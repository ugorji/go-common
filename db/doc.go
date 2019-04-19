/*
This package manages db interactions.

DATASTORE
 
It supports the following use-cases:
 - Transparent Caching (check/use cache on load and save)
 - Hooks (PostLoadHook, PreSaveHook, PostSaveHook)
 - Embedded Types and Map Fields
 - Should we store/index fields in the datastore?
 - Ancestor entities 

Each entity may define zero or more of the following methods:
 - PostLoadHook(), PreSaveHook(), PostSaveHook(), FromDatastoreKey(*Key), DatastoreKey() *Key

Within a transaction, the semantics are different as described below.

This package exposes the following methods:
 - PostLoad(*SafeStore, ctx, tx, []*Key, []interface{}): 
   update id field (or call FromDatastoreKey)
   Calls PostLoadHook
   If tx = false, Puts into caches.
 - PreSave (*SafeStore, ctx, req, tx, []*Key, []interface{}): 
   Calls PreSaveHook
 - PostSave(*SafeStore, ctx, req, tx, []*Key, []interface{}): 
   update id field (or call FromDatastoreKey)
   Calls PostSaveHook
   If tx = true,  Remove from caches where successful.
   If tx = false, Put into caches where successful.

 - Get(*SafeStore, ctx, tx, []*Key, []*interface{}): 
   Checks caches first if tx=false. Find delta, and check datastore for those. 
   Calls PostLoad for returned set from datastore.
 - Put(*SafeStore, ctx, tx, []*Key, []*interface{}): 
   Call PreSave. Put into datastore. Call PostSave. 
 - Delete(*SafeStore, ctx, tx, []*Key)
   Delete from datastore and caches. 

Application code will call these methods to get things out, mainly:
 - Get(...), Put(...), Delete(...)

ANCESTOR

There is built in support for one level of ancestors. This is what is ever needed 
(there is no need to do more than one level ancestors since only one root is required
for entity grouping for transactions, co-location, etc).

An application can use DatastoreKeyAware to do more exotic Ancestor systems (with more 
than one level of depth). However, with the simple builtin support, there's just one 
level of depth (if your struct defines pkind, pkey and optionally pshape).

This ancestor support may be critical to support:
 - fast 1-PC single-entity-group transaction support
 - strongly consistent queries (with the new HRD)

DatastoreKeyAware
 
This allows an entity to define how it wants to convert to/fro a datastore.Key instance. 
With this, we have a solution for ancestor entities, etc. A type with support for 
ancestors can implement DatastoreKeyAware. If it is not implemented, we use the simple
model of looking for the Kind and KeyField, looking at it's field type (int64 or string),
and setting fields from the Key or creating a Key appropriately.

SYNTHETIC FIELDS
 

The Hook methods can also be used to support complex embedded types. To do this:
 - define some exported Synthetic field which are stored in the data
 - on PostLoadHook: use those synthetic field values to populate your real fields
                    zero out those synthetic fields
 - on PreSaveHook:  use your real field values to populate your synthetic fields
 - on PostSaveHook: zero out those synthetic fields

For synthetic fields, by convention, call them: 
 - If Marshal: X__<FieldName>
 - If Structured Map: X__<FieldName>_Key and  X__<FieldName>_Value
 - If structered struct: X__<FieldName>_<Embedded_field_name>

AUTO HOOKS
 
The package *can* do a lot of this "hooks" automatically. To support this, the struct is 
configured for auto-hooks. With auto-hooks, many structs will not need to define their 
own hooks. Auto-hooks supports the following:
 - support embedded fields (only one-level deep)

Assuming primitives are the following:
 - number
 - string
 - bool

Auto Hooks support the following fields
 - struct (with only primitive fields) ie MyStruct
 - pointer to such struct              ie *MyStruct
 - slice of such structs               ie []MyStruct
 - slice of pointers of such struct    ie []*MyStruct
 - map of primitive to primitive       e.g. map[string]int

With auto hook support, the user only has to define the corresponding synthetic fields 
according to our convention.

Some concerns:
 - Reflection is about 20X slower than direct calls
 - User needs to create synthetic fields

Benefits
 - Writing the code to support embedded fields is boilerplate, and prone to error. 
   For even just one field, it could be up about 20 lines of code spread across 
   PostLoadHook, PreSaveHook and PostSaveHook


CONFIGURATION
 
To configure struct-level things,
 - create a struct field called _struct bool
 - set values there for db, for example:
   _struct bool `db:"keyf=Id,kind=U,auto"`
   This means: perform autohooks, and this struct uses key Id, and has kind U.

In general, db struct tag value on field _struct is parsed by:
 - tokenizing on comma
 - each token is a key[=value], and value is a singular[|singular]*
 
The following keys in the struct tag "db" value on field _struct have meaning:
 - autohook: "auto"[=<value>] where value is true or false (default: false)
 - request cache: "rc"[=<value>] where value is true or false (default: false)
 - process cache: "pc"[=<value>] where value is true or false (default: false)
 - memcache: "mc"[=<value>] where value is true or false (default: false)
 - datastore: "ds"[=<value>] where value is true or false (default: false)
 - process timeout: "pcto"=<value> where value is \d+(s|m|h|d). If 0, no timeout.
 - memcache timeout: "mcto"=<value> where value is \d+(s|m|h|d). If 0, no timeout.
 - key field: "keyf"=<value> (value is FieldName. That field must be a int64 or string)
 - shape field: "shapef"=<value> (Optional: value is FieldName. That field must be a string)
 - kind: "kind"=<value> The kind of the datastore
 - shape: "shape"=<value> The (optional) shape of the type in the datastore
 - parent key field: "pkeyf"=<value> (same as key but for optional ancestor)
 - parent shape field: "pshapef"=<value> (Optional: value is FieldName. That field must be a string)
 - parent kind: "pkind"=<value> (same as kind but for optional ancestor)
 - parent shape: "pshape"=<value> (same as shape but for optional ancestor)


Note that only fields with a "db" tag are store'able/index'able. To use the field name
as default datastore column name, just make a non-blank db e.g. `db:"-"`

If autoHook is supported, the following keys in the struct tag "db" value on other 
fields have meaning:
 - ftype=struc: meaning this is a structured field
 - ftype=marshal: means store as a marshal
 - ftype=expando: means expand this (each mapping is a property on the entity)
 - dbname=fieldname
 - store=y,!y,z,!z (for always, not always, empty value, not empty value)
 - index=y,!y,z,!z
.
   Example: Atn map[string]string    `db:"dbname=atn,ftype=expando"

We maintain a cache of info about each struct in the package (so we don't have to 
continously reflect and parse the same strings). This contains 
 - information from _struct field
 - all structured and marshal field names, and corresponding synthetic field names 
   (if auto hooks are supported)

By default, we store and index fields whose value is not equal to their "zero" value
ie if not defined, we act like this is set: store=!z,index=!z



QUERIES
 
For Queries, the app has to pass in the actual datastore columns (not the field names). 
Since the app knows how to coerce itself into a datastore entity, and defines its own
datastore column names, it's ok for it to also directly pass them in the query. 

This way, there's no need to create a wrapper over the query infrastructure provided OOTB.

If you want your queries to integrate with the cache, then your application code may
call PostLoad(...) right after each call to Next(...). However, we
prefer that Queries are always KeysOnly, and a subsequent request does a GET which 
leverages the caches, for the following reasons:
 - Datastore costs are reduced (due to potential cache hits)
 - Queries are evantually consistent, so query results may be stale. Better to use do a 
   keys only query, followed by Batch GET.

If Not a KeysOnly query, and you don't want your query to integrate with the cache 
(as is recommended), then call PostLoadNoCaching(...) after each iteration.

POLYMORPHIC
 
 
Polymorphic support is very interesting.
 - It allows different "shapes" of a given "kind" 
       to be represented by different "types"
       but all stay in the same "table" (same entity)
 - It allows us run query for different "types" (all the same "kind")

To do this:
 - A type is uniquely identified by a kind, and an optional shape
 - The shape is reflected in the datastore in 2 places (if present): 
   - Mandatory: part of the key e.g. TP/:sh:
   - Optional:  Define a ShapeField for the type
     The ShapeField must be a string field
     During PostLoad and PreSave and InQuery, we will ensure the ShapeField is 
       set to/fro the datastore.Key
 - The shape is easily identified
   - If the Key uses a string name (not long id), and the first 4 bytes are 
     colon(:), [0-9a-zA-Z], [0-9a-ZA-Z], colon(:)
     The shape is the 2-character, 2-byte slice in between the colons.
 - If you want to query on the ShapeField
   (e.g. to restrict queries to only return types with a given shape),
   then make the ShapeField stored as a db column, and index it.
 - A query from now on takes a Kind and a shape (not a template interface{})
   Already, templates have to know the datastore intimately.
 - The KeyField/ParentKeyField value will not include the Shape information.
 - During a Query, you can limit results to entities with a given shape.

Note the following:
 - You will have to register all your data types at startup 
   (especially those with polymorphic support or for which you have to query)
 - For those with polymorphic support, register a base type with shape="". 
   This will allow us support true polymorphic queries e.g. when no shape is given in the query
   For example: 
   - You have types: A, B{A}, C{A}, where A is the base type, and B,C are polymorphic
   - Register A, B, C to all have same kind="A"
   - Register A with shape="", B with shape="B", C with shape="C"
 - For better query support, define the ShapeField and ensure it's 
   stored and indexed, and pass it in your queries.
   (This is however completely optional)
 - If your polymorphic entity uses ancestors, set the pshape=""
 - If you want to query on your polymorphic entity that has ancestors, 
   Define shapef, and pshapef=

CACHING
 

There are two caches: 
 - L1 (in-memory, in-process, short-lived, request-scoped) 
 - L2 (in-memory, in-process, process-wide) 
 - L3 (out-of-process, long-lived, non-deterministic, shared, memcache)

Both caches will use the string hash (web-safe) representation of a *Key as the cache key. 
The value will be:
 - For L1: request-scoped, an interface{} (the actual entity)
 - For L2: process-wide, an interface{} (the actual entity)
 - For L3: memcache, an uncompressed marshal. 

Cache must support storing cache misses. This way, we do not keep requesting a 
value from the datastore when we know it's not there. We do this by putting:
 - L1: Use false
 - L2: Use false.
 - L3: Use false.

During a Get, any Key misses are also stored in the Cache as negatives. This way,
we don't continually look in the datastore for information that isn't there.

At the struct level, the following can be configured:
 - should this entity be stored in the request cache
 - should this entity be stored in the process cache
 - should this entity be stored in memcache
 - should this entity be stored in datastore
 - what is the memcache timeout for this entity

Note on Process Wide Cache:
 - A Process-wide cache shared across requests is dangerous because it cannot be cleared
 - It is however good for things which change very infrequently (like configuration)
 - Keeping those long-lived in memcache (4ms response) may be a fair tradeoff

ADDENDUM
 

We include support for process-wide caching. This must be used carefully, and only for those
entities which fit this profile:
   - Finite, contained number of entities i.e. not growing outside of developer/admin's control
   - Entities do not change much
   - OK if stale for a little bit
   - Typically used in a read-only fashion

In general, follow these guidelines also:
   - If you put in process cache, then turn off memcache. 
   - You either want to cache things in memcache, or your things are contained enough 
     that you can pre-load them for each process and cache them in-process

The InstanceCache will also support a Query Cache. In this mode:
   - Queries are cached using their canonical (GQL) form as a map/cache key (with prefix)
   - Note that

GUIDELINES
 

Some guidelines are below:
   - For easy debugging, log a FINE message when an entity is found, so we know where it was
     found i.e. request cache, process cache, memcache or datastore
   - Within a Get (DatastoreGet, CacheGet), the retrieved values are stored in the slice passed
     to the function. You will need to get them back from the slice. For example, if you do:
        topics := []{...}
        dst = []{a, b} //len 2
        dst = append(dst, topics...)
        db.CacheGet(..., dst)
        topics = dst[2:] //else dst has retrieved values, but topics might have old stale ones
   - There is support for ensuring an entity has a valid Id before sending to datastore.

*/
package db

// BUG(ugorji): Shape is not fully thought out. Consider removing it??? 

/*
TODO:
    
  Clean up the API:
  Only keep those listed below exported (as they are being used):
    Save
    Load
    LoadOne
    HackRegisterSliceType
        # Called by app startup, so we can register slice types for storage.
    GetStructMeta
    CacheEntryDecode
        # Called By gaeapp so it can decode what was stored in Memcache.
    
    Property
    PropertyList
    OrmToIntf
        # Called by Gae Driver, as it gets user information as db.PropertyList
        # and can convert it into the interface.
    GetLoadedStructMetaFromKind
    QueryAsString
    QuerySupport
    PreSaveHooker
    DatastoreKey
    EntitiesForKeys
    StructMetas
    GetStructMeta
    TypeMeta
    EntityForKey
    QueryIterFunc
  
*/
