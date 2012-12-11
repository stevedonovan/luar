package luar


import lua "github.com/aarzilli/golua/lua"
import "fmt"
//import "os"
import "reflect"
import "unsafe"


// Lua proxy objects for Slices and Maps
type ValueProxy struct {
    value reflect.Value
    t reflect.Type
}

const (
    SLICE_META = "sliceMT"
    MAP_META = "mapMT"
    STRUCT_META = "structMT"
    INTERFACE_META = "interfaceMT"
    CHANNEL_META = "ChannelMT"
)

func makeValueProxy(L *lua.State, val reflect.Value,  proxyMT string) {
	rawptr := L.NewUserdata(uintptr(unsafe.Sizeof(ValueProxy{})))
	ptr := (*ValueProxy)(rawptr)
    ptr.value = val
    ptr.t = val.Type()
    L.LGetMetaTable(proxyMT)
    L.SetMetaTable(-2)
}

var valueOf = reflect.ValueOf

func valueOfProxy(L *lua.State, idx int) (reflect.Value,reflect.Type) {
    vp := (*ValueProxy)(L.ToUserdata(idx))
    return vp.value, vp.t
}

func isValueProxy(L *lua.State, idx int) bool {
    res := false
    if L.IsUserdata(idx) {
        L.GetMetaTable(idx)
        if ! L.IsNil(-1) {
            L.GetField(-1,"luago.value")
            res = ! L.IsNil(-1)
            L.Pop(1)
        }
        L.Pop(1)
    }
    return res
}

func unwrapProxy(L *lua.State, idx int) interface{} {
    if isValueProxy(L,idx) {
        v,_ := valueOfProxy(L,idx)
        return v.Interface()
    } 
    return nil
}

func channel_send(L *lua.State) int {
    fmt.Println("yield send")

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
    fmt.Println("yield recv")

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
        LT :=  L.NewThread()
        L.PushValue(1)
        lua.XMove(L,LT,1)
        res := LT.Resume(0)
        for res != 0 {
            if res == 2 {
                emsg := LT.ToString(-1)
                fmt.Println("error",emsg)
            }
            ch,t := valueOfProxy(LT,-2)
            
            if LT.ToBoolean(-1) {  // send on a channel                
                val := valueOf(LuaToGo(LT, t.Elem(),-3))
                ch.Send(val)            
                res = LT.Resume(0)
            } else { // receive on a channel
                val,ok := ch.Recv()
                GoToLua(LT,t.Elem(),val)
                LT.PushBoolean(ok)            
                res = LT.Resume(2)
            }           
        }
    } ()
    return 0
}

func MakeChannel(L *lua.State) int {
    ch := make(chan interface{})
    makeValueProxy(L,valueOf(ch),CHANNEL_META)
    return 1
}

func initializeProxies(L *lua.State) {
    add_index := func (name string, fun lua.LuaGoFunction) {
        L.PushGoCallback(fun)
        L.SetField(-2,name)
    }
    flagValue := func () {
        L.PushBoolean(true)
        L.SetField(-2,"luago.value")
        L.Pop(1)    
    }
    L.NewMetaTable(SLICE_META)
    add_index("__index", slice__index)
    add_index("__newindex",slice__newindex)
    add_index("__len",slicemap__len)
    flagValue()
    L.NewMetaTable(MAP_META)
    add_index("__index",map__index)
    add_index("__newindex",map__newindex)
    add_index("__len",slicemap__len)
    flagValue()
    L.NewMetaTable(STRUCT_META)
    add_index("__index",struct__index)
    add_index("__newindex",struct__newindex)
    flagValue()
    L.NewMetaTable(INTERFACE_META)
    add_index("__index",interface__index)
    flagValue()
    L.NewMetaTable(CHANNEL_META)
    //~ RegisterFunctions(L,"*",FMap {
        //~ "Send":channel_send,
        //~ "Recv":channel_recv,
    //~ })
    L.NewTable()
    L.PushGoFunction(channel_send)
    L.SetField(-2,"Send")
    L.PushGoFunction(channel_recv)
    L.SetField(-2,"Recv")
    L.SetField(-2,"__index")
    flagValue()
}

func slice_slice(L *lua.State) int {
    slice,_ := valueOfProxy(L,1)
    i1,i2 := L.ToInteger(2), L.ToInteger(3)
    newslice := slice.Slice(i1,i2)
    makeValueProxy(L,newslice,SLICE_META)
    return 1
}


func slice__index (L *lua.State) int {
    slice,_ := valueOfProxy(L,1)
    if L.IsNumber(2) {
        idx := L.ToInteger(2)
        ret := slice.Index(idx-1)
        GoToLua(L,ret.Type(),ret)
    } else {
        name := L.ToString(2)
        switch name {
        case "Slice":
            L.PushGoFunction(slice_slice)
        default:
            fmt.Println("unknown slice method")
        }    
    }
    return 1    
}

func slice__newindex (L *lua.State) int {
    slice,t := valueOfProxy(L,1)
    idx := L.ToInteger(2)
    val := LuaToGo(L,t.Elem(),3)
    slice.Index(idx-1).Set(valueOf(val))
    return 0
}

func slicemap__len(L *lua.State) int {
    val,_ := valueOfProxy(L,1)
    L.PushInteger(int64(val.Len()))
    return 1
}

func map__index (L *lua.State) int {
    val,t := valueOfProxy(L, 1)
    key := LuaToGo(L,t.Key(),2)
    ret := val.MapIndex(valueOf(key))
    GoToLua(L,ret.Type(),ret)
    return 1
}

func map__newindex (L *lua.State) int {
    m,t := valueOfProxy(L,1)
    key := LuaToGo(L,t.Key(),2)
    val :=  LuaToGo(L, t.Elem(),3)
    m.SetMapIndex(valueOf(key),valueOf(val))
    return 0
}

func callGoMethod (L *lua.State, name string, st reflect.Value) {
    ret := st.MethodByName(name)
    if ! ret.IsValid() {
        fmt.Println("whoops")
    }
    L.PushGoFunction(GoLuaFunc(L,ret))
}

func struct__index (L *lua.State) int {
    st,t := valueOfProxy(L,1)
    name := L.ToString(2)
    est := st
    if t.Kind() == reflect.Ptr {
        est = st.Elem()
    }
    ret := est.FieldByName(name)
    if ! ret.IsValid() { // no such field, try for method?
        callGoMethod(L,name,st)
    } else {
        GoToLua(L,ret.Type(),ret)
    }
    return 1    
}

func interface__index (L *lua.State) int {
    st,_ := valueOfProxy(L,1)
    name := L.ToString(2)
    callGoMethod(L,name,st)
    return 1
}

func struct__newindex (L *lua.State) int {
    st,t := valueOfProxy(L,1)
    name := L.ToString(2)    
    if t.Kind() == reflect.Ptr {
        st = st.Elem()
    }
    field := st.FieldByName(name)
    val := LuaToGo(L,field.Type(),3)
    field.Set(valueOf(val))
    return 0
}

// end of proxy code


var (
    tslice = make([]interface{},0)
    tmap = make(map[string]interface{})
)

// return the Lua table at 'idx' as a copied Go slice. If 't' is nil then the slice
// type is []interface{}
func CopyTableToSlice (L *lua.State, t reflect.Type, idx int) interface{} {
    if t == nil {
        t = reflect.TypeOf(tslice)
    }
    te := t.Elem()
    n := int(L.ObjLen(idx))
    slice := reflect.MakeSlice(t,n,n)
    for i := 1; i <= n; i++ {
        L.RawGeti(idx,i)
        val := LuaToGo(L,te,-1)
        slice.Index(i-1).Set(valueOf(val))
        L.Pop(1)
    }
    return slice.Interface()
}

// return the Lua table at 'idx' as a copied Go map. If 't' is nil then the map
// type is map[string]interface{}
func CopyTableToMap(L *lua.State, t reflect.Type, idx int) interface{} {
    if t == nil {
        t = reflect.TypeOf(tmap)
    }
    te,tk  := t.Elem(), t.Key()
    m := reflect.MakeMap(t)
    L.PushNil()
    if idx < 0 {
        idx--
    }
    for L.Next(idx) != 0 {
        // key at -2, value at -1
        key := valueOf(LuaToGo(L,tk,-2))
        val := valueOf(LuaToGo(L,te,-1))
        m.SetMapIndex (key,val)
        fmt.Println("key",key,val)
        L.Pop(1)        
    }
    return m.Interface()
}


// /*
func LuaGoFunc(L *lua.State, t reflect.Type, idx int) interface{} {
   // targs, tout := functionArgRetTypes(t)
    return func(args... interface{}) interface{} {
        return len(args)
        //L.Call(n,1
    
    }    
}
//*/

// Push a Go value 'val' of type 't' on the Lua stack.
func GoToLua (L *lua.State, t reflect.Type, val reflect.Value) {    
    if t == nil {
        t = val.Type()
    }
    if t.Kind() == reflect.Ptr {
        t = t.Elem()
    }
    switch(t.Kind()) {
    case reflect.Float64:
    case reflect.Float32: {
        L.PushNumber(val.Float())
    }
    case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32: {
        L.PushNumber(float64(val.Int()))
    }
    case reflect.Uint, reflect.Uint8: {
        L.PushNumber(float64(val.Uint()))
    }
    case reflect.String: {
        L.PushString(val.String())
    }
    case reflect.Bool: {
        L.PushBoolean(val.Bool())
    }
    case reflect.Slice: {
        makeValueProxy(L,val,SLICE_META)
    }
    case reflect.Map: {
        makeValueProxy(L,val,MAP_META)
    }
    case reflect.Struct: {
        makeValueProxy(L,val,STRUCT_META)
    }
    default: {
       // fmt.Println("unhandled go type",t,t.Kind())
        if val.IsNil() {
            L.PushNil()
        } else {
            makeValueProxy(L,val,INTERFACE_META)
        }
    }
    }                
}

// Convert a Lua value 'idx' on the stack to the Go value of desired type 't'. Handles
// numerical and string types in a straightforward way, and will convert tables to 
// either map or slice types. 
func LuaToGo (L *lua.State, t reflect.Type, idx int) interface{}{
    var value interface{}
        
    switch(t.Kind()) {
    // various numerical types are tedious but straightforward
    case reflect.Float64:    {
        ptr := new(float64)
        *ptr = L.ToNumber(idx)
        value = *ptr
    }
    case reflect.Float32: {
        ptr := new(float32)
        *ptr = float32(L.ToNumber(idx))
        value = *ptr
    }
    case reflect.Int: {
        ptr := new(int)
        *ptr = int(L.ToNumber(idx))
        value = *ptr
    }
    case reflect.Int8: {
        ptr := new(byte)
        *ptr = byte(L.ToNumber(idx))
        value = *ptr
    }    
    case reflect.String: {
        tos := L.ToString(idx)
        ptr := new(string)
        *ptr = tos
        value = *ptr
    }
    case reflect.Bool: {
        ptr := new(bool)
        *ptr = bool(L.ToBoolean(idx))
        value = *ptr
    }
    case reflect.Slice: {
        // if we get a table, then copy its values to a new slice       
        if L.IsTable(idx) {
            value = CopyTableToSlice(L,t,idx)
        } else  {
            value = unwrapProxy(L,idx)
        }        
    }
    case reflect.Map: {
        if L.IsTable(idx) {
            value = CopyTableToMap(L,t,idx)
        } else {
            value = unwrapProxy(L,idx)
        }
    }
    case reflect.Func: {
        if L.IsFunction(idx) {
            value = LuaGoFunc(L,t,idx)
        }
    }
    case reflect.Interface: {     
        if L.IsTable(idx) {
            // have to make an executive decision here: tables with non-zero
            // length are assumed to be slices!
            if L.ObjLen(idx) > 0 {
                value = CopyTableToSlice(L,nil,idx)
            } else {
                value = CopyTableToMap(L,nil,idx)
            }
        } else
        if L.IsNumber(idx) {
            value = L.ToNumber(idx)
        } else
        if L.IsString(idx) {
            value = L.ToString(idx)
        } else
        if L.IsBoolean(idx) {
            value = L.ToBoolean(idx)
        } else {
            value = unwrapProxy(L,idx)
        }
    }
     default: {
        fmt.Println("unhandled type",t)
         value = 20
    }

    }
    return value  
    
}

func functionArgRetTypes(funt reflect.Type) (targs, tout []reflect.Type) {
    targs = make([]reflect.Type,funt.NumIn())
    for i, _ := range targs {
        targs[i] = funt.In(i)
    }
    tout = make([]reflect.Type,funt.NumOut())
    for i,_ := range tout {
        tout[i] = funt.Out(i)
    }
    return
}

// GoLuaFunc converts an arbitrary Go function into a Lua-compatible GoFunction.
// There are special optimized cases for functions that go from strings to strings,
// and doubles to doubles, but otherwise Go 
// reflection is used to provide a generic wrapper function
func GoLuaFunc (L *lua.State, fun interface{}) lua.LuaGoFunction {
    switch f := fun.(type) {
    case func(*lua.State) int: return f
    case func(string) string: return func(L *lua.State) int {
        L.PushString(f(L.ToString(1)))
        return 1
    }
    case func(float64) float64: return func(L *lua.State) int {
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
    return func (L *lua.State) int {
        var lastT reflect.Type
        isVariadic := funt.IsVariadic()
        if isVariadic {
            lastT = targs[len(targs)-1].Elem()
            targs = targs[0:len(targs)-1]
        }
        args := make([]reflect.Value,len(targs))        
        for i,t := range targs {
            val := LuaToGo(L,t,i+1)
            args[i] = valueOf(val)
        }
        if isVariadic {
            n := L.GetTop();
            for i := len(targs)+1; i <= n; i++ {
                val := valueOf(LuaToGo(L,lastT,i))
                args = append(args,val)
            }
        }
        resv := funv.Call(args)
        for i, val := range resv {
            GoToLua(L,tout[i],val)
        }
        return len(resv)
    }    
}

type Map map[string]interface{}

func register(L *lua.State, table string, values Map, convertFun bool) {
    pop := true
    if table == "*" {
        pop = false
    } else
    if len(table) > 0 {
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
                L.PushGoFunction(GoLuaFunc(L,val))
            } else {                
                lf := val.(func(*lua.State)int)
                L.PushGoFunction(lf)
            }            
        } else {
            GoToLua(L,t,valueOf(val))
        }
        L.SetField(-2,name)
    }    
    if pop {
        L.Pop(1)
    }
}

func RawRegister(L *lua.State, table string, values Map) {
    register(L,table,values,false);    
}

// make a number of Go functions or values available in Lua code. If table is non-nil,
// then create or reuse a global table of that name and put the values
// in it. If table is '*' then assume that the table is already on the stack.
// values is a map of strings to Go values.
func Register(L *lua.State, table string, values Map) {
    register(L,table,values,true);    
}

/// useful functions

func copyMapToTable(L *lua.State) int {
    vmap := valueOf(unwrapProxy(L,1))
    if vmap.IsValid() && vmap.Type().Kind() == reflect.Map {
        n := vmap.Len()
        L.CreateTable(0,n)
        for _,key := range vmap.MapKeys() {
            GoToLua(L,nil,key)
            GoToLua(L,nil,vmap.MapIndex(key))
            L.SetTable(-3)
        }            
        return 1
    } else {
        L.PushNil()
        L.PushString("not a map!")        
    }  
    return 2    
}

func copySliceToTable(L *lua.State) int {
    vslice := valueOf(unwrapProxy(L,1))
    if vslice.IsValid() && vslice.Type().Kind() == reflect.Slice {
        n := vslice.Len()
        L.CreateTable(n,0)
        for i := 0; i < n; i++ {
            L.PushInteger(int64(i+1))
            GoToLua(L,nil,vslice.Index(i))
            L.SetTable(-3)
        }            
        return 1
    } else {
        L.PushNil()
        L.PushString("not a slice!")        
    }  
    return 2    
}

// make and initialize a new Lua state
func Init() *lua.State {
    var L = lua.NewState()
    L.OpenLibs();         
    initializeProxies(L)
    RawRegister(L,"luar",Map{
        "map2table":copyMapToTable,
        "slice2table":copySliceToTable,
    })
    return L
}



