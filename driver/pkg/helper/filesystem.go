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
	"errors"
	"io/ioutil"
	"os"
	"time"

	"k8s.io/klog"
	"k8s.io/utils/mount"
)

func (dv *driverHelper) CheckMount(targetPath string) (bool, error) {
	notMnt, err := mount.New("").IsLikelyNotMountPoint(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			if err = os.MkdirAll(targetPath, 0750); err != nil {
				return false, err
			}
			notMnt = true
		} else {
			return false, err
		}
	}
	return notMnt, nil
}

func (dv *driverHelper) WaitForMount(path string, timeout time.Duration) error {
	var elapsed time.Duration
	var interval = 10 * time.Millisecond
	for {
		notMount, err := mount.New("").IsLikelyNotMountPoint(path)
		if err != nil {
			return err
		}
		if !notMount {
			return nil
		}
		time.Sleep(interval)
		elapsed = elapsed + interval
		if elapsed >= timeout {
			return errors.New("timeout waiting for mount")
		}
	}
}

func (dv *driverHelper) GetDatasetDirectoryNames(targetPath string) []string {
	file, err := os.Open(targetPath)
	if err != nil {
		klog.Errorf("unable to open unmount target path: %s", targetPath)
		return []string{}
	}
	datasets, err := file.Readdirnames(0)
	if err != nil {
		klog.Errorf("unable to list files under target path: %s", targetPath)
		return []string{}
	}
	return datasets
}

func (dv *driverHelper) CleanMountPoint(targetPath string) error {
	return mount.CleanupMountPoint(targetPath, mount.New(""), false)
}

func (dv *driverHelper) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

func (dv *driverHelper) WriteFile(name, content string, flag int, perm os.FileMode) error {
	file, err := os.OpenFile(name, flag, perm)
	if err != nil {
		return err
	}
	_, err = file.WriteString(content)
	if err != nil {
		return err
	}
	file.Close()
	return nil
}

func (dv *driverHelper) FileStat(path string) (os.FileInfo, error) {
	return os.Stat(path)
}

func (dv *driverHelper) ReadDir(path string) ([]os.FileInfo, error) {
	return ioutil.ReadDir(path)
}

func (dv *driverHelper) RemoveFile(path string) error {
	return os.Remove(path)
}
