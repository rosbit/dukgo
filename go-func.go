package djs

/*
#include "duktape.h"
static const char *getCString(duk_context *ctx, duk_idx_t idx);
extern duk_ret_t goFuncBridge(duk_context *ctx);
*/
import "C"
import (
	elutils "github.com/rosbit/go-embedding-utils"
	"reflect"
	"fmt"
)

var (
	FUNC_NAME string = "\xFF_fn_"
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

func getGoFuncValue(ctx *C.duk_context, funcName string) (funcPtr interface{}, err error) {
	jsCtx, e := getContext(ctx)
	if e != nil {
		err = e
		return
	}
	if len(jsCtx.env) == 0 {
		err = fmt.Errorf("no env found")
		return
	}
	fn, ok := jsCtx.env[funcName]
	if !ok {
		err = fmt.Errorf("func name %s not found", funcName)
		return
	}
	funcPtr = fn
	return
}

//export goFuncBridge
func goFuncBridge(ctx *C.duk_context) C.duk_ret_t {
	var cNativeFunc *C.char
	var cNativeFuncLen C.int
	getStrPtrLen(&FUNC_NAME, &cNativeFunc, &cNativeFuncLen)

	// get pointer of Golang function attached to goFuncBridge
	// [ arg1 arg2 ... argN ]
	argc := int(C.duk_get_top(ctx))
	C.duk_push_current_function(ctx); // [ args ... goFuncBridge ]
	C.duk_get_prop_lstring(ctx, -1, cNativeFunc, C.size_t(cNativeFuncLen)) // [ args ... goFuncBridge goFnPtr ]
	name := C.GoString(C.getCString(ctx, -1))
	C.duk_pop_n(ctx, 2) // [ args ... ]

	fn, err := getGoFuncValue(ctx, name)
	if err != nil {
		return C.DUK_RET_ERROR
	}
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
	pushJsValue(ctx, v) // [ args ... v ]
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
	var cNativeFuncLen C.int
	getStrPtrLen(&FUNC_NAME, &cNativeFunc, &cNativeFuncLen)

	// [ ... funcName ]
	C.duk_push_c_function(ctx, (C.duk_c_function)(C.goFuncBridge), nargs) // [ ... funcName goFuncBridge ]
	C.duk_push_null(ctx) // [ ... funcName goFuncBridge null ]
	C.duk_copy(ctx, -3, -1) // [ ... funcName goFuncBridge funcName ]
	C.duk_put_prop_lstring(ctx, -2, cNativeFunc, C.size_t(cNativeFuncLen)) // [ ... funcName goFuncBridge ] with goFuncBridge[_fn_] = funcName
}

