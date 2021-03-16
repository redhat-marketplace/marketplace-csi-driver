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
	"fmt"
	"net"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/redhat-marketplace/marketplace-csi-driver/driver/pkg/helper"
	"google.golang.org/grpc"
	"k8s.io/klog"
)

// COSDriver data structure
type COSDriver struct {
	name     string
	nodeName string
	version  string
	endpoint string
	server   *grpc.Server
	helper   helper.DriverHelper
}

// NewCOSDriver returns a new instance of COS driver
func NewCOSDriver(name, node, version, endpoint string) (*COSDriver, error) {
	return &COSDriver{
		name:     name,
		nodeName: node,
		version:  version,
		endpoint: endpoint,
		helper:   helper.NewDriverHelper(),
	}, nil
}

// Run COS driver
func (driver *COSDriver) Run() error {
	scheme, address, err := driver.parseEndpoint(driver.endpoint)
	if err != nil {
		return err
	}

	listener, err := net.Listen(scheme, address)
	if err != nil {
		return err
	}
	logHandler := func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		resp, err := handler(ctx, req)
		if err == nil {
			klog.V(4).Infof("Method %s completed", info.FullMethod)
		} else {
			klog.Errorf("Method %s failed with error: %v", info.FullMethod, err)
		}
		return resp, err
	}

	klog.V(1).Infof("Starting Red Hat Marketplace CSI Driver - driver: `%s`, version(internal): `%s`, gRPC socket: `%s`", driver.name, driver.version, driver.endpoint)
	driver.server = grpc.NewServer(grpc.UnaryInterceptor(logHandler))
	csi.RegisterIdentityServer(driver.server, driver)
	csi.RegisterNodeServer(driver.server, driver)
	csi.RegisterControllerServer(driver.server, driver)
	return driver.server.Serve(listener)
}

func (driver *COSDriver) parseEndpoint(endpoint string) (string, string, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", "", fmt.Errorf("could not parse endpoint: %v", err)
	}

	var address string
	if len(u.Host) == 0 {
		address = filepath.FromSlash(u.Path)
	} else {
		address = path.Join(u.Host, filepath.FromSlash(u.Path))
	}

	scheme := strings.ToLower(u.Scheme)
	if scheme == "unix" {
		if err := driver.helper.RemoveFile(address); err != nil && !os.IsNotExist(err) {
			return "", "", fmt.Errorf("could not remove unix socket %q: %v", address, err)
		}
	} else {
		return "", "", fmt.Errorf("unsupported protocol: %s", scheme)
	}

	return scheme, address, nil
}
