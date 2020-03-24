# sipp
[![GoDoc](https://godoc.org/github.com/tidwall/sipp?status.svg)](https://godoc.org/github.com/tidwall/sipp)


Simple interprocess plugins

## Create a plugin 

Here's a plugin that provides a simple echo service.

```go
package main

import "github.com/tidwall/sipp"

func main() {
    sipp.Handle(func(input []byte) []byte {
        return input
    })
}
```

Save to echo.go and compile:

```
go build echo.go
```

You now have a plugin named `echo`.

## Use the plugin

```go
package main

import "github.com/tidwall/sipp"

func main() {
    p, err := sipp.Open("echo")
    if err != nil{
        panic(err)
    }
    println(p.Send([]byte("hello")).Output())
}
```

Prints `hello`


## Contact

Josh Baker [@tidwall](http://twitter.com/tidwall)

## License

Source code is available under the MIT [License](/LICENSE).