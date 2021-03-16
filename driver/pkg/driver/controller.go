/*
Copyright 2021 IBM Corp.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package driver

import (
	"context"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/kubernetes-csi/csi-lib-utils/protosanitizer"
	"github.com/redhat-marketplace/marketplace-csi-driver/driver/pkg/common"
	"k8s.io/klog"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// CreateVolume ...
func (driver *COSDriver) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	klog.V(4).Infof("Method CreateVolume called with: %s", protosanitizer.StripSecrets(req))

	v := driver.helper.GetValidator()
	volumeID := common.SanitizeKubernetesIdentifier(req.GetName())
	err := v.CheckCreateVolumeAllowed(req)
	if err != nil {
		return nil, err
	}
	return &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			VolumeId:      volumeID,
			CapacityBytes: 10000000000000,
			VolumeContext: req.GetParameters(),
		},
	}, nil
}

// DeleteVolume ...
func (driver *COSDriver) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	klog.V(4).Infof("Method DeleteVolume called with: %s", protosanitizer.StripSecrets(req))

	v := driver.helper.GetValidator()
	err := v.CheckDeleteVolumeAllowed(req)
	if err != nil {
		return nil, err
	}

	return &csi.DeleteVolumeResponse{}, nil
}

// ControllerGetCapabilities ...
func (driver *COSDriver) ControllerGetCapabilities(ctx context.Context, req *csi.ControllerGetCapabilitiesRequest) (*csi.ControllerGetCapabilitiesResponse, error) {
	klog.V(4).Infof("Method ControllerGetCapabilities called with: %s", protosanitizer.StripSecrets(req))

	return &csi.ControllerGetCapabilitiesResponse{
		Capabilities: []*csi.ControllerServiceCapability{
			{
				Type: &csi.ControllerServiceCapability_Rpc{
					Rpc: &csi.ControllerServiceCapability_RPC{
						Type: csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
					},
				},
			},
		},
	}, nil
}

// ValidateVolumeCapabilities ...
func (driver *COSDriver) ValidateVolumeCapabilities(ctx context.Context, req *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {
	klog.V(4).Infof("Method ValidateVolumeCapabilities called with: %s", protosanitizer.StripSecrets(req))

	v := driver.helper.GetValidator()
	err := v.CheckValidateVolumeCapabilitiesAllowed(req)
	if err != nil {
		return nil, err
	}

	return &csi.ValidateVolumeCapabilitiesResponse{
		Confirmed: &csi.ValidateVolumeCapabilitiesResponse_Confirmed{
			VolumeCapabilities: req.GetVolumeCapabilities(),
			VolumeContext:      req.GetVolumeContext(),
			Parameters:         req.GetParameters(),
		},
	}, nil
}

// ControllerPublishVolume ...
func (driver *COSDriver) ControllerPublishVolume(ctx context.Context, req *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
	klog.V(4).Infof("Method ControllerPublishVolume called with: %s", protosanitizer.StripSecrets(req))

	return nil, status.Error(codes.Unimplemented, "")
}

// ControllerUnpublishVolume ...
func (driver *COSDriver) ControllerUnpublishVolume(ctx context.Context, req *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
	klog.V(4).Infof("Method ControllerUnpublishVolume called with: %s", protosanitizer.StripSecrets(req))

	return nil, status.Error(codes.Unimplemented, "")
}

// GetCapacity ...
func (driver *COSDriver) GetCapacity(ctx context.Context, req *csi.GetCapacityRequest) (*csi.GetCapacityResponse, error) {
	klog.V(4).Infof("Method GetCapacity called with: %s", protosanitizer.StripSecrets(req))

	return nil, status.Error(codes.Unimplemented, "")
}

// ListVolumes ...
func (driver *COSDriver) ListVolumes(ctx context.Context, req *csi.ListVolumesRequest) (*csi.ListVolumesResponse, error) {
	klog.V(4).Infof("Method ListVolumes called with: %s", protosanitizer.StripSecrets(req))

	return nil, status.Error(codes.Unimplemented, "")
}

// CreateSnapshot ...
func (driver *COSDriver) CreateSnapshot(ctx context.Context, req *csi.CreateSnapshotRequest) (*csi.CreateSnapshotResponse, error) {
	klog.V(4).Infof("Method CreateSnapshot called with: %s", protosanitizer.StripSecrets(req))

	return nil, status.Error(codes.Unimplemented, "")
}

// DeleteSnapshot ...
func (driver *COSDriver) DeleteSnapshot(ctx context.Context, req *csi.DeleteSnapshotRequest) (*csi.DeleteSnapshotResponse, error) {
	klog.V(4).Infof("Method DeleteSnapshot called with: %s", protosanitizer.StripSecrets(req))

	return nil, status.Error(codes.Unimplemented, "")
}

// ListSnapshots ...
func (driver *COSDriver) ListSnapshots(ctx context.Context, req *csi.ListSnapshotsRequest) (*csi.ListSnapshotsResponse, error) {
	klog.V(4).Infof("Method ListSnapshots called with: %s", protosanitizer.StripSecrets(req))

	return nil, status.Error(codes.Unimplemented, "")
}

// ControllerExpandVolume ...
func (driver *COSDriver) ControllerExpandVolume(ctx context.Context, req *csi.ControllerExpandVolumeRequest) (*csi.ControllerExpandVolumeResponse, error) {
	klog.V(4).Infof("Method ControllerExpandVolume called with: %s", protosanitizer.StripSecrets(req))

	return nil, status.Error(codes.Unimplemented, "")
}

// ControllerGetVolume ...
func (driver *COSDriver) ControllerGetVolume(ctx context.Context, req *csi.ControllerGetVolumeRequest) (*csi.ControllerGetVolumeResponse, error) {
	klog.V(4).Infof("Method ControllerGetVolume called with: %s", protosanitizer.StripSecrets(req))

	return nil, status.Error(codes.Unimplemented, "")
}
