package luar

import (
	"os"
	"reflect"
	"runtime"
	"strconv"
	"sync"
	"testing"
)

// Calling Go functions from Lua.
// Returning multiple values is straightforward.
// All Go number types map to Lua numbers, which are (usually) doubles.
//
// Arbitrary Go functions can be registered to be callable from Lua. Here the
// functions are put into the global table.
func TestGoFunCall(t *testing.T) {
	id := func(x float32, a string) (float32, string) {
		return x, a
	}

	sum := func(args []float64) float64 {
		res := 0.0
		for _, val := range args {
			res += val
		}
		return res
	}

	sumv := func(args ...float64) float64 {
		return sum(args)
	}

	// [10,20] -> {'0':100, '1':400}
	squares := func(args []int) (res map[string]int) {
		res = make(map[string]int)
		for i, val := range args {
			res[strconv.Itoa(i)] = val * val
		}
		return
	}

	IsNilInterface := func(v interface{}) bool {
		return v == nil
	}

	IsNilPointer := func(v *person) bool {
		return v == nil
	}

	tdt := []struct{ desc, code string }{
		{"go function call", `x, a = id(42, 'foo')
assert(x == 42 and a == 'foo')`},
		{"auto-convert table to slice", `res = sum{1, 10, 100}
assert(res == 111)`},
		{"variadic call", `res = sumv(1, 10, 100)
assert(res == 111)`},

		// A map is returned as a map-proxy, which we may explicitly convert to a
		// table.
		{"init proxy", `proxy = squares{10, 20}
assert(proxy['0'] == 100)
assert(proxy['1'] == 400)`},
		{"copy proxy to table", `proxy = squares{10, 20}
t = luar.unproxify(proxy)
assert(type(t)=='table')
assert(t['0'] == 100)
assert(t['1'] == 400)`},
		{"change proxy, not table", `proxy = squares{10, 20}
t = luar.unproxify(proxy)
proxy['0'] = 0
assert(t['0'] == 100)`},

		{"pass nil to Go functions", `assert(IsNilInterface(nil))
assert(IsNilPointer(nil))`},
	}

	for _, v := range tdt {
		L := Init()
		defer L.Close()
		Register(L, "", Map{
			"id":             id,
			"sum":            sum,
			"sumv":           sumv,
			"squares":        squares,
			"IsNilInterface": IsNilInterface,
			"IsNilPointer":   IsNilPointer,
		})
		err := L.DoString(v.code)
		if err != nil {
			t.Error(v.desc+":", err)
		}
	}
}

func TestArray(t *testing.T) {
	L := Init()
	defer L.Close()

	a := [2]int{17, 18}

	Register(L, "", Map{
		"a": a,
		"b": &a,
	})

	const code_a = `
assert(#a == 2)
assert(type(a) == 'table')
assert(a[1] == 17)
assert(a[2] == 18)
a[2] = 180
`
	const code_b = `
assert(#b == 2)
assert(type(b) == 'userdata')
assert(b[1] == 17)
assert(b[2] == 18)
for _, v in ipairs(b) do
assert(b[1] == 17)
break
end
b[1] = 170
`

	err := L.DoString(code_a)
	if err != nil {
		t.Error(err)
	}

	if a[0] != 17 || a[1] != 18 {
		t.Error("table copy has modified its source")
	}

	err = L.DoString(code_b)
	if err != nil {
		t.Error(err)
	}

	if a[0] != 170 || a[1] != 18 {
		t.Error("table proxy has not modified its source")
	}

	{
		var output [2]int
		L.GetGlobal("a")
		res := LuaToGo(L, reflect.TypeOf(output), -1)
		output = res.([2]int)
		if output[0] != 17 || output[1] != 180 {
			t.Error("table conversion produced unexpected values", output)
		}
	}

	{
		var output *[2]int
		L.GetGlobal("a")
		res := LuaToGo(L, reflect.TypeOf(output), -1)
		output = res.(*[2]int)
		if output[0] != 17 || output[1] != 180 {
			t.Error("table conversion produced unexpected values", output)
		}
	}

	{
		var output *[2]int
		L.GetGlobal("b")
		res := LuaToGo(L, reflect.TypeOf(output), -1)
		output = res.(*[2]int)
		if output[0] != 170 || output[1] != 18 {
			t.Error("table conversion produced unexpected values", output)
		}
	}
}

func TestComplex(t *testing.T) {
	L := Init()
	defer L.Close()

	c := 2 + 3i
	a := NewA(32)

	Register(L, "", Map{
		"c": c,
		"a": a,
	})

	const code = `
assert(c == luar.complex(2, 3))
assert(c.real == 2)
assert(c.imag == 3)
c = c+c
assert(c == luar.complex(4, 6))
c = -c
assert(c == luar.complex(-4, -6))
c = 2*c
assert(c == luar.complex(-8, -12))

z = c / a
assert(z == luar.complex(-0.25, -0.375))
`

	err := L.DoString(code)
	if err != nil {
		t.Error(err)
	}
}

func TestChan(t *testing.T) {
	L1 := Init()
	defer L1.Close()
	L2 := Init()
	defer L2.Close()

	c := make(chan int)

	Register(L1, "", Map{
		"c": c,
	})

	Register(L2, "", Map{
		"c": c,
	})

	const code1 = `
c.send(17)
`

	const code2 = `
v = c.recv()
assert(v == 17)
`

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		err := L1.DoString(code1)
		if err != nil {
			t.Error(err)
		}
		wg.Done()
	}()

	err := L2.DoString(code2)
	if err != nil {
		t.Error(err)
	}

	wg.Wait()
}

func TestNamespace(t *testing.T) {
	keys := func(m map[string]interface{}) (res []string) {
		res = make([]string, 0)
		for k := range m {
			res = append(res, k)
		}
		return
	}

	values := func(m map[string]interface{}) (res []interface{}) {
		res = make([]interface{}, 0)
		for _, v := range m {
			res = append(res, v)
		}
		return
	}

	const code = `
-- Passing a 'hash-like' Lua table converts to a Go map.
local T = {one=1, two=2}
local k = gons.keys(T)

-- Can't depend on deterministic ordering in returned slice proxy.
assert( (k[1]=='one' and k[2]=='two') or (k[2]=='one' and k[1]=='two') )

local v = gons.values(T)
assert(v[1]==1 or v[2]==1)
v = luar.unproxify(v)
assert( (v[1]==1 and v[2]==2) or (v[2]==1 and v[1]==2) )`

	L := Init()
	defer L.Close()

	Register(L, "gons", Map{
		"keys":   keys,
		"values": values,
	})

	err := L.DoString(code)
	if err != nil {
		t.Error(err)
	}
}

type person struct {
	Name string
	Age  int
}

type hasName interface {
	GetName() string
}

func (p *person) GetName() string {
	return p.Name
}

func newPerson(name string, age int) *person {
	return &person{name, age}
}

func newName(t *person) hasName {
	return t
}

func getName(o hasName) string {
	return o.GetName()
}

func TestStructAccess(t *testing.T) {
	const code = `
-- 't' is a struct proxy.
-- We can always directly get and set public fields.
local t = NewPerson("Alice", 16)
assert(t.Name == 'Alice')
assert(t.Age == 16)
t.Name = 'Caterpillar'

-- Note a pitfall: we don't use colon notation here.
assert(t.GetName() == 'Caterpillar')

-- Interfaces.
t = NewPerson("Alice", 16)
it = NewName(t)
assert(it.GetName()=='Alice')
assert(GetName(it)=='Alice')
assert(GetName(t)=='Alice')
assert(luar.type(t).String() == "*luar.person")
assert(luar.type(it).String() == "*luar.person")
`

	L := Init()
	defer L.Close()

	Register(L, "", Map{
		"NewPerson": newPerson,
		"NewName":   newName,
		"GetName":   getName,
	})

	err := L.DoString(code)
	if err != nil {
		t.Error(err)
	}
}

func TestStructCopy(t *testing.T) {
	L := Init()
	defer L.Close()

	a := person{Name: "foo", Age: 17}
	GoToLua(L, nil, reflect.ValueOf(a), true)
	L.SetGlobal("a")

	const code = `
assert(a.Name =='foo')
assert(a.Age ==17)
`

	err := L.DoString(code)
	if err != nil {
		t.Error(err)
	}
}

type directStruct struct {
	v []string
}

func (s *directStruct) GetSlice() []string {
	return s.v
}

// Issue is here: when not return a pointer, GetSlice() cannot be found.
func newDirectStruct(initial string) directStruct {
	r := directStruct{}
	r.v = append(r.v, initial)
	return r
}

func TestDirectStructMethod(t *testing.T) {
	L := Init()
	defer L.Close()

	Register(L, "", Map{
		"newDirectStruct": newDirectStruct,
	})

	err := L.DoString(`
s = newDirectStruct("bar")
v = s.GetSlice()
assert(v[1] == "bar")
`)

	if err != nil {
		t.Error(err)
	}
}

func TestInterfaceAccess(t *testing.T) {
	const code = `
-- Calling methods on an interface.
local f, err = OsOpen("luar_test.go")
local buff = byteBuffer(100)
assert(#buff == 100)
local k, err = f.Read(buff)
assert(k == 100)
local s = bytesToString(buff)
assert(s:match '^package luar')
f.Close()`

	// There are some basic constructs which need help from the Go side...
	// Fortunately it's very easy to import them!
	byteBuffer := func(sz int) []byte {
		return make([]byte, sz)
	}
	bytesToString := func(bb []byte) string {
		return string(bb)
	}

	L := Init()
	defer L.Close()

	Register(L, "", Map{
		"OsOpen":        os.Open,
		"byteBuffer":    byteBuffer,
		"bytesToString": bytesToString,
	})

	err := L.DoString(code)
	if err != nil {
		t.Error(err)
	}
}

type mySlice []int

func (m *mySlice) Foo() int {
	return len(*m)
}

func TestSliceMethod(t *testing.T) {
	L := Init()
	defer L.Close()

	a := mySlice{17, 170}

	Register(L, "", Map{
		"a": a,
	})

	const code = `
assert(a.Foo() == 2)
assert(a[1] == 17)
a = a.append(18.5, 19)
a = a.append(unpack({3, 2}))
sub = a.sub(3, 4)
assert(sub[1] == 18)
assert(sub[2] == 19)
`

	err := L.DoString(code)
	if err != nil {
		t.Error(err)
	}
}

type myMap map[string]int

func (m *myMap) Foo() int {
	return len(*m)
}

func TestMapMethod(t *testing.T) {
	L := Init()
	defer L.Close()

	a := myMap{"foo": 17, "bar": 170}
	b := myMap{"Foo": 17}

	Register(L, "", Map{
		"a": a,
		"b": b,
	})

	const code = `
assert(a.Foo() == 2)
assert(a.foo == 17)
assert(b.Foo == 17)
assert(luar.method(b, "Foo")() == 1)
`

	err := L.DoString(code)
	if err != nil {
		t.Error(err)
	}

}

func TestLuaCallSlice(t *testing.T) {
	L := Init()
	defer L.Close()

	const code = `
Libs = {}
function Libs.fun(s,i,t,m)
	assert(s == 'hello')
	assert(i == 42)
    --// slices and maps passed as proxies
	assert(type(t) == 'userdata' and t[1] == 42)
	assert(type(m) == 'userdata' and m.name == 'Joe')
	return 'ok'
end`

	err := L.DoString(code)
	if err != nil {
		t.Error(err)
	}

	fun := NewLuaObjectFromName(L, "Libs.fun")
	got, _ := fun.Call("hello", 42, []int{42, 66, 104}, map[string]string{
		"name": "Joe",
	})
	if got != "ok" {
		t.Error("did not get correct slice of slices!")
	}
}

func TestLuaCallfSlice(t *testing.T) {
	L := Init()
	defer L.Close()

	const code = `
function return_slices()
    return {{'one'}, luar.null, {'three'}}
end`

	err := L.DoString(code)
	if err != nil {
		t.Error(err)
	}

	fun := NewLuaObjectFromName(L, "return_slices")
	results, _ := fun.Callf(Types([][]string{}))
	sstrs := results[0].([][]string)
	if !(sstrs[0][0] == "one" && sstrs[1] == nil && sstrs[2][0] == "three") {
		t.Error("did not get correct slice of slices!")
	}
}

// See if Go values are properly anchored.
func TestAnchoring(t *testing.T) {
	L := Init()
	defer L.Close()

	const code = `local s = luar.slice(2)
s[1] = 10
s[2] = 20
gc()
assert(#s == 2 and s[1]==10 and s[2]==20)
s = nil`

	Register(L, "", Map{
		"gc": runtime.GC,
	})

	err := L.DoString(code)
	if err != nil {
		t.Error(err)
	}
}

type A int

func NewA(i int) A {
	return A(i)
}

func (a A) String() string {
	return strconv.Itoa(int(a))
}

func (a A) FooA() string {
	return "FooA"
}

type B int

func NewB(i int) B {
	return B(i)
}

func (b B) FooB() string {
	return "FooB"
}

type C string

func NewC(s string) C {
	return C(s)
}

func (c C) FooC() string {
	return "FooC"
}

func TestArithNewTypes(t *testing.T) {
	L := Init()
	defer L.Close()

	i := A(17)
	j := B(18)
	s := C("foo")

	Register(L, "", Map{
		"i":    i,
		"j":    j,
		"s":    s,
		"newA": NewA,
		"newB": NewB,
		"newC": NewC,
	})

	const code = `
f = i + j
assert(f == 35)

i2 = i+i
assert(i2 == newA(34))

i3 = i+10
assert(i3 == newA(27))

j2 = j + j
assert(j2 == newB(36))

s = s .. "bar"
assert(#s == 6)
assert(s == newC("foobar"))
assert(s.FooC() == "FooC")
`

	err := L.DoString(code)
	if err != nil {
		t.Error(err)
	}
}

func TestTypeDiscipline(t *testing.T) {
	tdt := []struct{ desc, code string }{
		{"call methods on objects 'derived' from primitive types", `assert(a.String() == '5')`},
		{"get underlying primitive value", `assert(luar.unproxify(a) == 5)`},
		{"arith ops on derived types", `assert(new_a(8) == new_a(8))
assert(new_a(5) ~= new_a(6))
assert(new_a(5) < new_a(8))
assert(new_a(8) > new_a(5))
assert(((new_a(8) * new_a(5)) / new_a(4)) % new_a(7) == new_a(3))`},
	}

	L := Init()
	defer L.Close()

	a := A(5)
	b := B(9)

	Register(L, "", Map{
		"a":     a,
		"b":     b,
		"new_a": NewA,
	})

	for _, v := range tdt {
		err := L.DoString(v.code)
		if err != nil {
			t.Error(v.desc, err)
		}
	}

	L.GetGlobal("a")
	aType := reflect.TypeOf(a)
	al := LuaToGo(L, aType, -1)
	alType := reflect.TypeOf(al)

	if alType != aType {
		t.Error("types were not converted properly")
	}

	// Binary op with different type must fail.
	const fail = `assert(b != new_a(9))`
	err := L.DoString(fail)
	if err == nil {
		t.Error(err)
	}
}

// Map non-existent entry should be nil.
func TestTypeMap(t *testing.T) {
	L := Init()
	defer L.Close()

	m := map[string]string{"test": "art"}

	Register(L, "", Map{
		"m": m,
	})

	const code = `assert(m.test == 'art')
assert(m.Test == nil)`

	// Accessing map with wrong key type must fail.
	const code2 = `_=m[5]`

	err := L.DoString(code)
	if err != nil {
		t.Error(err)
	}
}

func TestMapIpair(t *testing.T) {
	L := Init()
	defer L.Close()

	m := map[interface{}]string{
		-1:  "ko",
		0:   "ko",
		1:   "foo",
		2:   "bar",
		"3": "baz",
	}

	Register(L, "", Map{"m": m})

	const code = `
t = {}
for k, v in ipairs(m) do
t[k] = v
end
assert(t[0] == nil)
assert(t[1] == "foo")
assert(t[2] == "bar")
assert(t[3] == nil)
`

	err := L.DoString(code)
	if err != nil {
		t.Error(err)
	}
}

// 'nil' in Go slices and maps is represented by luar.null.
func TestTypeConversion(t *testing.T) {
	L := Init()
	defer L.Close()

	const code = `
tab = luar.unproxify(sl)
assert(#tab == 4)
assert(tab[1] == luar.null)
assert(tab[3] == luar.null)

tab2 = luar.unproxify(mn)
assert(tab2.bee == luar.null and tab2.dee == luar.null)
`

	sl := [][]int{
		nil,
		{1, 2},
		nil,
		{10, 20},
	}

	mn := map[string][]int{
		"aay": {1, 2},
		"bee": nil,
		"cee": {10, 20},
		"dee": nil,
	}

	Register(L, "", Map{
		"sl": sl,
		"mn": mn,
	})

	err := L.DoString(code)
	if err != nil {
		t.Error(err)
	}
}

func TestCycleLuaToGo(t *testing.T) {
	L := Init()
	defer L.Close()

	{
		var output []interface{}
		L.DoString(`t = {17}; t[2] = t`)
		L.GetGlobal("t")
		v := LuaToGo(L, reflect.TypeOf(output), -1)
		output = v.([]interface{})
		output_1 := output[1].([]interface{})
		if &output_1[0] != &output[0] {
			t.Errorf("address of repeated element differs")
		}
	}

	{
		var output []interface{}
		L.DoString(`t = {17}; v = {t}; t[2] = v`)
		L.GetGlobal("t")
		v := LuaToGo(L, reflect.TypeOf(output), -1)
		output = v.([]interface{})
		output_1 := output[1].([]interface{})
		output_1_0 := output_1[0].([]interface{})
		if &output_1_0[0] != &output[0] {
			t.Errorf("address of repeated element differs")
		}
	}

	{
		var output []interface{}
		L.DoString(`t = {17}; v = {t, t}; t[2] = v; t[3] = v; t[4] = t`)
		L.GetGlobal("t")
		v := LuaToGo(L, reflect.TypeOf(output), -1)
		output = v.([]interface{})
		output_2 := output[2].([]interface{})
		output_2_0 := output_2[0].([]interface{})
		if &output_2_0[0] != &output[0] {
			t.Errorf("address of repeated element differs")
		}
	}

	{
		var output map[string]interface{}
		L.DoString(`t = {foo=17}; t["bar"] = t`)
		L.GetGlobal("t")
		m := LuaToGo(L, reflect.TypeOf(output), -1)
		output = m.(map[string]interface{})
		output_1 := output["bar"].(map[string]interface{})
		output["foo"] = 18
		if output["foo"] != output_1["foo"] {
			t.Errorf("address of repeated element differs")
		}
	}

	{
		var output map[string]interface{}
		L.DoString(`t = {foo=17}; v = {baz=t}; t["bar"] = v`)
		L.GetGlobal("t")
		m := LuaToGo(L, reflect.TypeOf(output), -1)
		output = m.(map[string]interface{})
		output_bar := output["bar"].(map[string]interface{})
		output_bar_baz := output_bar["baz"].(map[string]interface{})
		output["foo"] = 18
		if output["foo"] != output_bar_baz["foo"] {
			t.Errorf("address of repeated element differs")
		}
	}
}

type list struct {
	V    int
	Next *list
}

func TestCycleGoToLua(t *testing.T) {
	L := Init()
	defer L.Close()

	{
		s := make([]interface{}, 2)
		s[0] = 17
		s[1] = s
		GoToLua(L, nil, reflect.ValueOf(s), true)
		output := L.ToPointer(-1)
		L.RawGeti(-1, 2)
		output_1 := L.ToPointer(-1)
		L.SetTop(0)
		if output != output_1 {
			t.Errorf("address of repeated element differs")
		}
	}

	{
		s := make([]interface{}, 2)
		s[0] = 17
		s2 := make([]interface{}, 2)
		s2[0] = 18
		s2[1] = s
		s[1] = s2
		GoToLua(L, nil, reflect.ValueOf(s), true)
		output := L.ToPointer(-1)
		L.RawGeti(-1, 2)
		L.RawGeti(-1, 2)
		output_1_1 := L.ToPointer(-1)
		L.SetTop(0)
		if output != output_1_1 {
			t.Errorf("address of repeated element differs")
		}
	}

	{
		s := map[string]interface{}{}
		s["foo"] = 17
		s["bar"] = s
		GoToLua(L, nil, reflect.ValueOf(s), true)
		output := L.ToPointer(-1)
		L.GetField(-1, "bar")
		output_bar := L.ToPointer(-1)
		L.SetTop(0)
		if output != output_bar {
			t.Errorf("address of repeated element differs")
		}
	}

	{
		s := map[string]interface{}{}
		s["foo"] = 17
		s2 := map[string]interface{}{}
		s2["bar"] = 18
		s2["baz"] = s
		s["qux"] = s2
		GoToLua(L, nil, reflect.ValueOf(s), true)
		output := L.ToPointer(-1)
		L.GetField(-1, "qux")
		L.GetField(-1, "baz")
		output_qux_baz := L.ToPointer(-1)
		L.SetTop(0)
		if output != output_qux_baz {
			t.Errorf("address of repeated element differs")
		}
	}

	{
		l1 := &list{V: 17}
		l2 := &list{V: 18}
		l1.Next = l2
		l2.Next = l1
		GoToLua(L, nil, reflect.ValueOf(l1), true)
		output_l1 := L.ToPointer(-1)
		L.GetField(-1, "Next")
		L.GetField(-1, "Next")
		output_l1_l2_l1 := L.ToPointer(-1)
		L.SetTop(0)
		if output_l1 != output_l1_l2_l1 {
			t.Errorf("address of repeated element differs")
		}
	}

	{
		l1 := &list{V: 17}
		l2 := &list{V: 18}
		l1.Next = l2
		l2.Next = l1
		CopyStructToTable(L, reflect.ValueOf(l1))
		// Note that root table is only repeated if we call CopyStructToTable on the
		// pointer.
		output_l1 := L.ToPointer(-1)
		L.GetField(-1, "Next")
		output_l1_l2 := L.ToPointer(-1)
		L.GetField(-1, "Next")
		output_l1_l2_l1 := L.ToPointer(-1)
		L.GetField(-1, "Next")
		output_l1_l2_l1_l2 := L.ToPointer(-1)

		L.SetTop(0)
		if output_l1 != output_l1_l2_l1 || output_l1_l2 != output_l1_l2_l1_l2 {
			t.Errorf("address of repeated element differs")
		}
	}

	{
		a := [2]interface{}{}
		a[0] = 17
		a[1] = &a

		// Pass reference so that first element can be part of the cycle.
		Register(L, "", Map{"a": &a})

		const code = `
assert(#a == 2)
assert(a[1] == 17)
a[1] = 18
assert(a[1] == 18)
assert(a[2][1] == 18)
`
		err := L.DoString(code)
		if err != nil {
			t.Error(err)
		}
	}
}

func TestStringIpairs(t *testing.T) {
	L := Init()
	defer L.Close()

	a := C("naïveté")

	Register(L, "", Map{"a": a})

	const code = `
for k, v in ipairs(a) do
if k == 3 then
assert(v == "ï")
break
end
end
`

	err := L.DoString(code)
	if err != nil {
		t.Error(err)
	}
}
