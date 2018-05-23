package cloud_provider

import (
	"encoding/json"
	"time"

	"github.com/packethost/packngo"
)

const (
	GB                  int64 = 1024 * 1024 * 1024
	MaxVolumeSizeGb           = 10000
	DefaultVolumeSizeGb       = 100
	MinVolumeSizeGb           = 5

	// DiskTypeSSD      = "pd-ssd"
	// DiskTypeStandard = "pd-standard"

	// diskTypeDefault = DiskTypeStandard
)

type VolumeProvider interface {
	ListVolumes() ([]packngo.Volume, *packngo.Response, error)
	Get(volumeID string) (*packngo.Volume, *packngo.Response, error)
	Delete(volumeID string) (*packngo.Response, error)
	Create(*packngo.VolumeCreateRequest) (*packngo.Volume, *packngo.Response, error)
	Attach(volumeID, deviceID string) (*packngo.VolumeAttachment, *packngo.Response, error)
	Detach(attachmentID string) (*packngo.Response, error)
	GetNodes() ([]packngo.Device, *packngo.Response, error)
}

type VolumeDescription struct {
	Name    string
	Created time.Time
}

func (desc VolumeDescription) String() string {
	serialized, err := json.Marshal(desc)
	if err != nil {
		return ""
	}
	return string(serialized)
}

func NewVolumeDescription(name string) VolumeDescription {
	return VolumeDescription{
		Name:    name,
		Created: time.Now(),
	}
}

func ReadDescription(serialized string) (VolumeDescription, error) {
	desc := VolumeDescription{}
	err := json.Unmarshal([]byte(serialized), &desc)
	return desc, err
}

type NodeVolumeManager interface{}
