/*
Copyright 2025 OneX Team.

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

package condition

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Getter is an interface that allows getting conditions.
type Getter interface {
	GetConditions() []metav1.Condition
}

// Get returns the condition with the given type.
func Get(from Getter, conditionType ConditionType) *metav1.Condition {
	for _, c := range from.GetConditions() {
		if c.Type == string(conditionType) {
			return &c
		}
	}
	return nil
}

// Has returns true if the condition with the given type exists.
func Has(from Getter, conditionType ConditionType) bool {
	return Get(from, conditionType) != nil
}

// IsTrue returns true if the condition with the given type has status True.
func IsTrue(from Getter, conditionType ConditionType) bool {
	if c := Get(from, conditionType); c != nil {
		return c.Status == metav1.ConditionTrue
	}
	return false
}

// IsFalse returns true if the condition with the given type has status False.
func IsFalse(from Getter, conditionType ConditionType) bool {
	if c := Get(from, conditionType); c != nil {
		return c.Status == metav1.ConditionFalse
	}
	return false
}

// IsUnknown returns true if the condition with the given type has status Unknown.
func IsUnknown(from Getter, conditionType ConditionType) bool {
	if c := Get(from, conditionType); c != nil {
		return c.Status == metav1.ConditionUnknown
	}
	return true
}
