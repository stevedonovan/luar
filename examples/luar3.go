package main

import "fmt"
import "github.com/stevedonovan/luar"

func main() {
    L := luar.Init()
    defer L.Close()
    
    L.GetGlobal("print")
    print := luar.NewLuaObject(L,-1)    
    print.Call("one two",12)
    
    L.GetGlobal("package")
    pack := luar.NewLuaObject(L,-1)    
    fmt.Println(pack.Get("path"))
    
    lcopy := luar.NewLuaObjectFromValue
    
    gsub := luar.NewLuaObjectFromName(L,"string.gsub")
    
    rmap := lcopy(L,luar.Map {
        "NAME": "Dolly",
        "HOME": "where you belong",
    })
    
    res,err := gsub.Call("hello $NAME go $HOME","%$(%u+)",rmap)
    if res == nil {
        fmt.Println("error",err)
    } else {
        fmt.Println("result",res)
    }   
    
    
}
