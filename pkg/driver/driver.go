package driver

import (
	"encoding/json"
	"io/ioutil"

	"github.com/StackPointCloud/csi-packet/pkg/cloud_provider"
)

type PacketDriver struct {
	name     string
	nodeID   string
	endpoint string
	config   cloud_provider.Config
}

func NewPacketDriver(endpoint, nodeID, configurationPath string) (*PacketDriver, error) {

	var config cloud_provider.Config
	if configurationPath != "" {
		configBytes, err := ioutil.ReadFile(configurationPath)
		if err != nil {
			return nil, err
		}
		err = json.Unmarshal(configBytes, &config)
		if err != nil {
			return nil, err
		}
	}
	return &PacketDriver{
		name:     "csi-packet", // this could be configurable, and must match a plugin directory name for kubelet to use
		nodeID:   nodeID,
		endpoint: endpoint,
		config:   config,
	}, nil
}

func (d *PacketDriver) Run() {

	s := NewNonBlockingGRPCServer()
	identity := NewPacketIdentityServer(d)
	var controller *PacketControllerServer
	if d.config.AuthToken != "" {
		p, _ := cloud_provider.NewPacketProvider(d.config)
		controller = NewPacketControllerServer(p)
	}
	node := NewPacketNodeServer(d)
	s.Start(d.endpoint,
		identity,
		controller,
		node)
	s.Wait()
}
