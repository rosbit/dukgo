package djs

/*
#include "duktape.h"
extern duk_ret_t goFuncBridge(duk_context *ctx);
extern duk_ret_t freeTarget(duk_context *ctx);
*/
import "C"
import (
	elutils "github.com/rosbit/go-embedding-utils"
	"reflect"
	"unsafe"
)

func pushGoFunc(ctx *C.duk_context, funcVar interface{}) {
	t := reflect.TypeOf(funcVar)
	pushWrappedGoFunc(ctx, funcVar, t)
	return
}

//export goFuncBridge
func goFuncBridge(ctx *C.duk_context) C.duk_ret_t {
	var cNativeFunc *C.char
	getStrPtr(&idxName, &cNativeFunc)

	// get pointer of Golang function attached to goFuncBridge
	// [ arg1 arg2 ... argN ]
	argc := int(C.duk_get_top(ctx))
	C.duk_push_current_function(ctx); // [ args ... goFuncBridge ]
	C.duk_get_prop_string(ctx, -1, cNativeFunc) // [ args ... goFuncBridge idx-of-goFnPtr ]
	idx := uint32(C.duk_get_uint(ctx, -1))
	C.duk_pop_n(ctx, 2) // [ args ... ]

	ptr := getPtrStore(uintptr(unsafe.Pointer(ctx)))
	fnPtr, ok := ptr.lookup(idx)
	if !ok {
		return C.DUK_RET_ERROR
	}
	fnVarPtr, ok := fnPtr.(*interface{})
	if !ok {
		return C.DUK_RET_ERROR
	}
	fn := *fnVarPtr
	fnVal := reflect.ValueOf(fn)
	if fnVal.Kind() != reflect.Func {
		return C.DUK_RET_ERROR
	}
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
	pushJsProxyValue(ctx, v) // [ args ... v ]
	return 1
}

func pushWrappedGoFunc(ctx *C.duk_context, fnVar interface{}, fnType reflect.Type) {
	// args count
	argc := fnType.NumIn()
	nargs := C.int(C.DUK_VARARGS)
	if !fnType.IsVariadic() {
		nargs = C.int(argc)
	}

	var cNativeFunc *C.char
	getStrPtr(&idxName, &cNativeFunc)

	ptr := getPtrStore(uintptr(unsafe.Pointer(ctx)))
	idx := ptr.register(&fnVar)

	// [ ... ]
	C.duk_push_c_function(ctx, (C.duk_c_function)(C.goFuncBridge), nargs) // [ ... goFuncBridge ]
	C.duk_push_uint(ctx, C.duk_uint_t(idx)) // [ ... goFuncBridge idx-of-fnVarPtr ]
	C.duk_put_prop_string(ctx, -2, cNativeFunc) // [ ... goFuncBridge ] with goFuncBridge[idxName] = idx

	C.duk_push_c_function(ctx, (*[0]byte)(C.freeTarget), 1); // [ target finalizer ]
	C.duk_set_finalizer(ctx, -2); // [ target ] with finilizer = freeTarget
}

