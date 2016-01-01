##Configuration
> [Next: Stateful Worker Functions](stateful_worker_function.md)

There is nothing much to config when initiating __Dingo__:
```go
import "github.com/mission-liao/dingo"

// initiate with default configuration
app, err := dingo.NewApp("local", nil)

config := dingo.DefaultConfig()
config.Mappers(3) // set the count of mappers to 3
app, err := dingo.NewApp("local", config)
```

For configuration for database adaptors, please refer to package README.
 - [AMQP](../amqp/README.md)
 - [Redis](../redis/README.md)
