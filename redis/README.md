##Redis

Builtin supports using __Redis__ as both brokers and backends, we implement [__NamedBroker__](https://godoc.org/github.com/mission-liao/dingo#NamedBroker) and [Backend](https://godoc.org/github.com/mission-liao/dingo#Backend) in this component:
```go
import dgredis // package name is prefixed 'dg' to avoid confliction with "redigo/redis" pacakge

brk, err := dgredis.NewBroker(nil)  // create a Redis-Broker with default configuration
bkd, err := dgredis.NewBackend(nil) // create a Redis-Backend with default configuration
```

There is nothing much to config so far. More option would be added as needed.
```go
import dgredis

config := dgredis.DefaultRedisConfig()
cfg.Host("127.0.0.1") // host address
  .Port(123) // host port
  .Password("pwd123") // password
  .PollTimeout(1) // interval between polling, in seconds
  .MaxIdle(3) // ref(redigo): Maximum number of idle connections in the pool
  .IdleTimeout(240*time.Second) // ref(redigo): Close connections after remaining idle for this duration. If the value
                                // is zero, then idle connections are not closed. Applications should set
                                // the timeout to a value less than the server's timeout.

// create with new configuration
brk, err := dgredis.NewBroker(cfg)
bkd, err := dgredis.NewBackend(cfg)
```
