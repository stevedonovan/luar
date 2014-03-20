package main

import "fmt"
import "github.com/stevedonovan/luar"

type Config struct {
	Baggins bool
	Age     int
	Name    string
	Ponies  []string
	Father  *Config
}

func config(cfg *Config) {
	fmt.Println("config", cfg)
	fmt.Println("father", cfg.Father)
}

// an example of using Lua for configuration...
// Note that Lua names will be automatically
// title-cased!
const setup = `
config {
  baggins = true,
  age = 24,
  name = 'dumbo' ,
  ponies = {'whisper','fartarse'},
  father = {
      baggins = false,
      age = 77,
      name = 'Wombo',
  }
}
`

func main() {
	L := luar.Init()
	defer L.Close()

	// arbitrary Go functions can be registered
	// to be callable from Lua
	luar.Register(L, "", luar.Map{
		"config": config,
	})

	res := L.DoString(setup)
	if res != nil {
		fmt.Println("Error:", res)
	}

}
