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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/redhat-marketplace/marketplace-csi-driver/driver/pkg/helper"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/runtime"
	fakedyn "k8s.io/client-go/dynamic/fake"
	fake "k8s.io/client-go/kubernetes/fake"
)

var _ = Describe("CSI Driver", func() {

	var ctx context.Context
	var driver *COSDriver

	BeforeEach(func() {
		ctx = context.Background()
		socket := "/tmp/csi-dataset.sock"
		csiEndpoint := "unix://" + socket
		driver = &COSDriver{
			name:     "test-dataset-driver",
			nodeName: "test-node",
			version:  GetVersionJSON(),
			endpoint: csiEndpoint,
			helper: &fakeDriverHelper{
				Clientset: fake.NewSimpleClientset(),
				Dynclient: fakedyn.NewSimpleDynamicClient(runtime.NewScheme()),
				validator: helper.NewValidator(),
			},
		}
	})

	It("Should check driver mount command", func() {
		driver.helper = helper.NewDriverHelper()
		Expect(driver.helper.GetMountCommand()).To(Equal("s3fs"))
	})

	It("Should fail for unsupported feature", func() {
		driver.helper = helper.NewDriverHelper()
		Expect(driver.helper.GetMountCommand()).To(Equal("s3fs"))

		_, err := driver.ControllerPublishVolume(ctx, &csi.ControllerPublishVolumeRequest{})
		Expect(err).To(HaveOccurred(), "error expected for unsupported feature")
		Expect(status.Code(err)).To(Equal(codes.Unimplemented), "expected unimplemented response code")

		_, err = driver.ControllerUnpublishVolume(ctx, &csi.ControllerUnpublishVolumeRequest{})
		Expect(err).To(HaveOccurred(), "error expected for unsupported feature")
		Expect(status.Code(err)).To(Equal(codes.Unimplemented), "expected unimplemented response code")

		_, err = driver.GetCapacity(ctx, &csi.GetCapacityRequest{})
		Expect(err).To(HaveOccurred(), "error expected for unsupported feature")
		Expect(status.Code(err)).To(Equal(codes.Unimplemented), "expected unimplemented response code")

		_, err = driver.ListVolumes(ctx, &csi.ListVolumesRequest{})
		Expect(err).To(HaveOccurred(), "error expected for unsupported feature")
		Expect(status.Code(err)).To(Equal(codes.Unimplemented), "expected unimplemented response code")

		_, err = driver.CreateSnapshot(ctx, &csi.CreateSnapshotRequest{})
		Expect(err).To(HaveOccurred(), "error expected for unsupported feature")
		Expect(status.Code(err)).To(Equal(codes.Unimplemented), "expected unimplemented response code")

		_, err = driver.DeleteSnapshot(ctx, &csi.DeleteSnapshotRequest{})
		Expect(err).To(HaveOccurred(), "error expected for unsupported feature")
		Expect(status.Code(err)).To(Equal(codes.Unimplemented), "expected unimplemented response code")

		_, err = driver.ListSnapshots(ctx, &csi.ListSnapshotsRequest{})
		Expect(err).To(HaveOccurred(), "error expected for unsupported feature")
		Expect(status.Code(err)).To(Equal(codes.Unimplemented), "expected unimplemented response code")

		_, err = driver.ControllerExpandVolume(ctx, &csi.ControllerExpandVolumeRequest{})
		Expect(err).To(HaveOccurred(), "error expected for unsupported feature")
		Expect(status.Code(err)).To(Equal(codes.Unimplemented), "expected unimplemented response code")

		_, err = driver.ControllerGetVolume(ctx, &csi.ControllerGetVolumeRequest{})
		Expect(err).To(HaveOccurred(), "error expected for unsupported feature")
		Expect(status.Code(err)).To(Equal(codes.Unimplemented), "expected unimplemented response code")
	})

	It("Should validate volume requests", func() {
		driver.helper = helper.NewDriverHelper()
		_, err := driver.CreateVolume(ctx, &csi.CreateVolumeRequest{
			Name: "",
		})
		Expect(err).To(HaveOccurred(), "error expected for invalid input")
		Expect(status.Code(err)).To(Equal(codes.InvalidArgument), "expected invalid argument response code")

		_, err = driver.CreateVolume(ctx, &csi.CreateVolumeRequest{
			Name: "test",
		})
		Expect(err).To(HaveOccurred(), "error expected for invalid input")
		Expect(status.Code(err)).To(Equal(codes.InvalidArgument), "expected invalid argument response code")

		_, err = driver.CreateVolume(ctx, &csi.CreateVolumeRequest{
			Name: "test",
			VolumeCapabilities: []*csi.VolumeCapability{
				{
					AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY},
				},
			},
		})
		Expect(err).NotTo(HaveOccurred(), "error not expected")

		_, err = driver.DeleteVolume(ctx, &csi.DeleteVolumeRequest{
			VolumeId: "",
		})
		Expect(err).To(HaveOccurred(), "error expected for invalid input")
		Expect(status.Code(err)).To(Equal(codes.InvalidArgument), "expected invalid argument response code")

		_, err = driver.DeleteVolume(ctx, &csi.DeleteVolumeRequest{
			VolumeId: "test",
		})
		Expect(err).NotTo(HaveOccurred(), "error expected for invalid input")

		_, err = driver.ValidateVolumeCapabilities(ctx, &csi.ValidateVolumeCapabilitiesRequest{
			VolumeId: "",
		})
		Expect(err).To(HaveOccurred(), "error expected for invalid input")
		Expect(status.Code(err)).To(Equal(codes.InvalidArgument), "expected invalid argument response code")

		_, err = driver.ValidateVolumeCapabilities(ctx, &csi.ValidateVolumeCapabilitiesRequest{
			VolumeId: "test",
		})
		Expect(err).To(HaveOccurred(), "error expected for invalid input")
		Expect(status.Code(err)).To(Equal(codes.InvalidArgument), "expected invalid argument response code")

		_, err = driver.ValidateVolumeCapabilities(ctx, &csi.ValidateVolumeCapabilitiesRequest{
			VolumeId: "test",
			VolumeCapabilities: []*csi.VolumeCapability{
				{
					AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY},
				},
			},
		})
		Expect(err).NotTo(HaveOccurred(), "error not expected")
	})

})
