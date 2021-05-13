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

package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/redhat-marketplace/marketplace-csi-driver/driver/pkg/driver"
	"k8s.io/klog"
)

var (
	version        = "1.0.0"
	nodeNameFlag   = flag.String("node-name", "", "Node identifier")
	driverNameFlag = flag.String("driver-name", "csi.marketplace.redhat.com", "CSI driver name")
	endpointFlag   = flag.String("csi-endpoint", "unix:///csi/csi.sock", "CSI endpoint")
	modeFlag       = flag.String("exec-mode", "driver", "Container mode, either driver, reconcile")
	versionFlag    = flag.Bool("version", false, "Print the version and exit")
)

func main() {
	_ = flag.Set("alsologtostderr", "true")
	klog.InitFlags(nil)
	setEnvVarFlags()
	flag.Parse()
	driver.DriverVersion = version

	if *versionFlag {
		versionJSON := driver.GetVersionJSON()
		fmt.Println(versionJSON)
		os.Exit(0)
	}

	d, err := driver.NewCOSDriver(*driverNameFlag, *nodeNameFlag, version, *endpointFlag)
	if err != nil {
		klog.Error(err.Error())
		os.Exit(1)
	}

	klog.V(1).Info("Starting driver")
	if *modeFlag == "reconcile" {
		reconcileStart(d)
	} else {
		driverStart(d)
	}
}

func driverStart(d *driver.COSDriver) {
	if err := d.Run(); err != nil {
		klog.Error(err.Error())
		os.Exit(1)
	}
}

func reconcileStart(d *driver.COSDriver) {
	rc, err := driver.NewReconcileContext(d)
	if err != nil {
		klog.Error(err.Error())
		os.Exit(1)
	}
	klog.V(4).Info("Starting driver in reconcile mode")
	stopCh := make(chan struct{})

	go rc.StartInformers(stopCh)

	sigCh := make(chan os.Signal, 0)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL)
	<-sigCh
	close(stopCh)
}

func setEnvVarFlags() {
	flagset := flag.CommandLine

	set := map[string]string{}
	flagset.Visit(func(f *flag.Flag) {
		set[f.Name] = ""
	})

	flagset.VisitAll(func(f *flag.Flag) {
		envVar := strings.Replace(strings.ToUpper(f.Name), "-", "_", -1)

		if val := os.Getenv(envVar); val != "" {
			if _, defined := set[f.Name]; !defined {
				_ = flagset.Set(f.Name, val)
			}
		}

		// Display it in the help text too
		f.Usage = fmt.Sprintf("%s [%s]", f.Usage, envVar)
	})
}
