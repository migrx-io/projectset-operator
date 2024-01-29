package utils

import (
	"reflect"

	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// ResourceGenerationOrFinalizerChangedPredicate this predicate will fire an update event when the spec of a resource is changed (controller by ResourceGeneration), or when the finalizers are changed
type ResourceGenerationOrFinalizerChangedPredicate struct {
	predicate.Funcs
}

// Update implements default UpdateEvent filter for validating resource version change
func (ResourceGenerationOrFinalizerChangedPredicate) Update(e event.UpdateEvent) bool {
	if e.ObjectOld == nil {
		return false
	}
	if e.ObjectNew == nil {
		return false
	}
	if e.ObjectNew.GetGeneration() == e.ObjectOld.GetGeneration() && reflect.DeepEqual(e.ObjectNew.GetFinalizers(), e.ObjectOld.GetFinalizers()) {
		return false
	}
	return true
}
