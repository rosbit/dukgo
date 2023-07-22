package djs

// #include "duktape.h"
// extern duk_ret_t go_obj_get(duk_context *ctx);
// extern duk_ret_t go_obj_set(duk_context *ctx);
// extern duk_ret_t go_obj_has(duk_context *ctx);
// extern duk_ret_t freeTarket(duk_context *ctx);
import "C"
import (
	elutils "github.com/rosbit/go-embedding-utils"
	"reflect"
	"unsafe"
	"fmt"
	"strings"
)

func pushJsProxyValue(ctx *C.duk_context, v interface{}) {
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
		C.duk_push_bare_array(ctx)
		makeProxyObject(ctx, v)
		return
	case reflect.Map, reflect.Struct:
		C.duk_push_bare_object(ctx)
		makeProxyObject(ctx, v)
		return
	case reflect.Ptr:
		if vv.Elem().Kind() == reflect.Struct {
			C.duk_push_bare_object(ctx)
			makeProxyObject(ctx, v)
			return
		}
		pushJsProxyValue(ctx, vv.Elem().Interface())
		return
	case reflect.Func:
		pushGoFunc(ctx, v)
		return
	default:
		C.duk_push_undefined(ctx)
		return
	}
}

func getTargetIdx(ctx *C.duk_context, targetIdx ...C.duk_idx_t) (idx int) {
	// [ 0 ] target if no targetIdx
	// ...
	var tIdx C.duk_idx_t
	if len(targetIdx) > 0 {
		tIdx = targetIdx[0]
	}

	var name *C.char
	getStrPtr(&idxName, &name)
	C.duk_get_prop_string(ctx, tIdx, name) // [ ... idx ]
	idx = int(C.duk_to_int(ctx, -1))
	C.duk_pop(ctx) // [ ... ]
	return
}

func getTargetValue(ctx *C.duk_context, targetIdx ...C.duk_idx_t) (v interface{}, ok bool) {
	// [ 0 ] target if no targetIdx
	// ....
	idx := getTargetIdx(ctx, targetIdx...)

	ptr := getPtrStore(uintptr(unsafe.Pointer(ctx)))
	vPtr, o := ptr.lookup(idx)
	if !o {
		return
	}
	if vv, o := vPtr.(*interface{}); o {
		v = *vv
		ok = true
	}
	return
}

func go_arr_get(ctx *C.duk_context, vv reflect.Value) C.duk_ret_t {
	/* 'this' binding: handler
	 * [0]: target
	 * [1]: key
	 * [2]: receiver (proxy)
	 */
	if C.duk_is_string(ctx, 1) != 0 {
		key := C.GoString(C.duk_get_string(ctx, 1))
		if key == "length" {
			C.duk_push_int(ctx, C.duk_int_t(vv.Len()))
			return 1
		}
		C.duk_push_undefined(ctx)
		return 1
	}
	if C.duk_is_number(ctx, 1) == 0 {
		C.duk_push_undefined(ctx)
		return 1
	}
	key := int(C.duk_to_int(ctx, 1))
	l := vv.Len()
	if key < 0 || key >= l {
		C.duk_push_undefined(ctx)
		return 1
	}
	val := vv.Index(key)
	if !val.IsValid() || !val.CanInterface() {
		C.duk_push_undefined(ctx)
		return 1
	}
	pushJsProxyValue(ctx, val.Interface())
	return 1
}

func go_arr_set(ctx *C.duk_context, vv reflect.Value) C.duk_ret_t {
	/* 'this' binding: handler
	 * [0]: target
	 * [1]: key
	 * [2]: val
	 * [3]: receiver (proxy)
	 */
	if C.duk_is_number(ctx, 1) == 0 {
		C.duk_push_false(ctx)
		return 1
	}
	key := int(C.duk_to_int(ctx, 1))

	C.duk_dup(ctx, 2) // [ ... val ]
	goVal, err := fromJsValue(ctx)
	C.duk_pop(ctx)    // [ ... ]
	if err != nil {
		C.duk_push_false(ctx)
		return 1
	}

	l := vv.Len()
	if key < 0 || key >= l {
		C.duk_push_false(ctx)
		return 1
	}
	dest := vv.Index(key)
	if _, ok := goVal.(string); ok {
		goVal = fmt.Sprintf("%s", goVal) // deep copy
	}
	if err = elutils.SetValue(dest, goVal); err != nil {
		C.duk_push_false(ctx)
	} else {
		C.duk_push_true(ctx)
	}
	return 1
}

func go_arr_has(ctx *C.duk_context, vv reflect.Value) C.duk_ret_t {
	/* 'this' binding: handler
	 * [0]: target
	 * [1]: key
	 */
	if C.duk_is_string(ctx, 1) != 0 {
		key := C.GoString(C.duk_get_string(ctx, 1))
		if key == "length" {
			C.duk_push_true(ctx)
			return 1
		}
		C.duk_push_false(ctx)
		return 1
	}
	if C.duk_is_number(ctx, 1) == 0 {
		C.duk_push_false(ctx)
		return 1
	}
	key := int(C.duk_to_int(ctx, 1))
	l := vv.Len()
	if key < 0 || key >= l {
		C.duk_push_false(ctx)
		return 1
	}
	C.duk_push_true(ctx)
	return 1
}

func go_map_get(ctx *C.duk_context, vv reflect.Value) C.duk_ret_t {
	/* 'this' binding: handler
	 * [0]: target
	 * [1]: key
	 * [2]: receiver (proxy)
	 */
	if C.duk_is_string(ctx, 1) == 0 {
		C.duk_push_undefined(ctx)
		return 1
	}
	key := C.GoString(C.duk_get_string(ctx, 1))
	val := vv.MapIndex(reflect.ValueOf(key))
	if !val.IsValid() || !val.CanInterface() {
		C.duk_push_undefined(ctx)
		return 1
	}
	pushJsProxyValue(ctx, val.Interface())
	return 1
}

func go_map_set(ctx *C.duk_context, vv reflect.Value) C.duk_ret_t {
	/* 'this' binding: handler
	 * [0]: target
	 * [1]: key
	 * [2]: val
	 * [3]: receiver (proxy)
	 */
	if C.duk_is_string(ctx, 1) == 0 {
		C.duk_push_false(ctx)
		return 1
	}
	key := C.GoString(C.duk_get_string(ctx, 1))

	C.duk_dup(ctx, 2) // [ ... val ]
	goVal, err := fromJsValue(ctx)
	C.duk_pop(ctx)    // [ ... ]
	if err != nil {
		C.duk_push_false(ctx)
		return 1
	}

	mapT := vv.Type()
	elType := mapT.Elem()
	dest := elutils.MakeValue(elType)
	if _, ok := goVal.(string); ok {
		goVal = fmt.Sprintf("%s", goVal) // deep copy
	}
	if err = elutils.SetValue(dest, goVal); err == nil {
		vv.SetMapIndex(reflect.ValueOf(key), dest)
		C.duk_push_true(ctx)
	} else {
		C.duk_push_false(ctx)
	}
	return 1
}

func go_map_has(ctx *C.duk_context, vv reflect.Value) C.duk_ret_t {
	/* 'this' binding: handler
	 * [0]: target
	 * [1]: key
	 */
	if C.duk_is_string(ctx, 1) == 0 {
		C.duk_push_false(ctx)
		return 1
	}
	key := C.GoString(C.duk_get_string(ctx, 1))
	val := vv.MapIndex(reflect.ValueOf(key))
	if !val.IsValid() {
		C.duk_push_false(ctx)
	} else {
		C.duk_push_true(ctx)
	}
	return 1
}

func go_struct_get(ctx *C.duk_context, structVar reflect.Value) C.duk_ret_t {
	/* 'this' binding: handler
	 * [0]: target
	 * [1]: key
	 * [2]: receiver (proxy)
	 */
	if C.duk_is_string(ctx, 1) == 0 {
		C.duk_push_undefined(ctx)
		return 1
	}
	key := C.GoString(C.duk_get_string(ctx, 1))
	var structE reflect.Value
	switch structVar.Kind() {
	case reflect.Struct:
		structE = structVar
	case reflect.Ptr:
		if structVar.Elem().Kind() != reflect.Struct {
			C.duk_push_undefined(ctx)
			return 1
		}
		structE = structVar.Elem()
	default:
		C.duk_push_undefined(ctx)
		return 1
	}
	name := upperFirst(key)
	fv := structE.FieldByName(name)
	if !fv.IsValid() {
		fv = structE.MethodByName(name)
		if !fv.IsValid() {
			if structE == structVar {
				C.duk_push_undefined(ctx)
				return 1
			}
			fv = structVar.MethodByName(name)
			if !fv.IsValid() {
				C.duk_push_undefined(ctx)
				return 1
			}
		}
		if fv.CanInterface() {
			pushGoFunc(ctx, fv.Interface())
			return 1
		}
		C.duk_push_undefined(ctx)
		return 1
	}
	if !fv.CanInterface() {
		C.duk_push_undefined(ctx)
		return 1
	}
	pushJsProxyValue(ctx, fv.Interface())
	return 1
}

func go_struct_set(ctx *C.duk_context, vv reflect.Value) C.duk_ret_t {
	/* 'this' binding: handler
	 * [0]: target
	 * [1]: key
	 * [2]: val
	 * [3]: receiver (proxy)
	 */
	if C.duk_is_string(ctx, 1) == 0 {
		C.duk_push_false(ctx)
		return 1
	}
	key := C.GoString(C.duk_get_string(ctx, 1))

	C.duk_dup(ctx, 2) // [ ... val ]
	goVal, err := fromJsValue(ctx)
	C.duk_pop(ctx)    // [ ... ]
	if err != nil {
		C.duk_push_false(ctx)
		return 1
	}

	var structE reflect.Value
	switch vv.Kind() {
	case reflect.Struct:
		structE = vv
	case reflect.Ptr:
		if vv.Elem().Kind() != reflect.Struct {
			C.duk_push_undefined(ctx)
			return 1
		}
		structE = vv.Elem()
	default:
		C.duk_push_false(ctx)
		return 1
	}
	name := upperFirst(key)
	fv := structE.FieldByName(name)
	if !fv.IsValid() {
		C.duk_push_false(ctx)
		return 1
	}
	if _, ok := goVal.(string); ok {
		goVal = fmt.Sprintf("%s", goVal) // deep copy
	}
	if err = elutils.SetValue(fv, goVal); err != nil {
		C.duk_push_false(ctx)
		return 1
	}
	C.duk_push_true(ctx)
	return 1
}

func go_struct_has(ctx *C.duk_context, vv reflect.Value) C.duk_ret_t {
	// 'this' binding: handler
	// [0]: target
	// [1]: key
	if C.duk_is_string(ctx, 1) == 0 {
		C.duk_push_false(ctx)
		return 1
	}
	key := C.GoString(C.duk_get_string(ctx, 1))

	var structE reflect.Value
	switch vv.Kind() {
	case reflect.Struct:
		structE = vv
	case reflect.Ptr:
		if vv.Elem().Kind() != reflect.Struct {
			C.duk_push_false(ctx)
			return 1
		}
		structE = vv.Elem()
	default:
		C.duk_push_false(ctx)
		return 1
	}
	name := upperFirst(key)
	fv := structE.FieldByName(name)
	if !fv.IsValid() {
		C.duk_push_false(ctx)
		return 1
	}
	C.duk_push_true(ctx)
	return 1
}

//export go_obj_get
func go_obj_get(ctx *C.duk_context) C.duk_ret_t {
	/* 'this' binding: handler
	 * [0]: target
	 * [1]: key
	 * [2]: receiver (proxy)
	 */
	v, ok := getTargetValue(ctx)
	if !ok {
		C.duk_push_undefined(ctx)
		return 1
	}
	if v == nil {
		C.duk_push_undefined(ctx)
		return 1
	}
	switch vv := reflect.ValueOf(v); vv.Kind() {
	case reflect.Slice, reflect.Array:
		return go_arr_get(ctx, vv)
	case reflect.Map:
		return go_map_get(ctx, vv)
	case reflect.Struct, reflect.Ptr:
		return go_struct_get(ctx, vv)
	default:
		C.duk_push_undefined(ctx)
		return 1
	}
}

//export go_obj_set
func go_obj_set(ctx *C.duk_context) C.duk_ret_t {
	/* 'this' binding: handler
	 * [0]: target
	 * [1]: key
	 * [2]: val
	 * [3]: receiver (proxy)
	 */
	v, ok := getTargetValue(ctx)
	if !ok {
		C.duk_push_false(ctx)
		return 1
	}
	if v == nil {
		C.duk_push_false(ctx)
		return 1
	}
	switch vv := reflect.ValueOf(v); vv.Kind() {
	case reflect.Slice, reflect.Array:
		return go_arr_set(ctx, vv)
	case reflect.Map:
		return go_map_set(ctx, vv)
	case reflect.Struct, reflect.Ptr:
		return go_struct_set(ctx, vv)
	default:
		C.duk_push_false(ctx)
		return 1
	}
}

//export go_obj_has
func go_obj_has(ctx *C.duk_context) C.duk_ret_t {
	// 'this' binding: handler
	// [0]: target
	// [1]: key
	v, ok := getTargetValue(ctx)
	if !ok {
		C.duk_push_false(ctx)
		return 1
	}
	if v == nil {
		C.duk_push_false(ctx)
		return 1
	}
	switch vv := reflect.ValueOf(v); vv.Kind() {
	case reflect.Slice, reflect.Array:
		return go_arr_has(ctx, vv)
	case reflect.Map:
		return go_map_has(ctx, vv)
	case reflect.Struct, reflect.Ptr:
		return go_struct_has(ctx, vv)
	default:
		C.duk_push_false(ctx)
		return 1
	}
}

func bindProxyTarget(ctx *C.duk_context) {
	var name *C.char

	// [ target handler ]
	C.duk_dup(ctx, -2)       // [ target handler copied-target ]
	C.duk_swap(ctx, -2, -1)  // [ target copied-target handler ]
	C.duk_push_proxy(ctx, 0) // [ target proxy(target,handler) ]

	C.duk_swap(ctx, -2, -1)  // [ proxy handler ]
	getStrPtr(&target, &name)
	C.duk_put_prop_string(ctx, -2, name) // [ proxy ] with proxy["target"] = target
}

func getBoundProxyTarget(ctx *C.duk_context) (targetV interface{}, isProxy bool, err error) {
	var name *C.char

	// [ proxy ]
	getStrPtr(&target, &name)
	isProxy = C.duk_get_prop_string(ctx, -1, name) != 0 // [ proxy target/undefined ]
	defer C.duk_pop(ctx) // [ proxy ] target/undefned poped

	if isProxy {
		v, ok := getTargetValue(ctx, -1)
		if !ok {
			err = fmt.Errorf("no target found")
			return
		}
		targetV = v
	}
	return
}

//export freeTarket
func freeTarket(ctx *C.duk_context) C.duk_ret_t {
	// Object being finalized is at stack index 0
	// fmt.Printf("--- freeTarket is called\n")
	idx := getTargetIdx(ctx)
	ptr := getPtrStore(uintptr(unsafe.Pointer(ctx)))
	ptr.remove(idx)
	return 0
}

func makeProxyObject(ctx *C.duk_context, v interface{}) {
	var name *C.char

	// [ target ]
	ptr := getPtrStore(uintptr(unsafe.Pointer(ctx)))
	idx := ptr.register(&v)
	C.duk_push_int(ctx, C.int(idx)) // [ target idx ]
	getStrPtr(&idxName, &name)
	C.duk_put_prop_string(ctx, -2, name)  // [ target ] with taget[name] = idx

	C.duk_push_c_function(ctx, (*[0]byte)(C.freeTarket), 1); // [ target finalizer ]
	C.duk_set_finalizer(ctx, -2); // [ target ] with finilizer = freeTarket

	getStrPtr(&goObjProxyHandler, &name)
	C.duk_get_global_string(ctx, name) // [ target handler ]

	bindProxyTarget(ctx) // [ Proxy(target,handler) ]
}

func registerGoObjProxyHandler(ctx *C.duk_context) {
	var name *C.char

	C.duk_push_object(ctx)  // [ handler ]

	C.duk_push_c_function(ctx, (C.duk_c_function)(C.go_obj_get), 3) // [ handler getter ]
	getStrPtr(&get, &name)
	C.duk_put_prop_string(ctx, -2, name)  // [ handler ] with handler[get] = getter

	C.duk_push_c_function(ctx, (C.duk_c_function)(C.go_obj_set), 4) // [ handler setter ]
	getStrPtr(&set, &name)
	C.duk_put_prop_string(ctx, -2, name)  // [ handler ] with handler[set] = setter

	C.duk_push_c_function(ctx, (C.duk_c_function)(C.go_obj_has), 2) // [ handler has-handler ]
	getStrPtr(&has, &name)
	C.duk_put_prop_string(ctx, -2, name) // [ handler ] with handler[has] = has-handler

	getStrPtr(&goObjProxyHandler, &name)
	C.duk_put_global_string(ctx, name) // [ ] with global[goObjProxyHandler] = handler
}

func pushString(ctx *C.duk_context, s string) {
	var cstr *C.char
	var sLen C.int
	getStrPtrLen(&s, &cstr, &sLen)
	C.duk_push_lstring(ctx, cstr, C.size_t(sLen))
}

/*
func lowerFirst(name string) string {
	return strings.ToLower(name[:1]) + name[1:]
}*/
func upperFirst(name string) string {
	return strings.ToUpper(name[:1]) + name[1:]
}

