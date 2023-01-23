package vnet

import (
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/pion/logging"
	"github.com/pion/transport/v2"
)

var (
	errNATRequriesMapping            = errors.New("1:1 NAT requires more than one mapping")
	errMismatchLengthIP              = errors.New("length mismtach between mappedIPs and localIPs")
	errNonUDPTranslationNotSupported = errors.New("non-udp translation is not supported yet")
	errNoAssociatedLocalAddress      = errors.New("no associated local address")
	errNoNATBindingFound             = errors.New("no NAT binding found")
	errHasNoPermission               = errors.New("has no permission")
)

type natConfig struct {
	name          string
	natType       transport.NATType
	mappedIPs     []net.IP // mapped IPv4
	localIPs      []net.IP // local IPv4, required only when the mode is NATModeNAT1To1
	loggerFactory logging.LoggerFactory
}

type mapping struct {
	proto   string              // "udp" or "tcp"
	local   string              // "<local-ip>:<local-port>"
	mapped  string              // "<mapped-ip>:<mapped-port>"
	bound   string              // key: "[<remote-ip>[:<remote-port>]]"
	filters map[string]struct{} // key: "[<remote-ip>[:<remote-port>]]"
	expires time.Time           // time to expire
}

type networkAddressTranslator struct {
	name           string
	natType        transport.NATType
	mappedIPs      []net.IP            // mapped IPv4
	localIPs       []net.IP            // local IPv4, required only when the mode is NATModeNAT1To1
	outboundMap    map[string]*mapping // key: "<proto>:<local-ip>:<local-port>[:remote-ip[:remote-port]]
	inboundMap     map[string]*mapping // key: "<proto>:<mapped-ip>:<mapped-port>"
	udpPortCounter int
	mutex          sync.RWMutex
	log            logging.LeveledLogger
}

func newNAT(config *natConfig) (*networkAddressTranslator, error) {
	natType := config.natType

	if natType.Mode == transport.NATModeNAT1To1 {
		// 1:1 NAT behavior
		natType.MappingBehavior = transport.EndpointIndependent
		natType.FilteringBehavior = transport.EndpointIndependent
		natType.PortPreservation = true
		natType.MappingLifeTime = 0

		if len(config.mappedIPs) == 0 {
			return nil, errNATRequriesMapping
		}
		if len(config.mappedIPs) != len(config.localIPs) {
			return nil, errMismatchLengthIP
		}
	} else {
		// Normal (NAPT) behavior
		natType.Mode = transport.NATModeNormal
		if natType.MappingLifeTime == 0 {
			natType.MappingLifeTime = transport.DefaultNATMappingLifeTime
		}
	}

	return &networkAddressTranslator{
		name:        config.name,
		natType:     natType,
		mappedIPs:   config.mappedIPs,
		localIPs:    config.localIPs,
		outboundMap: map[string]*mapping{},
		inboundMap:  map[string]*mapping{},
		log:         config.loggerFactory.NewLogger("vnet"),
	}, nil
}

func (n *networkAddressTranslator) getPairedMappedIP(locIP net.IP) net.IP {
	for i, ip := range n.localIPs {
		if ip.Equal(locIP) {
			return n.mappedIPs[i]
		}
	}
	return nil
}

func (n *networkAddressTranslator) getPairedLocalIP(mappedIP net.IP) net.IP {
	for i, ip := range n.mappedIPs {
		if ip.Equal(mappedIP) {
			return n.localIPs[i]
		}
	}
	return nil
}

func (n *networkAddressTranslator) translateOutbound(from Chunk) (Chunk, error) {
	n.mutex.Lock()
	defer n.mutex.Unlock()

	to := from.Clone()

	if from.Network() == udp {
		if n.natType.Mode == transport.NATModeNAT1To1 {
			// 1:1 NAT behavior
			srcAddr := from.SourceAddr().(*net.UDPAddr) //nolint:forcetypeassert
			srcIP := n.getPairedMappedIP(srcAddr.IP)
			if srcIP == nil {
				n.log.Debugf("[%s] drop outbound chunk %s with not route", n.name, from.String())
				return nil, nil // nolint:nilnil
			}
			srcPort := srcAddr.Port
			if err := to.setSourceAddr(fmt.Sprintf("%s:%d", srcIP.String(), srcPort)); err != nil {
				return nil, err
			}
		} else {
			// Normal (NAPT) behavior
			var bound, filterKey string
			switch n.natType.MappingBehavior {
			case transport.EndpointIndependent:
				bound = ""
			case transport.EndpointAddrDependent:
				bound = from.getDestinationIP().String()
			case transport.EndpointAddrPortDependent:
				bound = from.DestinationAddr().String()
			}

			switch n.natType.FilteringBehavior {
			case transport.EndpointIndependent:
				filterKey = ""
			case transport.EndpointAddrDependent:
				filterKey = from.getDestinationIP().String()
			case transport.EndpointAddrPortDependent:
				filterKey = from.DestinationAddr().String()
			}

			oKey := fmt.Sprintf("udp:%s:%s", from.SourceAddr().String(), bound)

			m := n.findOutboundMapping(oKey)
			if m == nil {
				// Create a new mapping
				mappedPort := 0xC000 + n.udpPortCounter
				n.udpPortCounter++

				m = &mapping{
					proto:   from.SourceAddr().Network(),
					local:   from.SourceAddr().String(),
					bound:   bound,
					mapped:  fmt.Sprintf("%s:%d", n.mappedIPs[0].String(), mappedPort),
					filters: map[string]struct{}{},
					expires: time.Now().Add(n.natType.MappingLifeTime),
				}

				n.outboundMap[oKey] = m

				iKey := fmt.Sprintf("udp:%s", m.mapped)

				n.log.Debugf("[%s] created a new NAT binding oKey=%s iKey=%s",
					n.name,
					oKey,
					iKey)

				m.filters[filterKey] = struct{}{}
				n.log.Debugf("[%s] permit access from %s to %s", n.name, filterKey, m.mapped)
				n.inboundMap[iKey] = m
			} else if _, ok := m.filters[filterKey]; !ok {
				n.log.Debugf("[%s] permit access from %s to %s", n.name, filterKey, m.mapped)
				m.filters[filterKey] = struct{}{}
			}

			if err := to.setSourceAddr(m.mapped); err != nil {
				return nil, err
			}
		}

		n.log.Debugf("[%s] translate outbound chunk from %s to %s", n.name, from.String(), to.String())

		return to, nil
	}

	return nil, errNonUDPTranslationNotSupported
}

func (n *networkAddressTranslator) translateInbound(from Chunk) (Chunk, error) {
	n.mutex.Lock()
	defer n.mutex.Unlock()

	to := from.Clone()

	if from.Network() == udp {
		if n.natType.Mode == transport.NATModeNAT1To1 {
			// 1:1 NAT behavior
			dstAddr := from.DestinationAddr().(*net.UDPAddr) //nolint:forcetypeassert
			dstIP := n.getPairedLocalIP(dstAddr.IP)
			if dstIP == nil {
				return nil, fmt.Errorf("drop %s as %w", from.String(), errNoAssociatedLocalAddress)
			}
			dstPort := from.DestinationAddr().(*net.UDPAddr).Port //nolint:forcetypeassert
			if err := to.setDestinationAddr(fmt.Sprintf("%s:%d", dstIP, dstPort)); err != nil {
				return nil, err
			}
		} else {
			// Normal (NAPT) behavior
			iKey := fmt.Sprintf("udp:%s", from.DestinationAddr().String())
			m := n.findInboundMapping(iKey)
			if m == nil {
				return nil, fmt.Errorf("drop %s as %w", from.String(), errNoNATBindingFound)
			}

			var filterKey string
			switch n.natType.FilteringBehavior {
			case transport.EndpointIndependent:
				filterKey = ""
			case transport.EndpointAddrDependent:
				filterKey = from.getSourceIP().String()
			case transport.EndpointAddrPortDependent:
				filterKey = from.SourceAddr().String()
			}

			if _, ok := m.filters[filterKey]; !ok {
				return nil, fmt.Errorf("drop %s as the remote %s %w", from.String(), filterKey, errHasNoPermission)
			}

			// See RFC 4847 Section 4.3.  Mapping Refresh
			// a) Inbound refresh may be useful for applications with no outgoing
			//   UDP traffic.  However, allowing inbound refresh may allow an
			//   external attacker or misbehaving application to keep a mapping
			//   alive indefinitely.  This may be a security risk.  Also, if the
			//   process is repeated with different ports, over time, it could
			//   use up all the ports on the NAT.

			if err := to.setDestinationAddr(m.local); err != nil {
				return nil, err
			}
		}

		n.log.Debugf("[%s] translate inbound chunk from %s to %s", n.name, from.String(), to.String())

		return to, nil
	}

	return nil, errNonUDPTranslationNotSupported
}

// caller must hold the mutex
func (n *networkAddressTranslator) findOutboundMapping(oKey string) *mapping {
	now := time.Now()

	m, ok := n.outboundMap[oKey]
	if ok {
		// check if this mapping is expired
		if now.After(m.expires) {
			n.removeMapping(m)
			m = nil // expired
		} else {
			m.expires = time.Now().Add(n.natType.MappingLifeTime)
		}
	}

	return m
}

// caller must hold the mutex
func (n *networkAddressTranslator) findInboundMapping(iKey string) *mapping {
	now := time.Now()
	m, ok := n.inboundMap[iKey]
	if !ok {
		return nil
	}

	// check if this mapping is expired
	if now.After(m.expires) {
		n.removeMapping(m)
		return nil
	}

	return m
}

// caller must hold the mutex
func (n *networkAddressTranslator) removeMapping(m *mapping) {
	oKey := fmt.Sprintf("%s:%s:%s", m.proto, m.local, m.bound)
	iKey := fmt.Sprintf("%s:%s", m.proto, m.mapped)

	delete(n.outboundMap, oKey)
	delete(n.inboundMap, iKey)
}
