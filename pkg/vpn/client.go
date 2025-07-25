package vpn

import (
	"context"
	"fmt"
	"net"

	"github.com/armon/go-socks5"
	"github.com/theapemachine/a2a-go/pkg/vpn/config"
	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/link/fdbased"
	"gvisor.dev/gvisor/pkg/tcpip/network/arp"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv4"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv6"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"gvisor.dev/gvisor/pkg/tcpip/transport/tcp"
	"gvisor.dev/gvisor/pkg/tcpip/transport/udp"
)

type Client struct {
	device *device.Device
	stack  *stack.Stack
	linkID tcpip.NICID
}

func NewClient(configFile string) (*Client, error) {
	cfg, err := config.Parse(configFile)
	if err != nil {
		return nil, err
	}

	tun, err := tun.CreateTUN("utun", 1500)
	if err != nil {
		return nil, err
	}

	dev := device.NewDevice(tun, conn.NewDefaultBind(), device.NewLogger(device.LogLevelVerbose, ""))

	s := stack.New(stack.Options{
		NetworkProtocols: []stack.NetworkProtocolFactory{
			ipv4.NewProtocol,
			ipv6.NewProtocol,
			arp.NewProtocol,
		},

		TransportProtocols: []stack.TransportProtocolFactory{
			tcp.NewProtocol,
			udp.NewProtocol,
		},
	})

	linkID := tcpip.NICID(1)
	linkEP, err := fdbased.New(&fdbased.Options{
		FDs: []int{int(tun.File().Fd())},
	})
	if err != nil {
		return nil, fmt.Errorf("could not create link endpoint: %v", err)
	}

	if err := s.CreateNIC(linkID, linkEP); err != nil {
		return nil, fmt.Errorf("could not create netstack NIC: %v", err)
	}

	ipcRequest := fmt.Sprintf("private_key=%s\n", cfg.Interface.PrivateKey)
	for _, peer := range cfg.Peers {
		ipcRequest += fmt.Sprintf("public_key=%s\nendpoint=%s\nallowed_ip=%s\n", peer.PublicKey, peer.Endpoint, peer.AllowedIPs)
		if peer.PresharedKey != "" {
			ipcRequest += fmt.Sprintf("preshared_key=%s\n", peer.PresharedKey)
		}
	}

	if err := dev.IpcSet(ipcRequest); err != nil {
		return nil, err
	}

	return &Client{device: dev, stack: s, linkID: linkID}, nil
}

func (c *Client) Up() error {
	err := c.device.Up()
	if err != nil {
		return err
	}

	return nil
}

func (c *Client) Down() {
	c.device.Close()
}

func (c *Client) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	remote, err := net.ResolveTCPAddr(network, address)
	if err != nil {
		return nil, err
	}
	return gonet.DialTCP(c.stack, tcpip.FullAddress{
		NIC:  c.linkID,
		Addr: tcpip.AddrFrom4([4]byte(remote.IP.To4())),
		Port: uint16(remote.Port),
	}, ipv4.ProtocolNumber)
}

func (c *Client) StartProxy() (string, error) {
	conf := &socks5.Config{
		Dial: c.DialContext,
	}

	server, err := socks5.New(conf)
	if err != nil {
		return "", err
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}

	go func() {
		if err := server.Serve(listener); err != nil {
			// handle error
		}
	}()

	return listener.Addr().String(), nil
}
