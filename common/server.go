package common

type Server interface {
	Init() error // TODO: allow configuration
	Close() error
}
