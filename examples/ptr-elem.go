// may access struct values from Lua which are pointers
// to values.  E.g. in goprotobuf non-repeated fields are pointers.
package main

import "fmt"
import "github.com/stevedonovan/luar"

type T struct {
    A *int32
    B int64
    C *string
}

func newT() *T {
    pi := new(int32)
    *pi = 10
    ps := new(string)
    *ps = "hello"
    return &T{A:pi, B:20,C:ps}
}

const setup = `
    local t = newT()
    print(t.A, t.B, t.C)
`

func main() {
	L := luar.Init()
	defer L.Close()

	luar.Register(L, "", luar.Map{
		"newT": newT,
	})

	res := L.DoString(setup)
	if res != nil {
		fmt.Println("Error:", res)
	}

}
