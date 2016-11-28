// Copyright (c) 2010-2016 Steve Donovan

package luar

import (
	"fmt"
	"reflect"

	"github.com/aarzilli/golua/lua"
)

// NullT is the type of 'luar.null'.
type NullT int

// Map is an alias for passing maps of strings to values to luar.
type Map map[string]interface{}

var (
	// Null is the definition of 'luar.null' which is used in place of 'nil' when
	// converting slices and structs.
	Null = NullT(0)
)

var (
	tslice = typeof((*[]interface{})(nil))
	tmap   = typeof((*map[string]interface{})(nil))
	nullv  = reflect.ValueOf(Null)
)

// visitor holds the index to the table in LUA_REGISTRYINDEX with all the tables
// we ran across during a GoToLua conversion.
type visitor struct {
	L     *lua.State
	index int
}

func newVisitor(L *lua.State) visitor {
	var v visitor
	v.L = L
	v.L.NewTable()
	v.index = v.L.Ref(lua.LUA_REGISTRYINDEX)
	return v
}

func (v *visitor) close() {
	v.L.Unref(lua.LUA_REGISTRYINDEX, v.index)
}

// Mark value on top of the stack as visited using the registry index.
func (v *visitor) mark(val reflect.Value) {
	ptr := val.Pointer()
	v.L.RawGeti(lua.LUA_REGISTRYINDEX, v.index)
	// Copy value on top.
	v.L.PushValue(-2)
	// Set value to table.
	v.L.RawSeti(-2, int(ptr))
	v.L.Pop(1)
}

// Push visited value on top of the stack.
// If the value was not visited, return false and push nothing.
func (v *visitor) push(val reflect.Value) bool {
	ptr := val.Pointer()
	v.L.RawGeti(lua.LUA_REGISTRYINDEX, v.index)
	v.L.RawGeti(-1, int(ptr))
	if v.L.IsNil(-1) {
		// Not visited.
		v.L.Pop(2)
		return false
	}
	v.L.Replace(-2)
	return true
}

// Init makes and initialize a new pre-configured Lua state.
//
// It populates the 'luar' table with some helper functions/values:
//
//   method: ProxyMethod
//   type: ProxyType
//   unproxify: Unproxify
//
//   chan: MakeChan
//   complex: MakeComplex
//   map: MakeMap
//   slice: MakeSlice
//
//   null: Null
//
// It replaces the pairs/ipairs functions so that __pairs/__ipairs can be used,
// Lua 5.2 style. It allows for looping over Go composite types and strings.
//
// It is not required for using the 'GoToLua' and 'LuaToGo' functions.
func Init() *lua.State {
	var L = lua.NewState()
	L.OpenLibs()
	Register(L, "luar", Map{
		// Functions.
		"unproxify": Unproxify,

		"method": ProxyMethod,
		"type":   ProxyType, // TODO: Replace with the version from the 'proxytype' branch.

		"chan":    MakeChan,
		"complex": Complex,
		"map":     MakeMap,
		"slice":   MakeSlice,

		// Values.
		"null": Null,
	})
	Register(L, "", Map{
		"ipairs": ProxyIpairs,
		"pairs":  ProxyPairs,
	})
	return L
}

func isNil(v reflect.Value) bool {
	nullables := [...]bool{
		reflect.Chan:      true,
		reflect.Func:      true,
		reflect.Interface: true,
		reflect.Map:       true,
		reflect.Ptr:       true,
		reflect.Slice:     true,
	}

	kind := v.Type().Kind()
	if int(kind) >= len(nullables) {
		return false
	}
	return nullables[kind] && v.IsNil()
}

func copyMapToTable(L *lua.State, vmap reflect.Value, visited visitor) {
	n := vmap.Len()
	L.CreateTable(0, n)
	visited.mark(vmap)
	for _, key := range vmap.MapKeys() {
		v := vmap.MapIndex(key)
		goToLua(L, key, true, visited)
		if isNil(v) {
			v = nullv
		}
		goToLua(L, v, false, visited)
		L.SetTable(-3)
	}
}

// Also for arrays.
func copySliceToTable(L *lua.State, vslice reflect.Value, visited visitor) {
	ref := vslice
	for vslice.Kind() == reflect.Ptr {
		// For arrays.
		vslice = vslice.Elem()
	}

	n := vslice.Len()
	L.CreateTable(n, 0)
	if vslice.Kind() == reflect.Slice {
		visited.mark(vslice)
	} else if ref.Kind() == reflect.Ptr {
		visited.mark(ref)
	}

	for i := 0; i < n; i++ {
		L.PushInteger(int64(i + 1))
		v := vslice.Index(i)
		if isNil(v) {
			v = nullv
		}
		goToLua(L, v, false, visited)
		L.SetTable(-3)
	}
}

func copyStructToTable(L *lua.State, vstruct reflect.Value, visited visitor) {
	// If 'vstruct' is a pointer to struct, use the pointer to mark as visited.
	ref := vstruct
	for vstruct.Kind() == reflect.Ptr {
		vstruct = vstruct.Elem()
	}

	n := vstruct.NumField()
	L.CreateTable(n, 0)
	if ref.Kind() == reflect.Ptr {
		visited.mark(ref)
	}

	for i := 0; i < n; i++ {
		st := vstruct.Type()
		field := st.Field(i)
		key := field.Name
		tag := field.Tag.Get("lua")
		if tag != "" {
			key = tag
		}
		goToLua(L, key, false, visited)
		v := vstruct.Field(i)
		goToLua(L, v, false, visited)
		L.SetTable(-3)
	}
}

func callGo(L *lua.State, funv reflect.Value, args []reflect.Value) []reflect.Value {
	defer func() {
		if x := recover(); x != nil {
			RaiseError(L, "error %s", x)
		}
	}()
	resv := funv.Call(args)
	return resv
}

// Elegant little 'cheat' suggested by Kyle Lemons, avoiding the 'Call using
// zero Value argument' problem.
// See http://play.golang.org/p/TZyOLzu2y-.
func valueOfNil(ival interface{}) reflect.Value {
	if ival == nil {
		return reflect.ValueOf(&ival).Elem()
	}
	return reflect.ValueOf(ival)
}

func goLuaFunc(L *lua.State, fun reflect.Value) lua.LuaGoFunction {
	switch f := fun.Interface().(type) {
	case func(*lua.State) int:
		return f
	}

	funT := fun.Type()
	tArgs := make([]reflect.Type, funT.NumIn())
	for i := range tArgs {
		tArgs[i] = funT.In(i)
	}

	return func(L *lua.State) int {
		var lastT reflect.Type
		origTArgs := tArgs
		isVariadic := funT.IsVariadic()

		if isVariadic {
			n := len(tArgs)
			lastT = tArgs[n-1].Elem()
			tArgs = tArgs[0 : n-1]
		}

		args := make([]reflect.Value, len(tArgs))
		for i, t := range tArgs {
			val := LuaToGo(L, t, i+1)
			args[i] = valueOfNil(val)
		}

		if isVariadic {
			n := L.GetTop()
			for i := len(tArgs) + 1; i <= n; i++ {
				iVal := LuaToGo(L, lastT, i)
				args = append(args, valueOfNil(iVal))
			}
			tArgs = origTArgs
		}
		resV := callGo(L, fun, args)
		for _, val := range resV {
			if val.Kind() == reflect.Struct {
				// If the function returns a struct (and not a pointer to a struct),
				// calling GoToLua directly will convert it to a table, making the
				// mathods inaccessible. We work around that issue by forcibly passing a
				// pointer to a struct.
				n := reflect.New(val.Type())
				n.Elem().Set(val)
				val = n
			}
			GoToLuaProxy(L, val)
		}
		return len(resV)
	}
}

// GoToLua pushes a Go value 'val' on the Lua stack.
//
// It unboxes interfaces.
//
// Pointers are followed recursively. Slices, structs and maps are copied over as tables.
func GoToLua(L *lua.State, val interface{}) {
	v := newVisitor(L)
	goToLua(L, val, false, v)
	v.close()
}

// GoToLuaProxy is like GoToLua but pushes a proxy on the Lua stack when it makes sense.
//
// A proxy is a Lua userdata that wraps a Go value.
//
// Pointers are preserved.
//
// Structs and arrays need to be passed as pointers to be proxified, otherwise
// they will be copied as tables.
//
// Predeclared scalar types are never proxified as they have no methods.
func GoToLuaProxy(L *lua.State, val interface{}) {
	v := newVisitor(L)
	goToLua(L, val, true, v)
	v.close()
}

// TODO: Check if we really need multiple pointer levels since pointer methods
// can be called on non-pointers.
func goToLua(L *lua.State, v interface{}, proxify bool, visited visitor) {
	var val reflect.Value
	val, ok := v.(reflect.Value)
	if !ok {
		val = reflect.ValueOf(v)
	}
	if !val.IsValid() {
		L.PushNil()
		return
	}

	// Unbox interface.
	if val.Kind() == reflect.Interface && !val.IsNil() {
		val = reflect.ValueOf(val.Interface())
	}

	// Follow pointers if not proxifying. We save the original pointer Value in case we proxify.
	ptrVal := val
	for val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	if !val.IsValid() {
		L.PushNil()
		return
	}

	// As a special case, we always proxify nullv, the empty element for slices and maps.
	if val.CanInterface() && val.Interface() == nullv.Interface() {
		makeValueProxy(L, val, cInterfaceMeta)
		return
	}

	switch val.Kind() {
	case reflect.Float64, reflect.Float32:
		if proxify && isNewType(val.Type()) {
			makeValueProxy(L, ptrVal, cNumberMeta)
		} else {
			L.PushNumber(val.Float())
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if proxify && isNewType(val.Type()) {
			makeValueProxy(L, ptrVal, cNumberMeta)
		} else {
			L.PushNumber(float64(val.Int()))
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if proxify && isNewType(val.Type()) {
			makeValueProxy(L, ptrVal, cNumberMeta)
		} else {
			L.PushNumber(float64(val.Uint()))
		}
	case reflect.String:
		if proxify && isNewType(val.Type()) {
			makeValueProxy(L, ptrVal, cStringMeta)
		} else {
			L.PushString(val.String())
		}
	case reflect.Bool:
		if proxify && isNewType(val.Type()) {
			makeValueProxy(L, ptrVal, cInterfaceMeta)
		} else {
			L.PushBoolean(val.Bool())
		}
	case reflect.Complex128, reflect.Complex64:
		makeValueProxy(L, ptrVal, cComplexMeta)
	case reflect.Array:
		// It needs be a pointer to be a proxy, otherwise values won't be settable.
		if proxify && ptrVal.Kind() == reflect.Ptr {
			makeValueProxy(L, ptrVal, cSliceMeta)
		} else {
			// See the case of struct.
			if ptrVal.Kind() == reflect.Ptr && visited.push(ptrVal) {
				return
			}
			copySliceToTable(L, ptrVal, visited)
		}
	case reflect.Slice:
		if proxify {
			makeValueProxy(L, ptrVal, cSliceMeta)
		} else {
			if visited.push(val) {
				return
			}
			copySliceToTable(L, val, visited)
		}
	case reflect.Map:
		if proxify {
			makeValueProxy(L, ptrVal, cMapMeta)
		} else {
			if visited.push(val) {
				return
			}
			copyMapToTable(L, val, visited)
		}
	case reflect.Struct:
		if proxify && ptrVal.Kind() == reflect.Ptr {
			if ptrVal.CanInterface() {
				switch v := ptrVal.Interface().(type) {
				case error:
					L.PushString(v.Error())
				case *LuaObject:
					v.Push()
				default:
					makeValueProxy(L, ptrVal, cStructMeta)
				}
			} else {
				makeValueProxy(L, ptrVal, cStructMeta)
			}
		} else {
			// Use ptrVal instead of val to detect cycles from the very first element, if a pointer.
			if ptrVal.Kind() == reflect.Ptr && visited.push(ptrVal) {
				return
			}
			copyStructToTable(L, ptrVal, visited)
		}
	case reflect.Chan:
		makeValueProxy(L, ptrVal, cChannelMeta)
	case reflect.Func:
		L.PushGoFunction(goLuaFunc(L, val))
	default:
		if v, ok := val.Interface().(error); ok {
			L.PushString(v.Error())
		} else if val.IsNil() {
			L.PushNil()
		} else {
			makeValueProxy(L, ptrVal, cInterfaceMeta)
		}
	}
}

func luaIsEmpty(L *lua.State, idx int) bool {
	L.PushNil()
	if idx < 0 {
		idx--
	}
	if L.Next(idx) != 0 {
		L.Pop(2)
		return false
	}
	return true
}

func copyTableToMap(L *lua.State, t reflect.Type, idx int, visited map[uintptr]interface{}) interface{} {
	if t == nil {
		t = tmap
	}
	te, tk := t.Elem(), t.Key()
	m := reflect.MakeMap(t)

	// See copyTableToSlice.
	ptr := L.ToPointer(idx)
	if !luaIsEmpty(L, idx) {
		visited[ptr] = m.Interface()
	}

	L.PushNil()
	if idx < 0 {
		idx--
	}
	for L.Next(idx) != 0 {
		// key at -2, value at -1
		key := reflect.ValueOf(luaToGo(L, tk, -2, visited))
		val := reflect.ValueOf(luaToGo(L, te, -1, visited))
		if val.Interface() == nullv.Interface() {
			val = reflect.Zero(te)
		}
		m.SetMapIndex(key, val)
		L.Pop(1)
	}
	return m.Interface()
}

// Also for arrays.
func copyTableToSlice(L *lua.State, t reflect.Type, idx int, visited map[uintptr]interface{}) interface{} {
	if t == nil {
		t = tslice
	}

	ref := t
	// There is probably no point at accepting more than one level of dreference.
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	n := int(L.ObjLen(idx))

	var slice reflect.Value
	if t.Kind() == reflect.Array {
		slice = reflect.New(t)
		slice = slice.Elem()
	} else {
		slice = reflect.MakeSlice(t, n, n)
	}

	// Do not add empty slices to the list of visited elements.
	// The empty Lua table is a single instance object and gets re-used across maps, slices and others.
	if n > 0 {
		ptr := L.ToPointer(idx)
		if ref.Kind() == reflect.Ptr {
			visited[ptr] = slice.Addr().Interface()
		} else {
			visited[ptr] = slice.Interface()
		}
	}

	te := t.Elem()
	for i := 1; i <= n; i++ {
		L.RawGeti(idx, i)
		val := reflect.ValueOf(luaToGo(L, te, -1, visited))
		if val.Interface() == nullv.Interface() {
			val = reflect.Zero(te)
		}
		slice.Index(i - 1).Set(val)
		L.Pop(1)
	}

	if ref.Kind() == reflect.Ptr {
		return slice.Addr().Interface()
	}
	return slice.Interface()
}

func copyTableToStruct(L *lua.State, t reflect.Type, idx int, visited map[uintptr]interface{}) interface{} {
	if t == nil {
		RaiseError(L, "type argument must be non-nill")
	}
	wasPtr := t.Kind() == reflect.Ptr
	if wasPtr {
		t = t.Elem()
	}
	s := reflect.New(t) // T -> *T
	ref := s.Elem()

	// See copyTableToSlice.
	ptr := L.ToPointer(idx)
	if !luaIsEmpty(L, idx) {
		if wasPtr {
			visited[ptr] = s.Interface()
		} else {
			visited[ptr] = s.Elem().Interface()
		}
	}

	// Associate Lua keys with Go fields: tags have priority over matching field
	// name.
	fields := map[string]string{}
	st := ref.Type()
	for i := 0; i < ref.NumField(); i++ {
		field := st.Field(i)
		tag := field.Tag.Get("lua")
		if tag != "" {
			fields[tag] = field.Name
			continue
		}
		fields[field.Name] = field.Name
	}

	L.PushNil()
	if idx < 0 {
		idx--
	}
	for L.Next(idx) != 0 {
		key := L.ToString(-2)
		f := ref.FieldByName(fields[key])
		if f.CanSet() && f.IsValid() {
			val := reflect.ValueOf(luaToGo(L, f.Type(), -1, visited))
			f.Set(val)
		}
		L.Pop(1)
	}
	if wasPtr {
		return s.Interface()
	}
	return s.Elem().Interface()
}

// LuaToGo converts the Lua value at index 'idx' to the Go value of desired type 't'.
// Handles numerical and string types in a straightforward way, and will convert
// tables to either map or slice types.
// If 't' is nil or an interface, the type is inferred from the Lua value.
func LuaToGo(L *lua.State, t reflect.Type, idx int) interface{} {
	return luaToGo(L, t, idx, map[uintptr]interface{}{})
}

func luaToGo(L *lua.State, t reflect.Type, idx int, visited map[uintptr]interface{}) interface{} {
	var value interface{}

	var kind reflect.Kind
	if t != nil {
		if t.Kind() == reflect.Ptr {
			kind = t.Elem().Kind()
		} else if t.Kind() == reflect.Interface {
			// Let the Lua type drive the conversion.
			t = nil
		} else {
			kind = t.Kind()
		}
	}

	switch L.Type(idx) {
	case lua.LUA_TNIL:
		if t == nil {
			return nil
		}
		switch kind {
		default:
			value = reflect.Zero(t).Interface()
		}
	case lua.LUA_TBOOLEAN:
		if t == nil {
			kind = reflect.Bool
		}
		switch kind {
		case reflect.Bool:
			ptr := new(bool)
			*ptr = L.ToBoolean(idx)
			value = *ptr
		default:
			value = reflect.Zero(t).Interface()
		}
	case lua.LUA_TSTRING:
		if t == nil {
			kind = reflect.String
		}
		switch kind {
		case reflect.String:
			tos := L.ToString(idx)
			ptr := new(string)
			*ptr = tos
			value = *ptr
		default:
			value = reflect.Zero(t).Interface()
		}
	case lua.LUA_TNUMBER:
		if t == nil {
			// Infering the type here (e.g. int if round value) would break backward
			// compatibility and drift away from Lua's design: the numeric type is
			// specified at compile time.
			kind = reflect.Float64
		}
		switch kind {
		case reflect.Float64:
			ptr := new(float64)
			*ptr = L.ToNumber(idx)
			value = *ptr
		case reflect.Float32:
			ptr := new(float32)
			*ptr = float32(L.ToNumber(idx))
			value = *ptr
		case reflect.Int:
			ptr := new(int)
			*ptr = int(L.ToNumber(idx))
			value = *ptr
		case reflect.Int8:
			ptr := new(int8)
			*ptr = int8(L.ToNumber(idx))
			value = *ptr
		case reflect.Int16:
			ptr := new(int16)
			*ptr = int16(L.ToNumber(idx))
			value = *ptr
		case reflect.Int32:
			ptr := new(int32)
			*ptr = int32(L.ToNumber(idx))
			value = *ptr
		case reflect.Int64:
			ptr := new(int64)
			*ptr = int64(L.ToNumber(idx))
			value = *ptr
		case reflect.Uint:
			ptr := new(uint)
			*ptr = uint(L.ToNumber(idx))
			value = *ptr
		case reflect.Uint8:
			ptr := new(uint8)
			*ptr = uint8(L.ToNumber(idx))
			value = *ptr
		case reflect.Uint16:
			ptr := new(uint16)
			*ptr = uint16(L.ToNumber(idx))
			value = *ptr
		case reflect.Uint32:
			ptr := new(uint32)
			*ptr = uint32(L.ToNumber(idx))
			value = *ptr
		case reflect.Uint64:
			ptr := new(uint64)
			*ptr = uint64(L.ToNumber(idx))
			value = *ptr
		default:
			value = reflect.Zero(t).Interface()
		}
	case lua.LUA_TTABLE:
		if t == nil {
			kind = reflect.Interface
		}
		fallthrough
	default:
		istable := L.IsTable(idx)
		// If we don't know the type and the Lua object is userdata,
		// then it might be a proxy for a Go object. Otherwise wrap
		// it up as a LuaObject.
		if t == nil && !istable {
			if isValueProxy(L, idx) {
				v, _ := valueOfProxy(L, idx)
				return v.Interface()
			}
			return NewLuaObject(L, idx)
		}
		switch kind {
		case reflect.Array:
			if istable {
				ptr := L.ToPointer(idx)
				if val, ok := visited[ptr]; ok {
					return val
				}
				value = copyTableToSlice(L, t, idx, visited)
			} else {
				value = mustUnwrapProxy(L, idx)
			}
		case reflect.Slice:
			// if we get a table, then copy its values to a new slice
			if istable {
				ptr := L.ToPointer(idx)
				if val, ok := visited[ptr]; ok {
					return val
				}
				value = copyTableToSlice(L, t, idx, visited)
			} else {
				value = mustUnwrapProxy(L, idx)
			}
		case reflect.Map:
			if istable {
				ptr := L.ToPointer(idx)
				if val, ok := visited[ptr]; ok {
					return val
				}
				value = copyTableToMap(L, t, idx, visited)
			} else {
				value = mustUnwrapProxy(L, idx)
			}
		case reflect.Struct:
			if istable {
				ptr := L.ToPointer(idx)
				if val, ok := visited[ptr]; ok {
					return val
				}
				value = copyTableToStruct(L, t, idx, visited)
			} else {
				value = mustUnwrapProxy(L, idx)
			}
		case reflect.Interface:
			if istable {
				ptr := L.ToPointer(idx)
				if val, ok := visited[ptr]; ok {
					return val
				}
				// We have to make an executive decision here: tables with non-zero
				// length are assumed to be slices!
				if L.ObjLen(idx) > 0 {
					value = copyTableToSlice(L, nil, idx, visited)
				} else {
					value = copyTableToMap(L, nil, idx, visited)
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
				value = mustUnwrapProxy(L, idx)
			}
		default:
			value = mustUnwrapProxy(L, idx)
		}
	}

	return value
}

func isNewType(t reflect.Type) bool {
	types := [...]reflect.Type{
		reflect.Invalid:    nil, // Invalid Kind = iota
		reflect.Bool:       typeof((*bool)(nil)),
		reflect.Int:        typeof((*int)(nil)),
		reflect.Int8:       typeof((*int8)(nil)),
		reflect.Int16:      typeof((*int16)(nil)),
		reflect.Int32:      typeof((*int32)(nil)),
		reflect.Int64:      typeof((*int64)(nil)),
		reflect.Uint:       typeof((*uint)(nil)),
		reflect.Uint8:      typeof((*uint8)(nil)),
		reflect.Uint16:     typeof((*uint16)(nil)),
		reflect.Uint32:     typeof((*uint32)(nil)),
		reflect.Uint64:     typeof((*uint64)(nil)),
		reflect.Uintptr:    typeof((*uintptr)(nil)),
		reflect.Float32:    typeof((*float32)(nil)),
		reflect.Float64:    typeof((*float64)(nil)),
		reflect.Complex64:  typeof((*complex64)(nil)),
		reflect.Complex128: typeof((*complex128)(nil)),
		reflect.String:     typeof((*string)(nil)),
	}

	pt := types[int(t.Kind())]
	return pt != t
}

// RaiseError raises a Lua error from Go code.
func RaiseError(L *lua.State, format string, args ...interface{}) {
	// TODO: Rename to Fatalf?
	// TODO: Don't use and always return errors? Test what happens in examples. Can we continue?
	// TODO: Use golua's RaiseError?
	L.Where(1)
	pos := L.ToString(-1)
	L.Pop(1)
	panic(L.NewError(pos + fmt.Sprintf(format, args...)))
}

// Register makes a number of Go values available in Lua code.
// 'values' is a map of strings to Go values.
//
// - If table is non-nil, then create or reuse a global table of that name and
// put the values in it.
//
// - If table is '' then put the values in the global table (_G).
//
// - If table is '*' then assume that the table is already on the stack.
func Register(L *lua.State, table string, values Map) {
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
		GoToLuaProxy(L, val)
		L.SetField(-2, name)
	}
	if pop {
		L.Pop(1)
	}
}

func assertValid(L *lua.State, v reflect.Value, parent reflect.Value, name string, what string) {
	if !v.IsValid() {
		RaiseError(L, "no %s named `%s` for type %s", what, name, parent.Type())
	}
}

// Closest we'll get to a typeof operator.
func typeof(v interface{}) reflect.Type {
	return reflect.TypeOf(v).Elem()
}

// Types is a convenience function for converting a set of values into a
// corresponding slice of their types.
func Types(values ...interface{}) []reflect.Type {
	res := make([]reflect.Type, len(values))
	for i, arg := range values {
		res[i] = reflect.TypeOf(arg)
	}
	return res
}
