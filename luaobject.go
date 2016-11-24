package luar

// TODO: Test all these lo functions, test call on non-functions and get/set on non-tables.

import (
	"errors"
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

// NewLuaObject creates a new LuaObject from stack index.
func NewLuaObject(L *lua.State, idx int) *LuaObject {
	tp := L.LTypename(idx)
	L.PushValue(idx)
	ref := L.Ref(lua.LUA_REGISTRYINDEX)
	return &LuaObject{L, ref, tp}
}

// NewLuaObjectFromName creates a new LuaObject from global qualified name.
func NewLuaObjectFromName(L *lua.State, path string) *LuaObject {
	lookup(L, path, 0)
	val := NewLuaObject(L, -1)
	L.Pop(1)
	return val
}

// NewLuaObjectFromValue creates a new LuaObject from a Go value.
// Note that this will convert any slices or maps into Lua tables.
func NewLuaObjectFromValue(L *lua.State, val interface{}) *LuaObject {
	GoToLua(L, val)
	return NewLuaObject(L, -1)
}

// Call calls a Lua function, given the desired result array and the arguments.
// 'results' must be a pointer or a slice.
// If the function returns more values than can be stored in the 'results'
// argument, they will be ignored.
func (lo *LuaObject) Call(results interface{}, args ...interface{}) error {
	// TODO: Allow for dynamic results len. Should be pointer to Slice/Struct?
	/*
			New rules to implement:
			- If call returns one element, then elem is stored to whatever pointer it is.
			- If multiple, then must be a pointer to slice/struct.

		Return error if cannot convert or if non pointer to slice/struct on multiple.
	*/
	L := lo.L
	// Push the function.
	lo.Push()
	// Push the args.
	for _, arg := range args {
		GoToLuaProxy(L, arg)
	}

	res := reflect.ValueOf(results)
	switch res.Kind() {
	case reflect.Ptr:
		err := L.Call(len(args), 1)
		if err != nil {
			return err
		}
		defer L.Pop(1)
		return LuaToGo(L, -1, res.Interface())

	case reflect.Slice:
		resStart := L.GetTop()
		nresults := res.Len()
		err := L.Call(len(args), nresults)
		if err != nil {
			return err
		}
		resT := res.Type().Elem()
		for i := 0; i < nresults; i++ {
			val := reflect.New(resT)
			err = LuaToGo(L, resStart+i, val.Interface())
			if err != nil {
				L.Pop(nresults)
				return err
			}
			val = val.Elem()
			res.Index(i).Set(val)
		}
		// Nullify the remaining elements if any.
		for i := nresults; i < res.Len(); i++ {
			res.Index(i).Set(reflect.Zero(resT))
		}
		L.Pop(nresults)

	default:
		return errors.New("result argument must be a pointer or a slice")
	}

	return nil
}

// Close frees the Lua reference of this object.
func (lo *LuaObject) Close() {
	lo.L.Unref(lua.LUA_REGISTRYINDEX, lo.Ref)
}

// Get returns the Go value indexed at 'key' in the Lua object.
func (lo *LuaObject) Get(key string, a interface{}) error {
	// The object is assumed to be a table.
	lo.Push()
	defer lo.L.Pop(2)
	lookup(lo.L, key, -1)
	return LuaToGo(lo.L, -1, a)
}

// Geti return the value indexed at 'idx'.
func (lo *LuaObject) Geti(idx int64, a interface{}) error {
	// The object is assumed to be a table.
	lo.Push()
	defer lo.L.Pop(1)
	lo.L.PushInteger(idx)
	lo.L.GetTable(-2)
	return LuaToGo(lo.L, -1, a)
}

// GetObject returns the Lua object indexed at 'key' in the Lua object.
func (lo *LuaObject) GetObject(key string) *LuaObject {
	// The object is assumed to be a table.
	lo.Push()
	lookup(lo.L, key, -1)
	val := NewLuaObject(lo.L, -1)
	lo.L.Pop(2)
	return val
}

// Push pushes this Lua object on the stack.
func (lo *LuaObject) Push() {
	lo.L.RawGeti(lua.LUA_REGISTRYINDEX, lo.Ref)
}

// Set sets the value at index 'idx'.
func (lo *LuaObject) Set(idx interface{}, a interface{}) {
	// The object is assumed to be a table.
	lo.Push()
	GoToLuaProxy(lo.L, idx)
	GoToLuaProxy(lo.L, a)
	lo.L.SetTable(-3)
	lo.L.Pop(1)
}

// Setv copies values between two tables in the same state.
func (lo *LuaObject) Setv(src *LuaObject, keys ...string) {
	L := lo.L
	lo.Push()
	src.Push()
	for _, key := range keys {
		L.GetField(-1, key)
		L.SetField(-3, key)
	}
	L.Pop(2)
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

	// TODO: Error check?
	LuaToGo(L, -2, &ti.Key)
	LuaToGo(L, -1, &ti.Value)
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
