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
            "MSG","hello",  // can also register constants
        })

        L.DoString(test)

    }

This example shows how Go slices and maps are marshalled to Lua tables and vice versa:

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

So an arbitrary Go function is callable from Lua, and list-like
tables become slices on the Go side.  The Go function returns a map,
which is wrapped as a proxy object. You can however then copy this to
a Lua table explicitly (there is also luar.slice2table)

You may pass a Lua table to
an imported Go function; if the table is 'array-like' then it can be
converted to a Go slice; if it is 'map-like' then it is converted to a
Go map.  Usually non-primitive Go values are passed to Lua as wrapped
userdata which can be naturally indexed if they represent slices,
maps or structs.  Methods defined on structs can be called, again
using reflection.

The consequence is that a person wishing to use Lua from Go does not
have to use the old-fashioned tedious method needed for C or C++, but
at some cost in speed and memory.

## luar for Configuration

Here is luar used for reading in configuration information in Lua format:

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

The examples folder covers most of luar's features.

## luar for Calling Lua Functions

Any Lua value can be wrapped inside a luar.LuaObject. These have Get and Set methods for
accessing table-like objects, and a Call method for calling functions.

Here is the very flexible Lua function `string.gsub` being called from Go (examples/luar3.go):

    gsub := luar.NewLuaObjectFromName(L,"string.gsub")
    res,err := gsub.Call("hello $NAME go $HOME","%$(%u+)",luar.Map {
        "NAME": "Dolly",
        "HOME": "where you belong",
    })
