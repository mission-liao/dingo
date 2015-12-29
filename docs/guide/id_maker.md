## ID Maker
[Next](README.md)

The way that __Dingo__ generates new IDs for new tasks can be customized. It relies on the interface [dingo.IDMaker](https://godoc.org/github.com/mission-liao/dingo#IDMaker) which has these methods:
```go
NewID() (string, error)
```
Internally, __Dingo__ uses uuid4 for IDs, its uniqueness is 99.9999% promised, but not 100%. In production, you may rely on some distributed ID generator like:
 - http://horicky.blogspot.tw/2007/11/distributed-uuid-generation.html
 - Any SQL database that have sequence type

Two things to note:
 - No matter what type you generated, convert it to string
 - The uniqueness of task/report is by comparing __(name, id)__ pairs, so you can have separated __IDMakers__ for different kinds of tasks.
