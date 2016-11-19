package luar

import (
	"reflect"
	"strings"

	"github.com/aarzilli/golua/lua"
)

// LuaObject encapsulates a Lua object like a table or a function.
type LuaObject struct {
	L    *lua.State
	Ref  int
	Type string
}

// Global creates a new LuaObject refering to the global environment.
func Global(L *lua.State) *LuaObject {
	L.GetGlobal("_G")
	val := NewLuaObject(L, -1)
	L.Pop(1)
	return val
}

// NewLuaObject creates a new LuaObject from stack index.
func NewLuaObject(L *lua.State, idx int) *LuaObject {
	tp := L.LTypename(idx)
	L.PushValue(idx)
	ref := L.Ref(lua.LUA_REGISTRYINDEX)
	return &LuaObject{L, ref, tp}
}

// NewLuaObjectFromName creates a new LuaObject from global qualified name, using
// Lookup.
func NewLuaObjectFromName(L *lua.State, path string) *LuaObject {
	lookup(L, path, 0)
	val := NewLuaObject(L, -1)
	L.Pop(1)
	return val
}

// NewLuaObjectFromValue creates a new LuaObject from a Go value.
// Note that this _will_ convert any slices or maps into Lua tables.
func NewLuaObjectFromValue(L *lua.State, val interface{}) *LuaObject {
	GoToLua(L, nil, reflect.ValueOf(val), true)
	return NewLuaObject(L, -1)
}

// Get returns the Go value indexed at 'key' in the Lua object.
func (lo *LuaObject) Get(key string) interface{} {
	lo.Push() // the table
	lookup(lo.L, key, -1)
	val := LuaToGo(lo.L, nil, -1)
	lo.L.Pop(2)
	return val
}

// Geti return the value indexed at 'idx'.
func (lo *LuaObject) Geti(idx int64) interface{} {
	L := lo.L
	lo.Push() // the table
	L.PushInteger(idx)
	L.GetTable(-2)
	val := LuaToGo(L, nil, -1)
	L.Pop(1) // the  table
	return val
}

// GetObject returns the Lua object indexed at 'key' in the Lua object.
func (lo *LuaObject) GetObject(key string) *LuaObject {
	lo.Push() // the table
	lookup(lo.L, key, -1)
	val := NewLuaObject(lo.L, -1)
	lo.L.Pop(2)
	return val
}

// Set sets the value at a given index 'idx'.
func (lo *LuaObject) Set(idx interface{}, val interface{}) interface{} {
	L := lo.L
	lo.Push() // the table
	GoToLua(L, nil, reflect.ValueOf(idx), false)
	GoToLua(L, nil, reflect.ValueOf(val), false)
	L.SetTable(-3)
	L.Pop(1) // the  table
	return val
}

// Setv copies values between two tables in the same state.
func (lo *LuaObject) Setv(src *LuaObject, keys ...string) {
	L := lo.L
	lo.Push()  // destination table at -2
	src.Push() // source table at -1
	for _, key := range keys {
		L.GetField(-1, key) // pushes value
		L.SetField(-3, key) // pops value
	}
	L.Pop(2) // clear the tables
}

// Callf calls a Lua function, given the desired return types and the arguments.
//
// Callf is used whenever:
//
// - the Lua function has multiple return values;
//
// - and/or you have exact types for these values.
//
// The first argument may be `nil` and can be used to access multiple return
// values without caring about the exact conversion.
func (lo *LuaObject) Callf(rtypes []reflect.Type, args ...interface{}) (res []interface{}, err error) {
	L := lo.L
	if rtypes == nil {
		rtypes = []reflect.Type{nil}
	}
	res = make([]interface{}, len(rtypes))
	lo.Push()                  // the function...
	for _, arg := range args { // push the args
		GoToLua(L, nil, reflect.ValueOf(arg), false)
	}
	err = L.Call(len(args), 1)
	if err == nil {
		for i, t := range rtypes {
			res[i] = LuaToGo(L, t, -1)
		}
		L.Pop(len(rtypes))
	}
	return
}

// Call calls a Lua function and return a single value, converted in a default way.
func (lo *LuaObject) Call(args ...interface{}) (res interface{}, err error) {
	var sres []interface{}
	sres, err = lo.Callf(nil, args...)
	if err != nil {
		res = nil
		return
	}
	return sres[0], nil
}

// Push pushes this Lua object on the stack.
func (lo *LuaObject) Push() {
	lo.L.RawGeti(lua.LUA_REGISTRYINDEX, lo.Ref)
}

// Close frees the Lua reference of this object.
func (lo *LuaObject) Close() {
	lo.L.Unref(lua.LUA_REGISTRYINDEX, lo.Ref)
}

// LuaTableIter is the Go equivalent of a Lua table iterator.
type LuaTableIter struct {
	lo    *LuaObject
	first bool
	Key   interface{}
	Value interface{}
}

// Iter creates a Lua table iterator.
func (lo *LuaObject) Iter() *LuaTableIter {
	return &LuaTableIter{lo, true, nil, nil}
}

// Next gets the next key/value pair from the table.
func (ti *LuaTableIter) Next() bool {
	L := ti.lo.L
	if ti.first {
		ti.lo.Push() // the table
		L.PushNil()  // the key
		ti.first = false
	}
	// table is under the key
	if L.Next(-2) == 0 {
		return false
	}

	ti.Key = LuaToGo(L, nil, -2)
	ti.Value = LuaToGo(L, nil, -1)
	L.Pop(1) // drop value, key is now on top
	return true
}

// lookup will search a Lua value by its full name.
//
// If idx is 0, then this name is assumed to start in the global table, e.g.
// "string.gsub". With non-zero idx, it can be used to look up subfields of a
// table. It terminates with a nil value if we cannot continue the lookup.
func lookup(L *lua.State, path string, idx int) {
	parts := strings.Split(path, ".")
	if idx != 0 {
		L.PushValue(idx)
	} else {
		L.GetGlobal("_G")
	}
	for _, field := range parts {
		L.GetField(-1, field)
		L.Remove(-2) // remove table
		if L.IsNil(-1) {
			break
		}
	}
}
