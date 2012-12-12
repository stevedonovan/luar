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
    
/*
    L.GetGlobal("string")
    strtab := luar.NewLuaObject(L,-1)    
    iter := strtab.Iter()
    for iter.Next() {
        fmt.Println(iter.Key,iter.Value)
    }
*/
    
    gsub := luar.NewLuaObjectFromName(L,"string.gsub")
    res,err := gsub.Call("hello $NAME go $HOME","%$(%u+)",luar.Map {
        "NAME": "Dolly",
        "HOME": "where you belong",
    })
    if res == nil {
        fmt.Println("error",err)
    } else {
        fmt.Println("result",res)
    }   
    
    
}
