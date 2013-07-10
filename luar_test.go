package luar

import "testing"
import "strconv"
import "os"
import "reflect"

// I _still_ like asserts ;)
func assertEq(t *testing.T, msg string, v1, v2 interface{}) {
	if v1 != v2 {
		t.Error("were not equal: " + msg)
	}
}

func fun2(x float32, a string) (float32, string) {
	return x, a
}

func sum(args []float64) float64 {
	res := 0.0
	for _, val := range args {
		res += val
	}
	return res
}

func sumv(args ...float64) float64 {
	return sum(args)
}

// [10,20] -> {'0':10,'1':20}
func squares(args []int) (res map[string]int) {
	res = make(map[string]int)
	for i, val := range args {
		res[strconv.Itoa(i)] = val * val
	}
	return
}

func keys(m map[string]interface{}) (res []string) {
	res = make([]string, 0)
	for k := range m {
		res = append(res, k)
	}
	return
}

func values(m map[string]interface{}) (res []interface{}) {
	res = make([]interface{}, 0)
	for _, v := range m {
		res = append(res, v)
	}
	return
}

func Nil(v interface{}) string {
	if v == nil {
		return "bad"
	} else {
		return "good"
	}
}

func NilTest(v *Test) string {
	if v == nil {
		return "bad"
	} else {
		return "good"
	}
}

const calling = `
--// Calling Go functions from Lua //////
--//  returning multiple values is straightforward
--// all Go number types map to Lua numbers, which are (usually) doubles
local x,a = fun2(42,'hello')
assert(x == 42 and a == 'hello')
--// Lua tables auto-convert to slices when passed
local res = sum{1,10,100}
assert(res == 111)
--// variadic form
res = sumv(1,10,100)
assert(res == 111)
res = squares {10,20,30,40}
--// a map is returned as a map-proxy,
assert(res['0'] == 100)
assert(res['1'] == 400)
--// which we may explicitly convert to a table
res = luar.map2table(res)
assert(type(res)=='table')
assert(res['0'] == 100)
assert(res['1'] == 400)
--// passing a 'hash-like' Lua table converts to  a Go map
local T = {one=1,two=2}
local k = gu.keys(T)
--// can't depend on deterministic ordering in returned slice proxy
assert( (k[1]=='one' and k[2]=='two') or (k[2]=='one' and k[1]=='two') )
local v = gu.values(T)
assert(v[1]==1 or v[2]==1)
do return end
v = luar.slice2table(v)
assert( (v[1]==1 and v[2]==2) or (v[2]==1 and v[1]==2) )

--// passing nils to Go functions
assert(Nil(nil) == 'bad')
assert(NilTest(nil) == 'bad')
`

func Test_callingGoFun(t *testing.T) {
	L := Init()
	defer L.Close()

	// arbitrary Go functions can be registered
	// to be callable from Lua; here the  functions are put into the global table
	Register(L, "", Map{
		"fun2":    fun2,
		"sum":     sum,
		"sumv":    sumv,
		"squares": squares,
		"Nil":     Nil,
		"NilTest": Nil,
	})

	// can register them as a Lua table for namespacing...
	Register(L, "gu", Map{
		"keys":   keys,
		"values": values,
	})

	code := calling
	err := L.DoString(code)
	if err != nil {
		t.Error(err)
	}
}

// dispatching methods on a struct

type Test struct {
	Name string
	Age  int
}

type HasName interface {
	GetName() string
}

func (self *Test) GetName() string {
	return self.Name
}

func NewTest(name string, age int) *Test {
	return &Test{name, age}
}

func NewName(t *Test) HasName {
	return t
}

func GetName(o HasName) string {
	return o.GetName()
}

func NewTestV(name string, age int) Test {
	return Test{name, age}
}

func UnpacksTest(t Test) (string, int) {
	return t.Name, t.Age
}

type Empty struct {
}

func NewEmpty(i int) *Empty {
	if i == 0 {
		return nil
	}
	return &Empty{}
}

const accessing_structs = `
local t = NewTest("Alice",16)
--//t is a struct proxy...
--//can always directly get & set public fields
assert(t.Name == 'Alice')
assert(t.Age == 16)
t.Name = 'Caterpillar'
--// note a weirdness - you don't use colon notation here
assert(t.GetName() == 'Caterpillar')
--// can call methods on struct values as well
t = NewTestV("Alfred",24)
assert(t.GetName() == 'Alfred')
assert(t.Age == 24)
local name,age = UnpacksTest {Name = 'Bob', Age = 22}
assert (name == 'Bob' and age == 22)
print 'finis'

--// function returning ptr or interface, handling return nil
--// pull #7 hirochachacha
assert(NewEmpty(0) == nil)
--// interfaces
 t = NewTest("Alice",16)
it = NewName(t)
assert(it.GetName()=='Alice')
assert(GetName(it)=='Alice')
assert(GetName(t)=='Alice')
assert(luar.type(t).String() == "*luar.Test")
assert(luar.type(it).String() == "*luar.Test")
print 'finished'

`

// there are some basic constructs which need help from the Go side...
// Fortunately it's very easy to import them!

func byteBuffer(sz int) []byte {
	return make([]byte, sz)
}

func bytesToString(bb []byte) string {
	return string(bb)
}

const calling_interface = `
--// calling methods on an interface
local f,err = OsOpen("luar_test.go")
local buff = byteBuffer(100)
assert(#buff == 100)
local k,err = f.Read(buff)
assert(k == 100)
local s = bytesToString(buff)
assert(s:match '^package luar')
f.Close()

`

func Test_callingStructs(t *testing.T) {
	L := Init()
	defer L.Close()

	Register(L, "", Map{
		"NewTest":       NewTest,
		"NewTestV":      NewTestV,
		"UnpacksTest":   UnpacksTest,
		"OsOpen":        os.Open,
		"byteBuffer":    byteBuffer,
		"bytesToString": bytesToString,
		"NewEmpty":      NewEmpty,
		"NewName":       NewName,
		"GetName":       GetName,
	})

	code := accessing_structs + calling_interface
	err := L.DoString(code)
	if err != nil {
		t.Error(err)
	}
}

// using Lua to parse configuration files
const config = `
return {
  baggins = true,
  age = 24,
  name = 'dumbo' ,
  marked = {1,2},
  options = {
      leave = true,
      cancel = 'always',
	  tags = {strong=true,foolish=true},
  }
}
`

func Test_parsingConfig(t *testing.T) {
	L := Init()
	defer L.Close()

	err := L.DoString(config)
	if err != nil {
		t.Error(err)
	}
	// there will be a table on the Lua stack
	if !L.IsTable(-1) {
		t.Error("did not return a table")
	}
	v := CopyTableToMap(L, nil, -1)
	// extract table from the returned interface...
	m := v.(map[string]interface{})
	assertEq(t, "baggins", m["baggins"], true)
	assertEq(t, "name", m["name"], "dumbo")
	marked := m["marked"].([]interface{})
	assertEq(t, "slice len", len(marked), 2)
	// a little gotcha here - Lua numbers are doubles..
	assertEq(t, "val", marked[0], 1.0)
	assertEq(t, "val", marked[1], 2.0)
	options := m["options"].(map[string]interface{})
	assertEq(t, "leave", options["leave"], true)

	// another way to do this. using LuaObject to manipulate the table
	L.DoString(config)
	lo := NewLuaObject(L, -1)
	assertEq(t, "lbag", lo.Get("baggins"), true)
	assertEq(t, "lname", lo.Get("name"), "dumbo")
	// can get the field itself as a Lua object, and so forth
	opts := lo.GetObject("options")
	assertEq(t, "opts", opts.Get("leave"), true)
	// note that these Get methods understand nested fields ('chains')
	assertEq(t, "chain", lo.Get("options.leave"), true)
	assertEq(t, "chain", lo.Get("options.tags.strong"), true)
	// nested fields don't crash but return nil
	assertEq(t, "chain", lo.Get("options.tags.extra.flakey"), nil)
	markd := lo.GetObject("marked")
	assertEq(t, "marked1", markd.Geti(1), 1.0)
	iter := lo.Iter()
	keys := []string{}
	for iter.Next() {
		keys = append(keys, iter.Key.(string))
	}
	if !compareNoOrder(keys, []string{"baggins", "options", "marked", "age", "name"}) {
		t.Error("keys were not the same!")
	}

}

func findInSlice(ss []string, s string) int {
	for i, v := range ss {
		if v == s {
			return i
		}
	}
	return -1
}

func compareNoOrder(s1, s2 []string) bool {
	if len(s1) != len(s2) {
		return false
	}
	for _, s := range s1 {
		if findInSlice(s2, s) == -1 {
			return false
		}
	}
	return true
}

const luaf = `
Libs = {}
function Libs.fun(s,i,t,m)
	assert(s == 'hello')
	assert(i == 42)
    --// slices and maps passed as proxies
	assert(type(t) == 'userdata' and t[1] == 42)
	assert(type(m) == 'userdata' and m.name == 'Joe')
	return 'ok'
end
function Libs.return_strings()
    return {'one','two','three'}
end
`

func Test_callingLua(t *testing.T) {
	L := Init()
	defer L.Close()

	// the very versatile string.gsub function
	lobj := NewLuaObjectFromName
	gsub := lobj(L, "string.gsub")
	// this is a Lua table... copies Go object, doesn't create proxy
	replacements := NewLuaObjectFromValue(L, Map{
		"NAME": "Dolly",
		"HOME": "where you belong",
	})
	// we do this because string.gsub wants either a string,function or table for its second arg
	res, err := gsub.Call("hello $NAME go $HOME", "%$(%u+)", replacements)
	if res == nil {
		t.Error(err)
	}
	assertEq(t, "hello", res, "hello Dolly go where you belong")

	err = L.DoString(luaf)
	if err != nil {
		t.Error(err)
	}

	fun := lobj(L, "Libs.fun")
	res, err = fun.Call("hello", 42, []int{42, 66, 104}, map[string]string{
		"name": "Joe",
	})
	assertEq(t, "fun", res, "ok")

	// can force the type and number of returned values using Callf
	fun = lobj(L, "Libs.return_strings")
	returns := Types([]string{}) // []reflect.Type
	results, err := fun.Callf(returns)
	// first returned result should be a slice of strings
	strs := results[0].([]string)
	if !(strs[0] == "one" && strs[1] == "two" && strs[2] == "three") {
		t.Error("did not get correct slice of strings!")
	}

	println("that's all folks!")

}

type A int

const gtypes1 = `
a = 10
assert(m.test == 'art')
assert(m.Test == nil)
`

const gtypes2 = `
print(m[5])
`

func Test_passingTypes(t *testing.T) {
	L := Init()
	defer L.Close()

	a := A(5)
	m := map[string]string{"test": "art"}

	Register(L, "", Map{
		"a": a,
		"m": m,
	})

	err := L.DoString(gtypes1)
	if err != nil {
		t.Error(err)
	}

	L.GetGlobal("a")
	aType := reflect.TypeOf(a)
	al := LuaToGo(L, aType, -1)
	alType := reflect.TypeOf(al)

	if alType != aType {
		t.Error("types were not converted properly")
	}

	err = L.DoString(gtypes2)
	if err == nil {
		t.Error("must not be able to index map with wrong type!")
	}

}
