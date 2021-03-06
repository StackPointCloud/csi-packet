package packet

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/packethost/packngo"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

const (
	ConsumerToken = "csi-packet"
	BillingHourly = "hourly"
)

type Config struct {
	AuthToken  string `json:"auth-token"`
	ProjectID  string `json:"project-id"`
	FacilityID string `json:"facility-id"`
}

type PacketVolumeProvider struct {
	config Config
}

var _ VolumeProvider = &PacketVolumeProvider{}

func VolumeIDToName(id string) string {
	// "3ee59355-a51a-42a8-b848-86626cc532f0" -> "volume-3ee59355"
	uuidElements := strings.Split(id, "-")
	return fmt.Sprintf("volume-%s", uuidElements[0])
}

func NewPacketProvider(config Config) (*PacketVolumeProvider, error) {
	if config.AuthToken == "" {
		return nil, errors.New("AuthToken not specified")
	}
	if config.ProjectID == "" {
		return nil, errors.New("ProjectID not specified")
	}
	logger := log.WithFields(log.Fields{"project_id": config.ProjectID})
	logger.Info("Creating provider")

	if config.FacilityID == "" {
		facilityCode, err := GetPacketFacilityCodeMetadata()
		if err != nil {
			logger.Errorf("Cannot get facility code %v", err)
			return nil, errors.Wrap(err, "cannot construct PacketVolumeProvider")
		}
		c := constructClient(config.AuthToken)
		facilities, resp, err := c.Facilities.List()
		if err != nil {
			if resp.StatusCode == http.StatusForbidden {
				return nil, fmt.Errorf("cannot construct PacketVolumeProvider, access denied to search facilities")
			}
			return nil, errors.Wrap(err, "cannot construct PacketVolumeProvider")
		}
		for _, facility := range facilities {
			if facility.Code == facilityCode {
				config.FacilityID = facility.ID
				logger.WithField("facility_id", facility.ID).Infof("facility found")
				break
			}
		}
	}
	if config.FacilityID == "" {
		logger.Errorf("FacilityID not specified and cannot be found")
		return nil, fmt.Errorf("FacilityID not specified and cannot be found")
	}

	provider := PacketVolumeProvider{config}
	return &provider, nil
}

func constructClient(authToken string) *packngo.Client {
	tr := &http.Transport{
		MaxIdleConns:       10,
		IdleConnTimeout:    30 * time.Second,
		DisableCompression: true,
	}
	client := &http.Client{Transport: tr}

	// client.Transport = logging.NewTransport("Packet", client.Transport)
	return packngo.NewClientWithAuth(ConsumerToken, authToken, client)
}

// Client() returns a new client for accessing Packet's API.
func (p *PacketVolumeProvider) client() *packngo.Client {
	return constructClient(p.config.AuthToken)
}

// ListVolume wraps the packet api as an interface method
func (p *PacketVolumeProvider) ListVolumes() ([]packngo.Volume, *packngo.Response, error) {
	return p.client().Volumes.List(p.config.ProjectID, &packngo.ListOptions{})
}

// Get wraps the packet api as an interface method
func (p *PacketVolumeProvider) Get(volumeUUID string) (*packngo.Volume, *packngo.Response, error) {
	return p.client().Volumes.Get(volumeUUID)
}

// Delete wraps the packet api as an interface method
func (p *PacketVolumeProvider) Delete(volumeUUID string) (*packngo.Response, error) {
	resp, err := p.client().Volumes.Delete(volumeUUID)
	if resp.StatusCode == http.StatusNotFound {
		return resp, nil
	}
	return resp, err
}

// Create wraps the packet api as an interface method
func (p *PacketVolumeProvider) Create(createRequest *packngo.VolumeCreateRequest) (*packngo.Volume, *packngo.Response, error) {

	createRequest.FacilityID = p.config.FacilityID

	return p.client().Volumes.Create(createRequest, p.config.ProjectID)
}

// Attach wraps the packet api as an interface method
func (p *PacketVolumeProvider) Attach(volumeID, deviceID string) (*packngo.VolumeAttachment, *packngo.Response, error) {
	volume, httpResponse, err := p.client().Volumes.Get(volumeID)
	if err != nil || httpResponse.StatusCode != http.StatusOK {
		return nil, httpResponse, errors.Wrap(err, "prechecking existence of volume attachment")
	}
	for _, attachment := range volume.Attachments {
		if attachment.Device.ID == deviceID {
			return p.client().VolumeAttachments.Get(attachment.ID)
		}
	}
	return p.client().VolumeAttachments.Create(volumeID, deviceID)
}

// Detach wraps the packet api as an interface method
func (p *PacketVolumeProvider) Detach(attachmentId string) (*packngo.Response, error) {
	return p.client().VolumeAttachments.Delete(attachmentId)
}

func (p *PacketVolumeProvider) GetNodes() ([]packngo.Device, *packngo.Response, error) {
	return p.client().Devices.List(p.config.ProjectID, &packngo.ListOptions{})
}
