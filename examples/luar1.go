package main

import "flag"
import "fmt"
import "os"
import "strings"
import "runtime"
import lua "github.com/stevedonovan/golua/lua51"
import "github.com/stevedonovan/luar"

func error(L *lua.State) {
    fmt.Println("Error:",L.ToString(-1))
    os.Exit(1)    
}

var expr = flag.String("e","","expression to be evaluated")
var libs = flag.String("l","","library to be loaded")

// test passing basic types
func test(x float64, i int, s string, b bool) {
    fmt.Println("test got",x,i,s,b)
}

func tostring() string {
    return "tostring"
}

func slice() []int {
    return []int {1,2,3}
}

func mapr() map[string]string {
    return map[string]string {
        "one":"ein",
        "two":"zwei",
        "three":"drei",
    }
}


func gotslice(si []int) int {
    return len(si)
}

func any(a interface{}) {
    fmt.Println("we got",a)
    switch v := a.(type) {
    case string:
        fmt.Println("string",v)
    case float64:
        fmt.Println("number",v)
    default:
        fmt.Println("unknown",v)
    }
}

func mapit (m map[string]int, a map[int]string) {
    for k,v := range m {
        fmt.Println("string",k,v)
    }
    for k,v := range a {
        fmt.Println("int",k,v)
    }    
}

// dispatching methods on a struct

type Test struct {
    Name string
    Age int
}

func (self *Test) Method(s string) int {
    return len(s + self.Name)
}

func (self *Test) GetName() string {
    return self.Name
}

func structz () *Test {
    return &Test{"Hello",25}
}

func makebslice(sz int) []byte {
    return make([]byte,sz)
}

func bslice2string (slice []byte) string {
    return string(slice)
}

func main() {
    L := luar.Init()
    flag.Parse()
    
    luar.Register(L,"",luar.Map{
        "test": test,
        "tos": tostring,
        "slice":slice,
        "gotslice":gotslice,
        "any":any,
        "mapit":mapit,
        "mapr":mapr,
        "structz":structz,
        "Println":fmt.Println,
        "Printf":fmt.Printf,
        "Fprintf":fmt.Fprintf,
        "Open":os.Open,
        "Create":os.Create,
        "Fields":strings.Fields,
        "Title":strings.Title,
        "Join":strings.Join,
        "NewReader":strings.NewReader,
        "makebslice":makebslice,
        "bslice2string":bslice2string,
        "makechan":luar.MakeChannel,
        "go":luar.GoLua,
        "gosched":runtime.Gosched,
        "const":42,
    })    
    
    
    if len(*libs) > 0 {
        L.PushString("require")
        L.PushString(*libs)
        if L.PCall(1,0,0) != 0 {
            error(L)
        }
    }

    if len(*expr) > 0 {
        res := L.DoString(*expr)
        if ! res {
            error(L)
        }
    }  
    
    args := flag.Args()
    if len(args) == 0 {
        if len(*expr) == 0 {
            fmt.Println("please supply a Lua script name")
        }
        return
    }
    
    script := args[0]    
    res := L.DoFile(script)
    if ! res {
        error(L)
    }


	L.Close();
}
