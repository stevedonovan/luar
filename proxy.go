package luar

import (
	"fmt"
	"math"
	"math/cmplx"
	"reflect"
	"strconv"
	"sync"

	"github.com/aarzilli/golua/lua"
)

// Lua proxy objects for Go slices, maps and structs
type valueProxy struct {
	value reflect.Value
	t     reflect.Type
}

const (
	cNumberMeta    = "numberMT"
	cComplexMeta   = "complexMT"
	cStringMeta    = "stringMT"
	cSliceMeta     = "sliceMT"
	cMapMeta       = "mapMT"
	cStructMeta    = "structMT"
	cInterfaceMeta = "interfaceMT"
	cChannelMeta   = "channelMT"
)

var proxyMap = map[*valueProxy]reflect.Value{}
var proxymu = &sync.Mutex{}

func isPointerToPrimitive(v reflect.Value) bool {
	return v.Kind() == reflect.Ptr && v.Elem().IsValid() && types[int(v.Elem().Kind())] != nil
}

func makeValueProxy(L *lua.State, val reflect.Value, proxyMT string) {
	// The metatable needs be set up in the Lua state before the proxy is created,
	// otherwise closing the state will fail on calling the garbage collector. Not
	// really sure why this happens though...
	L.LGetMetaTable(proxyMT)
	if L.IsNil(-1) {
		flagValue := func() {
			L.SetMetaMethod("__tostring", proxy__tostring)
			L.SetMetaMethod("__gc", proxy__gc)
			L.SetMetaMethod("__eq", proxy__eq)
			L.PushBoolean(true)
			L.SetField(-2, "luago.value")
			L.Pop(1)
		}
		switch proxyMT {
		case cNumberMeta:
			L.NewMetaTable(proxyMT)
			L.SetMetaMethod("__index", interface__index)
			L.SetMetaMethod("__lt", number__lt)
			L.SetMetaMethod("__add", number__add)
			L.SetMetaMethod("__sub", number__sub)
			L.SetMetaMethod("__mul", number__mul)
			L.SetMetaMethod("__div", number__div)
			L.SetMetaMethod("__mod", number__mod)
			L.SetMetaMethod("__pow", number__pow)
			L.SetMetaMethod("__unm", number__unm)
			flagValue()
		case cComplexMeta:
			L.NewMetaTable(proxyMT)
			L.SetMetaMethod("__index", complex__index)
			L.SetMetaMethod("__add", number__add)
			L.SetMetaMethod("__sub", number__sub)
			L.SetMetaMethod("__mul", number__mul)
			L.SetMetaMethod("__div", number__div)
			L.SetMetaMethod("__pow", number__pow)
			L.SetMetaMethod("__unm", number__unm)
			flagValue()
		case cStringMeta:
			L.NewMetaTable(proxyMT)
			L.SetMetaMethod("__index", string__index)
			L.SetMetaMethod("__len", string__len)
			L.SetMetaMethod("__lt", string__lt)
			L.SetMetaMethod("__concat", string__concat)
			L.SetMetaMethod("__ipairs", string__ipairs)
			L.SetMetaMethod("__pairs", string__ipairs)
			flagValue()
		case cSliceMeta:
			L.NewMetaTable(proxyMT)
			L.SetMetaMethod("__index", slice__index)
			L.SetMetaMethod("__newindex", slice__newindex)
			L.SetMetaMethod("__len", slicemap__len)
			L.SetMetaMethod("__ipairs", slice__ipairs)
			L.SetMetaMethod("__pairs", slice__ipairs)
			flagValue()
		case cMapMeta:
			L.NewMetaTable(proxyMT)
			L.SetMetaMethod("__index", map__index)
			L.SetMetaMethod("__newindex", map__newindex)
			L.SetMetaMethod("__len", slicemap__len)
			L.SetMetaMethod("__ipairs", map__ipairs)
			L.SetMetaMethod("__pairs", map__pairs)
			flagValue()
		case cStructMeta:
			L.NewMetaTable(proxyMT)
			L.SetMetaMethod("__index", struct__index)
			L.SetMetaMethod("__newindex", struct__newindex)
			flagValue()
		case cInterfaceMeta:
			L.NewMetaTable(proxyMT)
			L.SetMetaMethod("__index", interface__index)
			flagValue()
		case cChannelMeta:
			L.NewMetaTable(proxyMT)
			L.SetMetaMethod("__index", channel__index)
			flagValue()
		}
	}
	L.Pop(1)
	rawptr := L.NewUserdata(typeof((*valueProxy)(nil)).Size())
	ptr := (*valueProxy)(rawptr)
	ptr.value = val
	ptr.t = val.Type()
	proxymu.Lock()
	proxyMap[ptr] = val
	proxymu.Unlock()
	L.LGetMetaTable(proxyMT)
	L.SetMetaTable(-2)
}

func valueOfProxy(L *lua.State, idx int) (reflect.Value, reflect.Type) {
	vp := (*valueProxy)(L.ToUserdata(idx))
	return vp.value, vp.t
}

func luaToGoValue(L *lua.State, idx int) (reflect.Value, reflect.Type) {
	v := LuaToGo(L, nil, idx)
	return reflect.ValueOf(v), reflect.TypeOf(v)
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

func mustUnwrapProxy(L *lua.State, idx int) interface{} {
	if !isValueProxy(L, idx) {
		RaiseError(L, fmt.Sprintf("arg #%d is not a Go object", idx))
	}
	v, _ := valueOfProxy(L, idx)
	return v.Interface()
}

func pushGoMethod(L *lua.State, name string, st reflect.Value) {
	method := st.MethodByName(name)
	if !method.IsValid() {
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
		method = st.MethodByName(name)
		assertValid(L, method, st, name, "method")
	}
	GoToLua(L, nil, method, true)
}

// From Lua's specs: "A metamethod only is selected when both objects being
// compared have the same type and the same metamethod for the selected
// operation." Thus both arguments must be proxies for this function to be
// called. No need to check for type equality: Go's "==" operator will do it for
// us.
func proxy__eq(L *lua.State) int {
	v1 := LuaToGo(L, nil, 1)
	v2 := LuaToGo(L, nil, 2)
	L.PushBoolean(v1 == v2)
	return 1
}

func proxy__gc(L *lua.State) int {
	vp := (*valueProxy)(L.ToUserdata(1))
	proxymu.Lock()
	delete(proxyMap, vp)
	proxymu.Unlock()
	return 0
}

func proxy__tostring(L *lua.State) int {
	v, _ := valueOfProxy(L, 1)
	L.PushString(fmt.Sprintf("%v", v))
	return 1
}

func slice__index(L *lua.State) int {
	v, _ := valueOfProxy(L, 1)
	for v.Kind() == reflect.Ptr {
		// For arrays.
		v = v.Elem()
	}
	if L.IsNumber(2) {
		idx := L.ToInteger(2)
		if idx < 1 || idx > v.Len() {
			RaiseError(L, "slice/array get: index out of range")
		}
		v := v.Index(idx - 1)
		GoToLua(L, nil, v, false)
	} else if L.IsString(2) {
		name := L.ToString(2)
		if v.Kind() == reflect.Array {
			pushGoMethod(L, name, v)
			return 1
		}
		switch name {
		case "append":
			f := func(L *lua.State) int {
				narg := L.GetTop()
				args := []reflect.Value{}
				for i := 1; i <= narg; i++ {
					elem := reflect.ValueOf(LuaToGo(L, v.Type().Elem(), i))
					args = append(args, elem)
				}
				newslice := reflect.Append(v, args...)
				makeValueProxy(L, newslice, cSliceMeta)
				return 1
			}
			L.PushGoFunction(f)
		case "cap":
			L.PushInteger(int64(v.Cap()))
		case "sub":
			f := func(L *lua.State) int {
				i1, i2 := L.ToInteger(1), L.ToInteger(2)
				newslice := v.Slice(i1-1, i2)
				makeValueProxy(L, newslice, cSliceMeta)
				return 1
			}
			L.PushGoFunction(f)
		default:
			pushGoMethod(L, name, v)
		}
	} else {
		RaiseError(L, "slice/array requires integer index")
	}
	return 1
}

func slice__newindex(L *lua.State) int {
	v, t := valueOfProxy(L, 1)
	for v.Kind() == reflect.Ptr {
		// For arrays.
		v = v.Elem()
		t = t.Elem()
	}
	idx := L.ToInteger(2)
	val := reflect.ValueOf(LuaToGo(L, t.Elem(), 3))
	if idx < 1 || idx > v.Len() {
		RaiseError(L, "slice/array set: index out of range")
	}
	v.Index(idx - 1).Set(val)
	return 0
}

func slicemap__len(L *lua.State) int {
	v, _ := valueOfProxy(L, 1)
	for v.Kind() == reflect.Ptr {
		// For arrays.
		v = v.Elem()
	}
	L.PushInteger(int64(v.Len()))
	return 1
}

func map__index(L *lua.State) int {
	v, t := valueOfProxy(L, 1)
	key := reflect.ValueOf(LuaToGo(L, t.Key(), 2))
	val := v.MapIndex(key)
	if val.IsValid() {
		GoToLua(L, nil, val, false)
		return 1
	} else if key.Kind() == reflect.String {
		name := key.String()

		// From 'pushGoMethod':
		val := v.MethodByName(name)
		if !val.IsValid() {
			T := v.Type()
			// Could not resolve this method. Perhaps it's defined on the pointer?
			if T.Kind() != reflect.Ptr {
				if v.CanAddr() { // easy if we can get a pointer directly
					v = v.Addr()
				} else { // otherwise have to create and initialize one...
					vp := reflect.New(T)
					vp.Elem().Set(v)
					v = vp
				}
			}
			val = v.MethodByName(name)
			// Unlike 'pushGoMethod', do not panic.
			if !val.IsValid() {
				L.PushNil()
				return 1
			}
		}
		GoToLua(L, nil, val, true)
		return 1
	}
	return 0
}

func map__newindex(L *lua.State) int {
	v, t := valueOfProxy(L, 1)
	key := reflect.ValueOf(LuaToGo(L, t.Key(), 2))
	val := reflect.ValueOf(LuaToGo(L, t.Elem(), 3))
	v.SetMapIndex(key, val)
	return 0
}

func map__pairs(L *lua.State) int {
	v, _ := valueOfProxy(L, 1)
	keys := v.MapKeys()
	idx := -1
	n := v.Len()
	iter := func(L *lua.State) int {
		idx++
		if idx == n {
			L.PushNil()
			return 1
		}
		GoToLua(L, nil, keys[idx], false)
		val := v.MapIndex(keys[idx])
		GoToLua(L, nil, val, false)
		return 2
	}
	L.PushGoFunction(iter)
	return 1
}

func map__ipairs(L *lua.State) int {
	v, _ := valueOfProxy(L, 1)
	keys := v.MapKeys()
	intKeys := map[uint64]reflect.Value{}

	// Filter integer keys.
	for _, k := range keys {
		if k.Kind() == reflect.Interface {
			k = k.Elem()
		}
		switch unsizedKind(k) {
		case reflect.Int64:
			i := k.Int()
			if i > 0 {
				intKeys[uint64(i)] = k
			}
		case reflect.Uint64:
			intKeys[k.Uint()] = k
		}
	}

	idx := uint64(0)
	iter := func(L *lua.State) int {
		idx++
		if _, ok := intKeys[idx]; !ok {
			L.PushNil()
			return 1
		}
		GoToLua(L, nil, reflect.ValueOf(idx), false)
		val := v.MapIndex(intKeys[idx])
		GoToLua(L, nil, val, false)
		return 2
	}
	L.PushGoFunction(iter)
	return 1
}

func slice__ipairs(L *lua.State) int {
	v, _ := valueOfProxy(L, 1)
	for v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	n := v.Len()
	idx := -1
	iter := func(L *lua.State) int {
		idx++
		if idx == n {
			L.PushNil()
			return 1
		}
		GoToLua(L, nil, reflect.ValueOf(idx+1), false) // report as 1-based index
		val := v.Index(idx)
		GoToLua(L, nil, val, false)
		return 2
	}
	L.PushGoFunction(iter)
	return 1
}

func struct__index(L *lua.State) int {
	v, t := valueOfProxy(L, 1)
	name := L.ToString(2)
	vp := v
	if t.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	field := v.FieldByName(name)
	if !field.IsValid() || !field.CanSet() {
		// No such exported field, try for method.
		pushGoMethod(L, name, vp)
	} else {
		if isPointerToPrimitive(field) {
			GoToLua(L, nil, field.Elem(), false)
		} else {
			GoToLua(L, nil, field, false)
		}
	}
	return 1
}

func interface__index(L *lua.State) int {
	v, _ := valueOfProxy(L, 1)
	name := L.ToString(2)
	pushGoMethod(L, name, v)
	return 1
}

func struct__newindex(L *lua.State) int {
	v, t := valueOfProxy(L, 1)
	name := L.ToString(2)
	if t.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	field := v.FieldByName(name)
	assertValid(L, field, v, name, "field")
	val := reflect.ValueOf(LuaToGo(L, field.Type(), 3))
	if isPointerToPrimitive(field) {
		field.Elem().Set(val)
	} else {
		field.Set(val)
	}
	return 0
}

func isPredeclaredType(t reflect.Type) bool {
	return t == reflect.TypeOf(0.0) || t == reflect.TypeOf("")
}

// pushNumberValue pushes the number resulting from an arithmetic operation.
//
// At least one operand must be a proxy for this function to be called. See the
// main documentation for the conversion rules.
func pushNumberValue(L *lua.State, i interface{}, t1, t2 reflect.Type) {
	v := reflect.ValueOf(i)
	isComplex := unsizedKind(v) == reflect.Complex128
	mt := cNumberMeta
	if isComplex {
		mt = cComplexMeta
	}
	if t1 == t2 || isPredeclaredType(t2) {
		makeValueProxy(L, v.Convert(t1), mt)
	} else if isPredeclaredType(t1) {
		makeValueProxy(L, v.Convert(t2), mt)
	} else if isComplex {
		complexType := reflect.TypeOf(0i)
		makeValueProxy(L, v.Convert(complexType), cComplexMeta)
	} else {
		L.PushNumber(valueToNumber(L, v))
	}
}

// Shorthand for kind-switches.
func unsizedKind(v reflect.Value) reflect.Kind {
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return reflect.Int64
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return reflect.Uint64
	case reflect.Float64, reflect.Float32:
		return reflect.Float64
	case reflect.Complex128, reflect.Complex64:
		return reflect.Complex128
	}
	return v.Kind()
}

func valueToNumber(L *lua.State, v reflect.Value) float64 {
	switch unsizedKind(v) {
	case reflect.Int64:
		return float64(v.Int())
	case reflect.Uint64:
		return float64(v.Uint())
	case reflect.Float64:
		return v.Float()
	case reflect.String:
		if f, err := strconv.ParseFloat(v.String(), 64); err == nil {
			return f
		}
		RaiseError(L, "cannot convert to number")
	}
	RaiseError(L, "cannot convert to number")
	return 0
}

func valueToComplex(L *lua.State, v reflect.Value) complex128 {
	if unsizedKind(v) == reflect.Complex128 {
		return v.Complex()
	}
	return complex(valueToNumber(L, v), 0)
}

func valueToString(L *lua.State, v reflect.Value) string {
	switch unsizedKind(v) {
	case reflect.Uint64, reflect.Int64, reflect.Float64:
		return fmt.Sprintf("%v", valueToNumber(L, v))
	case reflect.String:
		return v.String()
	}

	RaiseError(L, "cannot convert to string")
	return ""
}

// commonKind returns the kind to which v1 and v2 can be converted with the
// least information loss.
func commonKind(v1, v2 reflect.Value) reflect.Kind {
	k1 := unsizedKind(v1)
	k2 := unsizedKind(v2)
	if k1 == k2 && (k1 == reflect.Uint64 || k1 == reflect.Int64) {
		return k1
	}
	if k1 == reflect.Complex128 || k2 == reflect.Complex128 {
		return reflect.Complex128
	}
	return reflect.Float64
}

func number__lt(L *lua.State) int {
	v1, _ := luaToGoValue(L, 1)
	v2, _ := luaToGoValue(L, 2)
	switch commonKind(v1, v2) {
	case reflect.Uint64:
		L.PushBoolean(v1.Uint() < v2.Uint())
	case reflect.Int64:
		L.PushBoolean(v1.Int() < v2.Int())
	case reflect.Float64:
		L.PushBoolean(valueToNumber(L, v1) < valueToNumber(L, v2))
	}
	return 1
}

func number__add(L *lua.State) int {
	v1, t1 := luaToGoValue(L, 1)
	v2, t2 := luaToGoValue(L, 2)
	var result interface{}
	switch commonKind(v1, v2) {
	case reflect.Uint64:
		result = v1.Uint() + v2.Uint()
	case reflect.Int64:
		result = v1.Int() + v2.Int()
	case reflect.Float64:
		result = valueToNumber(L, v1) + valueToNumber(L, v2)
	case reflect.Complex128:
		result = valueToComplex(L, v1) + valueToComplex(L, v2)
	}
	pushNumberValue(L, result, t1, t2)
	return 1
}

func number__sub(L *lua.State) int {
	v1, t1 := luaToGoValue(L, 1)
	v2, t2 := luaToGoValue(L, 2)
	var result interface{}
	switch commonKind(v1, v2) {
	case reflect.Uint64:
		result = v1.Uint() - v2.Uint()
	case reflect.Int64:
		result = v1.Int() - v2.Int()
	case reflect.Float64:
		result = valueToNumber(L, v1) - valueToNumber(L, v2)
	case reflect.Complex128:
		result = valueToComplex(L, v1) - valueToComplex(L, v2)
	}
	pushNumberValue(L, result, t1, t2)
	return 1
}

func number__mul(L *lua.State) int {
	v1, t1 := luaToGoValue(L, 1)
	v2, t2 := luaToGoValue(L, 2)
	var result interface{}
	switch commonKind(v1, v2) {
	case reflect.Uint64:
		result = v1.Uint() * v2.Uint()
	case reflect.Int64:
		result = v1.Int() * v2.Int()
	case reflect.Float64:
		result = valueToNumber(L, v1) * valueToNumber(L, v2)
	case reflect.Complex128:
		result = valueToComplex(L, v1) * valueToComplex(L, v2)
	}
	pushNumberValue(L, result, t1, t2)
	return 1
}

func number__div(L *lua.State) int {
	v1, t1 := luaToGoValue(L, 1)
	v2, t2 := luaToGoValue(L, 2)
	var result interface{}
	switch commonKind(v1, v2) {
	case reflect.Uint64:
		result = v1.Uint() / v2.Uint()
	case reflect.Int64:
		result = v1.Int() / v2.Int()
	case reflect.Float64:
		result = valueToNumber(L, v1) / valueToNumber(L, v2)
	case reflect.Complex128:
		result = valueToComplex(L, v1) / valueToComplex(L, v2)
	}
	pushNumberValue(L, result, t1, t2)
	return 1
}

func number__mod(L *lua.State) int {
	v1, t1 := luaToGoValue(L, 1)
	v2, t2 := luaToGoValue(L, 2)
	var result interface{}
	switch commonKind(v1, v2) {
	case reflect.Uint64:
		result = v1.Uint() % v2.Uint()
	case reflect.Int64:
		result = v1.Int() % v2.Int()
	case reflect.Float64:
		result = math.Mod(valueToNumber(L, v1), valueToNumber(L, v2))
	}
	pushNumberValue(L, result, t1, t2)
	return 1
}

func number__pow(L *lua.State) int {
	v1, t1 := luaToGoValue(L, 1)
	v2, t2 := luaToGoValue(L, 2)
	var result interface{}
	switch commonKind(v1, v2) {
	case reflect.Uint64:
		result = math.Pow(float64(v1.Uint()), float64(v2.Uint()))
	case reflect.Int64:
		result = math.Pow(float64(v1.Int()), float64(v2.Int()))
	case reflect.Float64:
		result = math.Pow(valueToNumber(L, v1), valueToNumber(L, v2))
	case reflect.Complex128:
		result = cmplx.Pow(valueToComplex(L, v1), valueToComplex(L, v2))
	}
	pushNumberValue(L, result, t1, t2)
	return 1
}

func number__unm(L *lua.State) int {
	v1, t1 := luaToGoValue(L, 1)
	var result interface{}
	switch unsizedKind(v1) {
	case reflect.Uint64:
		result = -v1.Uint()
	case reflect.Int64:
		result = -v1.Int()
	case reflect.Float64, reflect.String:
		result = -valueToNumber(L, v1)
	case reflect.Complex128:
		result = -v1.Complex()
	}
	v := reflect.ValueOf(result)
	if unsizedKind(v1) == reflect.Complex128 {
		makeValueProxy(L, v.Convert(t1), cComplexMeta)
	} else if predeclaredScalarType(t1) != nil {
		makeValueProxy(L, v.Convert(t1), cNumberMeta)
	} else {
		L.PushNumber(v.Float())
	}
	return 1
}

func string__len(L *lua.State) int {
	v1, _ := luaToGoValue(L, 1)
	L.PushInteger(int64(v1.Len()))
	return 1
}

func string__lt(L *lua.State) int {
	v1, _ := luaToGoValue(L, 1)
	v2, _ := luaToGoValue(L, 2)
	L.PushBoolean(v1.String() < v2.String())
	return 1
}

// Lua accepts concatenation with string and number.
func string__concat(L *lua.State) int {
	v1, t1 := luaToGoValue(L, 1)
	v2, t2 := luaToGoValue(L, 2)
	s1 := valueToString(L, v1)
	s2 := valueToString(L, v2)
	result := s1 + s2

	if t1 == t2 || isPredeclaredType(t2) {
		v := reflect.ValueOf(result)
		makeValueProxy(L, v.Convert(t1), cStringMeta)
	} else if isPredeclaredType(t1) {
		v := reflect.ValueOf(result)
		makeValueProxy(L, v.Convert(t2), cStringMeta)
	} else {
		L.PushString(result)
	}

	return 1
}

func complex__index(L *lua.State) int {
	v, _ := valueOfProxy(L, 1)
	name := L.ToString(2)
	switch name {
	case "real":
		L.PushNumber(real(v.Complex()))
	case "imag":
		L.PushNumber(imag(v.Complex()))
	default:
		pushGoMethod(L, name, v)
	}
	return 1
}

func channel__index(L *lua.State) int {
	v, t := valueOfProxy(L, 1)
	name := L.ToString(2)
	switch name {
	case "recv":
		f := func(L *lua.State) int {
			val, ok := v.Recv()
			if ok {
				GoToLua(L, nil, val, false)
			} else {
				L.PushNil()
			}
			return 1
		}
		L.PushGoFunction(f)
	case "send":
		f := func(L *lua.State) int {
			val := reflect.ValueOf(LuaToGo(L, t.Elem(), 1))
			v.Send(val)
			return 0
		}
		L.PushGoFunction(f)
	case "close":
		f := func(L *lua.State) int {
			v.Close()
			return 0
		}
		L.PushGoFunction(f)
	default:
		pushGoMethod(L, name, v)
	}
	return 1
}

func string__index(L *lua.State) int {
	v, _ := valueOfProxy(L, 1)
	name := L.ToString(2)
	if name == "sub" {
		f := func(L *lua.State) int {
			i1, i2 := L.ToInteger(1), L.ToInteger(2)
			vn := v.Slice(i1-1, i2)
			makeValueProxy(L, vn, cStringMeta)
			return 1
		}
		L.PushGoFunction(f)

	} else {
		pushGoMethod(L, name, v)
	}
	return 1
}

func string__ipairs(L *lua.State) int {
	v, _ := valueOfProxy(L, 1)
	for v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	r := []rune(v.String())
	n := len(r)
	idx := -1
	iter := func(L *lua.State) int {
		idx++
		if idx == n {
			L.PushNil()
			return 1
		}
		GoToLua(L, nil, reflect.ValueOf(idx+1), false) // report as 1-based index
		GoToLua(L, nil, reflect.ValueOf(string(r[idx])), false)
		return 2
	}
	L.PushGoFunction(iter)
	return 1
}
