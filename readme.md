## luar Lua Reflection Bindings for Go

luar is designed to make using Lua from Go more convenient.

Direct bindings to Lua already exist - luago has about 16
forks, and I'm using Alessandro Arzilli's [up-to-date fork](https://github.com/aarzilli/golua) which
does not use makefiles and simply requires that pkg-config exists
and there is a lua5.1 package.  So on Debian/Ubuntu it is
directly go-gettable if the Lua package is installed.

As a consequence, luar is also go-gettable.

    go get github.com/stevedonovan/luar

However, pkg-config is not universal, and Lua 5.1 may not
be available as a lua5.1 package. However, cgo does not make any
great demands on pkg-config.  For instance, it works on Windows if
you copy this nonsense to a file pkg-config.bat on your %PATH%:

    @echo off
    set LUAPATH=/Users/steve/lua/lua-5.1.4/src
    if "%1"=="--cflags" echo -I%LUAPATH%
    if "%1"=="--libs"  echo %LUAPATH%/lua51.dll

We link against the DLL, as is recommended for Mingw, and then everything
works if a copy of lua51.dll is on your DLL path.

More sophisticated operating system users should have no difficulty
in emulating this trick!

## Binding to Go functions by Reflection

luago is pretty much a plain bridge to the C API and manages some of
the GC issues and so forth.  luar attempts to go further. Any Go
function can be made available to Lua scripts, without having to write
C-style wrappers.  This can be done because Go has a powerful type
reflection system:

The first convenience is that ordinary Go functions may be registered directly:

```go
package main

import "fmt"
import "github.com/stevedonovan/luar"

const test = `
for i = 1,10 do
    Print(MSG,i)
end
`

func main() {
    L := luar.Init()
    defer L.Close()

    luar.Register(L,"",luar.Map{
        "Print":fmt.Println,
        "MSG":"hello",  // can also register constants
    })

    L.DoString(test)

}
```

This example shows how Go slices and maps are marshalled to Lua tables and vice versa:

```go
package main

import "fmt"
import "strconv"
import "github.com/stevedonovan/luar"

func GoFun (args []int) (res map[string]int) {
    res = make(map[string]int)
    for i,val := range args {
        res[strconv.Itoa(i)] = val*val
    }
    return
}

const code = `
print 'here we go'
--// Lua tables auto-convert to slices
local res = GoFun {10,20,30,40}
--// the result is a map-proxy
print(res['1'],res['2'])
--// which we may explicitly convert to a table
res = luar.map2table(res)
for k,v in pairs(res) do
      print(k,v)
end
`
func main() {
    L := luar.Init()
    defer L.Close()

    // arbitrary Go functions can be registered
    // to be callable from Lua
    luar.Register(L,"",luar.Map{
        "GoFun":GoFun,
    })

    res := L.DoString(code)
    if res != nil {
        fmt.Println("Error:",res)
    }
}
```

So an arbitrary Go function is callable from Lua, and list-like
tables become slices on the Go side.  The Go function returns a map,
which is wrapped as a proxy object. You can however then copy this to
a Lua table explicitly (there is also `luar.slice2table`)

You may pass a Lua table to
an imported Go function; if the table is 'array-like' then it can be
converted to a Go slice; if it is 'map-like' then it is converted to a
Go map.  Usually non-primitive Go values are passed to Lua as wrapped
userdata which can be naturally indexed if they represent slices,
maps or structs.  Methods defined on structs can be called, again
using reflection. Do note that these methods will be callable using
_dot-notation_ rather than colon notation!

The consequence is that a person wishing to use Lua from Go does not
have to use the old-fashioned tedious method needed for C or C++, but
at some cost in speed and memory.

## luar for Configuration

Here is luar used for reading in configuration information in Lua format:

```go
const setup = `
return {
    baggins = true,
   age = 24,
   name = 'dumbo' ,
   marked = {1,2},
   options = {
       leave = true,
       cancel = 'always'
    }
}
`

....
 res = L.DoString(setup)
 // there will be a table on the stack!
 v := luar.CopyTableToMap(L,nil,-1)
 fmt.Println("returned map",v)
 m := v.(map[string]interface{})
 for k,v := range m {
       fmt.Println(k,v)
 }
 ```

The examples directory covers most of luar's features.

## luar for Calling Lua Functions

Any Lua value can be wrapped inside a luar.LuaObject. These have Get and Set methods for
accessing table-like objects, and a Call method for calling functions.

Here is the very flexible Lua function `string.gsub` being called from Go (examples/luar3.go):

```go
    gsub := luar.NewLuaObjectFromName(L,"string.gsub")
    gmap := luar.NewLuaObjectFromvalue(luar.Map {
        "NAME": "Dolly",
        "HOME": "where you belong",
    })    
    res,err := gsub.Call("hello $NAME go $HOME","%$(%u+)",gmap)
    --> res is now "hello Dolly go where you belong"
```
    
Here we do have to explicitly copy the map to a Lua table, because `gsub`
will not handle userdata types.  These functions are rather verbose, but it's
easy to create aliases:

```go
    var lookup = luar.NewLuaObjectFromName
    var lcopy = luar.NewLuaObjectFromValue
    ....
```

`luar.Callf` is used whenever:
   * the Lua function has multiple return values
   * and/or you have exact types for these values
   
For instance, in the tests the following Lua function is defined:

```lua
function Libs.return_strings()
    return {'one','two','three'}
end
```

Using `Call` we would get a generic `[]interface{}`, which is awkward to work 
with.  But the return type can be specified:

```go
    fun := luar.NewLuaObjectFromName(L,"Libs.return_strings")
    returns := luar.Types([]string{})  // --> []reflect.Type
    results,err := fun.Callf(returns)    // -> []interface{}
    // first returned result is a slice of strings
    strs := results[0].([]string)
```

The first argument may be `nil` and can be used to access multiple return
values without caring about the exact conversion.

## An interactive REPL for Golua

`luar.go` in the examples directory provides a useful Lua REPL for exploring
Go in Lua. 
You will need to do `go get github.com/GeertJohan/go.linenoise`
to get line history and tab completion.  This is an extended REPL and comes
with pretty-printing:

```lua
$ ./luar
luar prompt
Lua 5.1.4  Copyright (C) 1994-2008 Lua.org, PUC-Rio
> = 10,'10',{10}
10	"10"	{10}
```

One use for the `luar` REPL is to explore Go libraries. `regexp.Compile` is 
exported as `regexp`, so we can do this. note that the endlessly useful `fmt.Println` 
is available as `println` from Lua. Starting a line with a period ('dot') wraps
that line in `println`; starting a line with '=' wraps it with `print` (as is usual
with the standard Lua prompt.)

```lua
> p = regexp '[a-z]+\\S*'
> ms =  p.FindAllString('boo woo koo',99)
> = #ms
3
> println(ms)
[boo woo koo]
> . ms
[boo woo koo]
```

The next session explores the `luar` function `slice`, which generates a Go
slice. This is automatically wrapped as a proxy object. Note that the indexing
is one-based, and that Go slices have a fixed size!  The metatable for slice proxies
has an `__ipairs` metamethod. Although luar is (currently) based on Lua 5.1,
it loads code to provide a 5.2-compatible `pairs` and `ipairs`.

The inverse of `slice` is `slice2table`. 

```lua
> s = luar.slice(2) // create a Go slice
> = #s
2
> = s[1]
nil
> = s[2]
nil
> = s[3] // has exactly two elements!
[string "print( s[3])"]:1:  slice get: index out of range
> = s
[]interface {}
> for i,v in ipairs(s) do print (i,v) end
1	10
2	20
> = luar.slice2table(s)
{10,20}
> println(s)
[10 20]
> . s
[10 20]
```
A similar operation is `luar.map` (with corresponding `luar.map2table`).
Using `luar.type` we can find the Go type of a proxy (it returns `nil` if this isn't
a Go type). By getting the type of a value we can then do _reflection_ and 
find out what methods a type has, etc.

```lua
> m = luar.map()
> m.one = 1
> m.two = 2
> m.three = 3
> println(m)
map[one:1 two:2 three:3]
> for k,v in pairs(m) do print(k,v) end
three	3
one	1
two	2
> mt = luar.type(m)
> = mt.String()
"map[string]interface {}"
> = mt.Key().String()
"string"
> mtt = luar.type(mt)
> = mtt.String()
"*reflect.rtype"
> = mtt.NumMethod()
31

```
tab-completion is implemented in such Lua code:  the Lua completion code
merely requires that a type implement `__pairs`. This allows tab to
expand `mtt.S` to `mtt.String` in the last example.


```lua
local function sdump(st)
    local t = luar.type(st)
    local val = luar.value(st)
    local nm = t.NumMethod()
    local mt = t --// type to find methods on ptr receiver
    if t.Kind() == 22 then --// pointer!
        t = t.Elem()
        val = val.Elem()
    end
    local n = t.NumField()
    local cc = {}
    for i = 1,n do
        local f,v = t.Field(i-1)
        if f.PkgPath == "" then --// only public fields!
            v = val.Field(i-1)    
            cc[f.Name] = v.Interface()
        end
    end
    --// then public methods...
    for i = 1,nm do
        local m = mt.Method(i-1)
        if m.PkgPath == "" then --// again, only public
            cc[m.Name] = true
        end
    end
    return cc
end
        
mt = getmetatable(__DUMMY__)
mt.__pairs = function(st)
    local cc = sdump(st)
    return pairs(cc)
end
```

`sdump` is pretty much the way this would be encoded in Go itself; again, the
eccentric dot-notation makes it more familiar.  This `luar` interpreter is mostly
Lua embedded in Go source!


