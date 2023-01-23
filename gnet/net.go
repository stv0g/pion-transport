package gnet

import (
	"errors"
	"net"
	"time"

	"github.com/pion/transport/v2"
	g "github.com/stv0g/gont/pkg"
)

var errNotImplemented = errors.New("not implemented yet")

type Net struct {
	node g.Node
}

// ListenPacket announces on the local network address.
func (n *Net) ListenPacket(network string, address string) (conn net.PacketConn, err error) {
	err = n.node.RunFunc(func() error {
		conn, err = net.ListenPacket(network, address)
		return err
	})
	return
}

// ListenUDP acts like ListenPacket for UDP networks.
func (n *Net) ListenUDP(network string, lAddr *net.UDPAddr) (conn transport.UDPConn, err error) {
	err = n.node.RunFunc(func() error {
		conn, err = net.ListenUDP(network, lAddr)
		return err
	})
	return
}

// Dial connects to the address on the named network.
func (n *Net) Dial(network, address string) (conn net.Conn, err error) {
	err = n.node.RunFunc(func() error {
		conn, err = net.Dial(network, address)
		return err
	})
	return
}

// DialUDP acts like Dial for UDP networks.
func (n *Net) DialUDP(network string, lAddr, rAddr *net.UDPAddr) (conn transport.UDPConn, err error) {
	err = n.node.RunFunc(func() error {
		conn, err = net.DialUDP(network, lAddr, rAddr)
		return err
	})
	return
}

// DialTCP acts like Dial for TCP networks.
func (n *Net) DialTCP(network string, lAddr, rAddr *net.TCPAddr) (conn transport.TCPConn, err error) {
	err = n.node.RunFunc(func() error {
		conn, err = net.DialTCP(network, lAddr, rAddr)
		return err
	})
	return
}

// ResolveIPAddr returns an address of IP end point.
func (n *Net) ResolveIPAddr(network, address string) (*net.IPAddr, error) {
	// TODO: Implement
	return nil, errNotImplemented
}

// ResolveUDPAddr returns an address of UDP end point.
func (n *Net) ResolveUDPAddr(network, address string) (*net.UDPAddr, error) {
	// TODO: Implement
	return nil, errNotImplemented
}

// ResolveTCPAddr returns an address of TCP end point.
func (n *Net) ResolveTCPAddr(network, address string) (*net.TCPAddr, error) {
	// TODO: Implement
	return nil, errNotImplemented
}

// Interfaces returns a list of the system's network interfaces.
func (n *Net) Interfaces() ([]*transport.Interface, error) {
	// TODO: Implement
	return nil, errNotImplemented
}

// InterfaceByIndex returns the interface specified by index.
func (n *Net) InterfaceByIndex(index int) (*transport.Interface, error) {
	// TODO: Implement
	return nil, errNotImplemented
}

// InterfaceByName returns the interface specified by name.
func (n *Net) InterfaceByName(name string) (*transport.Interface, error) {
	// TODO: Implement
	return nil, errNotImplemented
}

func (n *Net) CreateDialer(dialer *net.Dialer) transport.Dialer {
	// TODO: Implement
	return nil
}

type TCPListener struct {
	net.Listener
}

// AcceptTCP accepts the next incoming call and returns the new
// connection.
func (l *TCPListener) AcceptTCP() (transport.TCPConn, error) {
	// TODO: Implement
	return nil, errNotImplemented
}

// SetDeadline sets the deadline associated with the listener.
// A zero time value disables the deadline.
func (l *TCPListener) SetDeadline(t time.Time) error {
	// TODO: Implement
	return errNotImplemented
}

// ListenTCP acts like Listen for TCP networks.
func (n *Net) ListenTCP(network string, lAddr *net.TCPAddr) (lst transport.TCPListener, err error) {
	var netLst *net.TCPListener
	err = n.node.RunFunc(func() error {
		netLst, err = net.ListenTCP(network, lAddr)
		return err
	})

	lst = &TCPListener{
		Listener: netLst,
	}

	return
}
