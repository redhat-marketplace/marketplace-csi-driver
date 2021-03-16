/*


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

package controllers

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	marketplacev1alpha1 "github.com/redhat-marketplace/marketplace-csi-driver/operator/api/v1alpha1"
	"github.com/redhat-marketplace/marketplace-csi-driver/operator/pkg/common"
	"github.com/redhat-marketplace/marketplace-csi-driver/operator/pkg/csiwebhook"
	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var _ = Context("CSI Webhook", func() {
	var (
		podcsi  csiwebhook.PodCSIVolumeMounter
		decoder *admission.Decoder
	)

	BeforeEach(func() {
		var err error
		decoder, err = admission.NewDecoder(scheme.Scheme)
		Expect(err).NotTo(HaveOccurred())
		Expect(decoder).NotTo(BeNil())
		podcsi = csiwebhook.PodCSIVolumeMounter{
			Log:    logf.Log.WithName("webhook"),
			Helper: common.NewControllerHelper(),
		}
		podcsi.InjectDecoder(decoder)
	})

	req := admission.Request{
		AdmissionRequest: admissionv1beta1.AdmissionRequest{
			Object: runtime.RawExtension{
				Raw: []byte(`{
	    "apiVersion": "v1",
	    "kind": "Pod",
	    "metadata": {
	        "name": "foo",
	        "namespace": "default"
	    },
	    "spec": {
	        "containers": [
	            {
	                "image": "bar:v2",
	                "name": "bar"
	            }
	        ]
	    }
	}`),
			},
			OldObject: runtime.RawExtension{
				Raw: []byte(`{
	    "apiVersion": "v1",
	    "kind": "Pod",
	    "metadata": {
	        "name": "foo",
	        "namespace": "default"
	    },
	    "spec": {
	        "containers": [
	            {
	                "image": "bar:v1",
	                "name": "bar"
	            }
	        ]
	    }
	}`),
			},
		},
	}

	reqWithVol := admission.Request{
		AdmissionRequest: admissionv1beta1.AdmissionRequest{
			Object: runtime.RawExtension{
				Raw: []byte(`{
	    "apiVersion": "v1",
	    "kind": "Pod",
	    "metadata": {
	        "name": "foo",
	        "namespace": "default"
	    },
	    "spec": {
	        "containers": [
	            {
	                "image": "bar:v2",
	                "name": "bar",
					"volumeMounts": [
						{
							"mountPath": "/var/test",
							"name": "rhm-datasets",
							"mountPropagation": "HostToContainer"
						}
					]
	            }
	        ],
			"volumes": [
				{
					"name": "rhm-datasets",
					"persistentVolumeClaim": {
						"claimName": "csi-rhm-cos-pvc"
					}
				}
			]
	    }
	}`),
			},
			OldObject: runtime.RawExtension{
				Raw: []byte(`{
	    "apiVersion": "v1",
	    "kind": "Pod",
	    "metadata": {
	        "name": "foo",
	        "namespace": "default"
	    },
	    "spec": {
	        "containers": [
	            {
	                "image": "bar:v1",
	                "name": "bar"
	            }
	        ]
	    }
	}`),
			},
		},
	}

	reqWithNonDatasetVol := admission.Request{
		AdmissionRequest: admissionv1beta1.AdmissionRequest{
			Object: runtime.RawExtension{
				Raw: []byte(`{
	    "apiVersion": "v1",
	    "kind": "Pod",
	    "metadata": {
	        "name": "foo",
	        "namespace": "default"
	    },
	    "spec": {
	        "containers": [
	            {
	                "image": "bar:v2",
	                "name": "bar",
					"volumeMounts": [
						{
							"mountPath": "/var/test",
							"name": "test-pvc",
							"mountPropagation": "HostToContainer"
						}
					]
	            }
	        ],
			"volumes": [
				{
					"name": "test-pvc",
					"persistentVolumeClaim": {
						"claimName": "test-pvc"
					}
				}
			]
	    }
	}`),
			},
			OldObject: runtime.RawExtension{
				Raw: []byte(`{
	    "apiVersion": "v1",
	    "kind": "Pod",
	    "metadata": {
	        "name": "foo",
	        "namespace": "default"
	    },
	    "spec": {
	        "containers": [
	            {
	                "image": "bar:v1",
	                "name": "bar"
	            }
	        ]
	    }
	}`),
			},
		},
	}

	badReq := admission.Request{
		AdmissionRequest: admissionv1beta1.AdmissionRequest{
			Object: runtime.RawExtension{
				Raw: []byte(`{
    "apiVersion": "v1",
    "kind": "Node",
    "metadata": {
        "name": "foo",
        "namespace": "default"
    },    
}`),
			},
			OldObject: runtime.RawExtension{
				Raw: []byte(`{
    "apiVersion": "v1",
    "kind": "Node",
    "metadata": {
        "name": "foo",
        "namespace": "default"
    },    
}`),
			},
		},
	}

	protetedReq := admission.Request{
		AdmissionRequest: admissionv1beta1.AdmissionRequest{
			Object: runtime.RawExtension{
				Raw: []byte(`{
	    "apiVersion": "v1",
	    "kind": "Pod",
	    "metadata": {
	        "name": "foo"
	    },
	    "spec": {
	        "containers": [
	            {
	                "image": "bar:v2",
	                "name": "bar"
	            }
	        ]
	    }
	}`),
			},
			OldObject: runtime.RawExtension{
				Raw: []byte(`{
	    "apiVersion": "v1",
	    "kind": "Pod",
	    "metadata": {
	        "name": "foo"
	    },
	    "spec": {
	        "containers": [
	            {
	                "image": "bar:v1",
	                "name": "bar"
	            }
	        ]
	    }
	}`),
			},
		},
	}

	It("Decode error bad request", func() {
		podcsi.InjectDecoder(decoder)

		resp := podcsi.Handle(context.TODO(), badReq)
		Expect(resp.Allowed).To(BeTrue(), "expected allowed to be true")
		reason := resp.Result.Reason
		Expect(fmt.Sprintf("%v", reason)).To(Equal("Unable to decode pod"), "expected decode error")
	})

	It("Decode error protected", func() {
		protetedReq.Namespace = "kube-system"
		podcsi.InjectDecoder(decoder)
		podcsi.Client = fake.NewFakeClient()

		resp := podcsi.Handle(context.TODO(), protetedReq)
		Expect(resp.Allowed).To(BeTrue(), "expected allowed to be true")
		reason := resp.Result.Reason
		Expect(fmt.Sprintf("%v", reason)).To(Equal("No mutation required"), "expected no mutation request")
	})

	It("Decode error no csi driver", func() {
		req.Namespace = "default"
		podcsi.InjectDecoder(decoder)
		podcsi.Client = fake.NewFakeClient()

		resp := podcsi.Handle(context.TODO(), req)
		Expect(resp.Allowed).To(BeTrue(), "expected allowed to be true")
		reason := resp.Result.Reason
		Expect(fmt.Sprintf("%v", reason)).To(Equal("Driver not set up"), "expected no mutation request")
	})

	It("Decode error no dataset pvc", func() {
		req.Namespace = "default"
		podcsi.InjectDecoder(decoder)

		csidriver := &marketplacev1alpha1.MarketplaceCSIDriver{
			ObjectMeta: metav1.ObjectMeta{
				Name:      common.ResourceNameS3Driver,
				Namespace: common.MarketplaceNamespace,
			},
			Spec: marketplacev1alpha1.MarketplaceCSIDriverSpec{
				Endpoint:      "test",
				MountRootPath: "/",
			},
		}
		podcsi.Client = fake.NewFakeClient(csidriver)

		resp := podcsi.Handle(context.TODO(), req)
		Expect(resp.Allowed).To(BeTrue(), "expected allowed to be true")
		reason := resp.Result.Reason
		Expect(fmt.Sprintf("%v", reason)).To(Equal("No mutation required"), "expected no mutation request")
	})

	It("Decode error get dataset pvc", func() {
		req.Namespace = "default"
		podcsi.InjectDecoder(decoder)

		csidriver := &marketplacev1alpha1.MarketplaceCSIDriver{
			ObjectMeta: metav1.ObjectMeta{
				Name:      common.ResourceNameS3Driver,
				Namespace: common.MarketplaceNamespace,
			},
			Spec: marketplacev1alpha1.MarketplaceCSIDriverSpec{
				Endpoint:      "test",
				MountRootPath: "/",
			},
		}
		pvc := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      common.DatasetPVCName,
				Namespace: "default",
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany},
			},
		}
		cl := newMarketplaceCSIDriverClient(fake.NewFakeClient(csidriver, pvc), "get", "PersistentVolumeClaim", common.DatasetPVCName)
		podcsi.Client = cl

		resp := podcsi.Handle(context.TODO(), req)
		Expect(resp.Allowed).To(BeTrue(), "expected allowed to be true")
		reason := resp.Result.Reason
		Expect(fmt.Sprintf("%v", reason)).To(Equal("No mutation required"), "expected no mutation request")
	})

	It("Json marshal error", func() {
		req.Namespace = "default"
		podcsi.Helper = NewFakeControllerHelper("", "", "*v1.Pod")
		podcsi.InjectDecoder(decoder)

		csidriver := &marketplacev1alpha1.MarketplaceCSIDriver{
			ObjectMeta: metav1.ObjectMeta{
				Name:      common.ResourceNameS3Driver,
				Namespace: common.MarketplaceNamespace,
			},
			Spec: marketplacev1alpha1.MarketplaceCSIDriverSpec{
				Endpoint:      "test",
				MountRootPath: "/var/test/datasets",
			},
		}
		pvc := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      common.DatasetPVCName,
				Namespace: "default",
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany},
			},
		}
		podcsi.Client = fake.NewFakeClient(csidriver, pvc)

		resp := podcsi.Handle(context.TODO(), req)
		Expect(resp.Allowed).To(BeTrue(), "expected allowed to be true")
		reason := resp.Result.Reason
		Expect(fmt.Sprintf("%v", reason)).To(Equal("Unable to marshall pod"), "expected json marshal error")
	})

	It("Successful Patch", func() {
		req.Namespace = "default"
		podcsi.InjectDecoder(decoder)

		csidriver := &marketplacev1alpha1.MarketplaceCSIDriver{
			ObjectMeta: metav1.ObjectMeta{
				Name:      common.ResourceNameS3Driver,
				Namespace: common.MarketplaceNamespace,
			},
			Spec: marketplacev1alpha1.MarketplaceCSIDriverSpec{
				Endpoint:      "test",
				MountRootPath: "/var/test/datasets",
			},
		}
		pvc := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      common.DatasetPVCName,
				Namespace: "default",
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany},
			},
		}
		podcsi.Client = fake.NewFakeClient(csidriver, pvc)

		resp := podcsi.Handle(context.TODO(), req)
		Expect(resp.Allowed).To(BeTrue(), "expected allowed to be true")
		volumeFound := false
		volumeMountFound := false
		for _, p := range resp.Patches {
			if p.Path == "/spec/volumes" {
				volumeFound = true
			} else if p.Path == "/spec/containers/0/volumeMounts" {
				volumeMountFound = true
			}
		}
		Expect(volumeFound).To(BeTrue(), "expected patch for volume")
		Expect(volumeMountFound).To(BeTrue(), "expected patch for volume mount")
	})

	It("Successful Patch pod with vols", func() {
		podcsi.InjectDecoder(decoder)

		csidriver := &marketplacev1alpha1.MarketplaceCSIDriver{
			ObjectMeta: metav1.ObjectMeta{
				Name:      common.ResourceNameS3Driver,
				Namespace: common.MarketplaceNamespace,
			},
			Spec: marketplacev1alpha1.MarketplaceCSIDriverSpec{
				Endpoint:      "test",
				MountRootPath: "/var/test/datasets",
			},
		}
		pvc := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      common.DatasetPVCName,
				Namespace: "default",
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany},
			},
		}
		podcsi.Client = fake.NewFakeClient(csidriver, pvc)

		resp := podcsi.Handle(context.TODO(), reqWithNonDatasetVol)
		Expect(resp.Allowed).To(BeTrue(), "expected allowed to be true")
		volumeFound := false
		volumeMountFound := false
		for _, p := range resp.Patches {
			if p.Path == "/spec/volumes/1" {
				volumeFound = true
			} else if p.Path == "/spec/containers/0/volumeMounts/1" {
				volumeMountFound = true
			}
		}
		Expect(volumeFound).To(BeTrue(), "expected patch for volume")
		Expect(volumeMountFound).To(BeTrue(), "expected patch for volume mount")
	})

	It("No patch for pod with pvc", func() {
		podcsi.InjectDecoder(decoder)

		csidriver := &marketplacev1alpha1.MarketplaceCSIDriver{
			ObjectMeta: metav1.ObjectMeta{
				Name:      common.ResourceNameS3Driver,
				Namespace: common.MarketplaceNamespace,
			},
			Spec: marketplacev1alpha1.MarketplaceCSIDriverSpec{
				Endpoint:      "test",
				MountRootPath: "/var/test/datasets",
			},
		}
		pvc := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      common.DatasetPVCName,
				Namespace: "default",
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany},
			},
		}
		podcsi.Client = fake.NewFakeClient(csidriver, pvc)

		resp := podcsi.Handle(context.TODO(), reqWithVol)
		Expect(resp.Allowed).To(BeTrue(), "expected allowed to be true")
		reason := resp.Result.Reason
		Expect(fmt.Sprintf("%v", reason)).To(Equal("No mutation required"), "expected no mutation request")
	})
})
