package share

type Server interface {
	Init() error // TODO: allow configuration
	Uninit() error
}
