// Copyright Steve Donovan 2013
//
package luar

import lua "github.com/aarzilli/golua/lua"
import "strings"
import "reflect"
import "unsafe"

// raise a Lua error from Go code
func RaiseError(L *lua.State, msg string) {
	L.Where(1)
	pos := L.ToString(-1)
	L.Pop(1)
	panic(L.NewError(pos + " " + msg))
}

func assertValid(L *lua.State, v reflect.Value, parent reflect.Value, name string, what string) {
	if !v.IsValid() {
		RaiseError(L, "no "+what+" named `"+name+"` for type "+parent.Type().String())
	}
}

// Lua proxy objects for Slices, Maps and Structs
type valueProxy struct {
	value reflect.Value
	t     reflect.Type
}

const (
	cSLICE_META     = "sliceMT"
	cMAP_META       = "mapMT"
	cSTRUCT_META    = "structMT"
	cINTERFACE_META = "interfaceMT"
	cCHANNEL_META   = "ChannelMT"
)

func makeValueProxy(L *lua.State, val reflect.Value, proxyMT string) {
	rawptr := L.NewUserdata(uintptr(unsafe.Sizeof(valueProxy{})))
	ptr := (*valueProxy)(rawptr)
	ptr.value = val
	ptr.t = val.Type()
	L.LGetMetaTable(proxyMT)
	L.SetMetaTable(-2)
}

var valueOf = reflect.ValueOf

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

func proxyType(L *lua.State) int {
	v := unwrapProxy(L, 1)
	if v != nil {
		GoToLua(L, nil, valueOf(reflect.TypeOf(v)), false)
	} else {
		L.PushNil()
	}
	return 1
}

func unwrapProxyValue(L *lua.State, idx int) reflect.Value {
	return valueOf(unwrapProxy(L, idx))
}

func channel_send(L *lua.State) int {
	L.PushValue(2)
	L.PushValue(1)
	L.PushBoolean(true)
	return L.Yield(3)
	//~ ch,t := valueOfProxy(L,1)
	//~ val := valueOf(LuaToGo(L, t.Elem(),2))
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
				val := luaToGoValue(LT, t.Elem(), -3)
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

func MakeChannel(L *lua.State) int {
	ch := make(chan interface{})
	makeValueProxy(L, valueOf(ch), cCHANNEL_META)
	return 1
}

func initializeProxies(L *lua.State) {
	flagValue := func() {
		L.PushBoolean(true)
		L.SetField(-2, "luago.value")
		L.Pop(1)
	}
	L.NewMetaTable(cSLICE_META)
	L.SetMetaMethod("__index", slice__index)
	L.SetMetaMethod("__newindex", slice__newindex)
	L.SetMetaMethod("__len", slicemap__len)
	L.SetMetaMethod("__ipairs", slice__ipairs)
	L.SetMetaMethod("__tostring", proxy__tostring)
	flagValue()
	L.NewMetaTable(cMAP_META)
	L.SetMetaMethod("__index", map__index)
	L.SetMetaMethod("__newindex", map__newindex)
	L.SetMetaMethod("__len", slicemap__len)
	L.SetMetaMethod("__pairs", map__pairs)
	L.SetMetaMethod("__tostring", proxy__tostring)
	flagValue()
	L.NewMetaTable(cSTRUCT_META)
	L.SetMetaMethod("__index", struct__index)
	L.SetMetaMethod("__newindex", struct__newindex)
	L.SetMetaMethod("__tostring", proxy__tostring)
	flagValue()
	L.NewMetaTable(cINTERFACE_META)
	L.SetMetaMethod("__index", interface__index)
	L.SetMetaMethod("__tostring", proxy__tostring)
	flagValue()
	L.NewMetaTable(cCHANNEL_META)
	//~ RegisterFunctions(L,"*",FMap {
	//~ "Send":channel_send,
	//~ "Recv":channel_recv,
	//~ })
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
	L.PushString(obj.Type().String())
	return 1
}

func sliceSub(L *lua.State) int {
	slice, _ := valueOfProxy(L, 1)
	i1, i2 := L.ToInteger(2), L.ToInteger(3)
	newslice := slice.Slice(i1-1, i2)
	makeValueProxy(L, newslice, cSLICE_META)
	return 1
}

func sliceAppend(L *lua.State) int {
	slice, _ := valueOfProxy(L, 1)
	val := luaToGoValue(L, nil, 2)
	newslice := reflect.Append(slice, val)
	makeValueProxy(L, newslice, cSLICE_META)
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
		GoToLua(L, ret.Type(), ret, false)
	} else {
		RaiseError(L, "slice requires integer index")
	}
	return 1
}

func slice__newindex(L *lua.State) int {
	slice, t := valueOfProxy(L, 1)
	idx := L.ToInteger(2)
	val := luaToGoValue(L, t.Elem(), 3)
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
	key := luaToGoValue(L, t.Key(), 2)
	ret := val.MapIndex(key)
	if ret.IsValid() {
		GoToLua(L, ret.Type(), ret, false)
		return 1
	}
	return 0
}

func map__newindex(L *lua.State) int {
	m, t := valueOfProxy(L, 1)
	key := luaToGoValue(L, t.Key(), 2)
	val := luaToGoValue(L, t.Elem(), 3)
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
		GoToLua(L, nil, valueOf(idx+1), false) // report as 1-based index
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
	if !ret.IsValid() { // no such field, try for method?
		callGoMethod(L, name, st)
	} else {
		GoToLua(L, ret.Type(), ret, false)
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
	val := luaToGoValue(L, field.Type(), 3)
	field.Set(val)
	return 0
}

// end of proxy code

var (
	tslice = make([]interface{}, 0)
	tmap   = make(map[string]interface{})
)

// Return the Lua table at 'idx' as a copied Go slice. If 't' is nil then the slice
// type is []interface{}
func CopyTableToSlice(L *lua.State, t reflect.Type, idx int) interface{} {
	if t == nil {
		t = reflect.TypeOf(tslice)
	}
	te := t.Elem()
	n := int(L.ObjLen(idx))
	slice := reflect.MakeSlice(t, n, n)
	for i := 1; i <= n; i++ {
		L.RawGeti(idx, i)
		val := luaToGoValue(L, te, -1)
		slice.Index(i - 1).Set(val)
		L.Pop(1)
	}
	return slice.Interface()
}

// Return the Lua table at 'idx' as a copied Go map. If 't' is nil then the map
// type is map[string]interface{}
func CopyTableToMap(L *lua.State, t reflect.Type, idx int) interface{} {
	if t == nil {
		t = reflect.TypeOf(tmap)
	}
	te, tk := t.Elem(), t.Key()
	m := reflect.MakeMap(t)
	L.PushNil()
	if idx < 0 {
		idx--
	}
	for L.Next(idx) != 0 {
		// key at -2, value at -1
		key := luaToGoValue(L, tk, -2)
		val := luaToGoValue(L, te, -1)
		m.SetMapIndex(key, val)
		L.Pop(1)
	}
	return m.Interface()
}

// Copy matching Lua table entries to a struct, given the struct type
// and the index on the Lua stack.
func CopyTableToStruct(L *lua.State, t reflect.Type, idx int) interface{} {
	was_ptr := t.Kind() == reflect.Ptr
	if was_ptr {
		t = t.Elem()
	}
	s := reflect.New(t) // T -> *T
	ref := s.Elem()
	L.PushNil()
	if idx < 0 {
		idx--
	}
	for L.Next(idx) != 0 {
		key := L.ToString(-2)
		f := ref.FieldByName(key)
		if f.IsValid() {
			val := luaToGoValue(L, f.Type(), -1)
			f.Set(val)
		}
		L.Pop(1)
	}
	if was_ptr {
		return s.Interface()
	}
	return s.Elem().Interface()
}

// Copy a Go slice to a Lua table
func CopySliceToTable(L *lua.State, vslice reflect.Value) int {
	if vslice.IsValid() && vslice.Type().Kind() == reflect.Slice {
		n := vslice.Len()
		L.CreateTable(n, 0)
		for i := 0; i < n; i++ {
			L.PushInteger(int64(i + 1))
			GoToLua(L, nil, vslice.Index(i), true)
			L.SetTable(-3)
		}
		return 1
	} else {
		L.PushNil()
		L.PushString("not a slice!")
	}
	return 2
}

// Copy a Go map to a Lua table
func CopyMapToTable(L *lua.State, vmap reflect.Value) int {
	if vmap.IsValid() && vmap.Type().Kind() == reflect.Map {
		n := vmap.Len()
		L.CreateTable(0, n)
		for _, key := range vmap.MapKeys() {
			val := vmap.MapIndex(key)
			GoToLua(L, nil, key, false)
			GoToLua(L, nil, val, true)
			L.SetTable(-3)
		}
		return 1
	} else {
		L.PushNil()
		L.PushString("not a map!")
	}
	return 2
}

// Push a Go value 'val' of type 't' on the Lua stack.
// If we haven't been given a concrete type, use the type of the value
// and unbox any interfaces.  You can force slices and maps to be copied
// over as tables by setting 'dontproxify' to true.
func GoToLua(L *lua.State, t reflect.Type, val reflect.Value, dontproxify bool) {
	if !val.IsValid() {
		L.PushNil()
		return
	}
	if t == nil {
		t = val.Type()
	}
	if t.Kind() == reflect.Interface && !val.IsNil() { // unbox interfaces!
		val = valueOf(val.Interface())
		t = val.Type()
	}
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	switch t.Kind() {
	case reflect.Float64, reflect.Float32:
		{
			L.PushNumber(val.Float())
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		{
			L.PushNumber(float64(val.Int()))
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		{
			L.PushNumber(float64(val.Uint()))
		}
	case reflect.String:
		{
			L.PushString(val.String())
		}
	case reflect.Bool:
		{
			L.PushBoolean(val.Bool())
		}
	case reflect.Slice:
		{
			if !dontproxify {
				makeValueProxy(L, val, cSLICE_META)
			} else {
				CopySliceToTable(L, val)
			}
		}
	case reflect.Map:
		{
			if !dontproxify {
				makeValueProxy(L, val, cMAP_META)
			} else {
				CopyMapToTable(L, val)
			}
		}
	case reflect.Struct:
		{
			if v, ok := val.Interface().(error); ok {
				L.PushString(v.Error())
			} else if v, ok := val.Interface().(*LuaObject); ok {
				v.Push()
			} else {
				if (val.Kind() == reflect.Ptr || val.Kind() == reflect.Interface) && !val.Elem().IsValid() {
					L.PushNil()
					return
				}
				makeValueProxy(L, val, cSTRUCT_META)
			}
		}
	default:
		{
			if v, ok := val.Interface().(error); ok {
				L.PushString(v.Error())
			} else if val.IsNil() {
				L.PushNil()
			} else {
				makeValueProxy(L, val, cINTERFACE_META)
			}
		}
	}
}

// Convert a Lua value 'idx' on the stack to the Go value of desired type 't'. Handles
// numerical and string types in a straightforward way, and will convert tables to
// either map or slice types.
func LuaToGo(L *lua.State, t reflect.Type, idx int) interface{} {
	return luaToGoValue(L, t, idx).Interface()
}

func luaToGo(L *lua.State, t reflect.Type, idx int) interface{} {
	var value interface{}
	var kind reflect.Kind

	if t == nil { // let the Lua type drive the conversion...
		switch L.Type(idx) {
		case lua.LUA_TNIL:
			return nil // well, d'oh
		case lua.LUA_TBOOLEAN:
			kind = reflect.Bool
		case lua.LUA_TSTRING:
			kind = reflect.String
		case lua.LUA_TTABLE:
			kind = reflect.Interface
		case lua.LUA_TNUMBER:
			kind = reflect.Float64
		default:
			return NewLuaObject(L, idx)
		}
	} else if t.Kind() == reflect.Ptr {
		kind = t.Elem().Kind()
	} else {
		kind = t.Kind()
	}

	switch kind {
	// various numerical types are tedious but straightforward
	case reflect.Float64:
		{
			ptr := new(float64)
			*ptr = L.ToNumber(idx)
			value = *ptr
		}
	case reflect.Float32:
		{
			ptr := new(float32)
			*ptr = float32(L.ToNumber(idx))
			value = *ptr
		}
	case reflect.Int:
		{
			ptr := new(int)
			*ptr = int(L.ToNumber(idx))
			value = *ptr
		}
	case reflect.Int8:
		{
			ptr := new(int8)
			*ptr = int8(L.ToNumber(idx))
			value = *ptr
		}
	case reflect.Int16:
		{
			ptr := new(int16)
			*ptr = int16(L.ToNumber(idx))
			value = *ptr
		}
	case reflect.Int32:
		{
			ptr := new(int32)
			*ptr = int32(L.ToNumber(idx))
			value = *ptr
		}
	case reflect.Int64:
		{
			ptr := new(int64)
			*ptr = int64(L.ToNumber(idx))
			value = *ptr
		}
	case reflect.Uint:
		{
			ptr := new(uint)
			*ptr = uint(L.ToNumber(idx))
			value = *ptr
		}
	case reflect.Uint8:
		{
			ptr := new(uint8)
			*ptr = uint8(L.ToNumber(idx))
			value = *ptr
		}
	case reflect.Uint16:
		{
			ptr := new(uint16)
			*ptr = uint16(L.ToNumber(idx))
			value = *ptr
		}
	case reflect.Uint32:
		{
			ptr := new(uint32)
			*ptr = uint32(L.ToNumber(idx))
			value = *ptr
		}
	case reflect.Uint64:
		{
			ptr := new(uint64)
			*ptr = uint64(L.ToNumber(idx))
			value = *ptr
		}
	case reflect.String:
		{
			tos := L.ToString(idx)
			ptr := new(string)
			*ptr = tos
			value = *ptr
		}
	case reflect.Bool:
		{
			ptr := new(bool)
			*ptr = bool(L.ToBoolean(idx))
			value = *ptr
		}
	case reflect.Slice:
		{
			// if we get a table, then copy its values to a new slice
			if L.IsTable(idx) {
				value = CopyTableToSlice(L, t, idx)
			} else {
				value = unwrapProxy(L, idx)
			}
		}
	case reflect.Map:
		{
			if L.IsTable(idx) {
				value = CopyTableToMap(L, t, idx)
			} else {
				value = unwrapProxy(L, idx)
			}
		}
	case reflect.Struct:
		{
			if L.IsTable(idx) {
				value = CopyTableToStruct(L, t, idx)
			} else {
				value = unwrapProxy(L, idx)
			}
		}
	case reflect.Interface:
		{
			if L.IsTable(idx) {
				// have to make an executive decision here: tables with non-zero
				// length are assumed to be slices!
				if L.ObjLen(idx) > 0 {
					value = CopyTableToSlice(L, nil, idx)
				} else {
					value = CopyTableToMap(L, nil, idx)
				}
			} else if L.IsNumber(idx) {
				value = L.ToNumber(idx)
			} else if L.IsString(idx) {
				value = L.ToString(idx)
			} else if L.IsBoolean(idx) {
				value = L.ToBoolean(idx)
			} else if L.IsNil(idx) {
				return nil
			} else {
				value = unwrapProxy(L, idx)
			}
		}
	default:
		{
			RaiseError(L, "unhandled type "+t.String())
			value = 20
		}

	}
	return value
}

// A wrapper of luaToGo that return reflect.Value
func luaToGoValue(L *lua.State, t reflect.Type, idx int) reflect.Value {
	if t == nil {
		return valueOf(luaToGo(L, nil, idx))
	}
	return valueOf(luaToGo(L, t, idx)).Convert(t)
}

func functionArgRetTypes(funt reflect.Type) (targs, tout []reflect.Type) {
	targs = make([]reflect.Type, funt.NumIn())
	for i := range targs {
		targs[i] = funt.In(i)
	}
	tout = make([]reflect.Type, funt.NumOut())
	for i := range tout {
		tout[i] = funt.Out(i)
	}
	return
}

// elegant little 'cheat' suggested by Kyle Lemons,
// avoiding the 'Call using zero Value argument' problem
// http://play.golang.org/p/TZyOLzu2y-
func valueOfNil(ival interface{}) reflect.Value {
	if ival == nil {
		return valueOf(&ival).Elem()
	} else {
		return valueOf(ival)
	}
}

// GoLuaFunc converts an arbitrary Go function into a Lua-compatible GoFunction.
// There are special optimized cases for functions that go from strings to strings,
// and doubles to doubles, but otherwise Go
// reflection is used to provide a generic wrapper function
func GoLuaFunc(L *lua.State, fun interface{}) lua.LuaGoFunction {
	switch f := fun.(type) {
	case func(*lua.State) int:
		return f
	case func(string) string:
		return func(L *lua.State) int {
			L.PushString(f(L.ToString(1)))
			return 1
		}
	case func(float64) float64:
		return func(L *lua.State) int {
			L.PushNumber(f(L.ToNumber(1)))
			return 1
		}
	default:
	}
	var funv reflect.Value
	switch ff := fun.(type) {
	case reflect.Value:
		funv = ff
	default:
		funv = valueOf(fun)
	}
	funt := funv.Type()
	targs, tout := functionArgRetTypes(funt)
	return func(L *lua.State) int {
		var lastT reflect.Type
		orig_targs := targs
		isVariadic := funt.IsVariadic()
		if isVariadic {
			n := len(targs)
			lastT = targs[n-1].Elem()
			targs = targs[0 : n-1]
		}
		args := make([]reflect.Value, len(targs))
		for i, t := range targs {
			val := LuaToGo(L, t, i+1)
			args[i] = valueOfNil(val)
			//println(i,args[i].String())
		}
		if isVariadic {
			n := L.GetTop()
			for i := len(targs) + 1; i <= n; i++ {
				ival := LuaToGo(L, lastT, i)
				args = append(args, valueOfNil(ival))
			}
			targs = orig_targs
		}
		resv := callGo(L, funv, args)
		for i, val := range resv {
			GoToLua(L, tout[i], val, false)
		}
		return len(resv)
	}
}

func callGo(L *lua.State, funv reflect.Value, args []reflect.Value) []reflect.Value {
	defer func() {
		if x := recover(); x != nil {
			RaiseError(L, "error "+x.(string))
		}
	}()
	resv := funv.Call(args)
	return resv
}

// Useful alias for passing maps of strings to values to luar
type Map map[string]interface{}

func register(L *lua.State, table string, values Map, convertFun bool) {
	pop := true
	if table == "*" {
		pop = false
	} else if len(table) > 0 {
		L.GetGlobal(table)
		if L.IsNil(-1) {
			L.NewTable()
			L.SetGlobal(table)
			L.GetGlobal(table)
		}
	} else {
		L.GetGlobal("_G")
	}
	for name, val := range values {
		t := reflect.TypeOf(val)
		if t.Kind() == reflect.Func {
			if convertFun {
				L.PushGoFunction(GoLuaFunc(L, val))
			} else {
				lf := val.(func(*lua.State) int)
				L.PushGoFunction(lf)
			}
		} else {
			GoToLua(L, t, valueOf(val), false)
		}
		L.SetField(-2, name)
	}
	if pop {
		L.Pop(1)
	}
}

// Make a number of 'raw' Go functions or values available in Lua code. (Raw
// Go functions access the Lua state directly and have signature (L *lua.State) int.)
func RawRegister(L *lua.State, table string, values Map) {
	register(L, table, values, false)
}

// Make a number of Go functions or values available in Lua code. If table is non-nil,
// then create or reuse a global table of that name and put the values
// in it. If table is '*' then assume that the table is already on the stack.
// values is a map of strings to Go values.
func Register(L *lua.State, table string, values Map) {
	register(L, table, values, true)
}

// Encapsulates a Lua object like a table or a function
type LuaObject struct {
	L    *lua.State
	Ref  int
	Type string
}

// Index the Lua object using a string key, returning Go equivalent
func (lo *LuaObject) Get(key string) interface{} {
	lo.Push() // the table
	Lookup(lo.L, key, -1)
	return LuaToGo(lo.L, nil, -1)
}

// Index the Lua object using a string key, returning Lua object
func (lo *LuaObject) GetObject(key string) *LuaObject {
	lo.Push() // the table
	Lookup(lo.L, key, -1)
	return NewLuaObject(lo.L, -1)
}

// Index the Lua object using integer index
func (lo *LuaObject) Geti(idx int64) interface{} {
	L := lo.L
	lo.Push() // the table
	L.PushInteger(idx)
	L.GetTable(-2)
	val := LuaToGo(L, nil, -1)
	L.Pop(1) // the  table
	return val
}

// Set the value at a given idx
func (lo *LuaObject) Set(idx interface{}, val interface{}) interface{} {
	L := lo.L
	lo.Push() // the table
	GoToLua(L, nil, valueOf(idx), false)
	GoToLua(L, nil, valueOf(val), false)
	L.SetTable(-3)
	L.Pop(1) // the  table
	return val
}

// Convenience function for converting a set of values into a corresponding
// slice of their types
func Types(values ...interface{}) []reflect.Type {
	res := make([]reflect.Type, len(values))
	for i, arg := range values {
		res[i] = reflect.TypeOf(arg)
	}
	return res
}

// Call a Lua function, given the desired return types and the arguments.
func (lo *LuaObject) Callf(rtypes []reflect.Type, args ...interface{}) (res []interface{}, err error) {
	L := lo.L
	if rtypes == nil {
		rtypes = []reflect.Type{nil}
	}
	res = make([]interface{}, len(rtypes))
	lo.Push()                  // the function...
	for _, arg := range args { // push the args
		GoToLua(L, nil, valueOf(arg), false)
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

// Call a Lua function, and return a single value, converted in a default way.
func (lo *LuaObject) Call(args ...interface{}) (res interface{}, err error) {
	var sres []interface{}
	sres, err = lo.Callf(nil, args...)
	if err != nil {
		res = nil
		return
	} else {
		return sres[0], nil
	}
}

// Push this Lua object on the stack
func (lo *LuaObject) Push() {
	lo.L.RawGeti(lua.LUA_REGISTRYINDEX, lo.Ref)
}

// free the Lua reference of this object
func (lo *LuaObject) Close() {
	lo.L.Unref(lua.LUA_REGISTRYINDEX, lo.Ref)
}

type LuaTableIter struct {
	lo    *LuaObject
	first bool
	Key   interface{}
	Value interface{}
}

// Create a Lua table iterator
func (lo *LuaObject) Iter() *LuaTableIter {
	return &LuaTableIter{lo, true, nil, nil}
}

// Get next key/value pair from table
//  iter := lo.Iter()
//  keys := []string{}
//  for iter.Next() {
//     keys = append(keys, iter.Key.(string))
//  }
//
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
	} else {
		ti.Key = LuaToGo(L, nil, -2)
		ti.Value = LuaToGo(L, nil, -1)
		L.Pop(1) // drop value, key is now on top
	}
	return true
}

// A new LuaObject from stack index.
func NewLuaObject(L *lua.State, idx int) *LuaObject {
	tp := L.LTypename(idx)
	L.PushValue(idx)
	ref := L.Ref(lua.LUA_REGISTRYINDEX)
	return &LuaObject{L, ref, tp}
}

// A new LuaObject from global qualified name, using Lookup.
func NewLuaObjectFromName(L *lua.State, path string) *LuaObject {
	Lookup(L, path, 0)
	return NewLuaObject(L, -1)
}

// A new LuaObject from a Go value. Note that this _will_ convert any
// slices or maps into Lua tables.
func NewLuaObjectFromValue(L *lua.State, val interface{}) *LuaObject {
	GoToLua(L, nil, valueOf(val), true)
	return NewLuaObject(L, -1)
}

// Look up a Lua value by its full name. If idx is 0, then this name
// is assumed to start in the global table, e.g. "string.gsub".
// With non-zero idx, can be used to look up subfields of a table.
// It terminates with a nil value if we cannot continue the lookup...
func Lookup(L *lua.State, path string, idx int) {
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

func map2table(L *lua.State) int {
	return CopyMapToTable(L, valueOf(unwrapProxy(L, 1)))
}

func slice2table(L *lua.State) int {
	return CopySliceToTable(L, valueOf(unwrapProxy(L, 1)))
}

func makeMap(L *lua.State) int {
	m := reflect.MakeMap(reflect.TypeOf(tmap))
	makeValueProxy(L, m, cMAP_META)
	return 1
}

func makeSlice(L *lua.State) int {
	n := L.OptInteger(1, 0)
	s := reflect.MakeSlice(reflect.TypeOf(tslice), n, n+1)
	makeValueProxy(L, s, cSLICE_META)
	return 1
}

const setup = `
local opairs = pairs
function pairs(t)
    local mt = getmetatable(t)
    if mt and mt.__pairs then
        return mt.__pairs(t)
    else
        return opairs(t)
    end
end
local oipairs = ipairs
function ipairs(t)
    local mt = getmetatable(t)
    if mt and mt.__ipairs then
        return mt.__ipairs(t)
    else
        return oipairs(t)
    end
end
`

// Make and initialize a new Lua state. Populates luar table with five functions:
// map2table, slice2table, map, slice, type, and value.
// This makes customized pairs/ipairs // functions available so that __pairs/__ipairs
// can be used, Lua 5.2 style.
func Init() *lua.State {
	var L = lua.NewState()
	L.OpenLibs()
	initializeProxies(L)
	L.DoString(setup)
	RawRegister(L, "luar", Map{
		"map2table":   map2table,
		"slice2table": slice2table,
		"map":         makeMap,
		"slice":       makeSlice,
		"type":        proxyType,
		"sub":         sliceSub,
		"append":      sliceAppend,
	})
	Register(L, "luar", Map{
		"value": reflect.ValueOf,
	})
	return L
}
