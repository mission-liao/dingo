package dingo

type Object interface {
	//
	Events() ([]<-chan *Event, error)

	//
	Close() error
}
