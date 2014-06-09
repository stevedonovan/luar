package main

import "fmt"
import "strconv"
import "github.com/stevedonovan/luar"

func GoFun(args []int) (res map[string]int) {
    res = make(map[string]int)
    for i, val := range args {
        res[strconv.Itoa(i)] = val * val
    }
    return
}

const code = `
print 'here we go'
-- Lua tables auto-convert to slices
local res = GoFun {10,20,30,40}
-- the result is a map-proxy
print(res)
print(res['1'],res['2'])
-- which we may explicitly convert to a table
res = luar.map2table(res)
for k,v in pairs(res) do
      print(k,v)
end
`

func main() {
	L := luar.Init()
	defer L.Close()

	// arbitrary Go functions can be registered
	// to be callable from Lua
	luar.Register(L, "", luar.Map{
		"GoFun": GoFun,
	})

	res := L.DoString(code)
	if res != nil {
		fmt.Println("Error:", res)
	}

}
