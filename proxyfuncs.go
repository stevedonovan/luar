package luar

// Those functions are meant to be registered in Lua to manipulate proxies.

import (
	"reflect"

	"github.com/aarzilli/golua/lua"
)

// Complex pushes a proxy to a Go complex on the stack.
//
// Arguments: real (number), imag (number)
//
// Returns: proxy (complex128)
func Complex(L *lua.State) int {
	v1, _ := luaToGoValue(L, 1)
	v2, _ := luaToGoValue(L, 2)
	result := complex(valueToNumber(L, v1), valueToNumber(L, v2))
	makeValueProxy(L, reflect.ValueOf(result), cComplexMeta)
	return 1
}

// MakeChan creates a 'chan interface{}' proxy and pushes it on the stack.
//
// Optional argument: size (number)
//
// Returns: proxy (chan interface{})
func MakeChan(L *lua.State) int {
	n := L.OptInteger(1, 0)
	ch := make(chan interface{}, n)
	makeValueProxy(L, reflect.ValueOf(ch), cChannelMeta)
	return 1
}

// MakeMap creates a 'map[string]interface{}' proxy and pushes it on the stack.
//
// Returns: proxy (map[string]interface{})
func MakeMap(L *lua.State) int {
	m := reflect.MakeMap(tmap)
	makeValueProxy(L, m, cMapMeta)
	return 1
}

// MakeSlice creates a '[]interface{}' proxy and pushes it on the stack.
//
// Optional argument: size (number)
//
// Returns: proxy ([]interface{})
func MakeSlice(L *lua.State) int {
	n := L.OptInteger(1, 0)
	s := reflect.MakeSlice(tslice, n, n+1)
	makeValueProxy(L, s, cSliceMeta)
	return 1
}

func ipairsAux(L *lua.State) int {
	i := L.CheckInteger(2) + 1
	L.PushInteger(int64(i))
	L.PushInteger(int64(i))
	L.GetTable(1)
	if L.Type(-1) == lua.LUA_TNIL {
		return 1
	}
	return 2
}

// ProxyIpairs implements Lua 5.2 'ipairs' functions.
// It respects the __ipairs metamethod.
//
// It is only useful for compatibility with Lua 5.1.
func ProxyIpairs(L *lua.State) int {
	// See Lua >=5.2 source code.
	if L.GetMetaField(1, "__ipairs") {
		L.PushValue(1)
		L.Call(1, 3)
		return 3
	}

	L.CheckType(1, lua.LUA_TTABLE)
	L.PushGoFunction(ipairsAux)
	L.PushValue(1)
	L.PushInteger(0)
	return 3
}

// ProxyMethod pushes the proxy method on the stack.
//
// Argument: proxy
//
// Returns: method (function)
func ProxyMethod(L *lua.State) int {
	if !isValueProxy(L, 1) {
		L.PushNil()
		return 1
	}
	v, _ := valueOfProxy(L, 1)
	name := L.ToString(2)
	pushGoMethod(L, name, v)
	return 1
}

func pairsAux(L *lua.State) int {
	L.CheckType(1, lua.LUA_TTABLE)
	L.SetTop(2) // Create a 2nd argument if there isn't one.
	if L.Next(1) != 0 {
		return 2
	}
	L.PushNil()
	return 1
}

// ProxyPairs implements Lua 5.2 'pairs' functions.
// It respects the __pairs metamethod.
//
// It is only useful for compatibility with Lua 5.1.
func ProxyPairs(L *lua.State) int {
	// See Lua >=5.2 source code.
	if L.GetMetaField(1, "__pairs") {
		L.PushValue(1)
		L.Call(1, 3)
		return 3
	}

	L.CheckType(1, lua.LUA_TTABLE)
	L.PushGoFunction(pairsAux)
	L.PushValue(1)
	L.PushNil()
	return 3
}

// ProxyType pushes the proxy type on the stack.
//
// Argument: proxy
//
// Returns: type (string)
func ProxyType(L *lua.State) int {
	if !isValueProxy(L, 1) {
		L.PushNil()
		return 1
	}
	v, _ := valueOfProxy(L, 1)
	if v.Interface() == nil {
		L.PushNil()
		return 1
	}
	GoToLua(L, nil, reflect.ValueOf(v.Type()), false)
	return 1
}

// Unproxify converts a proxy to an unproxified Lua value.
//
// Argument: proxy
//
// Returns: value (Lua value)
func Unproxify(L *lua.State) int {
	if !isValueProxy(L, 1) {
		L.PushNil()
		return 1
	}
	v, _ := valueOfProxy(L, 1)
	GoToLua(L, nil, v, true)
	return 1
}
