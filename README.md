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

# Issues

The `GoToLua` and `LuaToGo` functions take a `reflect.Type` parameter, which is
bad design. Sadly changing this would break backward compatibility.
