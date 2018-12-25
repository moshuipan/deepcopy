// copy from go src

package deepcopy

import (
	"reflect"
	"unsafe"
)

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
const (
	kindDirectIface = 1 << 5
	kindGCProg      = 1 << 6 // Type.gc points to GC program
	kindNoPointers  = 1 << 7
	kindMask        = (1 << 5) - 1
)

type tflag uint8
type nameOff int32
type typeOff int32

// Value copy from reflect.Value
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

// mapType represents a map type.
type mapType struct {
	rtype
	key           *rtype // map key type
	elem          *rtype // map element (value) type
	bucket        *rtype // internal bucket structure
	keysize       uint8  // size of key slot
	indirectkey   uint8  // store ptr to key instead of key itself
	valuesize     uint8  // size of value slot
	indirectvalue uint8  // store ptr to value instead of value itself
	bucketsize    uint16 // size of bucket
	reflexivekey  bool   // true if k==k for all keys
	needkeyupdate bool   // true if we need to update key on an overwrite
}

type rtype struct {
	size       uintptr
	ptrdata    uintptr        // number of bytes in the type that can contain pointers
	hash       uint32         // hash of type; avoids computation in hash tables
	tflag      tflag          // extra type information flags
	align      uint8          // alignment of variable with this type
	fieldAlign uint8          // alignment of struct field with this type
	kind       uint8          // enumeration for C
	alg        unsafe.Pointer // algorithm table
	gcdata     *byte          // garbage collection data
	str        nameOff        // string form
	ptrToThis  typeOff        // type for pointer to this type, may be zero
}

// interfaceType represents an interface type.
type interfaceType struct {
	rtype
	pkgPath interface{}   // import path
	methods []interface{} // sorted by hash
}
type uncommonType struct {
	pkgPath nameOff // import path; empty for built-in types like int, string
	mcount  uint16  // number of methods
	xcount  uint16  // number of exported methods
	moff    uint32  // offset from this uncommontype to [mcount]method
	_       uint32  // unused
}

// go linkename

//go:linkname typedmemmove reflect.typedmemmove
func typedmemmove(t, dst, src unsafe.Pointer)

//go:linkname mapassign reflect.mapassign
func mapassign(t unsafe.Pointer, m unsafe.Pointer, key, val unsafe.Pointer)

//go:linkname mapdelete reflect.mapdelete
func mapdelete(t unsafe.Pointer, m unsafe.Pointer, key unsafe.Pointer)

//go:linkname makeMethodValue reflect.makeMethodValue
func makeMethodValue(op string, v Value) Value

//go:linkname directlyAssignable reflect.directlyAssignable
func directlyAssignable(T, V unsafe.Pointer) bool

//go:linkname implements reflect.implements
func implements(T, V unsafe.Pointer) bool

//go:linkname unsafe_New reflect.unsafe_New
func unsafe_New(unsafe.Pointer) unsafe.Pointer

//go:linkname valueInterface reflect.valueInterface
func valueInterface(v Value, safe bool) interface{}

//go:linkname ifaceE2I reflect.ifaceE2I
func ifaceE2I(t unsafe.Pointer, src interface{}, dst unsafe.Pointer)

// func setValue(dsc *reflect.Value, value reflect.Value) {
// 	if canExport(dsc) && canExport(&value) {
// 		dsc.Set(value)
// 	} else {
// 		dscV := convertValue(dsc)
// 		v := convertValue(&value)
// 		switch dsc.Kind() {
// 		case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32,
// 			reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32,
// 			reflect.Uint64, reflect.Uintptr, reflect.Float32,
// 			reflect.Float64, reflect.Complex64, reflect.Complex128:
// 			*(*unsafe.Pointer)(dscV.ptr) = *(*unsafe.Pointer)(v.ptr)
// 			dscV.typ = v.typ
// 			dscV.flag = v.flag
// 		case reflect.Slice:
// 			ds := (*sliceHeader)(dscV.ptr)
// 			vs := (*sliceHeader)(v.ptr)
// 			(*ds) = (*vs)
// 			dscV.typ = v.typ
// 			dscV.flag = v.flag
// 		case reflect.String:
// 			*(*string)(dscV.ptr) = *(*string)(v.ptr)
// 		case reflect.Map:
// 			*(*unsafe.Pointer)(dscV.ptr) = v.ptr
// 		default:
// 			dscV.ptr = v.ptr
// 			dscV.typ = v.typ
// 			dscV.flag = v.flag
// 		}
// 	}
// }
func setValue(dsc *reflect.Value, value reflect.Value) {
	vv := convertValue(dsc)
	var target unsafe.Pointer
	if reflect.Kind(vv.flag&flagKindMask) == reflect.Interface {
		target = vv.ptr
	}
	xv := convertValue(&value)
	t := assignTo(*xv, "reflect.Set", (*rtype)(vv.typ), target)
	if t.flag&flagIndir != 0 {
		typedmemmove(vv.typ, vv.ptr, t.ptr)
	} else {
		*(*unsafe.Pointer)(vv.ptr) = t.ptr
	}
}

// func setMapIndex(dsc *reflect.Value, key, value reflect.Value) {
// 	if canExport(dsc) && canExport(&value) && canExport(&key) {
// 		dsc.SetMapIndex(key, value)
// 	} else {
// 		dscV := (*Value)(unsafe.Pointer(dsc))
// 		k := (*Value)(unsafe.Pointer(&key))
// 		v := (*Value)(unsafe.Pointer(&value))
// 		dFlag := dscV.flag
// 		dscV.flag = dscV.flag & (^flagRO)
// 		k.flag = k.flag & (^flagRO)
// 		v.flag = v.flag & (^flagRO)
// 		dsc.SetMapIndex(key, value)
// 		dscV.flag = dFlag
// 	}
// }

func canExport(x *reflect.Value) bool {
	v := (*Value)(unsafe.Pointer(x))
	return v.flag&flagRO == 0
}
func convertValue(value *reflect.Value) *Value {
	return (*Value)(unsafe.Pointer(value))
}

func setMapIndex(v, key, val reflect.Value) {
	vv := convertValue(&v)
	tt := (*mapType)(vv.typ)
	keyA := assignTo(*convertValue(&key), "reflect.Value.SetMapIndex", tt.key, nil)
	key = *(*reflect.Value)(unsafe.Pointer(&keyA))
	// key = key.assignTo("reflect.Value.SetMapIndex", tt.key, nil)
	var k unsafe.Pointer
	keyK := convertValue(&key)
	if keyK.flag&flagIndir != 0 {
		k = keyK.ptr
	} else {
		k = keyK.ptr
	}
	valV := convertValue(&val)
	if valV.typ == nil {
		mapdelete(vv.typ, vv.ptr, k)
		return
	}

	valA := assignTo(*convertValue(&val), "reflect.Value.SetMapIndex", tt.elem, nil)
	val = *(*reflect.Value)(unsafe.Pointer(&valA))

	valV = convertValue(&val)
	var e unsafe.Pointer
	if valV.flag&flagIndir != 0 {
		e = valV.ptr
	} else {
		e = valV.ptr
	}
	pointer := vv.ptr
	if vv.flag&flagIndir != 0 {
		pointer = *(*unsafe.Pointer)(vv.ptr)
	}

	mapassign(vv.typ, pointer, k, e)
}

func assignTo(v Value, context string, dst *rtype, target unsafe.Pointer) Value {
	if v.flag&flagMethod != 0 {
		v = makeMethodValue(context, v)
	}

	switch {
	case directlyAssignable(unsafe.Pointer(dst), v.typ):
		// Overwrite type so that they match.
		// Same memory layout, so no harm done.
		fl := v.flag&(flagAddr|flagIndir) | ro(v.flag)
		fl |= uintptr(reflect.Kind(dst.kind & kindMask))
		return Value{unsafe.Pointer(dst), v.ptr, fl}

	case implements(unsafe.Pointer(dst), v.typ):
		if target == nil {
			target = unsafe_New(unsafe.Pointer(dst))
		}
		if reflect.Kind(v.flag&flagKindMask) == reflect.Interface && isNil(v) {
			// A nil ReadWriter passed to nil Reader is OK,
			// but using ifaceE2I below will panic.
			// Avoid the panic by returning a nil dst (e.g., Reader) explicitly.
			return Value{unsafe.Pointer(dst), nil, uintptr(reflect.Interface)}
		}
		x := valueInterface(v, false)
		if rtypeNumMethod(dst) == 0 {
			*(*interface{})(target) = x
		} else {
			ifaceE2I(unsafe.Pointer(dst), x, target)
		}
		return Value{unsafe.Pointer(dst), target, flagIndir | uintptr(reflect.Interface)}
	}

	// Failed.
	z := (*rtype)(v.typ)
	panic(context + ": value of type " + string(z.str) + " is not assignable to type " + string(dst.str))
}

func ro(f uintptr) uintptr {
	if f&flagRO != 0 {
		return flagStickyRO
	}
	return 0
}

func isNil(v Value) bool {
	k := reflect.Kind(v.flag & flagKindMask)
	switch k {
	case reflect.Chan, reflect.Func, reflect.Map, reflect.Ptr:
		if v.flag&flagMethod != 0 {
			return false
		}
		ptr := v.ptr
		if v.flag&flagIndir != 0 {
			ptr = *(*unsafe.Pointer)(ptr)
		}
		return ptr == nil
	case reflect.Interface, reflect.Slice:
		// Both interface and slice are nil if first word is 0.
		// Both are always bigger than a word; assume flagIndir.
		return *(*unsafe.Pointer)(v.ptr) == nil
	}
	panic(&reflect.ValueError{"reflect.Value.IsNil", reflect.Kind(v.flag & flagKindMask)})
}

func rtypeNumMethod(t *rtype) int {
	if reflect.Kind(t.kind&kindMask) == reflect.Interface {
		tt := (*interfaceType)(unsafe.Pointer(t))
		return len(tt.methods)
	}
	type u struct {
		u uncommonType
	}
	com := &(*u)(unsafe.Pointer(t)).u
	return int(com.xcount)
}
