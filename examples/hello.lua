print(const)
print('tos',tos())
s = slice()
--~ mt = debug.getmetatable(s)
--~ print(mt,mt.__index)
print("slice index",s[2],#s)
m = mapr()

print("map index",m.one,#m)
m.four = '4'
print(m.four,#m )
print(gotslice{10,20,30})

st = structz()
print("struct fields", st.Name,st.Age)

print("calling method",st.Method("h"),st.GetName())

any(10)
any("hello")
--any {5,2,3}

 mapit ({one=1,two=2},{[2]="hello",[3]="dolly"})

-- mapit {one=1,two=2}


