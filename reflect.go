package main

import (
	"fmt"
	"reflect"
)

func extractBoolPointerFromRequest(req any) (*bool, error) {
	t := reflect.TypeOf(req)
	if t.Kind() != reflect.Pointer {
		return nil, fmt.Errorf("expected pointer to %s, got %s", t, t.Kind())
	}
	t = t.Elem()
	if t.Kind() != reflect.Struct {
		return nil, fmt.Errorf("expected pointer to struct, got %s", t.Kind())
	}
	f, ok := t.FieldByName("Stream")
	if !ok {
		return nil, fmt.Errorf("missing Stream field in %s", t)
	}
	// if f == nil {
	// 	return nil, nil
	// }
	if f.Type != reflect.TypeOf((*bool)(nil)) {
		return nil, fmt.Errorf("expected *bool, got %s", f.Type)
	}
	// get the value of the Stream field
	v := reflect.ValueOf(req).Elem().FieldByName("Stream")
	if v.Kind() != reflect.Ptr {
		return nil, fmt.Errorf("expected pointer to bool, got %s", v.Kind())
	}
	if v.IsNil() {
		return nil, nil
	}
	b := v.Interface().(*bool)
	return b, nil
}
