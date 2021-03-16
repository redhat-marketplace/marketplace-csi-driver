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

package common

import (
	"encoding/json"

	"github.com/redhat-marketplace/marketplace-csi-driver/operator/pkg/infrastructure"
	"k8s.io/apimachinery/pkg/api/equality"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	assets "github.com/redhat-marketplace/marketplace-csi-driver/operator/pkg/assets"
)

type ControllerHelperInterface interface {
	Asset(name string) ([]byte, error)
	GetCloudProviderType(c client.Client, log logr.Logger) (string, error)
	Marshal(v interface{}) ([]byte, error)
	DeepEqual(r1, r2 interface{}) bool
}

type ControllerHelper struct{}

func NewControllerHelper() ControllerHelperInterface {
	return &ControllerHelper{}
}

func (ch *ControllerHelper) Asset(name string) ([]byte, error) {
	return assets.Asset(name)
}

func (ch *ControllerHelper) GetCloudProviderType(c client.Client, log logr.Logger) (string, error) {
	return infrastructure.GetCloudProviderType(c, log)
}

func (ch *ControllerHelper) Marshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

// https://github.com/kubernetes/apimachinery/issues/75

func (ch *ControllerHelper) DeepEqual(r1, r2 interface{}) bool {
	return equality.Semantic.DeepEqual(r1, r2)
}
