package cache

import (
	"reflect"

	caches "github.com/mgdigital/gorm-cache/v2"
)

// deepCopyQuery returns a copy of a cached query whose destination shares no
// mutable state with the original. The gorm-cache library stores the result
// slice by reference and later hydration callbacks mutate the items in place;
// without a copy, concurrent requests served from the same cache entry would
// race on (and corrupt) each other's results.
func deepCopyQuery(q *caches.Query) *caches.Query {
	if q == nil {
		return nil
	}

	return &caches.Query{
		Dest:         deepCopyValue(reflect.ValueOf(q.Dest)).Interface(),
		RowsAffected: q.RowsAffected,
	}
}

func deepCopyValue(v reflect.Value) reflect.Value {
	if !v.IsValid() {
		return v
	}

	switch v.Kind() {
	case reflect.Pointer:
		if v.IsNil() {
			return v
		}

		copied := reflect.New(v.Type().Elem())
		copied.Elem().Set(deepCopyValue(v.Elem()))

		return copied

	case reflect.Interface:
		if v.IsNil() {
			return v
		}

		return deepCopyValue(v.Elem())

	case reflect.Slice:
		if v.IsNil() {
			return v
		}

		copied := reflect.MakeSlice(v.Type(), v.Len(), v.Len())
		for i := 0; i < v.Len(); i++ {
			copied.Index(i).Set(deepCopyValue(v.Index(i)))
		}

		return copied

	case reflect.Array:
		copied := reflect.New(v.Type()).Elem()
		for i := 0; i < v.Len(); i++ {
			copied.Index(i).Set(deepCopyValue(v.Index(i)))
		}

		return copied

	case reflect.Map:
		if v.IsNil() {
			return v
		}

		copied := reflect.MakeMapWithSize(v.Type(), v.Len())
		iter := v.MapRange()
		for iter.Next() {
			copied.SetMapIndex(iter.Key(), deepCopyValue(iter.Value()))
		}

		return copied

	case reflect.Struct:
		copied := reflect.New(v.Type()).Elem()
		for i := 0; i < v.NumField(); i++ {
			if copied.Field(i).CanSet() {
				copied.Field(i).Set(deepCopyValue(v.Field(i)))
			}
		}

		return copied

	default:
		return v
	}
}
