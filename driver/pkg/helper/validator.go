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

package helper

import (
	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type validator struct{}

type Validator interface {
	CheckCreateVolumeAllowed(req *csi.CreateVolumeRequest) error
	CheckDeleteVolumeAllowed(req *csi.DeleteVolumeRequest) error
	CheckValidateVolumeCapabilitiesAllowed(req *csi.ValidateVolumeCapabilitiesRequest) error
}

func NewValidator() Validator {
	return &validator{}
}

func (v *validator) CheckCreateVolumeAllowed(req *csi.CreateVolumeRequest) error {
	if len(req.GetName()) == 0 {
		return status.Error(codes.InvalidArgument, "missing name")
	}
	if req.GetVolumeCapabilities() == nil {
		return status.Error(codes.InvalidArgument, "missing volume capabilities")
	}
	return nil
}

func (v *validator) CheckDeleteVolumeAllowed(req *csi.DeleteVolumeRequest) error {
	if len(req.GetVolumeId()) == 0 {
		return status.Error(codes.InvalidArgument, "missing volume ID")
	}
	return nil
}

func (v *validator) CheckValidateVolumeCapabilitiesAllowed(req *csi.ValidateVolumeCapabilitiesRequest) error {
	if len(req.GetVolumeId()) == 0 {
		return status.Error(codes.InvalidArgument, "missing volume id")
	}
	if req.GetVolumeCapabilities() == nil {
		return status.Error(codes.InvalidArgument, "missing volume capabilities")
	}
	return nil
}
