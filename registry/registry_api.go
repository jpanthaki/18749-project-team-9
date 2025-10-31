package registry

type Registry interface {
	Start() error
	Stop() error
	Lookup(role string, id string) (string, error) //Lookup a role, id pair
	Register(role string, id string, addr string)  //Register an address
}
