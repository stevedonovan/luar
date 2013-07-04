/* luar provides a more convenient way to access Lua from Go, using
 Alessandro Arzilli's  golua (https://github.com/aarzilli/golua).
 Plain Go functions can be registered with luar and they will be called by reflection;
 methods on Go structs likewise.

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

    Go types like slices, maps and structs are passed over as Lua proxy objects,
    or alternatively copied as tables.
*/
package luar
