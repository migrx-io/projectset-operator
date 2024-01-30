package utils

import (
	"fmt"
	"reflect"
)

func IsEquevalStringMap(a map[string]string, b map[string]string) bool {

	return reflect.DeepEqual(a, b)

}

func IsMapSubset(mapSet interface{}, mapSubset interface{}) bool {

	mapSetValue := reflect.ValueOf(mapSet)
	mapSubsetValue := reflect.ValueOf(mapSubset)

	if fmt.Sprintf("%T", mapSet) != fmt.Sprintf("%T", mapSubset) {
		return false
	}

	if len(mapSetValue.MapKeys()) < len(mapSubsetValue.MapKeys()) {
		return false
	}

	if len(mapSubsetValue.MapKeys()) == 0 {
		return true
	}

	iterMapSubset := mapSubsetValue.MapRange()

	for iterMapSubset.Next() {
		k := iterMapSubset.Key()
		v := iterMapSubset.Value()

		value := mapSetValue.MapIndex(k)

		if !value.IsValid() || v.Interface() != value.Interface() {
			return false
		}
	}

	return true
}

func IsEquevalStringArray(a []string, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for _, ae := range a {
		if !includes(ae, b) {
			return false
		}
	}
	for _, be := range b {
		if !includes(be, a) {
			return false
		}
	}
	return true
}

func includes(a string, b []string) bool {
	for _, be := range b {
		if a == be {
			return true
		}
	}
	return false
}

func IsEquevalStringSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}
