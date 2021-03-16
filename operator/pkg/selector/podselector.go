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
package selector

import (
	"fmt"

	marketplacev1alpha1 "github.com/redhat-marketplace/marketplace-csi-driver/operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
)

func ValidateTerms(terms []marketplacev1alpha1.Selector) error {
	var termError error
	for _, term := range terms {
		for _, sel := range term.MatchExpressions {
			var op selection.Operator
			switch sel.Operator {
			case metav1.LabelSelectorOpIn:
				op = selection.In
			case metav1.LabelSelectorOpNotIn:
				op = selection.NotIn
			case metav1.LabelSelectorOpExists:
				op = selection.Exists
			case metav1.LabelSelectorOpDoesNotExist:
				op = selection.DoesNotExist
			default:
				termError = fmt.Errorf("invalid operator type: %s", sel.String())
				break
			}
			if termError != nil {
				break
			}
			_, termError = labels.NewRequirement(sel.Key, op, sel.Values)
			if termError != nil {
				break
			}
		}
		if termError != nil {
			break
		}
	}
	return termError
}
