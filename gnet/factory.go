package gnet

import "github.com/pion/transport/v2"

// Compile time assertion
var _ transport.Factory = (*Factory)(nil)

type Factory struct{}

func NewFactory(name string) (*Factory, error) {
	return &Factory{}, nil
}

func (f *Factory) NewNet() (transport.Net, error) {
	n := &Net{}

	return n, nil
}

func (n *Factory) NewRouter(config *transport.RouterConfig) (transport.Router, error) {
	// TODO: Implement
	return nil, errNotImplemented
}
