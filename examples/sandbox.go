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
    
    gettop := func() {
        fmt.Println("top",L.GetTop())
    }
    gettop()   
    
    // compile chunk
    res := L.LoadString(test)
    if res != 0 {
        msg := L.ToString(-1)
        fmt.Println("could not compile",msg)
    }
    
    // create environment for chunk
    L.NewTable();
    // "*" means use table on stack....
	luar.Register(L, "*", luar.Map{
		"Print":   fmt.Println,
	})
    env := luar.NewLuaObject(L,-1)
    G := luar.Global(L)
    //~ env.Set("print",G.Get("print"))
    //~ env.Set("io",G.Get("io"))
    // more convenient/efficient way to do a bulk copy    
    env.Setv(G,"print","io")
    L.SetfEnv(-2)
    
    // run chunk
    err := L.Call(0,0)
    if err != nil {
        fmt.Println("could not run",err)
    }
    
}
