/* A golua REPL with line editing, pretty-printing and tab completion.
    Import any Go functions and values into Lua and play with them
    interactively!    
*/
package main

import (  
	"fmt"    
    "strings"
	"github.com/GeertJohan/go.linenoise"
    "github.com/stevedonovan/luar"
)

// your packages go here
import (  
    "regexp"
    "reflect"
)

type MyStruct struct {
    Name string
    Age int
}


func main() {
    L := luar.Init()    
    
   	defer func() {
        L.Close()
		if x := recover(); x != nil {
			fmt.Println("runtime "+x.(error).Error())
		}
	}()
    
    err := L.DoString(lua_code)
    if err != nil {
        fmt.Println("initial " + err.Error())
        return
    } 
   
    complete := luar.NewLuaObjectFromName(L,"lua_candidates")           
    
    // Go functions or values you want to use interactively!    
    ST := &MyStruct{"Dolly",46}    
  
    luar.Register(L,"",luar.Map {
        "regexp":regexp.Compile,
        "println":fmt.Println,
        "ST":ST,
    })
    
    luar.Register(L,"reflect",luar.Map {
        "ValueOf":reflect.ValueOf,
    })
    
    fmt.Println("luar 1.2 Copyright (C) 2013 Steve Donovan")
	fmt.Println("Lua 5.1.4  Copyright (C) 1994-2008 Lua.org, PUC-Rio")
    linenoise.SetCompletionHandler(func(in string) []string {
        val,err := complete.Call(in)
        if err != nil {
            return []string{}
        } else {
            is :=  val.([]interface{})
            out := make([]string,len(is))
            for i,s := range is {
                out[i] = s.(string)
            }
            return out
        }
    })
	for {
    //    /* // ctrl-C/ctrl-D handling with ctrlc branch of go.linenoise
		str,err := linenoise.Line("> ")
        if err != nil {
            return
        }
       // */
        //str := linenoise.Line("> ")
        if len(str) > 0 {
            if str == "exit" {
                return
            }
            linenoise.AddHistory(str)
            if str[0] == '=' {
                str = "print(" + str[1:] + ")"
            }
            err := L.DoString(str)
            if err != nil {
                errs := err.Error()
                idx := strings.Index(errs,": ")
                if idx > -1 {
                    errs = errs[idx+2:]
                }
                fmt.Println(errs)
            }
        } else {
            fmt.Println("empty line. Use exit to get out")
        }
	}
}

const lua_code = `
local tostring = tostring
local append = table.insert
local function quote (v)
  if type(v) == 'string' then
    return ('%q'):format(v)
  else
    return tostring(v)
  end
end
local dump
dump = function(t, options)
  options = options or { }
  local limit = options.limit or 1000
  local buff = {tables={}}
  if type(t) == 'table' then
      buff.tables[t] = true
  end
  local k, tbuff = 1, nil
  local function put(v)
    buff[k] = v
    k = k + 1
  end
  local function put_value(value)
    if type(value) ~= 'table' then
      put(quote(value))
      if limit and k > limit then
        buff[k] = "..."
        error("buffer overrun")
      end
    else
      if not buff.tables[value] then
        buff.tables[value] = true
        tbuff(value)
      else
        put("<cycle>")
      end
    end
    return put(',')
  end
  function tbuff(t)
    local mt
    if not (options.raw) then
      mt = getmetatable(t)
    end
    if mt and mt.__tostring then
      return put(quote(t))
    elseif type(t) ~= 'table' and not (mt and mt.__pairs) then
      return put(quote(t))
    else
      put('{')
      local mt_pairs, indices = mt and mt.__pairs
      if not mt_pairs and #t > 0 then 
        indices = {}
        for i = 1, #t do
          indices[i] = true
        end
      end
      for key, value in pairs(t) do
        local _continue_0 = false
        repeat
          if indices and indices[key] then
            _continue_0 = true
            break
          end
          if type(key) ~= 'string' then
            key = '[' .. tostring(key) .. ']'
          elseif key:match('%s') then
            key = quote(key)
          end
          put(key .. ':')
          put_value(value)
          _continue_0 = true
        until true
        if not _continue_0 then
          break
        end
      end
      if indices then
        local _list_0 = t
        for _index_0 = 1, #_list_0 do
          local v = _list_0[_index_0]
          put_value(v)
        end
      end
      if buff[k - 1] == "," then
        k = k - 1
      end
      return put('}')
    end
  end
  tbuff(t)
  --pcall(tbuff, t)
  return table.concat(buff)
end
_G.tostring = dump

local append = table.insert

local function is_pair_iterable(t)
    local mt = getmetatable(t)
    return type(t) == 'table' or (mt and mt.__pairs)
end

function lua_candidates(line)
  -- identify the expression!
  local i1,i2 = line:find('[.:%w_]+$')
  if not i1 then return end
  local front,partial = line:sub(1,i1-1), line:sub(i1)
  local prefix, last = partial:match '(.-)([^.:]*)$'
  local t, all = _G
  local res = {}
  if #prefix > 0 then        
    local P = prefix:sub(1,-2)
    all = last == ''
    for w in P:gmatch '[^.:]+' do
      t = t[w]
      if not t then
        res = {line}
        return res
      end
    end
  end
  prefix = front .. prefix  
  local function append_candidates(t)  
    for k,v in pairs(t) do
      if all or k:sub(1,#last) == last then
        append(res,prefix..k)
      end
    end
  end
  local mt = getmetatable(t)
  if is_pair_iterable(t) then
    append_candidates(t)
  end
  if mt and is_pair_iterable(mt.__index) then
    append_candidates(mt.__index)
  end
  if #res == 0 then append(res,line) end
  return res
end

`
