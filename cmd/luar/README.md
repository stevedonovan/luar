# An interactive REPL for Golua

This commandline tool provides a useful Lua REPL for exploring Go in Lua.

You will to get 'go.linenoise':

	go get github.com/GeertJohan/go.linenoise

to get line history and tab completion. This is an extended REPL and comes with
pretty-printing:

```lua
$ ./luar
luar prompt
Lua 5.1.4  Copyright (C) 1994-2008 Lua.org, PUC-Rio
> = 10,'10',{10}
10	"10"	{10}
```

One use for the `luar` REPL is to explore Go libraries. `regexp.Compile` is
exported as `regexp`, so we can do this. note that the endlessly useful `fmt.Println`
is available as `println` from Lua. Starting a line with a period ('dot') wraps
that line in `println`; starting a line with '=' wraps it with `print` (as is usual
with the standard Lua prompt.)

```lua
> p = regexp '[a-z]+\\S*'
> ms =  p.FindAllString('boo woo koo',99)
> = #ms
3
> println(ms)
[boo woo koo]
> . ms
[boo woo koo]
```

The next session explores the `luar` function `slice`, which generates a Go
slice. This is automatically wrapped as a proxy object. Note that the indexing
is one-based, and that Go slices have a fixed size!  The metatable for slice proxies
has an `__ipairs` metamethod. Although luar is (currently) based on Lua 5.1,
it loads code to provide a 5.2-compatible `pairs` and `ipairs`.

The inverse of `slice` is `unproxify`.

```lua
> s = luar.slice(2) // create a Go slice
> = #s
2
> = s[1]
nil
> = s[2]
nil
> = s[3] // has exactly two elements!
[string "print( s[3])"]:1:  slice get: index out of range
> = s
[]interface {}
> for i,v in ipairs(s) do print (i,v) end
1	10
2	20
> = luar.unproxify(s)
{10,20}
> println(s)
[10 20]
> . s
[10 20]
```

A similar operation is `luar.map`.
Using `luar.type` we can find the Go type of a proxy (it returns `nil` if this isn't
a Go type). By getting the type of a value we can then do _reflection_ and
find out what methods a type has, etc.

```lua
> m = luar.map()
> m.one = 1
> m.two = 2
> m.three = 3
> println(m)
map[one:1 two:2 three:3]
> for k,v in pairs(m) do print(k,v) end
three	3
one	1
two	2
> mt = luar.type(m)
> = mt.String()
"map[string]interface {}"
> = mt.Key().String()
"string"
> mtt = luar.type(mt)
> = mtt.String()
"*reflect.rtype"
> = mtt.NumMethod()
31
```

Tab-completion is implemented in such Lua code: the Lua completion code
merely requires that a type implement `__pairs`. This allows tab to
expand `mtt.S` to `mtt.String` in the last example.

```lua
local function sdump(st)
    local t = luar.type(st)
    local val = luar.value(st)
    local nm = t.NumMethod()
    local mt = t --// type to find methods on ptr receiver
    if t.Kind() == 22 then --// pointer!
        t = t.Elem()
        val = val.Elem()
    end
    local n = t.NumField()
    local cc = {}
    for i = 1,n do
        local f,v = t.Field(i-1)
        if f.PkgPath == "" then --// only public fields!
            v = val.Field(i-1)
            cc[f.Name] = v.Interface()
        end
    end
    --// then public methods...
    for i = 1,nm do
        local m = mt.Method(i-1)
        if m.PkgPath == "" then --// again, only public
            cc[m.Name] = true
        end
    end
    return cc
end

mt = getmetatable(__DUMMY__)
mt.__pairs = function(st)
    local cc = sdump(st)
    return pairs(cc)
end
```

`sdump` is pretty much the way this would be encoded in Go itself; again, the
eccentric dot-notation makes it more familiar. This `luar` interpreter is mostly
Lua embedded in Go source!
