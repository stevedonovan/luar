package main

import "fmt"
import "strconv"
import "os"
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
// an example of using Lua for configuration...
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
func main() {
    L := luar.Init()
    defer L.Close()
    
    // arbitrary Go functions can be registered 
    // to be callable from Lua
    luar.Register(L,"",luar.Map{
        "GoFun":GoFun,
    })
    
    res := L.DoString(code)
    if ! res {
        fmt.Println("Error:",L.ToString(-1))
        os.Exit(1)    
    }
    
    res = L.DoString(setup)
    if ! res {
        fmt.Println("Error:",L.ToString(-1))
        os.Exit(1)    
    } else {
        // there will be a table on the stack!
        fmt.Println("table?",L.IsTable(-1))
        v := luar.CopyTableToMap(L,nil,-1)   
        fmt.Println("returned map",v)
        m := v.(map[string]interface{})
        for k,v := range m {
            fmt.Println(k,v)
        }
    }
}
