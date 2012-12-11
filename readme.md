luar is designed to make using Lua from Go more convenient.  It depends on the luago
bindings, originally (here) but uses the fork available from this account, which 
contains some small improvements and is immediately go-buildable.

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

This is done using Go reflection to marshall values between Lua and Go and back
again. (It's still possible to use C-style functions, but for that use `RawRegister`.)

Some special conversions apply when Lua passes tables to Go functions. If they're 
'array-like' they are passed as slices, and if they're 'table-like' they are passed
as maps. 

Lua will receive Go structs, slices and maps as wrapped userdata. Again reflection 
is used to look up keys in maps, index slices and fields in structs. Methods are 
resolved using reflection as well.


