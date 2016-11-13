/*
Package luar provides a convenient way to access Lua from Go.

It uses Alessandro Arzilli's golua (https://github.com/aarzilli/golua).

Most Go values can be passed to Lua: basic types, strings, complex numbers,
user-defined types, pointers, composite types, functions, channels, etc.

Composite types are processed recursively.

Methods can be called on user-defined types. That these methods will be callable
using _dot-notation_ rather than colon notation.

Slices, maps and structs can be copied as tables, or alternatively passed over
as Lua proxy objects which can be naturally indexed.

In the case of structs and string maps, fields have priority over methods. To
call shadowed methods, use 'luar.method(<value>, <method>)(<params>...)'.

Unexported struct fields are ignored. The "lua" tag is used to match fields in
struct conversion.

You may pass a Lua table to an imported Go function; if the table is
'array-like' then it is converted to a Go slice; if it is 'map-like' then it
is converted to a Go map.

Pointer values encode as the value pointed to when unproxified.

Usual operators (arithmetic, string concatenation, pairs/ipairs, etc.) work on
proxies too. The type of the result depends on the type of the operands. The
rules are as follows:

- If the operands are of the same type, use this type.

- If one type is a Lua number, use the other, user-defined type.

- If the types are different and not Lua numbers, convert to complex128 (proxy),
Lua number, or Lua string according to the result kind.

*/
package luar
