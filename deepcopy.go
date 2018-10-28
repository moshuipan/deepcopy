// deepcopy makes deep copies of things. A standard copy will copy the
// pointers: deep copy copies the values pointed to.  Unexported field
// values are not copied.
//
// Copyright (c)2014-2016, Joel Scoble (github.com/mohae), all rights reserved.
// License: MIT, for more details check the included LICENSE file.
package deepcopy

import (
	"reflect"
	"unsafe"
)

// Interface for delegating copy process to type
// type Interface interface {
// 	DeepCopy() interface{}
// }

// Iface is an alias to Copy; this exists for backwards compatibility reasons.
// func Iface(iface interface{}) interface{} {
// 	return Copy(iface)
// }

// Copy creates a deep copy of whatever is passed to it and returns the copy
// in an interface{}.  The returned value will need to be asserted to the
// correct type.
func Copy(src interface{}) interface{} {
	if src == nil {
		return nil
	}

	// Make the interface a reflect.Value
	original := reflect.ValueOf(src)

	// Make a copy of the same type as the original.
	cpy := reflect.New(original.Type()).Elem()

	// Recursively copy the original.
	copyRecursive(original, cpy)

	// Return the copy as an interface.
	return cpy.Interface()
}

// copyRecursive does the actual copying of the interface. It currently has
// limited support for what it can handle. Add as needed.
func copyRecursive(original, cpy reflect.Value) {
	// check for implement deepcopy.Interface
	// if original.CanInterface() {
	// 	if copier, ok := original.Interface().(Interface); ok {
	// 		setValue(spy,reflect.ValueOf(copier.DeepCopy()))
	// 		return
	// 	}
	// }

	// handle according to original's Kind
	switch original.Kind() {
	case reflect.Ptr:
		// Get the actual value being pointed to.
		originalValue := original.Elem()

		// if  it isn't valid, return.
		if !originalValue.IsValid() {
			return
		}
		setValue(&cpy, reflect.New(originalValue.Type()))
		copyRecursive(originalValue, cpy.Elem())

	case reflect.Interface:
		// If this is a nil, don't do anything
		if original.IsNil() {
			return
		}
		// Get the value for the interface, not the pointer.
		originalValue := original.Elem()

		// Get the value by calling Elem().
		copyValue := reflect.New(originalValue.Type()).Elem()
		copyRecursive(originalValue, copyValue)
		setValue(&cpy, copyValue)

	case reflect.Struct:
		// t, ok := original.Interface().(time.Time)
		// if ok {
		// 	setValue(cpy, reflect.ValueOf(t))
		// 	return
		// }
		// Go through each field of the struct and copy it.
		for i := 0; i < original.NumField(); i++ {
			// The Type's StructField for a given field is checked to see if StructField.PkgPath
			// is set to determine if the field is exported or not because CanSet() returns false
			// for settable fields.  I'm not sure why.  -mohae
			// if original.Type().Field(i).PkgPath != "" {
			// 	continue
			// }
			copyRecursive(original.Field(i), cpy.Field(i))
		}

	case reflect.Slice:
		if original.IsNil() {
			return
		}
		// Make a new slice and copy each element.
		setValue(&cpy, reflect.MakeSlice(original.Type(), original.Len(), original.Cap()))
		for i := 0; i < original.Len(); i++ {
			copyRecursive(original.Index(i), cpy.Index(i))
		}

	case reflect.Map:
		if original.IsNil() {
			return
		}
		setValue(&cpy, reflect.MakeMap(original.Type()))
		for _, key := range original.MapKeys() {
			originalValue := original.MapIndex(key)
			copyValue := reflect.New(originalValue.Type()).Elem()
			copyRecursive(originalValue, copyValue)
			copyKey := reflect.New(key.Type()).Elem()
			// setValue(&copyKey, key)
			copyRecursive(key, copyKey)
			setMapIndex(&cpy, copyKey, copyValue)
		}

	default:
		setValue(&cpy, original)
	}
}

func setValue(dsc *reflect.Value, value reflect.Value) {
	if canExport(dsc) && canExport(&value) {
		dsc.Set(value)
	} else {
		dscV := convertValue(dsc)
		v := convertValue(&value)
		switch dsc.Kind() {
		case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32,
			reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32,
			reflect.Uint64, reflect.Uintptr, reflect.Float32,
			reflect.Float64, reflect.Complex64, reflect.Complex128:
			*(*unsafe.Pointer)(dscV.ptr) = *(*unsafe.Pointer)(v.ptr)
			dscV.typ = v.typ
			dscV.flag = v.flag
		case reflect.Slice:
			ds := (*sliceHeader)(dscV.ptr)
			vs := (*sliceHeader)(v.ptr)
			(*ds) = (*vs)
			dscV.typ = v.typ
			dscV.flag = v.flag
		case reflect.String:
			*(*string)(dscV.ptr) = *(*string)(v.ptr)
		case reflect.Map:
			*(*unsafe.Pointer)(dscV.ptr) = v.ptr
		default:
			dscV.ptr = v.ptr
			dscV.typ = v.typ
			dscV.flag = v.flag
		}
		// flag := dscV.flag
		// dscV.flag = (dscV.flag & 0x9f) | 1<<8
		// v.flag = (dscV.flag & 0x9f) | 1<<8
		// dsc.Set(value)
		// dscV.flag = flag
	}
}

// TODO: support unexported map field
func setMapIndex(dsc *reflect.Value, key, value reflect.Value) {
	if canExport(dsc) && canExport(&value) && canExport(&key) {
		dsc.SetMapIndex(key, value)
	} else {
		dscV := (*Value)(unsafe.Pointer(dsc))
		k := (*Value)(unsafe.Pointer(&key))
		v := (*Value)(unsafe.Pointer(&value))
		dFlag := dscV.flag
		dscV.flag = dscV.flag & (^flagRO)
		k.flag = k.flag & (^flagRO)
		v.flag = v.flag & (^flagRO)
		dsc.SetMapIndex(key, value)
		dscV.flag = dFlag
	}
}

func canExport(x *reflect.Value) bool {
	v := (*Value)(unsafe.Pointer(x))
	return v.flag&flagRO == 0
}
func convertValue(value *reflect.Value) *Value {
	return (*Value)(unsafe.Pointer(value))
}

type Value struct {
	typ  unsafe.Pointer
	ptr  unsafe.Pointer
	flag uintptr
}
type sliceHeader struct {
	Data unsafe.Pointer
	Len  int
	Cap  int
}

// A header for a Go map.
type hmap struct {
	// Note: the format of the hmap is also encoded in cmd/compile/internal/gc/reflect.go.
	// Make sure this stays in sync with the compiler's definition.
	count     int // # live cells == size of map.  Must be first (used by len() builtin)
	flags     uint8
	B         uint8  // log_2 of # of buckets (can hold up to loadFactor * 2^B items)
	noverflow uint16 // approximate number of overflow buckets; see incrnoverflow for details
	hash0     uint32 // hash seed

	buckets    unsafe.Pointer // array of 2^B Buckets. may be nil if count==0.
	oldbuckets unsafe.Pointer // previous bucket array of half the size, non-nil only when growing
	nevacuate  uintptr        // progress counter for evacuation (buckets less than this have been evacuated)

	extra unsafe.Pointer // optional fields
}
type emptyInterface struct {
	typ  *unsafe.Pointer
	word unsafe.Pointer
}

const (
	flagKindWidth           = 5 // there are 27 kinds
	flagKindMask    uintptr = 1<<flagKindWidth - 1
	flagStickyRO    uintptr = 1 << 5
	flagEmbedRO     uintptr = 1 << 6
	flagIndir       uintptr = 1 << 7
	flagAddr        uintptr = 1 << 8
	flagMethod      uintptr = 1 << 9
	flagMethodShift         = 10
	flagRO          uintptr = flagStickyRO | flagEmbedRO
)
