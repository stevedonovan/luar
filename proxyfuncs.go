package luar

// Those functions are meant to be registered in Lua to manipulate proxies.

import (
	"reflect"

	"github.com/aarzilli/golua/lua"
)

// ArrayToTable defines 'luar.array2table' when 'Init' is called.
func ArrayToTable(L *lua.State) int {
	return CopyArrayToTable(L, reflect.ValueOf(mustUnwrapProxy(L, 1)))
}

// Complex pushes a proxy to a Go complex on the stack.
// It defines 'luar.complex' when 'Init' is called.
func Complex(L *lua.State) int {
	v1, _ := valueOfProxyOrScalar(L, 1)
	v2, _ := valueOfProxyOrScalar(L, 2)
	result := complex(valueToNumber(L, v1), valueToNumber(L, v2))
	makeValueProxy(L, reflect.ValueOf(result), cComplexMeta)
	return 1
}

// ComplexReal defines 'luar.real' when 'Init' is called.
// It is the equivalent of Go's 'real' function.
// WARNING: Deprecated, use the 'real' index instead.
func ComplexReal(L *lua.State) int {
	v := mustUnwrapProxy(L, 1)
	val := reflect.ValueOf(v)
	if unsizedKind(val) != reflect.Complex128 {
		RaiseError(L, "not a complex")
	}
	L.PushNumber(real(val.Complex()))
	return 1
}

// ComplexImag defines 'luar.imag' when 'Init' is called.
// It is the equivalent of Go's 'imag' function.
// WARNING: Deprecated, use the 'imag' index instead.
func ComplexImag(L *lua.State) int {
	v := mustUnwrapProxy(L, 1)
	val := reflect.ValueOf(v)
	if unsizedKind(val) != reflect.Complex128 {
		RaiseError(L, "not a complex")
	}
	L.PushNumber(imag(val.Complex()))
	return 1
}

// MakeChan creates a 'chan interface{}' proxy and pushes it on the stack.
// Init() registers it as 'luar.chan'.
func MakeChan(L *lua.State) int {
	n := L.OptInteger(1, 0)
	ch := make(chan interface{}, n)
	makeValueProxy(L, reflect.ValueOf(ch), cChannelMeta)
	return 1
}

// MakeMap defines 'luar.map' when 'Init' is called.
func MakeMap(L *lua.State) int {
	m := reflect.MakeMap(tmap)
	makeValueProxy(L, m, cMapMeta)
	return 1
}

// MakeSlice defines 'luar.slice' when 'Init' is called.
func MakeSlice(L *lua.State) int {
	n := L.OptInteger(1, 0)
	s := reflect.MakeSlice(tslice, n, n+1)
	makeValueProxy(L, s, cSliceMeta)
	return 1
}

// MapToTable defines 'luar.map2table' when 'Init' is called.
func MapToTable(L *lua.State) int {
	return CopyMapToTable(L, reflect.ValueOf(mustUnwrapProxy(L, 1)))
}

// ProxyMethod pushes the proxy method on the stack.
func ProxyMethod(L *lua.State) int {
	st := mustUnwrapProxy(L, 1)
	if st == nil {
		L.PushNil()
		return 1
	}
	val := reflect.ValueOf(st)
	name := L.ToString(2)
	pushGoMethod(L, name, val)
	return 1
}

// ProxyRaw unproxifies a value.
// It defines 'luar.raw' when 'Init' is called.
func ProxyRaw(L *lua.State) int {
	v := mustUnwrapProxy(L, 1)
	val := reflect.ValueOf(v)
	tp := predeclaredScalarType(val.Type())
	if tp != nil {
		val = val.Convert(tp)
		GoToLua(L, nil, val, false)
	} else {
		L.PushNil()
	}
	return 1
}

// ProxyType defines 'luar.type' when 'Init' is called.
func ProxyType(L *lua.State) int {
	v := mustUnwrapProxy(L, 1)
	if v != nil {
		GoToLua(L, nil, reflect.ValueOf(reflect.TypeOf(v)), false)
	} else {
		L.PushNil()
	}
	return 1
}

// SliceAppend defines 'luar.append' when 'Init' is called.
func SliceAppend(L *lua.State) int {
	slice, _ := valueOfProxy(L, 1)
	val := reflect.ValueOf(LuaToGo(L, nil, 2))
	newslice := reflect.Append(slice, val)
	makeValueProxy(L, newslice, cSliceMeta)
	return 1
}

// SliceSub defines 'luar.sub' when 'Init' is called.
func SliceSub(L *lua.State) int {
	slice, _ := valueOfProxy(L, 1)
	i1, i2 := L.ToInteger(2), L.ToInteger(3)
	newslice := slice.Slice(i1-1, i2)
	makeValueProxy(L, newslice, cSliceMeta)
	return 1
}

// SliceToTable defines 'luar.slice2table' when 'Init' is called.
func SliceToTable(L *lua.State) int {
	return CopySliceToTable(L, reflect.ValueOf(mustUnwrapProxy(L, 1)))
}

// StructToTable defines 'luar.struct2table' when 'Init' is called.
func StructToTable(L *lua.State) int {
	return CopyStructToTable(L, reflect.ValueOf(mustUnwrapProxy(L, 1)))
}

func Unproxify(L *lua.State) int {
	GoToLua(L, nil, reflect.ValueOf(mustUnwrapProxy(L, 1)), true)
	return 1
}
