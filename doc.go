/*
   luar provides a more convenient way to access Lua from Go, using
   Alessandro Arzilli's  golua (https://github.com/aarzilli/golua).
   Plain Go functions can be registered with luar and they will be called by reflection;
   methods on Go structs likewise.

   // import ...

   type MyStruct struct {
     Name string
     Age int
   }

   const test = `
   for i = 1,5 do
       Print(MSG,i)
   end
   Print(ST)
   print(ST.Name,ST.Age)
   --// slices!
   for i,v in pairs(S) do
      print(i,v)
   end
   `

   func main() {
       L := luar.Init()
       defer L.Close()

       S := []string {"alfred","alice","bob","frodo"}

       ST := &MyStruct{"Dolly",46}

       luar.Register(L,"",luar.Map{
           "Print":fmt.Println,
           "MSG":"hello",  // can also register constants
           "ST":ST, // and other values
           "S":S,
       })

       L.DoString(test)

    }

   Go types like slices, maps and structs are passed over as Lua proxy objects,
   or alternatively copied as tables.
*/
package luar
