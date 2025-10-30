package registry

type Registry interface {
	Start() error
	Stop() error
}
