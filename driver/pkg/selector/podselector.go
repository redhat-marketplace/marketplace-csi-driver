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

	"github.com/redhat-marketplace/marketplace-csi-driver/driver/pkg/common"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/klog"
)

func GetPodLabels(pod *corev1.Pod) labels.Set {
	if pod == nil {
		return make(map[string]string)
	}
	var lbls map[string]string
	if len(pod.Labels) == 0 {
		lbls = make(map[string]string)
	} else {
		lbls = pod.Labels
	}
	lbls["metadata.name"] = pod.Name
	lbls["metadata.generateName"] = pod.GenerateName
	return lbls
}

func PodMatches(lbls *labels.Set, terms []common.Selector) (bool, []error) {
	if len(terms) == 0 {
		//no targeting specified, all pods are eligible
		return true, nil
	}
	var errs []error
	for _, term := range terms {
		if len(term.MatchExpressions) == 0 && len(term.MatchLabels) == 0 {
			// bad selector term, ignore
			klog.V(4).Infof("BAD SELECTOR IGNORE: %v", term)
			continue
		}
		match, merrs := matches(lbls, term)
		if len(merrs) > 0 {
			errs = append(errs, merrs...)
			continue
		}
		if match {
			return true, nil
		}
	}
	return false, errs
}

func matches(lbls *labels.Set, term common.Selector) (bool, []error) {
	var errs []error
	selector := labels.NewSelector()

	if len(term.MatchLabels) > 0 {
		for k, v := range term.MatchLabels {
			r, err := labels.NewRequirement(k, selection.In, []string{v})
			if err != nil {
				errs = append(errs, err)
			} else {
				selector = selector.Add(*r)
			}
		}
	}

	if len(term.MatchExpressions) > 0 {
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
				errs = append(errs, fmt.Errorf("invalid operator type: %s", sel.String()))
				continue
			}
			r, err := labels.NewRequirement(sel.Key, op, sel.Values)
			if err != nil {
				errs = append(errs, err)
			} else {
				selector = selector.Add(*r)
			}
		}
	}

	if len(errs) > 0 {
		klog.V(4).Infof("Error parsing selectors: %s", errs[0])
		return false, errs
	}
	klog.V(4).Infof("match? %t, selector: %v", selector.Matches(lbls), selector)
	return selector.Matches(lbls), nil
}
