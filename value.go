package djs

// #include "duktape.h"
import "C"
import (
	"reflect"
	"fmt"
	"strings"
	"encoding/json"
)

func pushJsValue(ctx *C.duk_context, v interface{}) {
	if v == nil {
		C.duk_push_null(ctx)
		return
	}

	vv := reflect.ValueOf(v)
	switch vv.Kind() {
	case reflect.Bool:
		if v.(bool) {
			C.duk_push_true(ctx)
		} else {
			C.duk_push_false(ctx)
		}
		return
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		C.duk_push_number(ctx, C.duk_double_t(vv.Int()))
		return
	case reflect.Uint,reflect.Uint8,reflect.Uint16,reflect.Uint32,reflect.Uint64:
		C.duk_push_number(ctx, C.duk_double_t(vv.Uint()))
		return
	case reflect.Float32, reflect.Float64:
		C.duk_push_number(ctx, C.duk_double_t(vv.Float()))
		return
	case reflect.String:
		pushString(ctx, v.(string))
		return
	case reflect.Slice:
		t := vv.Type()
		if t.Elem().Kind() == reflect.Uint8 {
			pushString(ctx, string(v.([]byte)))
			return
		}
		fallthrough
	case reflect.Array:
		pushArr(ctx, vv)
		return
	case reflect.Map:
		pushObj(ctx, vv)
		return
	case reflect.Struct:
		pushStruct(ctx, vv)
		return
	case reflect.Ptr:
		if vv.Elem().Kind() == reflect.Struct {
			pushStruct(ctx, vv)
			return
		}
		pushJsValue(ctx, vv.Elem().Interface())
		return
	case reflect.Func:
		if err := pushGoFunc(ctx, v); err != nil {
			C.duk_push_undefined(ctx)
		}
		return
	default:
		// return fmt.Errorf("unsupported type %v", vv.Kind())
		C.duk_push_undefined(ctx)
		return
	}
}

func fromJsValue(ctx *C.duk_context) (goVal interface{}, err error) {
	var length C.size_t

	switch (C.duk_get_type(ctx, -1)) {
	case C.DUK_TYPE_UNDEFINED, C.DUK_TYPE_NULL, C.DUK_TYPE_NONE:
		return
	case C.DUK_TYPE_BOOLEAN:
		goVal = uint32(C.duk_get_boolean(ctx, -1)) != 0
		return
	case C.DUK_TYPE_NUMBER:
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
		if C.duk_is_function(ctx, -1) != 0 {
			/*
			C.duk_push_null(ctx);   // [ ... null ]
			C.duk_copy(ctx, -2, -1); // [ ... func ]
			createJsFunc
			*/
			//TODO
			err = fmt.Errorf("under implementation")
			return
		}
		if C.duk_is_buffer_data(ctx, -1) != 0 {
			b := (*C.char)(C.duk_get_buffer_data(ctx, -1, &length))
			goVal = toBytes(b, int(length))
			return
		}
		if C.duk_get_error_code(ctx, -1) != 0 {
			s := C.duk_safe_to_lstring(ctx, -1, &length)
			err = fmt.Errorf(*(toString(s, int(length))))
			return
		}

		// if uint32(C.duk_is_array(ctx, -1)) != 0 {
			// array
		// } else {
			// object
		// }
		C.duk_json_encode(ctx, -1)
		s := C.duk_get_lstring(ctx, -1, &length)
		b := toBytes(s, int(length))
		err = json.Unmarshal(b, &goVal)
		return
	// case C.DUK_TYPE_POINTER:
	// case C.DUK_TYPE_LIGHTFUNC:
	default:
		err = fmt.Errorf("unsupporting type")
		return
	}
}

func pushString(ctx *C.duk_context, s string) {
	var cstr *C.char
	var sLen C.int
	getStrPtrLen(&s, &cstr, &sLen)
	C.duk_push_lstring(ctx, cstr, C.size_t(sLen))
}

func pushArr(ctx *C.duk_context, v reflect.Value) {
	arr_idx := C.duk_push_array(ctx) // [ arr ]
	if v.IsNil() {
		return
	}

	l := v.Len()
	for i:=0; i<l; i++ {
		elm := v.Index(i).Interface()
		pushJsValue(ctx, elm) // [ arr elm ]
		C.duk_put_prop_index(ctx, arr_idx, C.duk_uarridx_t(i)) // [ arr ] with arr[i] = elm
	}
}

func pushObj(ctx *C.duk_context, v reflect.Value) {
	C.duk_push_object(ctx) // [ obj ]
	if v.IsNil() {
		return
	}

	mr := v.MapRange()
	for mr.Next() {
		k := mr.Key()
		v := mr.Value()

		pushJsValue(ctx, k.Interface()) // [ obj k ]
		pushJsValue(ctx, v.Interface()) // [ obj k v ]

		C.duk_put_prop(ctx, -3) // [ obj ] with obj[k] = v
	}
}

// struct
func pushStruct(ctx *C.duk_context, structVar reflect.Value) {
	var structE reflect.Value
	if structVar.Kind() == reflect.Ptr {
		structE = structVar.Elem()
	} else {
		structE = structVar
	}
	structT := structE.Type()

	/*
	if structE == structVar {
		// struct is unaddressable, so make a copy of struct to an Elem of struct-pointer.
		// NOTE: changes of the copied struct cannot effect the original one. it is recommended to use the pointer of struct.
		structVar = reflect.New(structT) // make a struct pointer
		structVar.Elem().Set(structE)    // copy the old struct
		structE = structVar.Elem()       // structE is the copied struct
	}*/

	obj_idx := C.duk_push_object(ctx) // [ obj ]
	for i:=0; i<structT.NumField(); i++ {
		name := structT.Field(i).Name
		fv := structE.FieldByName(name)

		lName := lowerFirst(name)
		pushJsValue(ctx, lName)          // [ obj lName ]
		pushJsValue(ctx, fv.Interface()) // [ obj lName fv ]
		C.duk_put_prop(ctx, obj_idx) // [ obj ] with obj[lName] = fv
	}
}

func lowerFirst(name string) string {
	return strings.ToLower(name[:1]) + name[1:]
}
func upperFirst(name string) string {
	return strings.ToUpper(name[:1]) + name[1:]
}

