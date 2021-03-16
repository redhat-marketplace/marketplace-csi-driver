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

package infrastructure

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GetCloudProviderType(c client.Client, log logr.Logger) (string, error) {
	platformType := "unknown"
	infrastructureResource := &unstructured.Unstructured{}
	infrastructureResource.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "config.openshift.io",
		Kind:    "Infrastructure",
		Version: "v1",
	})
	err := c.Get(context.Background(), client.ObjectKey{
		Name: "cluster",
	}, infrastructureResource)
	if err != nil {
		if !errors.IsNotFound(err) && !meta.IsNoMatchError(err) {
			log.Error(err, "Failed to retrieve Infrastructure resource")
		}
		return platformType, err
	}
	platformType = getStringValue(infrastructureResource, platformType, log, "status", "platformStatus", "type")
	return strings.ToLower(platformType), nil
}

func getStringValue(u *unstructured.Unstructured, defValue string, log logr.Logger, fldPath ...string) string {
	fld, exists, err := unstructured.NestedString(u.Object, fldPath...)
	if err != nil {
		log.Error(err, fmt.Sprintf("Error reading %s for %s", strings.Join(fldPath, "."), u.GetName()))
		return defValue
	}
	if !exists {
		log.Info(fmt.Sprintf("Missing field %s for %s", strings.Join(fldPath, "."), u.GetName()))
		return defValue
	}
	return fld
}
