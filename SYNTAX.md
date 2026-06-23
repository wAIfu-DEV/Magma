# Syntax

## Basics

**Variable definition**  
Shape:
> Zero-initialized:      \<var name> \<var type>  
> Manual initialization: \<var name> \<var type> = \<expression>  
> Error destructuring: \<value var name> \<value type>, \<error var name> \<error type> = \<throwing func call>

```
myVar u32 = 0
# is equivalent to
myVar u32
```
```
# Definition from result of throwing function call (without try keyword)
value i32, err error = throwingFunc()
```
---
**Assignment**  
Shape:
> Normal: \<lvalue expression> = \<expression>

```
# variable definition
myVar u32

# assignments
myVar = 0
myVar = someFunc()
```
---
**Lvalue**
```
# Lvalues are expressions that can be assigned to.

# named variables
myVar = 0

# struct members
myStruct.member = 0

# slice index
myArray[i] = 0
```
---
**Function definition**  
Shape:  
> Normal: \<func name> ( \<arg name> \<arg type>, ... ) \<return type> : \<body> ..  
> Throwing: \<func name> ( \<arg name> \<arg type>, ... ) !\<return type> : \<body> ..  
> Public: pub \<func name> ( \<arg name> \<arg type>, ... ) \<return type> : \<body> ..  
> Member: \<owner struct name>.\<member func name> ( \<arg name> \<arg type>, ... ) \<return type> : \<body> ..  
```
pub main(args str[]) !void:
..


```
