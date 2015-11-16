package common

var InstT = struct {
	DEFAULT  int
	REPORTER int
	STORE    int
	PRODUCER int
	CONSUMER int
	MONITOR  int
	MAPPER   int
	WORKER   int
	BRIDGE   int
	ALL      int
}{
	0,
	(1 << 0),
	(1 << 1),
	(1 << 2),
	(1 << 3),
	(1 << 4),
	(1 << 5),
	(1 << 6),
	(1 << 7),
	int(^uint(0) >> 1), // Max int
}

type Object interface {
	//
	Events() ([]<-chan *Event, error)

	//
	Close() error
}

type EventProducer interface {
	//
}
