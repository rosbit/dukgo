package djs

/*
#include "duktape.h"
extern duk_ret_t goFuncBridge(duk_context *ctx);
*/
import "C"
import (
	elutils "github.com/rosbit/go-embedding-utils"
	"reflect"
	"unsafe"
	"fmt"
)

var (
	NATIVE_FUNC string = "\xFF_nf_"
)

func pushGoFunc(ctx *C.duk_context, funcVar interface{}) (err error) {
	t := reflect.TypeOf(funcVar)
	if t.Kind() != reflect.Func {
		err = fmt.Errorf("funcVar expected to be a func")
		return
	}

	pushWrappedGoFunc(ctx, funcVar, t)
	return
}

//export goFuncBridge
func goFuncBridge(ctx *C.duk_context) C.duk_ret_t {
	var cNativeFunc *C.char
	var cNativeFuncLen C.int
	getStrPtrLen(&NATIVE_FUNC, &cNativeFunc, &cNativeFuncLen)

	// get pointer of Golang function attached to goFuncBridge
	// [ arg1 arg2 ... argN ]
	argc := int(C.duk_get_top(ctx))
	C.duk_push_current_function(ctx); // [ args ... goFuncBridge ]
	C.duk_get_prop_lstring(ctx, -1, cNativeFunc, C.size_t(cNativeFuncLen)) // [ args ... goFuncBridge goFnPtr ]
	ptr := C.duk_get_pointer(ctx, -1)
	fn := *((*interface{})(ptr))
	C.duk_pop_n(ctx, 2) // [ args ... ]

	fnVal := reflect.ValueOf(fn)
	fnType := fnVal.Type()

	// make args for Golang function
	helper := elutils.NewGolangFuncHelperDiretly(fnVal, fnType)
	getArgs := func(i int) interface{} {
		C.duk_push_null(ctx)  // [ args ... null ] 
		C.duk_copy(ctx, C.duk_idx_t(i - argc - 1), -1) // [ args ... argI ]
		defer C.duk_pop(ctx) // [ args ... ]

		if goVal, err := fromJsValue(ctx); err == nil {
			return goVal
		}
		return nil
	}
	v, e := helper.CallGolangFunc(argc, "djs-func", getArgs) // call Golang function

	// convert result (in var v) of Golang function to that of JS.
	// 1. error
	if e != nil {
		return C.DUK_RET_ERROR
	}

	// 2. no result
	if v == nil {
		return 0 // undefined
	}

	// 3. array or scalar
	if vv, ok := v.([]interface{}); ok {
		C.duk_push_array(ctx) // [ args ... arr ]
		for i, rv := range vv {
			pushJsValue(ctx, rv) // [ args ... arr rv ]
			C.duk_put_prop_index(ctx, -2, C.duk_uarridx_t(i)) // [ args ... arr ] arr with i-th value rv
		}
	} else {
		pushJsValue(ctx, v) // [ args ... v ]
	}
	return 1
}

func pushWrappedGoFunc(ctx *C.duk_context, fnVar interface{}, fnType reflect.Type) {
	fnVarPtr := &fnVar

	// args count
	argc := fnType.NumIn()
	nargs := C.int(C.DUK_VARARGS)
	if !fnType.IsVariadic() {
		nargs = C.int(argc)
	}

	var cNativeFunc *C.char
	var cNativeFuncLen C.int
	getStrPtrLen(&NATIVE_FUNC, &cNativeFunc, &cNativeFuncLen)

	// [ ... ]
	C.duk_push_c_function(ctx, (C.duk_c_function)(C.goFuncBridge), nargs) // [ ... goFuncBridge ]
	C.duk_push_pointer(ctx, unsafe.Pointer(fnVarPtr)) // [ ... goFuncBridge fnVarPtr ]
	C.duk_put_prop_lstring(ctx, -2, cNativeFunc, C.size_t(cNativeFuncLen)) // [ ... goFuncBridge ] with goFuncBridge[_nf_] = fnVarPtr
}

