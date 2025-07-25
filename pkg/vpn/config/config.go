package config

import (
	"gopkg.in/ini.v1"
)

type Config struct {
	Interface struct {
		PrivateKey string
		Address    string
		DNS        string
	}
	Peers []struct {
		PublicKey    string
		PresharedKey string
		Endpoint     string
		AllowedIPs   string
	}
}

func Parse(filePath string) (*Config, error) {
	cfg, err := ini.Load(filePath)
	if err != nil {
		return nil, err
	}

	config := &Config{}
	iface := cfg.Section("Interface")
	config.Interface.PrivateKey = iface.Key("PrivateKey").String()
	config.Interface.Address = iface.Key("Address").String()
	config.Interface.DNS = iface.Key("DNS").String()

	for _, section := range cfg.Sections() {
		if section.Name() == "Peer" {
			peer := struct {
				PublicKey    string
				PresharedKey string
				Endpoint     string
				AllowedIPs   string
			}{}
			peer.PublicKey = section.Key("PublicKey").String()
			peer.PresharedKey = section.Key("PresharedKey").String()
			peer.Endpoint = section.Key("Endpoint").String()
			peer.AllowedIPs = section.Key("AllowedIPs").String()
			config.Peers = append(config.Peers, peer)
		}
	}

	return config, nil
} 