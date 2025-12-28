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

// Setter is an interface that allows setting conditions.
type Setter interface {
	GetConditions() []metav1.Condition
	SetConditions(conditions []metav1.Condition)
}

// SetTrue is used to set a condition to True.
func SetTrue(to Setter, conditionType ConditionType) {
	Set(to, TrueCondition(conditionType))
}

// SetFalse is used to set a condition to False.
func SetFalse(to Setter, conditionType ConditionType, reason ConditionReason, message string) {
	Set(to, FalseCondition(conditionType, reason, message))
}

// SetUnknown is used to set a condition to Unknown.
func SetUnknown(to Setter, conditionType ConditionType, reason, message string) {
	Set(to, UnknownCondition(conditionType, reason, message))
}

// Set is used to set a condition with a specific status.
func Set(to Setter, condition metav1.Condition) {
	conditions := to.GetConditions()
	setCondition(to, conditions, condition)
}

func setCondition(to Setter, conditions []metav1.Condition, condition metav1.Condition) {
	for i := range conditions {
		if conditions[i].Type == string(condition.Type) {
			if conditions[i].Status != condition.Status ||
				conditions[i].Reason != condition.Reason ||
				conditions[i].Message != condition.Message {
				conditions[i] = condition
			}
			to.SetConditions(conditions)
			return
		}
	}

	conditions = append(conditions, condition)
	to.SetConditions(conditions)
}

// TrueCondition returns a condition with Status=True.
func TrueCondition(conditionType ConditionType) metav1.Condition {
	return metav1.Condition{
		Type:               string(conditionType),
		Status:             metav1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
	}
}

// FalseCondition returns a condition with Status=False.
func FalseCondition(conditionType ConditionType, reason ConditionReason, message string) metav1.Condition {
	return metav1.Condition{
		Type:               string(conditionType),
		Status:             metav1.ConditionFalse,
		Reason:             string(reason),
		Message:            message,
		LastTransitionTime: metav1.Now(),
	}
}

// UnknownCondition returns a condition with Status=Unknown.
func UnknownCondition(conditionType ConditionType, reason, message string) metav1.Condition {
	return metav1.Condition{
		Type:               string(conditionType),
		Status:             metav1.ConditionUnknown,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.Now(),
	}
}
