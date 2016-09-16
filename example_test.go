package luar_test

// TODO: Not all examples are interactive in godoc. Why?

// TODO: Lua's print() function will output to stdout instead of being compared
// to the desired result. Workaround: register one of Go's printing functions
// from the fmt library.

import (
	"fmt"
	"log"
	"reflect"
	"sort"
	"strconv"

	"github.com/aarzilli/golua/lua"
	"github.com/stevedonovan/luar"
)

type person struct {
	Name string
	Age  int
}

func Example() {
	const test = `
for i = 1, 3 do
		Print(msg, i)
end
Print(user)
Print(user.Name, user.Age)
`

	L := luar.Init()
	defer L.Close()

	user := &person{"Dolly", 46}

	luar.Register(L, "", luar.Map{
		// Go functions may be registered directly.
		"Print": fmt.Println,
		// Constants can be registered.
		"msg": "foo",
		// And other values as well.
		"user": user,
	})

	L.DoString(test)
	// Output:
	// foo 1
	// foo 2
	// foo 3
	// &{Dolly 46}
	// Dolly 46
}

type Ref struct {
	index  int
	number *int
	title  *string
}

func newRef() *Ref {
	n := new(int)
	*n = 10
	t := new(string)
	*t = "foo"
	return &Ref{index: 17, number: n, title: t}
}

func Example_pointers() {
	const test = `
-- Pointers to structs and structs within pointers are automatically dereferenced.
local t = newRef()
Print(t.index, t.number, t.title)
`

	L := luar.Init()
	defer L.Close()

	luar.Register(L, "", luar.Map{
		"Print":  fmt.Println,
		"newRef": newRef,
	})

	L.DoString(test)
	// Output:
	// 17 10 foo
}

// Slices must be looped with 'ipairs'.
func Example_slices() {
	const test = `
for i, v in ipairs(names) do
	 Print(i, v)
end
`

	L := luar.Init()
	defer L.Close()

	names := []string{"alfred", "alice", "bob", "frodo"}

	luar.Register(L, "", luar.Map{
		"Print": fmt.Println,
		"names": names,
	})

	L.DoString(test)
	// Output:
	// 1 alfred
	// 2 alice
	// 3 bob
	// 4 frodo
}

// Using Lua to parse configuration files.
const config = `return {
	baggins = true,
	age = 24,
	name = 'dumbo' ,
	marked = {1,2},
	options = {
		leave = true,
		cancel = 'always',
		tags = {strong=true, foolish=true},
	}
}`

// Read configuration in Lua format.
func ExampleCopyTableToMap() {
	L := luar.Init()
	defer L.Close()

	err := L.DoString(config)
	if err != nil {
		log.Fatal(err)
	}

	// There should be a table on the Lua stack.
	if !L.IsTable(-1) {
		log.Fatal("no table on stack")
	}

	v := luar.CopyTableToMap(L, nil, -1)
	// Extract table from the returned interface.
	m := v.(map[string]interface{})
	marked := m["marked"].([]interface{})
	options := m["options"].(map[string]interface{})

	fmt.Printf("%#v\n", m["baggins"])
	fmt.Printf("%#v\n", m["name"])
	fmt.Printf("%#v\n", len(marked))
	fmt.Printf("%.1f\n", marked[0])
	fmt.Printf("%.1f\n", marked[1])
	fmt.Printf("%#v\n", options["leave"])
	// Output:
	// true
	// "dumbo"
	// 2
	// 1.0
	// 2.0
	// true
}

func ExampleGoToLua() {
	// The luar's Init function is only required for proxy use.
	L := lua.NewState()
	defer L.Close()
	L.OpenLibs()

	input := "Hello world!"
	luar.GoToLua(L, nil, reflect.ValueOf(input), true)
	L.SetGlobal("input")

	L.PushGoFunction(luar.GoLuaFunc(L, fmt.Println))
	L.SetGlobal("Print")
	L.DoString("Print(input)")
	// Output:
	// Hello world!
}

func GoFun(args []int) (res map[string]int) {
	res = make(map[string]int)
	for i, val := range args {
		res[strconv.Itoa(i)] = val * val
	}
	return
}

// This example shows how Go slices and maps are marshalled to Lua tables and
// vice versa. This requires the Lua state to be initialized with `luar.Init()`.
//
// An arbitrary Go function is callable from Lua, and list-like tables become
// slices on the Go side. The Go function returns a map, which is wrapped as a
// proxy object. You can however then copy this to a Lua table explicitly. There
// is also `luar.slice2table` on the Lua side.
func ExampleInit() {
	const code = `
-- Lua tables auto-convert to slices.
local res = GoFun {10,20,30,40}

-- The result is a map-proxy.
print(res['1'], res['2'])

-- Which we may explicitly convert to a table.
res = luar.map2table(res)
for k,v in pairs(res) do
	print(k,v)
end
`

	L := luar.Init()
	defer L.Close()

	luar.Register(L, "", luar.Map{
		"GoFun": GoFun,
		"print": fmt.Println,
	})

	res := L.DoString(code)
	if res != nil {
		fmt.Println("Error:", res)
	}
	// Output:
	// 400 900
	// 1 400
	// 0 100
	// 3 1600
	// 2 900
}

func ExampleLuaObject_Callf() {
	L := luar.Init()
	defer L.Close()

	returns := luar.Types([]string{}) // []reflect.Type

	const code = `
function return_strings()
    return {'one', luar.null, 'three'}
end`

	err := L.DoString(code)
	if err != nil {
		log.Fatal(err)
	}

	fun := luar.NewLuaObjectFromName(L, "return_strings")
	// Using `Call` we would get a generic `[]interface{}`, which is awkward to
	// work with. But the return type can be specified:
	results, err := fun.Callf(returns)
	if err != nil {
		log.Fatal(err)
	}

	strs := results[0].([]string)

	fmt.Println(strs[0])
	// We get an empty string corresponding to a luar.null in a table,
	// since that's the empty 'zero' value for a string.
	fmt.Println(strs[1])
	fmt.Println(strs[2])
	// Output:
	// one
	//
	// three
}

func ExampleMap() {
	const code = `
print(#M)
print(M.one)
print(M.two)
print(M.three)
`

	L := luar.Init()
	defer L.Close()

	M := luar.Map{
		"one":   "ein",
		"two":   "zwei",
		"three": "drei",
	}

	luar.Register(L, "", luar.Map{
		"M":     M,
		"print": fmt.Println,
	})

	err := L.DoString(code)
	if err != nil {
		fmt.Println("error", err.Error())
	}
	// Output:
	// 3
	// ein
	// zwei
	// drei
}

// Another way to do parse configs: using LuaObject to manipulate the table.
func ExampleNewLuaObject() {
	L := luar.Init()
	defer L.Close()

	err := L.DoString(config)
	if err != nil {
		log.Fatal(err)
	}

	lo := luar.NewLuaObject(L, -1)
	// Can get the field itself as a Lua object, and so forth.
	opts := lo.GetObject("options")
	marked := lo.GetObject("marked")

	fmt.Printf("%#v\n", lo.Get("baggins"))
	fmt.Printf("%#v\n", lo.Get("name"))
	fmt.Printf("%#v\n", opts.Get("leave"))
	// Note that these Get methods understand nested fields.
	fmt.Printf("%#v\n", lo.Get("options.leave"))
	fmt.Printf("%#v\n", lo.Get("options.tags.strong"))
	// Non-existent nested fields don't crash but return nil.
	fmt.Printf("%#v\n", lo.Get("options.tags.extra.flakey"))
	fmt.Printf("%.1f\n", marked.Geti(1))

	iter := lo.Iter()
	keys := []string{}
	for iter.Next() {
		keys = append(keys, iter.Key.(string))
	}
	sort.Strings(keys)

	fmt.Println("Keys:")
	for _, v := range keys {
		fmt.Println(v)
	}
	// Output:
	// true
	// "dumbo"
	// true
	// true
	// true
	// <nil>
	// 1.0
	// Keys:
	// age
	// baggins
	// marked
	// name
	// options

}

func ExampleNewLuaObjectFromValue() {
	L := luar.Init()
	defer L.Close()

	gsub := luar.NewLuaObjectFromName(L, "string.gsub")

	// We do have to explicitly copy the map to a Lua table, because `gsub`
	// will not handle userdata types.
	gmap := luar.NewLuaObjectFromValue(L, luar.Map{
		"NAME": "Dolly",
		"HOME": "where you belong",
	})
	res, err := gsub.Call("hello $NAME go $HOME", "%$(%u+)", gmap)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(res)
	// Output:
	// hello Dolly go where you belong
}

func ExampleLuaTableIter_Next() {
	const code = `
return {
  foo = 17,
  bar = 18,
}
`

	L := luar.Init()
	defer L.Close()

	err := L.DoString(code)
	if err != nil {
		log.Fatal(err)
	}

	lo := luar.NewLuaObject(L, -1)

	iter := lo.Iter()
	keys := []string{}
	values := map[string]float64{}
	for iter.Next() {
		k := iter.Key.(string)
		keys = append(keys, k)
		values[k] = iter.Value.(float64)
	}
	sort.Strings(keys)

	for _, v := range keys {
		fmt.Println(v, values[v])
	}
	// Output:
	// bar 18
	// foo 17
}

func ExampleRegister_sandbox() {
	const code = `
    Print("foo")
    Print(io ~= nil)
    Print(os == nil)
`

	L := luar.Init()
	defer L.Close()

	res := L.LoadString(code)
	if res != 0 {
		msg := L.ToString(-1)
		fmt.Println("could not compile", msg)
	}

	// Create a empty sandbox.
	L.NewTable()
	// "*" means "use table on top of the stack."
	luar.Register(L, "*", luar.Map{
		"Print": fmt.Println,
	})
	env := luar.NewLuaObject(L, -1)
	G := luar.Global(L)

	// We can copy any Lua object from "G" to env with 'Set', e.g.:
	//   env.Set("print", G.Get("print"))
	// A more convenient and efficient way is to do a bulk copy with 'Setv':
	env.Setv(G, "print", "io")

	// Set up sandbox.
	L.SetfEnv(-2)

	// Run 'code' chunk.
	err := L.Call(0, 0)
	if err != nil {
		fmt.Println("could not run", err)
	}
	// Output:
	// foo
	// true
	// true
}
