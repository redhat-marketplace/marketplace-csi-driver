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
	"github.com/redhat-marketplace/marketplace-csi-driver/driver/pkg/client"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog"
)

// NodePublishVolume ...
func (driver *COSDriver) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	klog.V(4).Infof("Method NodePublishVolume called with: %s", protosanitizer.StripSecrets(req))

	targetPath := req.GetTargetPath()
	if len(targetPath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Target path missing in request")
	}
	if req.GetVolumeCapability() == nil {
		return nil, status.Error(codes.InvalidArgument, "Volume capability missing in request")
	}

	notMnt, err := driver.helper.CheckMount(targetPath)
	if err != nil {
		klog.Errorf("Target path is not valid mount point, error: %s", err)
		return &csi.NodePublishVolumeResponse{}, nil
	}
	if !notMnt {
		klog.V(4).Info("IsLikelyNotMountPoint false exit early")
		return &csi.NodePublishVolumeResponse{}, nil
	}

	cs, dc, err := driver.helper.GetClientSet()
	if err != nil {
		klog.Errorf("Unable to initialize client, error: %s", err)
		return &csi.NodePublishVolumeResponse{}, nil
	}
	kc := client.NewKubeClient(ctx, cs, dc, driver.helper)

	volMountDetails, err := kc.GetVolumeMountDetails(req)
	if err != nil {
		klog.Errorf("Error retrieving volume mount details: %s", err)
		kc.UpdatePodOnNodePublishVolume(req, volMountDetails, 0, err)
		return &csi.NodePublishVolumeResponse{}, nil
	}

	mcount, err := MountDatasets(driver.helper, req, volMountDetails)
	kc.UpdatePodOnNodePublishVolume(req, volMountDetails, mcount, err)

	return &csi.NodePublishVolumeResponse{}, nil
}

// NodeUnpublishVolume ...
func (driver *COSDriver) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (response *csi.NodeUnpublishVolumeResponse, err error) {
	klog.V(4).Infof("Method NodeUnpublishVolume called with: %s", protosanitizer.StripSecrets(req))
	targetPath := req.GetTargetPath()
	if len(targetPath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Target path missing in request")
	}
	UnMountDatasets(driver.helper, req)
	return &csi.NodeUnpublishVolumeResponse{}, nil
}

// NodeGetInfo ...
func (driver *COSDriver) NodeGetInfo(ctx context.Context, req *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	klog.V(4).Infof("Method NodeGetInfo called with: %s", protosanitizer.StripSecrets(req))

	return &csi.NodeGetInfoResponse{NodeId: driver.nodeName}, nil
}

// NodeGetCapabilities ...
func (driver *COSDriver) NodeGetCapabilities(ctx context.Context, req *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	klog.V(4).Infof("Method NodeGetCapabilities called with: %s", protosanitizer.StripSecrets(req))

	return &csi.NodeGetCapabilitiesResponse{Capabilities: []*csi.NodeServiceCapability{
		{
			Type: &csi.NodeServiceCapability_Rpc{
				Rpc: &csi.NodeServiceCapability_RPC{
					Type: csi.NodeServiceCapability_RPC_STAGE_UNSTAGE_VOLUME,
				},
			},
		},
	}}, nil
}

// NodeStageVolume ...
func (driver *COSDriver) NodeStageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {
	klog.V(4).Infof("Method NodeStageVolume called with: %s", protosanitizer.StripSecrets(req))

	if len(req.GetVolumeId()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}

	if len(req.GetStagingTargetPath()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Staging target path missing in request")
	}

	if req.VolumeCapability == nil {
		return nil, status.Error(codes.InvalidArgument, "NodeStageVolume Volume Capability must be provided")
	}
	return &csi.NodeStageVolumeResponse{}, nil
}

// NodeUnstageVolume ...
func (driver *COSDriver) NodeUnstageVolume(ctx context.Context, req *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {
	klog.V(4).Infof("Method NodeUnstageVolume called with: %s", protosanitizer.StripSecrets(req))

	if len(req.GetVolumeId()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}

	if len(req.GetStagingTargetPath()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Staging target path missing in request")
	}
	return &csi.NodeUnstageVolumeResponse{}, nil
}

// NodeGetVolumeStats ...
func (driver *COSDriver) NodeGetVolumeStats(ctx context.Context, req *csi.NodeGetVolumeStatsRequest) (*csi.NodeGetVolumeStatsResponse, error) {
	klog.V(4).Infof("Method NodeGetVolumeStats called with: %s", protosanitizer.StripSecrets(req))

	return nil, status.Errorf(codes.Unimplemented, "NodeGetVolumeStats: not implemented by %s", driver.name)
}

// NodeExpandVolume ...
func (driver *COSDriver) NodeExpandVolume(ctx context.Context, req *csi.NodeExpandVolumeRequest) (*csi.NodeExpandVolumeResponse, error) {
	klog.V(4).Infof("Method NodeExpandVolume called with: %s", protosanitizer.StripSecrets(req))

	return nil, status.Errorf(codes.Unimplemented, "NodeExpandVolume: not implemented by %s", driver.name)
}
