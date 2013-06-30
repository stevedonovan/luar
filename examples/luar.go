package main

import (
	"fmt"
    "regexp"
	"github.com/GeertJohan/go.linenoise"
    "github.com/stevedonovan/luar"
)

const dumper = `
local tostring = tostring
local append = table.insert
local quote = function(v)
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
  local buff = {
    tables = {
      [t] = true
    }
  }
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
`

func main() {
    L := luar.Init()
    defer L.Close()
    
    err := L.DoString(dumper)
    if err != nil {
        fmt.Println("initial " + err.Error())
        return
    }
    
    M := luar.Map {
        "one":1,
        "two":2,
        "three":3,        
    }
    
    // put here Go functions or values you want to use interactively!
    luar.Register(L,"",luar.Map {
        "regexp":regexp.Compile,
        "M":M,
    })
    
    fmt.Println("luar prompt")
	fmt.Println("Lua 5.1.4  Copyright (C) 1994-2008 Lua.org, PUC-Rio")
	for {
		str := linenoise.Line("> ")
        if len(str) > 0 {
            if len(str) == 1 || str[0] == 0 {
                return
            } else
            if str == "exit" {
                return
            }
            linenoise.AddHistory(str)
            if str[0] == '=' {
                str = "print(" + str[1:] + ")"
            }
            err := L.DoString(str)
            if err != nil {
                fmt.Println(err)
            }
        } else {
            fmt.Println("ding!")
        }
	}
}

