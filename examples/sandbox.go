package main

import "fmt"
import "github.com/stevedonovan/luar"

const test = `
    Print(print,io)
    print("hello!")
`

func main() {
	L := luar.Init()
	defer L.Close()
    
    G := luar.NewLuaObjectFromName(L,"_G")
    pr := G.Get("print")
    
    // compile chunk
    L.LoadString(test)
    
    // create environment for chunk
    L.NewTable();
	luar.Register(L, "*", luar.Map{
		"Print":   fmt.Println,
	})
    env := luar.NewLuaObject(L,-1)
    env.Set("print",pr)
    L.SetfEnv(-2)
    
    // run chunk
    L.Call(0,0)
}
