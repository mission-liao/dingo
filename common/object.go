package common

type Object interface {
	//
	Events() ([]<-chan *Event, error)

	//
	Close() error
}

type EventProducer interface {
	//
}
