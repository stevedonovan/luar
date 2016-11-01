package luar

import (
	"fmt"
	"reflect"
	"sync"
	"unsafe"

	"github.com/aarzilli/golua/lua"
)

// Lua proxy objects for Go slices, maps and structs
type valueProxy struct {
	value reflect.Value
	t     reflect.Type
}

const (
	cSliceMeta     = "sliceMT"
	cMapMeta       = "mapMT"
	cStructMeta    = "structMT"
	cInterfaceMeta = "interfaceMT"
	cChannelMeta   = "ChannelMT"
)

var proxyMap = map[*valueProxy]reflect.Value{}
var proxymu = &sync.Mutex{}

func isPointerToPrimitive(v reflect.Value) bool {
	return v.Kind() == reflect.Ptr && v.Elem().IsValid() && types[int(v.Elem().Kind())] != nil
}

func makeValueProxy(L *lua.State, val reflect.Value, proxyMT string) {
	rawptr := L.NewUserdata(unsafe.Sizeof(valueProxy{}))
	ptr := (*valueProxy)(rawptr)
	ptr.value = val
	ptr.t = val.Type()
	proxymu.Lock()
	proxyMap[ptr] = val
	proxymu.Unlock()
	L.LGetMetaTable(proxyMT)
	L.SetMetaTable(-2)
}

func proxy__gc(L *lua.State) int {
	vp := (*valueProxy)(L.ToUserdata(1))
	proxymu.Lock()
	delete(proxyMap, vp)
	proxymu.Unlock()
	return 0
}

func valueOfProxy(L *lua.State, idx int) (reflect.Value, reflect.Type) {
	vp := (*valueProxy)(L.ToUserdata(idx))
	return vp.value, vp.t
}

func isValueProxy(L *lua.State, idx int) bool {
	res := false
	if L.IsUserdata(idx) {
		L.GetMetaTable(idx)
		if !L.IsNil(-1) {
			L.GetField(-1, "luago.value")
			res = !L.IsNil(-1)
			L.Pop(1)
		}
		L.Pop(1)
	}
	return res
}

func unwrapProxy(L *lua.State, idx int) interface{} {
	if isValueProxy(L, idx) {
		v, _ := valueOfProxy(L, idx)
		return v.Interface()
	}
	return nil
}

func unwrapProxyOrComplain(L *lua.State, idx int) interface{} {
	if isValueProxy(L, idx) {
		v, _ := valueOfProxy(L, idx)
		return v.Interface()
	}
	RaiseError(L, fmt.Sprintf("arg #%d is not a Go object!", idx))
	return nil
}

func proxyType(L *lua.State) int {
	v := unwrapProxy(L, 1)
	if v != nil {
		GoToLua(L, nil, reflect.ValueOf(reflect.TypeOf(v)), false)
	} else {
		L.PushNil()
	}
	return 1
}

func proxyRaw(L *lua.State) int {
	v := unwrapProxyOrComplain(L, 1)
	val := reflect.ValueOf(v)
	tp := isNewScalarType(val)
	if tp != nil {
		val = val.Convert(tp)
		GoToLua(L, nil, val, false)
	} else {
		L.PushNil()
	}
	return 1
}

func channel_send(L *lua.State) int {
	L.PushValue(2)
	L.PushValue(1)
	L.PushBoolean(true)
	return L.Yield(3)
	//~ ch,t := valueOfProxy(L,1)
	//~ val := reflect.ValueOf(LuaToGo(L, t.Elem(),2))
	//~ ch.Send(val)
	//~ return 0
}

func channel_recv(L *lua.State) int {
	L.PushValue(1)
	L.PushBoolean(false)
	return L.Yield(2)
	//~ ch,t := valueOfProxy(L,1)
	//~ L.Yield(0)
	//~ val,ok := ch.Recv()
	//~ GoToLua(L,t.Elem(),val)
	//~ L.PushBoolean(ok)
	//~ L.Resume(0)
	//~ return 2
}

// TODO: What is this for?
func GoLua(L *lua.State) int {
	go func() {
		LT := L.NewThread()
		L.PushValue(1)
		lua.XMove(L, LT, 1)
		res := LT.Resume(0)
		for res != 0 {
			if res == 2 {
				emsg := LT.ToString(-1)
				RaiseError(LT, emsg)
			}
			ch, t := valueOfProxy(LT, -2)

			if LT.ToBoolean(-1) { // send on a channel
				val := reflect.ValueOf(LuaToGo(LT, t.Elem(), -3))
				ch.Send(val)
				res = LT.Resume(0)
			} else { // receive on a channel
				val, ok := ch.Recv()
				GoToLua(LT, t.Elem(), val, false)
				LT.PushBoolean(ok)
				res = LT.Resume(2)
			}
		}
	}()
	return 0
}

// TODO: What is this for?
func MakeChannel(L *lua.State) int {
	ch := make(chan interface{})
	makeValueProxy(L, reflect.ValueOf(ch), cChannelMeta)
	return 1
}

// InitProxies sets up a Lua state for using Go<->Lua proxies.
// This need not be called if the Lua state was created with Init().
// This function is useful if you want to set up your Lua state manually, e.g.
// with a custom allocator.
func InitProxies(L *lua.State) {
	flagValue := func() {
		L.SetMetaMethod("__tostring", proxy__tostring)
		L.SetMetaMethod("__gc", proxy__gc)
		L.SetMetaMethod("__eq", proxy__eq)
		L.PushBoolean(true)
		L.SetField(-2, "luago.value")
		L.Pop(1)
	}

	L.NewMetaTable(cSliceMeta)
	L.SetMetaMethod("__index", slice__index)
	L.SetMetaMethod("__newindex", slice__newindex)
	L.SetMetaMethod("__len", slicemap__len)
	L.SetMetaMethod("__ipairs", slice__ipairs)
	flagValue()

	L.NewMetaTable(cMapMeta)
	L.SetMetaMethod("__index", map__index)
	L.SetMetaMethod("__newindex", map__newindex)
	L.SetMetaMethod("__len", slicemap__len)
	L.SetMetaMethod("__pairs", map__pairs)
	flagValue()

	L.NewMetaTable(cStructMeta)
	L.SetMetaMethod("__index", struct__index)
	L.SetMetaMethod("__newindex", struct__newindex)
	flagValue()

	L.NewMetaTable(cInterfaceMeta)
	L.SetMetaMethod("__index", interface__index)
	flagValue()

	L.NewMetaTable(cChannelMeta)
	L.NewTable()
	L.PushGoFunction(channel_send)
	L.SetField(-2, "Send")
	L.PushGoFunction(channel_recv)
	L.SetField(-2, "Recv")
	L.SetField(-2, "__index")
	flagValue()
}

func proxy__tostring(L *lua.State) int {
	obj, _ := valueOfProxy(L, 1)
	L.PushString(fmt.Sprintf("%v", obj))
	return 1
}

func proxy__eq(L *lua.State) int {
	v1, t1 := valueOfProxy(L, 1)
	v2, t2 := valueOfProxy(L, 2)
	if t1 != t2 {
		RaiseError(L, fmt.Sprintf("mismatched types %s and %s", t1, t2))
	}
	L.PushBoolean(v1.Interface() == v2.Interface())
	return 1
}

func sliceSub(L *lua.State) int {
	slice, _ := valueOfProxy(L, 1)
	i1, i2 := L.ToInteger(2), L.ToInteger(3)
	newslice := slice.Slice(i1-1, i2)
	makeValueProxy(L, newslice, cSliceMeta)
	return 1
}

func sliceAppend(L *lua.State) int {
	slice, _ := valueOfProxy(L, 1)
	val := reflect.ValueOf(LuaToGo(L, nil, 2))
	newslice := reflect.Append(slice, val)
	makeValueProxy(L, newslice, cSliceMeta)
	return 1
}

func slice__index(L *lua.State) int {
	slice, _ := valueOfProxy(L, 1)
	if L.IsNumber(2) {
		idx := L.ToInteger(2)
		if idx < 1 || idx > slice.Len() {
			RaiseError(L, "slice get: index out of range")
		}
		ret := slice.Index(idx - 1)
		GoToLua(L, nil, ret, false)
	} else {
		RaiseError(L, "slice requires integer index")
	}
	return 1
}

func slice__newindex(L *lua.State) int {
	slice, t := valueOfProxy(L, 1)
	idx := L.ToInteger(2)
	val := reflect.ValueOf(LuaToGo(L, t.Elem(), 3))
	if idx < 1 || idx > slice.Len() {
		RaiseError(L, "slice set: index out of range")
	}
	slice.Index(idx - 1).Set(val)
	return 0
}

func slicemap__len(L *lua.State) int {
	val, _ := valueOfProxy(L, 1)
	L.PushInteger(int64(val.Len()))
	return 1
}

func map__index(L *lua.State) int {
	val, t := valueOfProxy(L, 1)
	key := reflect.ValueOf(LuaToGo(L, t.Key(), 2))
	ret := val.MapIndex(key)
	if ret.IsValid() {
		GoToLua(L, nil, ret, false)
		return 1
	}
	return 0
}

func map__newindex(L *lua.State) int {
	m, t := valueOfProxy(L, 1)
	key := reflect.ValueOf(LuaToGo(L, t.Key(), 2))
	val := reflect.ValueOf(LuaToGo(L, t.Elem(), 3))
	m.SetMapIndex(key, val)
	return 0
}

func map__pairs(L *lua.State) int {
	m, _ := valueOfProxy(L, 1)
	keys := m.MapKeys()
	idx := -1
	n := m.Len()
	iter := func(L *lua.State) int {
		idx++
		if idx == n {
			L.PushNil()
			return 1
		}
		GoToLua(L, nil, keys[idx], false)
		val := m.MapIndex(keys[idx])
		GoToLua(L, nil, val, false)
		return 2
	}
	L.PushGoFunction(iter)
	return 1
}

func slice__ipairs(L *lua.State) int {
	s, _ := valueOfProxy(L, 1)
	n := s.Len()
	idx := -1
	iter := func(L *lua.State) int {
		idx++
		if idx == n {
			L.PushNil()
			return 1
		}
		GoToLua(L, nil, reflect.ValueOf(idx+1), false) // report as 1-based index
		val := s.Index(idx)
		GoToLua(L, nil, val, false)
		return 2
	}
	L.PushGoFunction(iter)
	return 1
}

func callGoMethod(L *lua.State, name string, st reflect.Value) {
	ret := st.MethodByName(name)
	if !ret.IsValid() {
		T := st.Type()
		// Could not resolve this method. Perhaps it's defined on the pointer?
		if T.Kind() != reflect.Ptr {
			if st.CanAddr() { // easy if we can get a pointer directly
				st = st.Addr()
			} else { // otherwise have to create and initialize one...
				VP := reflect.New(T)
				VP.Elem().Set(st)
				st = VP
			}
		}
		ret = st.MethodByName(name)
		assertValid(L, ret, st, name, "method")
	}
	L.PushGoFunction(GoLuaFunc(L, ret))
}

func struct__index(L *lua.State) int {
	st, t := valueOfProxy(L, 1)
	name := L.ToString(2)
	est := st
	if t.Kind() == reflect.Ptr {
		est = st.Elem()
	}
	ret := est.FieldByName(name)
	if !ret.IsValid() || !ret.CanSet() {
		// No such exported field, try for method.
		callGoMethod(L, name, st)
	} else {
		if isPointerToPrimitive(ret) {
			GoToLua(L, nil, ret.Elem(), false)
		} else {
			GoToLua(L, nil, ret, false)
		}
	}
	return 1
}

func interface__index(L *lua.State) int {
	st, _ := valueOfProxy(L, 1)
	name := L.ToString(2)
	callGoMethod(L, name, st)
	return 1
}

func struct__newindex(L *lua.State) int {
	st, t := valueOfProxy(L, 1)
	name := L.ToString(2)
	if t.Kind() == reflect.Ptr {
		st = st.Elem()
	}
	field := st.FieldByName(name)
	assertValid(L, field, st, name, "field")
	val := reflect.ValueOf(LuaToGo(L, field.Type(), 3))
	if isPointerToPrimitive(field) {
		field.Elem().Set(val)
	} else {
		field.Set(val)
	}
	return 0
}
