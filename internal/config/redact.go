package config

import "reflect"

// Redact returns a deep copy of c with every string field tagged
// secret:"true" zeroed. The original c is not mutated. Safe for logging
// and for serializing config for introspection endpoints.
//
// Nested structs are walked recursively. Slices, maps, and pointers to
// structs are supported, though config.Config uses only direct struct
// nesting today — the broader coverage is cheap insurance.
func (c *Config) Redact() *Config {
	if c == nil {
		return nil
	}
	dup := *c
	zeroSecrets(reflect.ValueOf(&dup).Elem())
	return &dup
}

func zeroSecrets(v reflect.Value) {
	switch v.Kind() {
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			f := v.Field(i)
			tag := v.Type().Field(i).Tag.Get("secret")
			if tag == "true" && f.Kind() == reflect.String && f.CanSet() {
				f.SetString("")
				continue
			}
			zeroSecrets(f)
		}
	case reflect.Ptr, reflect.Interface:
		if !v.IsNil() {
			zeroSecrets(v.Elem())
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < v.Len(); i++ {
			zeroSecrets(v.Index(i))
		}
	case reflect.Map:
		for _, k := range v.MapKeys() {
			// Maps in Go reflect are not addressable; copy out, zero, put back.
			elem := reflect.New(v.Type().Elem()).Elem()
			elem.Set(v.MapIndex(k))
			zeroSecrets(elem)
			v.SetMapIndex(k, elem)
		}
	}
}
