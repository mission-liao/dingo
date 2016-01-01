##Stateful Workers Functions
> [Next: Execution Option](option.md)

Two ways to provide a stateful workers for recording or enclosing globals:
 - [closure](stateful_worker.md#closure)
 - [method](stateful_worker.md#method)

Two things to note before you step in:
 - unless you only allocate 1 worker, or you have to avoid race condition of your state.
 - the function signature should be sync between __Caller__ and __Worker__, but they don't have to the same function/method: ex. you register a method of struct at worker-side to keep states, but you just need to register a function at caller-side, as long as their signature is the same.

###Closure
Suppose you have a database connection that should be created/maintained by __Worker__ side, and your worker function need that:
```go
// original worker function
func NewUser(name, email string) {
    ...
    // need a db connection,
    // one way to provide a db connection is by put it in global,
    //but the better way is enclosing it in closure
}

// a function to generate a closure
func genNewUser(db *db_conn) func (string, string) {
    return func(name, email string) {
        // it's safe to access 'db' as you db connection here
    }
}

// register it to dingo
app.Register("NewUser", genNewUser( /* provide the db connection here */ ))
```

###Method
Or you can define a struct to hold everything you need:
```go
//
// at worker side
//
type MyState struct {
    db *conn
}
func (me *MyState) NewUser(name, mail string) {
    // access db connection via 'me.db'
}

// create a new instance, register its method to dingo as worker function.
app.Register("NewUser", &MyState{db: your_db_conn}.NewUser)

//
// at caller side,
// you don't have to provide another instance, just make sure
// the signature is the same
//
app.Register("NewUser", func(string, string){})
```
