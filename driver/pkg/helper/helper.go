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
	"os"
	"time"

	"github.com/dgrijalva/jwt-go"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

type driverHelper struct{}

type DriverHelper interface {
	GetValidator() Validator

	GetClientSet() (kubernetes.Interface, dynamic.Interface, error)
	GetMountCommand() string

	ParseJWTclaims(token string) (jwt.MapClaims, error)
	GetClaimValue(claims jwt.MapClaims, key string) (string, error)

	MarshaljSON(v interface{}) ([]byte, error)
	UnMarshaljSON(data []byte, v interface{}) error

	GetDatasetDirectoryNames(targetPath string) []string
	CheckMount(targetPath string) (bool, error)
	CleanMountPoint(targetPath string) error
	WaitForMount(path string, timeout time.Duration) error
	MkdirAll(path string, perm os.FileMode) error
	WriteFile(name, content string, flag int, perm os.FileMode) error
	FileStat(path string) (os.FileInfo, error)
	ReadDir(path string) ([]os.FileInfo, error)
	RemoveFile(path string) error
}

func NewDriverHelper() DriverHelper {
	return &driverHelper{}
}

func (dv *driverHelper) GetValidator() Validator {
	return &validator{}
}

func (dv *driverHelper) GetMountCommand() string {
	return "s3fs"
}
