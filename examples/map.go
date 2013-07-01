package main

import (
	"fmt"
    "github.com/stevedonovan/luar"
)

const code = `
print(#M)
print(M.one)
for k,v in pairs(M) do
    print(k,v)
end
for i,v in ipairs(S) do
    print(i,v)
end
`

func main() {
    L := luar.Init()
    defer L.Close()
    
    M := luar.Map {
        "one":"ein",
        "two":"zwei",
        "three":"drei",        
    }
    
    S := []string {"alfred","alice","bob","frodo"}
    
    luar.Register(L,"",luar.Map {
        "M":M,
        "S":S,
    })
    
    err := L.DoString (code)
    if err != nil {
        fmt.Println("error",err.Error())
    }    
    
}

