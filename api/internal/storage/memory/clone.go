package memory

import "reflect"

// clone returns a deep copy of v: what the store keeps never shares memory
// with what a caller passed in or got back, as a database row would not. It
// follows pointers, slices, maps, interfaces and the exported fields of
// structs; unexported fields are copied as they are (time.Time's location,
// immutable).
func clone[T any](v T) T {
	src := reflect.ValueOf(&v).Elem()
	out := reflect.New(src.Type()).Elem()
	out.Set(deep(src))
	return out.Interface().(T)
}

func deep(v reflect.Value) reflect.Value {
	switch v.Kind() {
	case reflect.Pointer:
		if v.IsNil() {
			return v
		}
		p := reflect.New(v.Type().Elem())
		p.Elem().Set(deep(v.Elem()))
		return p
	case reflect.Slice:
		if v.IsNil() {
			return v
		}
		s := reflect.MakeSlice(v.Type(), v.Len(), v.Len())
		for i := range v.Len() {
			s.Index(i).Set(deep(v.Index(i)))
		}
		return s
	case reflect.Map:
		if v.IsNil() {
			return v
		}
		m := reflect.MakeMapWithSize(v.Type(), v.Len())
		iter := v.MapRange()
		for iter.Next() {
			m.SetMapIndex(deep(iter.Key()), deep(iter.Value()))
		}
		return m
	case reflect.Interface:
		if v.IsNil() {
			return v
		}
		i := reflect.New(v.Type()).Elem()
		i.Set(deep(v.Elem()))
		return i
	case reflect.Struct:
		s := reflect.New(v.Type()).Elem()
		s.Set(v)
		for i := range v.NumField() {
			if s.Field(i).CanSet() {
				s.Field(i).Set(deep(v.Field(i)))
			}
		}
		return s
	case reflect.Array:
		a := reflect.New(v.Type()).Elem()
		for i := range v.Len() {
			a.Index(i).Set(deep(v.Index(i)))
		}
		return a
	default:
		return v
	}
}
