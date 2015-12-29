##AMQP

Builtin supports using __AMQP__ as both brokers and backends, we implement [__NamedBroker__](https://godoc.org/github.com/mission-liao/dingo#NamedBroker) and [Backend](https://godoc.org/github.com/mission-liao/dingo#Backend) in this component:
```go
import dgamqp // package name is prefixed 'dg' to avoid confliction with "amqp" package

brk, err := dgamqp.NewBroker(nil)  // create a Redis-Broker with default configuration
bkd, err := dgamqp.NewBackend(nil) // create a Redis-Backend with default configuration
```

Internally, a [rabbitmq](www.rabbitmq.com) server would be installer for testing.

There is nothing much to config so far. More option would be added as needed.
```go
import dgamqp

config := dgamqp.DefaultAmqpConfig()
cfg.Host("127.0.0.1") // host address
  .Port(123) // host port
  .User("user123") // user name
  .Password("pwd123") // password
  .MaxChannel(128) // maximum channels preallocated

// create with new configuration
brk, err := dgamqp.NewBroker(cfg)
bkd, err := dgamqp.NewBackend(cfg)
```
