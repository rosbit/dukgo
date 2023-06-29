# go-duktape, makes duktape be embedded easily

[Duktape](http://duktape.org/index.html) is a thin, embeddable javascript engine.
Its [api](http://duktape.org/api.html) is very well documented. For most of developers
who are not familiar with C language, it is very tedious to call duk\_push\_xxx() and
duk\_pop() to make use of the power of Duktape. Though there are some binding
implementations of Duktape for languages other than C, most of them inherit the
methods of using API of Duktape.

`go-duktape` is a package wrapping Duktape and making it a **pragmatic embeddable** language.
With some helper functions provided by `go-duktape`, calling Golang functions from Javascript, 
or calling Javascript functions from Golang are both very simple. So, with the help of `go-duktape`, Duktape
can be embedded in Golang application easily.

### Install

The package is fully go-getable, So, just type

  `go get github.com/rosbit/go-duktape`

to install.

### Usage

#### 1. Evaluates expressions

```go
package main

import (
  djs "github.com/rosbit/go-duktape"
  "fmt"
)

func main() {
  ctx, err := djs.NewContext()
  if err != nil {
    fmt.Printf("%v\n", err)
    return
  }

  res, _ := ctx.Eval("a + b", map[string]interface{}{
     "a": 10,
     "b": 1,
  })
  fmt.Println("result is:", res)
}
```

#### 2. Go calls Javascript function

Suppose there's a Javascript file named `a.js` like this:

```javascript
function add(a, b) {
    return a+b
}
```

one can call the Javascript function `add()` in Go code like the following:

```go
package main

import (
  djs "github.com/rosbit/go-duktape"
  "fmt"
)

var add func(int, int)int

func main() {
  ctx, err := djs.NewContext()
  if err != nil {
     fmt.Printf("%v\n", err)
     return
  }

  if _, err := ctx.EvalFile("a.js", nil); err != nil {
     fmt.Printf("%v\n", err)
     return
  }

  // method 1: bind JS function with a golang var
  if err := ctx.BindFunc("add", &add); err != nil {
     fmt.Printf("%v\n", err)
     return
  }
  res := add(1, 2)

  // method 2: call JS function using CallFunc
  res, err := ctx.CallFunc("add", 1, 2)
  if err != nil {
     fmt.Printf("%v\n", err)
     return
  }

  fmt.Println("result is:", res)
}
```

#### 3. Javascript calls Go function

Javascript calling Go function is also easy. In the Go code, calling `EvalFile` with a map as env will
make Golang functions as Javascript global functions. There's the example:

```go
package main

import "github.com/rosbit/go-duktape"

// function to be called by Javascript
func adder(a1 float64, a2 float64) float64 {
    return a1 + a2
}

func main() {
  ctx, err := djs.NewContext()
  if err != nil {
      fmt.Printf("%v\n", err)
      return
  }

  if _, err := ctx.EvalFile("b.js", map[string]interface{}{
      "adder": adder,
  })  // b.js containing code calling "adder"
}
```

In Javascript code, one can call the registered function directly. There's the example `b.js`.

```javascript
r = adder(1, 100)   // the function "adder" is implemented in Go
console.log(r)
```

### Status

The package is not fully tested, so be careful.

### Contribution

Pull requests are welcome! Also, if you want to discuss something send a pull request with proposal and changes.

__Convention:__ fork the repository and make changes on your fork in a feature branch.
