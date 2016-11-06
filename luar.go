// Copyright (c) 2010-2016 Steve Donovan

package luar

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/aarzilli/golua/lua"
)

// RaiseError raises a Lua error from Go code.
func RaiseError(L *lua.State, msg string) {
	L.Where(1)
	pos := L.ToString(-1)
	L.Pop(1)
	panic(L.NewError(pos + " " + msg))
}

func assertValid(L *lua.State, v reflect.Value, parent reflect.Value, name string, what string) {
	if !v.IsValid() {
		RaiseError(L, fmt.Sprintf("no %s named `%s` for type %s", what, name, parent.Type()))
	}
}

// NullT is the type of 'luar.null'.
type NullT int

var (
	tslice    = typeof((*[]interface{})(nil))
	tmap      = typeof((*map[string]interface{})(nil))
	nullv     = reflect.ValueOf(Null)
	nullables = map[reflect.Kind]bool{
		reflect.Chan:      true,
		reflect.Func:      true,
		reflect.Interface: true,
		reflect.Map:       true,
		reflect.Ptr:       true,
		reflect.Slice:     true,
	}

	// Null is the definition of 'luar.null' which is used in place of 'nil' when
	// converting slices and structs.
	Null = NullT(0)
)

func isNil(val reflect.Value) bool {
	kind := val.Type().Kind()
	return nullables[kind] && val.IsNil()
}

// CopyTableToSlice returns the Lua table at 'idx' as a copied Go slice.
// If 't' is nil then the slice type is []interface{}
func CopyTableToSlice(L *lua.State, t reflect.Type, idx int) interface{} {
	return copyTableToSlice(L, t, idx, map[uintptr]interface{}{})
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

// CopyTableToMap returns the Lua table at 'idx' as a copied Go map.
// If 't' is nil then the map type is map[string]interface{}.
func CopyTableToMap(L *lua.State, t reflect.Type, idx int) interface{} {
	return copyTableToMap(L, t, idx, map[uintptr]interface{}{})
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

// CopyTableToStruct copies matching Lua table entries to a struct, given the
// struct type and the index on the Lua stack. Use the "lua" tag to set field
// names.
func CopyTableToStruct(L *lua.State, t reflect.Type, idx int) interface{} {
	return copyTableToStruct(L, t, idx, map[uintptr]interface{}{})
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

// CopySliceToTable copies a Go slice to a Lua table.
// 'nil' is represented as 'luar.null'.
// Defines 'luar.slice2table'.
func CopySliceToTable(L *lua.State, vslice reflect.Value) int {
	v := newVisitor(L)
	defer v.close()
	return copySliceToTable(L, vslice, v)
}

// CopyArrayToTable copies a Go array to a Lua table.
// 'nil' is represented as 'luar.null'.
// Defines 'luar.array2table'.
func CopyArrayToTable(L *lua.State, v reflect.Value) int {
	visitor := newVisitor(L)
	defer visitor.close()
	return copySliceToTable(L, v, visitor)
}

// Also for arrays.
func copySliceToTable(L *lua.State, vslice reflect.Value, visited visitor) int {
	ref := vslice
	for vslice.Kind() == reflect.Ptr {
		// For arrays.
		vslice = vslice.Elem()
	}

	if vslice.IsValid() && (vslice.Kind() == reflect.Slice || vslice.Kind() == reflect.Array) {
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
			goToLua(L, nil, v, true, visited)
			L.SetTable(-3)
		}
		return 1
	}
	L.PushNil()
	L.PushString("not a slice/array")
	return 2
}

// CopyStructToTable copies a Go struct to a Lua table.
// 'nil' is represented as 'luar.null'.
// Defines 'luar.struct2table'.
// Use the "lua" tag to set field names.
func CopyStructToTable(L *lua.State, vstruct reflect.Value) int {
	v := newVisitor(L)
	defer v.close()
	return copyStructToTable(L, vstruct, v)
}

func copyStructToTable(L *lua.State, vstruct reflect.Value, visited visitor) int {
	// If 'vstruct' is a pointer to struct, use the pointer to mark as visited.
	ref := vstruct
	for vstruct.Kind() == reflect.Ptr {
		vstruct = vstruct.Elem()
	}

	if vstruct.IsValid() && vstruct.Type().Kind() == reflect.Struct {
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
			goToLua(L, nil, reflect.ValueOf(key), true, visited)
			v := vstruct.Field(i)
			goToLua(L, nil, v, true, visited)
			L.SetTable(-3)
		}
		return 1
	}
	L.PushNil()
	L.PushString("not a struct")
	return 2
}

// CopyMapToTable copies a Go map to a Lua table.
// Defines 'luar.map2table'.
func CopyMapToTable(L *lua.State, vmap reflect.Value) int {
	v := newVisitor(L)
	defer v.close()
	return copyMapToTable(L, vmap, v)
}

func copyMapToTable(L *lua.State, vmap reflect.Value, visited visitor) int {
	if vmap.IsValid() && vmap.Type().Kind() == reflect.Map {
		n := vmap.Len()
		L.CreateTable(0, n)
		visited.mark(vmap)
		for _, key := range vmap.MapKeys() {
			v := vmap.MapIndex(key)
			goToLua(L, nil, key, false, visited)
			if isNil(v) {
				v = nullv
			}
			goToLua(L, nil, v, true, visited)
			L.SetTable(-3)
		}
		return 1
	}
	L.PushNil()
	L.PushString("not a map!")
	return 2
}

// closest we'll get to a typeof operator...
func typeof(v interface{}) reflect.Type {
	return reflect.TypeOf(v).Elem()
}

var types = []reflect.Type{
	nil, // Invalid Kind = iota
	typeof((*bool)(nil)),
	typeof((*int)(nil)),
	typeof((*int8)(nil)),
	typeof((*int16)(nil)),
	typeof((*int32)(nil)),
	typeof((*int64)(nil)),
	typeof((*uint)(nil)),
	typeof((*uint8)(nil)),
	typeof((*uint16)(nil)),
	typeof((*uint32)(nil)),
	typeof((*uint64)(nil)),
	nil, // Uintptr
	typeof((*float32)(nil)),
	typeof((*float64)(nil)),
	nil, // Complex64
	nil, // Complex128
	nil, // Array
	nil, // Chan
	nil, // Func
	nil, // Interface
	nil, // Map
	nil, // Ptr
	nil, // Slice
	typeof((*string)(nil)),
	nil, // Struct
	nil, // UnsafePointer
}

// Return the underlying type of a new scalar type, nil otherwise.
func predeclaredScalarType(t reflect.Type) reflect.Type {
	pt := types[int(t.Kind())]
	if pt != nil && pt != t {
		return pt
	}
	return nil
}

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

// Push visited value on top of the stack.
// If the value was not visited, return false.
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

func (v *visitor) close() {
	v.L.Unref(lua.LUA_REGISTRYINDEX, v.index)
}

// GoToLua pushes a Go value 'val' on the Lua stack.
//
// It unboxes interfaces.
// 't' is here for backward-compatibility and will be ignored.
//
// If not proxifying, pointers are followed recursively. Slices, structs and maps are copied over as tables.
//
// When proxifying, pointers are preserved. Structs and arrays need to be
// passed as pointers to be proxified, otherwise they will be copied as tables.
//
// Predeclared scalar types are never proxified (dontproxify is ignored) as they have no methods.
func GoToLua(L *lua.State, t reflect.Type, val reflect.Value, dontproxify bool) {
	v := newVisitor(L)
	goToLua(L, nil, val, dontproxify, v)
	v.close()
}

func goToLua(L *lua.State, t reflect.Type, val reflect.Value, dontproxify bool, visited visitor) {
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
		if !dontproxify && predeclaredScalarType(val.Type()) != nil {
			makeValueProxy(L, ptrVal, cNumberMeta)
		} else {
			L.PushNumber(val.Float())
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if !dontproxify && predeclaredScalarType(val.Type()) != nil {
			makeValueProxy(L, ptrVal, cNumberMeta)
		} else {
			L.PushNumber(float64(val.Int()))
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if !dontproxify && predeclaredScalarType(val.Type()) != nil {
			makeValueProxy(L, ptrVal, cNumberMeta)
		} else {
			L.PushNumber(float64(val.Uint()))
		}
	case reflect.String:
		if !dontproxify && predeclaredScalarType(val.Type()) != nil {
			makeValueProxy(L, ptrVal, cStringMeta)
		} else {
			L.PushString(val.String())
		}
	case reflect.Bool:
		if !dontproxify && predeclaredScalarType(val.Type()) != nil {
			makeValueProxy(L, ptrVal, cInterfaceMeta)
		} else {
			L.PushBoolean(val.Bool())
		}
	case reflect.Array:
		// It needs be a pointer to be a proxy, otherwise values won't be settable.
		if !dontproxify && ptrVal.Kind() == reflect.Ptr {
			makeValueProxy(L, ptrVal, cSliceMeta)
		} else {
			// See the case of struct.
			if ptrVal.Kind() == reflect.Ptr && visited.push(ptrVal) {
				return
			}
			copySliceToTable(L, ptrVal, visited)
		}
	case reflect.Slice:
		if !dontproxify {
			makeValueProxy(L, ptrVal, cSliceMeta)
		} else {
			if visited.push(val) {
				return
			}
			copySliceToTable(L, val, visited)
		}
	case reflect.Map:
		if !dontproxify {
			makeValueProxy(L, ptrVal, cMapMeta)
		} else {
			if visited.push(val) {
				return
			}
			copyMapToTable(L, val, visited)
		}
	case reflect.Struct:
		if !dontproxify && ptrVal.Kind() == reflect.Ptr {
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

// LuaToGo converts a Lua value 'idx' on the stack to the Go value of desired type 't'.
// Handles numerical and string types in a straightforward way, and will convert
// tables to either map or slice types.
func LuaToGo(L *lua.State, t reflect.Type, idx int) interface{} {
	return luaToGo(L, t, idx, map[uintptr]interface{}{})
}

func luaToGo(L *lua.State, t reflect.Type, idx int, visited map[uintptr]interface{}) interface{} {
	var value interface{}

	var kind reflect.Kind
	if t != nil { // let the Lua type drive the conversion...
		if t.Kind() == reflect.Ptr {
			kind = t.Elem().Kind()
		} else if t.Kind() == reflect.Interface {
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
				return unwrapProxy(L, idx)
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
				value = unwrapProxyOrComplain(L, idx)
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
				value = unwrapProxyOrComplain(L, idx)
			}
		case reflect.Map:
			if istable {
				ptr := L.ToPointer(idx)
				if val, ok := visited[ptr]; ok {
					return val
				}
				value = copyTableToMap(L, t, idx, visited)
			} else {
				value = unwrapProxyOrComplain(L, idx)
			}
		case reflect.Struct:
			if istable {
				ptr := L.ToPointer(idx)
				if val, ok := visited[ptr]; ok {
					return val
				}
				value = copyTableToStruct(L, t, idx, visited)
			} else {
				value = unwrapProxyOrComplain(L, idx)
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
				value = unwrapProxyOrComplain(L, idx)
			}
		default:
			value = unwrapProxyOrComplain(L, idx)
		}
	}

	return value
}

// elegant little 'cheat' suggested by Kyle Lemons,
// avoiding the 'Call using zero Value argument' problem
// http://play.golang.org/p/TZyOLzu2y-
func valueOfNil(ival interface{}) reflect.Value {
	if ival == nil {
		return reflect.ValueOf(&ival).Elem()
	}
	return reflect.ValueOf(ival)
}

// GoLuaFunc converts an arbitrary Go function into a Lua-compatible GoFunction.
// There are special optimized cases for functions that go from strings to
// strings, and doubles to doubles, but otherwise Go reflection is used to
// provide a generic wrapper function.
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

	var funV reflect.Value
	switch ff := fun.(type) {
	case reflect.Value:
		funV = ff
	default:
		funV = reflect.ValueOf(fun)
	}

	funT := funV.Type()
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
		resV := callGo(L, funV, args)
		for _, val := range resV {
			GoToLua(L, nil, val, false)
		}
		return len(resV)
	}
}

func callGo(L *lua.State, funv reflect.Value, args []reflect.Value) []reflect.Value {
	defer func() {
		if x := recover(); x != nil {
			RaiseError(L, fmt.Sprintf("error %s", x))
		}
	}()
	resv := funv.Call(args)
	return resv
}

// Map is an alias for passing maps of strings to values to luar.
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
		if t := reflect.TypeOf(val); t != nil && t.Kind() == reflect.Func {
			if convertFun {
				L.PushGoFunction(GoLuaFunc(L, val))
			} else {
				lf := val.(func(*lua.State) int)
				L.PushGoFunction(lf)
			}
		} else {
			GoToLua(L, nil, reflect.ValueOf(val), false)
		}
		L.SetField(-2, name)
	}
	if pop {
		L.Pop(1)
	}
}

// RawRegister makes a number of 'raw' Go functions or values available in Lua
// code. Raw Go functions access the Lua state directly and have signature
// '(*lua.State) int'.
func RawRegister(L *lua.State, table string, values Map) {
	register(L, table, values, false)
}

// Register makes a number of Go functions or values available in Lua code.
// If table is non-nil, then create or reuse a global table of that name and put
// the values in it. If table is '*' then assume that the table is already on
// the stack. values is a map of strings to Go values.
func Register(L *lua.State, table string, values Map) {
	register(L, table, values, true)
}

// LuaObject encapsulates a Lua object like a table or a function.
type LuaObject struct {
	L    *lua.State
	Ref  int
	Type string
}

// Get returns the Go value indexed at 'key' in the Lua object.
func (lo *LuaObject) Get(key string) interface{} {
	lo.Push() // the table
	Lookup(lo.L, key, -1)
	val := LuaToGo(lo.L, nil, -1)
	lo.L.Pop(2)
	return val
}

// GetObject returns the Lua object indexed at 'key' in the Lua object.
func (lo *LuaObject) GetObject(key string) *LuaObject {
	lo.Push() // the table
	Lookup(lo.L, key, -1)
	val := NewLuaObject(lo.L, -1)
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

// Types is a convenience function for converting a set of values into a
// corresponding slice of their types.
func Types(values ...interface{}) []reflect.Type {
	res := make([]reflect.Type, len(values))
	for i, arg := range values {
		res[i] = reflect.TypeOf(arg)
	}
	return res
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
	Lookup(L, path, 0)
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

// Global creates a new LuaObject refering to the global environment.
func Global(L *lua.State) *LuaObject {
	L.GetGlobal("_G")
	val := NewLuaObject(L, -1)
	L.Pop(1)
	return val
}

// Lookup will search a Lua value by its full name.
//
// If idx is 0, then this name is assumed to start in the global table, e.g.
// "string.gsub". With non-zero idx, it can be used to look up subfields of a
// table. It terminates with a nil value if we cannot continue the lookup.
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

// LuarSetup replaces the 'pairs' and 'ipairs' so they work on proxies as well.
const LuarSetup = `
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

// Init makes and initialize a new pre-configured Lua state. It is not required
// for using the 'GoToLua' and 'LuaToGo' functions; it is needed for proxy
// conversions however.
//
// It populates the 'luar' table with the following functions:
// 	'map2table', 'slice2table', 'array2table', 'struct2table', 'map', 'slice', 'type', 'sub', 'append', 'raw',
// and values:
//  'null'.
//
// This replaces the pairs/ipairs functions so that __pairs/__ipairs
// can be used, Lua 5.2 style.
func Init() *lua.State {
	var L = lua.NewState()
	L.OpenLibs()
	InitProxies(L)
	_ = L.DoString(LuarSetup) // Never fails.
	RawRegister(L, "luar", Map{
		// Functions.
		"map2table":    MapToTable,
		"slice2table":  SliceToTable,
		"array2table":  ArrayToTable,
		"struct2table": StructToTable,
		"map":          MakeMap,
		"slice":        MakeSlice,
		"type":         ProxyType,
		"sub":          SliceSub,
		"append":       SliceAppend,
		"raw":          ProxyRaw,
		// Values.
		"null": Null,
	})
	// TODO: What is this for?
	// Register(L, "luar", Map{
	// 	"value": reflect.ValueOf,
	// })
	return L
}
