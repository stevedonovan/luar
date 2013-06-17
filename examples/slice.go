package main

import "fmt"
import "github.com/stevedonovan/luar"

type SomeObject struct {
  Name string
}

func (o *SomeObject) GetName() string {
  return o.Name
}

type StructWithSlice struct {
  Slice [] SomeObject
}

func (s *StructWithSlice) GetSlice() []SomeObject {
  return s.Slice
}

func NewStructWithSlice(initial string) StructWithSlice {
  r := StructWithSlice{}
  r.Slice = append(r.Slice, SomeObject{initial})
  return r
}

func main() {
  lua := `
    fn = function(obj)
      slice = obj.GetSlice()
      print(type(slice), #slice)
      obj = slice[1]  -- slice[2] will raise a 'index out of range' error
      -- obj.Naam = 'howzit'  -- will raise a 'no field' error
      name = obj.GetName() 
      return name
    end
  `
  L := luar.Init()
  defer L.Close()

  L.DoString(lua)

  luafn := luar.NewLuaObjectFromName(L, "fn")
  gobj := NewStructWithSlice("string")
  res, err := luafn.Call(gobj)
  if err != nil {
    fmt.Println("error!",err);
  } else {
    fmt.Println("result",res)
  }
}
