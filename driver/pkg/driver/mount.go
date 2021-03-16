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
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/redhat-marketplace/marketplace-csi-driver/driver/pkg/common"
	"github.com/redhat-marketplace/marketplace-csi-driver/driver/pkg/helper"
	"github.com/redhat-marketplace/marketplace-csi-driver/driver/pkg/selector"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/kubernetes-csi/csi-lib-utils/protosanitizer"
	"k8s.io/klog"
)

func MountDatasets(helper helper.DriverHelper, req *csi.NodePublishVolumeRequest, vmtDetails common.VolumeMountDetails) (int, error) {
	klog.V(4).Infof("Method MountDatasets called with: %s", protosanitizer.StripSecrets(req))

	if len(vmtDetails.Datasets) == 0 {
		return 0, nil
	}

	pwFileName, err := setUpCredentials(helper, vmtDetails, req.GetTargetPath())
	if err != nil {
		return 0, err
	}

	var mountError error
	var mountedDatasetCount int
	lblSet := selector.GetPodLabels(vmtDetails.Pod)
	for _, ds := range vmtDetails.Datasets {
		ok, errs := selector.PodMatches(&lblSet, ds.PodSelectors)
		if len(errs) > 0 {
			klog.Errorf("error checking pod selector for ds: %s, err: %s", ds.Folder, errs[0].Error())
			mountError = errs[0]
			continue
		}
		if !ok {
			continue
		}
		err := mountDatasetS3(helper, vmtDetails, req.GetTargetPath(), pwFileName, ds, false)
		if err != nil {
			mountError = err
			continue
		}
		mountedDatasetCount++
	}
	return mountedDatasetCount, mountError
}

func ReconcileDatasets(helper helper.DriverHelper, vmtDetails common.VolumeMountDetails, targetPath string) (int, error) {
	klog.V(4).Infof("Method ReconcileDatasets called with path: %s", targetPath)
	if isPathNotExists(helper, targetPath) {
		klog.Warningf("Dataset path %s does not exist, skip reconcile", targetPath)
		return 0, nil
	}

	if len(vmtDetails.Datasets) == 0 {
		return 0, nil
	}

	pwFileName, err := setUpCredentials(helper, vmtDetails, targetPath)
	if err != nil {
		return 0, err
	}

	var mountError error
	var mountedDatasetCount int
	var processDataset bool
	dsFolders := getMountedFolders(helper, targetPath)
	lblSet := selector.GetPodLabels(vmtDetails.Pod)
	for _, ds := range vmtDetails.Datasets {
		//always fix up any existing mounted folders. revisit when/if we support delete from running pods.
		_, processDataset = dsFolders[ds.Folder]
		if !processDataset {
			ok, errs := selector.PodMatches(&lblSet, ds.PodSelectors)
			if len(errs) > 0 {
				klog.Errorf("error checking pod selector for ds: %s, err: %s", ds.Folder, errs[0].Error())
				mountError = errs[0]
				continue
			}
			processDataset = ok
		}

		if !processDataset {
			continue
		}

		if !isMountRequired(helper, targetPath, ds) {
			continue
		}

		err := mountDatasetS3(helper, vmtDetails, targetPath, pwFileName, ds, true)
		if err != nil {
			mountError = err
			continue
		}
		mountedDatasetCount++
	}
	return mountedDatasetCount, mountError
}

// UnMountDatasets unmount datasets at target path
func UnMountDatasets(helper helper.DriverHelper, req *csi.NodeUnpublishVolumeRequest) {
	targetPath := req.GetTargetPath()
	klog.V(4).Infof("UnMountDatasets running for path: %s", targetPath)
	if isPathNotExists(helper, targetPath) {
		klog.Warningf("Dataset path %s does not exist, skip unmount", targetPath)
		return
	}

	datasets := helper.GetDatasetDirectoryNames(targetPath)
	for _, fname := range datasets {
		klog.V(4).Infof("processing file %s", fname)
		umountPath := fmt.Sprintf("%s"+string(os.PathSeparator)+"%s", targetPath, fname)
		err := helper.CleanMountPoint(umountPath)
		if err != nil {
			klog.Errorf("Error unmounting path %s: %s", umountPath, err)
		}
	}
	helper.CleanMountPoint(targetPath)
}

func mountDatasetS3(helper helper.DriverHelper, vmtDetails common.VolumeMountDetails, path string, pwFileName string, dataset common.Dataset, isReconcile bool) error {
	klog.V(4).Infof("Mounting rhm dataset %s(%s) at target path %s", dataset.Folder, dataset.Bucket, path)
	targetPath := path + string(os.PathSeparator) + dataset.Folder
	if isPathNotExists(helper, targetPath) {
		err := helper.MkdirAll(targetPath, 0750)
		if err != nil {
			klog.Errorf("Create directory failed: %s", err)
			return err
		}
	}

	args := []string{
		fmt.Sprintf("%s", dataset.Bucket),
		fmt.Sprintf("%s", targetPath),
		"-o", "use_path_request_style",
		"-o", fmt.Sprintf("passwd_file=%s", pwFileName),
		"-o", fmt.Sprintf("url=%s", "https://"+string(vmtDetails.Endpoint)),
		"-o", "allow_other",
		"-o", "umask=0000",
	}

	if vmtDetails.Credential.Type != common.CredentialTypeHmac {
		args = append(args, "-o", "ibm_iam_auth")
	}

	if len(vmtDetails.MountOptions) > 0 {
		for _, val := range vmtDetails.MountOptions {
			args = append(args, "-o", val)
		}
	}

	cmd := exec.Command(helper.GetMountCommand(), args...)
	klog.V(4).Infof("Mounting s3fs volume with args: %s", args)

	out, err := cmd.CombinedOutput()
	if err != nil {
		klog.Errorf("Mount failed, response: %s", out)
		return err
	}
	klog.V(4).Infof("Mount response: %s", out)
	return helper.WaitForMount(targetPath, 10*time.Second)
}

func setUpCredentials(helper helper.DriverHelper, vmtDetails common.VolumeMountDetails, targetPath string) (string, error) {
	var formattedCredential string
	if vmtDetails.Credential.Type == common.CredentialTypeDefault {
		formattedCredential = fmt.Sprintf(":%s", vmtDetails.Credential.AccessKey)
	} else {
		formattedCredential = fmt.Sprintf("%s:%s", vmtDetails.Credential.AccessKey, vmtDetails.Credential.AccessSecret)
	}

	pwFileName, err := writeS3fsPass(helper, formattedCredential, targetPath)
	if err != nil {
		return "", err
	}
	return pwFileName, nil
}

func writeS3fsPass(helper helper.DriverHelper, pwFileContent, dsPath string) (string, error) {
	pwFileName := getPasswordFileName(dsPath)
	err := helper.WriteFile(pwFileName, pwFileContent, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return "", err
	}
	return pwFileName, nil
}

func getPasswordFileName(dsPath string) string {
	return fmt.Sprintf("%s"+string(os.PathSeparator)+".passwd-s3fs-rhm-cos", dsPath)
}

func isPathNotExists(helper helper.DriverHelper, path string) bool {
	_, err := helper.FileStat(path)
	if err != nil {
		return os.IsNotExist(err)
	}
	return false
}

func isMountRequired(helper helper.DriverHelper, path string, dataset common.Dataset) bool {
	cleanUpNeeded := false
	mountRequired := false
	targetPath := path + string(os.PathSeparator) + dataset.Folder
	_, err := helper.FileStat(targetPath)
	if err != nil {
		if !os.IsNotExist(err) {
			cleanUpNeeded = true
		}
		mountRequired = true
	}

	if !mountRequired {
		files, err := helper.ReadDir(targetPath)
		if err != nil {
			klog.Warningf("Error listing folders under %s: %s", targetPath, err)
			mountRequired = true
		} else {
			mountRequired = len(files) == 0
		}
		if mountRequired {
			cleanUpNeeded = true
		}
	}

	klog.V(4).Infof("isMountRequired: %t,  cleanUpNeeded: %t for  %s", mountRequired, cleanUpNeeded, dataset.Folder)
	if cleanUpNeeded {
		klog.V(4).Infof("Deleting folder for repair %s", dataset.Folder)
		err = helper.CleanMountPoint(targetPath)
		if err != nil {
			klog.Errorf("Error deleting path for repair %s: %s", targetPath, err)
			return false
		}
	}
	return mountRequired
}

func getMountedFolders(helper helper.DriverHelper, targetPath string) map[string]struct{} {
	folderSet := make(map[string]struct{})
	datasets := helper.GetDatasetDirectoryNames(targetPath)
	for _, name := range datasets {
		folderSet[name] = struct{}{}
	}
	return folderSet
}
