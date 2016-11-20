# Luar: Lua reflection bindings for Go

Luar is designed to make using Lua from Go more convenient. Go structs, slices
and maps can be automatically converted to Lua tables and vice-versa. The
resulting conversion can either be a copy or a proxy. In the latter case, any change
made to the result will reflect on the source.

Any Go function can be made available to Lua scripts, without having to write
C-style wrappers.

Luar support cyclic structures (`map[string]interface{}`, lists, etc.).

User-defined types can be made available to Lua as well: their exported methods
can be called and usual operations such as indexing or arithmetic can be
performed.

See the [documentation](http://godoc.org/github.com/stevedonovan/luar) for usage
instructions and examples.

# Installation

Install with

    go get <repo>/luar

Luar uses Alessandro Arzilli's [golua](https://github.com/aarzilli/golua).
See golua's homepage for further installation details.

# REPL

An example REPL is available in the `cmd` folder.


# Version 2

This is a rewrite of 1.0 with a cleaner API and most features preserved.
The changes are meant to make the library easier to use.

Warning: This is a development version, the API might change.

## Compatibility notice

The main differences with the previous version:

- The function prototypes of `GoToLua` and `LuaToGo` are simpler and do not
require the use of reflection from the callers. The `dontproxify` argument is
gone, use `GoToLuaProxy` to control proxification.

- Use `GoToLua` and `LuaToGo` instead of the `Copy*` functions and `GoLuaFunc`.

- Use `Register` instead of `RawRegister`.

- `InitProxies` is gone as it was not needed.

- Use `NewLuaObjectFromName(L, "_G")` instead of `Global`.

- `Lookup` is unexported. If ever needed, roll up your own function with some
copy/paste.

- Use `(*LuaObject) Call` instead of `(*LuaObject) Callf`. The protoype of
`(*LuaObject) Call` has changed in a fashion similar to `GoToLua` and `LuaToGo`.
`Types` is gone as it is no longer needed.

- Register `ProxyIpairs` and `ProxyPairs` instead of `LuarSetup`.

- Register and use `Unproxify` instead of `ArrayToTable`, `MapToTable`,
`ProxyRaw`, `SliceToTable`, `StructToTable`, .

- `ComplexReal` and `ComplexImag` have been replaced by the proxy attributes
`real` and `imag`, respectively.

- `SliceSub` and `SliceAppend` have been replaced by the proxy methods
`sub` and `append`, respectively.

The range of supported conversion has been extended:

- LuaToGo can convert to interfaces and pointers with several levels of indirection.

- LuaToGo can convert to non-empty maps and structs.
