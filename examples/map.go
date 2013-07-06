package main

import (
	"fmt"
	"github.com/stevedonovan/luar"
)

type MyStruct struct {
	Name string
	Age  int
}

const code = `
print(#M)
print(M.one)
print 'pairs over Go maps'
for k,v in pairs(M) do
    print(k,v)
end
print 'ipairs over Go slices'
for i,v in ipairs(S) do
    print(i,v)
end
`

func main() {
	L := luar.Init()
	defer L.Close()

	M := luar.Map{
		"one":   "ein",
		"two":   "zwei",
		"three": "drei",
	}

	S := []string{"alfred", "alice", "bob", "frodo"}

	ST := &MyStruct{"Dolly", 46}

	luar.Register(L, "", luar.Map{
		"M":  M,
		"S":  S,
		"ST": ST,
	})

	err := L.DoString(code)
	if err != nil {
		fmt.Println("error", err.Error())
	}

}
