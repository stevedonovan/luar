/*
Package luar provides a convenient way to access Lua from Go.
It uses Alessandro Arzilli's golua (https://github.com/aarzilli/golua). Plain Go
functions can be registered with luar and they will be called by reflection;
methods on Go structs likewise.

Go types like slices, maps and structs are passed over as Lua proxy objects,
or alternatively copied as tables.

You may pass a Lua table to an imported Go function; if the table is
'array-like' then it can be converted to a Go slice; if it is 'map-like' then it
is converted to a Go map. Usually non-primitive Go values are passed to Lua as
wrapped userdata which can be naturally indexed if they represent slices, maps
or structs. Methods defined on structs can be called, again using reflection. Do
note that these methods will be callable using _dot-notation_ rather than colon
notation!

Pointer values encode as the value pointed to when unproxified.

The "lua" tag is used to match fields in struct conversion.

*/
package luar
