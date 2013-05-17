 package main

import "fmt"
import "github.com/stevedonovan/luar"

const test = `
for i = 1,10 do
	Print(MSG,i)
end
`

func main() {
	L := luar.Init()
	defer L.Close()

	luar.Register(L,"",luar.Map{
		"Print":fmt.Println,
		"MSG":"hello",  // can also register constants
	})

	L.DoString(test)

}
