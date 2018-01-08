package haci

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	neturl "net/url"
	"strings"

	ccidr "github.com/apparentlymart/go-cidr/cidr"
	"gopkg.in/jmcvetta/napping.v3"
)

type Network struct {
	CreateDate  string   `json:"createDate"`
	CreateFrom  string   `json:"createFrom"`
	Description string   `json:"description"`
	Network     string   `json:"network"`
	Tags        []string `json:"tags"`
}

func (n Network) IP() (string, error) {
	ip, _, err := net.ParseCIDR(n.Network)

	if err != nil {
		return "", err
	}

	return ip.String(), nil
}

type Client interface {
	Get(network string) (Network, error)
	List(network string) ([]Network, error)
	Assign(network string, description string, cidr int, tags []string) (Network, error)
	Delete(network string) error
}

type WebClient struct {
	napping napping.Session
	URL     string
	Root    string
}

// A very simple and limited client for unit tests.
type FakeClient struct {
	Supernets map[string]*FakeSupernet
}

type FakeSupernet struct {
	Networks map[string]Network
	Network  net.IPNet
	Last     net.IP
}

func NewWebClient(url, username, password, root string) (haci *WebClient, err error) {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: transport}

	haci = &WebClient{
		napping: napping.Session{
			Log:      false,
			Client:   client,
			Userinfo: neturl.UserPassword(username, password),
		},
		URL:  strings.TrimRight(url, "/"),
		Root: root,
	}
	return
}

func NewFakeClient() *FakeClient {
	return &FakeClient{Supernets: map[string]*FakeSupernet{}}
}

func (c *WebClient) Get(network string) (network1 Network, err error) {
	resp, err := c.napping.Get(c.URL+"/RESTWrapper/getNetworkDetails",
		&neturl.Values{
			"rootName": {c.Root},
			"network":  {network},
		},
		&network1,
		nil)

	if err != nil {
		return Network{}, err
	}

	if resp.Status() != 200 {
		return Network{}, fmt.Errorf("lookup failed: %s", resp.RawText())
	}

	return
}

func (c *WebClient) List(network string) (networks []Network, err error) {
	resp, err := c.napping.Get(c.URL+"/RESTWrapper/getSubnets",
		&neturl.Values{
			"rootName": {c.Root},
			"supernet": {network},
		},
		&networks,
		nil)

	if err != nil {
		return []Network{}, err
	}

	if resp.Status() != 200 {
		return []Network{}, fmt.Errorf("list failed: %s", resp.RawText())
	}

	return
}

func (c *WebClient) Assign(network, description string, cidr int, tags []string) (network1 Network, err error) {
	resp, err := c.napping.Get(c.URL+"/RESTWrapper/assignFreeSubnet",
		&neturl.Values{
			"rootName":    {c.Root},
			"supernet":    {network},
			"description": {description},
			"cidr:":       {string(cidr)},
		},
		&network1,
		nil)

	if err != nil {
		return Network{}, err
	}

	if resp.Status() != 200 {
		return Network{}, fmt.Errorf("assignment failed: %s", resp.RawText())
	}

	return
}

func (c *WebClient) Delete(network string) (err error) {
	resp, err := c.napping.Get(c.URL+"/RESTWrapper/delNet",
		&neturl.Values{
			"rootName": {c.Root},
			"network":  {network},
		},
		nil,
		nil)

	if err != nil {
		return err
	}

	if resp.Status() != 200 {
		return fmt.Errorf("delete failed: %s", resp.RawText())
	}

	return
}

func (c *FakeClient) Get(network string) (Network, error) {
	for _, s := range c.Supernets {
		if n, ok := s.Networks[network]; ok {
			return n, nil
		}
	}
	return Network{}, nil
}

func (c *FakeClient) List(network string) (networks []Network, err error) {
	if s, ok := c.Supernets[network]; ok {
		for _, n := range s.Networks {
			networks = append(networks, n)
		}
	}

	return
}

func (c *FakeClient) Assign(network, description string, cidr int, tags []string) (network1 Network, err error) {

	ip, net, err := net.ParseCIDR(network)
	if err != nil {
		return Network{}, err
	}

	if _, ok := c.Supernets[network]; !ok {
		c.Supernets[network] = &FakeSupernet{Network: *net, Networks: map[string]Network{}, Last: ip}
	}

	newip := ccidr.Inc(c.Supernets[network].Last)
	netname := fmt.Sprintf("%s/32", newip.String())

	network1 = Network{
		Network:     netname,
		Description: description,
		Tags:        tags,
	}

	c.Supernets[network].Networks[netname] = network1
	c.Supernets[network].Last = newip

	return
}

func (c *FakeClient) Delete(network string) error {
	for _, s := range c.Supernets {
		delete(s.Networks, network)
	}
	return nil
}
