package policy

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ListOptions returns default list options.
func ListOptions() metav1.ListOptions {
	return metav1.ListOptions{}
}

// GetOptions returns default get options.
func GetOptions() metav1.GetOptions {
	return metav1.GetOptions{}
}

// UpdateOptions returns default update options.
func UpdateOptions() metav1.UpdateOptions {
	return metav1.UpdateOptions{}
}
