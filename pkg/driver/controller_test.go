package driver

import (
	"context"
	"net/http"
	"testing"

	"github.com/StackPointCloud/csi-packet/pkg/cloud_provider"
	"github.com/StackPointCloud/csi-packet/pkg/test"

	"github.com/stretchr/testify/assert"

	"github.com/packethost/packngo"

	"github.com/container-storage-interface/spec/lib/go/csi/v0"
	"github.com/golang/mock/gomock"
)

const (
	attachmentID     = "60bf5425-e59d-42c3-b9b9-ac0d8cfc86a2"
	providerVolumeID = "9b03a6ea-42fb-40c7-abaa-247445b36890"
	csiNodeIP        = "10.88.52.133"
	csiNodeName      = "spcfoobar-worker-1"
	nodeID           = "262c173c-c24d-4ad6-be1a-13fd9a523cfa"
)

func TestCreateVolume(t *testing.T) {
	csiVolumeName := "kubernetes-volume-request-0987654321"

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	provider := test.NewMockVolumeProvider(mockCtrl)
	volume := packngo.Volume{
		Size:        cloud_provider.DefaultVolumeSizeGb,
		ID:          providerVolumeID,
		Description: cloud_provider.NewVolumeDescription(csiVolumeName).String(),
	}
	resp := packngo.Response{
		&http.Response{
			StatusCode: http.StatusOK,
		},
		packngo.Rate{},
	}
	provider.EXPECT().ListVolumes().Return([]packngo.Volume{}, &resp, nil)
	provider.EXPECT().Create(gomock.Any()).Return(&volume, &resp, nil)

	controller := NewPacketControllerServer(provider)
	volumeRequest := csi.CreateVolumeRequest{}
	volumeRequest.Name = csiVolumeName
	volumeRequest.CapacityRange = &csi.CapacityRange{
		RequiredBytes: 10 * 1024 * 1024 * 1024,
		LimitBytes:    100 * 1024 * 1024 * 1024,
	}

	csiResp, err := controller.CreateVolume(context.TODO(), &volumeRequest)
	assert.Nil(t, err)
	assert.Equal(t, providerVolumeID, csiResp.GetVolume().Id)
	assert.Equal(t, cloud_provider.DefaultVolumeSizeGb*cloud_provider.GB, csiResp.GetVolume().GetCapacityBytes())

}

func TestIdempotentCreateVolume(t *testing.T) {

	csiVolumeName := "kubernetes-volume-request-0987654321"

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	provider := test.NewMockVolumeProvider(mockCtrl)
	volume := packngo.Volume{
		Size:        cloud_provider.DefaultVolumeSizeGb,
		ID:          providerVolumeID,
		Description: cloud_provider.NewVolumeDescription(csiVolumeName).String(),
	}
	resp := packngo.Response{
		&http.Response{
			StatusCode: http.StatusOK,
		},
		packngo.Rate{},
	}
	provider.EXPECT().ListVolumes().Return([]packngo.Volume{volume}, &resp, nil)

	controller := NewPacketControllerServer(provider)
	volumeRequest := csi.CreateVolumeRequest{}
	volumeRequest.Name = csiVolumeName

	csiResp, err := controller.CreateVolume(context.TODO(), &volumeRequest)
	assert.Nil(t, err)
	assert.Equal(t, providerVolumeID, csiResp.GetVolume().Id)
}

func TestListVolumes(t *testing.T) {

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	provider := test.NewMockVolumeProvider(mockCtrl)

	resp := packngo.Response{
		&http.Response{
			StatusCode: http.StatusOK,
		},
		packngo.Rate{},
	}
	provider.EXPECT().ListVolumes().Return([]packngo.Volume{}, &resp, nil)

	controller := NewPacketControllerServer(provider)
	volumeRequest := csi.ListVolumesRequest{}

	csiResp, err := controller.ListVolumes(context.TODO(), &volumeRequest)
	assert.Nil(t, err)
	assert.NotNil(t, csiResp)
	assert.Equal(t, 0, len(csiResp.Entries))

}

func TestDeleteVolume(t *testing.T) {

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	provider := test.NewMockVolumeProvider(mockCtrl)

	resp := packngo.Response{
		&http.Response{
			StatusCode: http.StatusOK,
		},
		packngo.Rate{},
	}
	provider.EXPECT().Delete(providerVolumeID).Return(&resp, nil)

	controller := NewPacketControllerServer(provider)
	volumeRequest := csi.DeleteVolumeRequest{
		VolumeId: providerVolumeID,
	}

	csiResp, err := controller.DeleteVolume(context.TODO(), &volumeRequest)
	assert.Nil(t, err)
	assert.NotNil(t, csiResp)

}

func TestPublishVolume(t *testing.T) {

	providerVolumeName := "name-assigned-by-provider"

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	provider := test.NewMockVolumeProvider(mockCtrl)

	resp := packngo.Response{
		&http.Response{
			StatusCode: http.StatusOK,
		},
		packngo.Rate{},
	}
	nodeIpAddress := packngo.IPAddressAssignment{}
	nodeIpAddress.Address = csiNodeIP
	nodeResp := []packngo.Device{
		packngo.Device{
			Hostname: csiNodeName,
			ID:       nodeID,
			Network: []*packngo.IPAddressAssignment{
				&nodeIpAddress,
			},
		},
	}
	volumeResp := packngo.Volume{
		ID:   providerVolumeID,
		Name: providerVolumeName,
		Attachments: []*packngo.VolumeAttachment{
			&packngo.VolumeAttachment{
				ID: attachmentID,
				Volume: packngo.Volume{
					ID: providerVolumeID,
				},
				Device: packngo.Device{
					ID: nodeID,
				},
			},
		},
	}
	attachResp := packngo.VolumeAttachment{
		ID:     attachmentID,
		Volume: volumeResp,
		Device: packngo.Device{
			ID: nodeID,
		},
	}
	provider.EXPECT().GetNodes().Return(nodeResp, &resp, nil)

	provider.EXPECT().Get(providerVolumeID).Return(&volumeResp, &resp, nil)

	provider.EXPECT().Attach(providerVolumeID, nodeID).Return(&attachResp, &resp, nil)

	controller := NewPacketControllerServer(provider)
	volumeRequest := csi.ControllerPublishVolumeRequest{
		VolumeId:         providerVolumeID,
		NodeId:           csiNodeIP,
		VolumeCapability: &csi.VolumeCapability{},
	}

	csiResp, err := controller.ControllerPublishVolume(context.TODO(), &volumeRequest)
	assert.Nil(t, err)
	assert.NotNil(t, csiResp)
	assert.NotNil(t, csiResp.GetPublishInfo())
	assert.Equal(t, attachmentID, csiResp.PublishInfo["AttachmentId"])
	assert.Equal(t, providerVolumeID, csiResp.PublishInfo["VolumeId"])
	assert.Equal(t, providerVolumeName, csiResp.PublishInfo["VolumeName"])

}

func TestUnpublishVolume(t *testing.T) {

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	provider := test.NewMockVolumeProvider(mockCtrl)

	resp := packngo.Response{
		&http.Response{
			StatusCode: http.StatusOK,
		},
		packngo.Rate{},
	}
	attachedVolume := packngo.Volume{
		ID: providerVolumeID,
		Attachments: []*packngo.VolumeAttachment{
			&packngo.VolumeAttachment{
				ID: attachmentID,
				Volume: packngo.Volume{
					ID: providerVolumeID,
				},
				Device: packngo.Device{
					ID: nodeID,
				},
			},
		},
	}

	provider.EXPECT().Get(providerVolumeID).Return(&attachedVolume, &resp, nil)
	provider.EXPECT().Detach(attachmentID).Return(&resp, nil)

	controller := NewPacketControllerServer(provider)
	volumeRequest := csi.ControllerUnpublishVolumeRequest{
		VolumeId: providerVolumeID,
		NodeId:   nodeID,
	}

	csiResp, err := controller.ControllerUnpublishVolume(context.TODO(), &volumeRequest)
	assert.Nil(t, err)
	assert.NotNil(t, csiResp)

}

func TestGetCapacity(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	provider := test.NewMockVolumeProvider(mockCtrl)

	capacityRequest := csi.GetCapacityRequest{}
	controller := NewPacketControllerServer(provider)
	csiResp, err := controller.GetCapacity(context.TODO(), &capacityRequest)
	assert.NotNil(t, err, "this method is not implemented")
	assert.Nil(t, csiResp, "this method is not implemented")
}

type volumeCapabilityTestCase struct {
	capabilitySet     []*csi.VolumeCapability
	isPacketSupported bool
	description       string
}

func getVolumeCapabilityTestCases() []volumeCapabilityTestCase {

	snwCap := csi.VolumeCapability{
		AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER},
	}
	snroCap := csi.VolumeCapability{
		AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_READER_ONLY},
	}
	mnmwCap := csi.VolumeCapability{
		AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER},
	}
	mnroCap := csi.VolumeCapability{
		AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY},
	}
	mnswCap := csi.VolumeCapability{
		AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_MULTI_NODE_SINGLE_WRITER},
	}

	return []volumeCapabilityTestCase{

		{
			capabilitySet:     []*csi.VolumeCapability{&snwCap},
			isPacketSupported: true,
			description:       "single node writer",
		},
		{
			capabilitySet:     []*csi.VolumeCapability{&snroCap},
			isPacketSupported: true,
			description:       "single node read only",
		},
		{
			capabilitySet:     []*csi.VolumeCapability{&mnroCap},
			isPacketSupported: false,
			description:       "multi node read only",
		},
		{
			capabilitySet:     []*csi.VolumeCapability{&mnswCap},
			isPacketSupported: false,
			description:       "multinode single writer",
		},
		{
			capabilitySet:     []*csi.VolumeCapability{&mnmwCap},
			isPacketSupported: false,
			description:       "multi node multi writer",
		},
		{
			capabilitySet:     []*csi.VolumeCapability{&mnmwCap, &mnroCap, &mnswCap, &snroCap, &snwCap},
			isPacketSupported: false,
			description:       "all capabilities",
		},
		{
			capabilitySet:     []*csi.VolumeCapability{&snroCap, &snwCap},
			isPacketSupported: true,
			description:       "single node capabilities",
		},
	}
}

func TestValidateVolumeCapabilities(t *testing.T) {

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	provider := test.NewMockVolumeProvider(mockCtrl)

	controller := NewPacketControllerServer(provider)

	for _, testCase := range getVolumeCapabilityTestCases() {

		request := &csi.ValidateVolumeCapabilitiesRequest{
			VolumeCapabilities: testCase.capabilitySet,
			VolumeId:           providerVolumeID,
		}

		resp, err := controller.ValidateVolumeCapabilities(context.TODO(), request)
		assert.Nil(t, err)
		assert.Equal(t, testCase.isPacketSupported, resp.Supported, testCase.description)

	}

}