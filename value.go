package djs

// #include "duktape.h"
// static const char *getCString(duk_context *ctx, duk_idx_t idx);
import "C"
import (
	"unsafe"
	"fmt"
	"math"
)

func fromJsValue(ctx *C.duk_context) (goVal interface{}, err error) {
	var length C.size_t

	switch C.duk_get_type(ctx, -1) {
	case C.DUK_TYPE_UNDEFINED, C.DUK_TYPE_NULL, C.DUK_TYPE_NONE:
		return
	case C.DUK_TYPE_BOOLEAN:
		goVal = uint32(C.duk_get_boolean(ctx, -1)) != 0
		return
	case C.DUK_TYPE_NUMBER:
		if C.duk_is_nan(ctx, -1) != 0 {
			goVal = math.NaN()
			return
		}
		goVal = float64(C.duk_get_number(ctx, -1))
		return
	case C.DUK_TYPE_STRING:
		s := C.duk_get_lstring(ctx, -1, &length)
		goVal = *(toString(s, int(length)))
		return
	case C.DUK_TYPE_BUFFER:
		b := (*C.char)(C.duk_get_buffer(ctx, -1, &length))
		goVal = toBytes(b, int(length))
		return
	case C.DUK_TYPE_OBJECT:
		switch {
		case C.duk_is_function(ctx, -1) != 0:
			goVal = fromJsFunc(ctx)
			return
		case C.duk_is_buffer_data(ctx, -1) != 0:
			b := (*C.char)(C.duk_get_buffer_data(ctx, -1, &length))
			goVal = toBytes(b, int(length))
			return
		case C.duk_get_error_code(ctx, -1) != 0:
			s := C.duk_safe_to_lstring(ctx, -1, &length)
			err = fmt.Errorf(*(toString(s, int(length))))
			return
		case C.duk_is_array(ctx, -1) != 0:
			// array
			return fromJsArr(ctx)
		case C.duk_is_c_function(ctx, -1) != 0:
			// c function
			return fromCFunc(ctx)
		default:
			// object
			return fromJsObj(ctx)
		}
	case C.DUK_TYPE_POINTER:
		goVal = unsafe.Pointer(C.duk_get_pointer(ctx, -1))
		return
	// case C.DUK_TYPE_LIGHTFUNC:
	default:
		err = fmt.Errorf("unsupporting type")
		return
	}
}

func fromJsArr(ctx *C.duk_context) (goVal interface{}, err error) {
	// [ ... arr ]
	var isProxy bool
	if goVal, isProxy = getTargetValue(ctx, -1); isProxy {
		return
	}

	l := C.duk_get_length(ctx, -1)
	if l == 0 {
		goVal = []interface{}{}
		return
	}

	length := int(l)
	res := make([]interface{}, length)
	for i:=0; i<length; i++ {
		C.duk_get_prop_index(ctx, -1, C.duk_uarridx_t(i)) // [ ... arr i-th-value ]
		val, e := fromJsValue(ctx)
		if e != nil {
			err = e
			C.duk_pop(ctx)
			return
		}
		if _, ok := val.(string); ok {
			val = fmt.Sprintf("%s", val) // deep copy
		}
		res[i] = val
		C.duk_pop(ctx) // [ ... arr ]
	}
	goVal = res
	return
}

func fromJsObj(ctx *C.duk_context) (goVal interface{}, err error) {
	// [ ... obj ]
	var isProxy bool
	if goVal, isProxy = getTargetValue(ctx, -1); isProxy {
		return
	}

	C.duk_enum(ctx, -1, 0) // [ ... obj enum ]
	res := make(map[string]interface{})
	for C.duk_next(ctx, -1, 1) != 0 {
		// [ ... obj enum key value ]
		key := C.GoString(C.getCString(ctx, -2))
		val, e := fromJsValue(ctx)
		if e != nil {
			err = e
			C.duk_pop_n(ctx, 3) // [ ... obj ]
			return
		}
		if _, ok := val.(string); ok {
			val = fmt.Sprintf("%s", val) // deep copy
		}
		res[key] = val
		C.duk_pop_n(ctx, 2) // [ ... obj enum ]
	}
	C.duk_pop(ctx) // [ ... obj ]
	goVal = res
	return
}

func fromCFunc(ctx *C.duk_context) (goVal interface{}, err error) {
	// [ ... c-function ]
	var isProxy bool
	if goVal, isProxy = getTargetValue(ctx, -1); isProxy {
		return
	}
	err = fmt.Errorf("cannot process such c-function")
	return
}
