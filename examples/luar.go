package main

import (
	"fmt"
    "regexp"
	"github.com/GeertJohan/go.linenoise"
    "github.com/stevedonovan/luar"
)

func main() {
    L := luar.Init()
    defer L.Close()
    
    luar.Register(L,"",luar.Map {
        "regexp":regexp.Compile,
    })
    
    fmt.Println("luar prompt")
	fmt.Println("Lua 5.1.4  Copyright (C) 1994-2008 Lua.org, PUC-Rio")
    fmt.Println("type exit to leave...")
	for {
		str := linenoise.Line("> ")
        if len(str) > 0 {
            if str == "exit" {
                return
            }
            linenoise.AddHistory(str)
            if str[0] == '=' {
                str = "print(" + str[1:] + ")"
            }
            err := L.DoString(str)
            if err != nil {
                fmt.Println(err)
            }
        } else {
            fmt.Println("ding!")
        }
	}
}

