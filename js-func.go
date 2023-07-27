package djs

// #include "duktape.h"
// extern duk_ret_t freeJsFunc(duk_context *ctx);
import "C"
import (
	elutils "github.com/rosbit/go-embedding-utils"
	"reflect"
	"time"
)

func bindFunc(ctx *C.duk_context, funcName string, funcVarPtr interface{}) (err error) {
	helper, e := elutils.NewEmbeddingFuncHelper(funcVarPtr)
	if e != nil {
		err = e
		return
	}
	helper.BindEmbeddingFunc(wrapFunc(ctx, funcName, helper))
	return
}

func wrapFunc(ctx *C.duk_context, funcName string, helper *elutils.EmbeddingFuncHelper) elutils.FnGoFunc {
	return func(args []reflect.Value) (results []reflect.Value) {
		// reload the function when calling go-function
		C.duk_push_global_object(ctx) // [ global ]
		getVar(ctx, funcName) // [ global function ]

		return callJsFuncFromGo(ctx, helper, args)
	}
}

// called by wrapFunc() and fromJsFunc::bindGoFunc()
func callJsFuncFromGo(ctx *C.duk_context, helper *elutils.EmbeddingFuncHelper, args []reflect.Value)  (results []reflect.Value) {
	// [ some-obj function ]

	// push js args
	argc := 0
	itArgs := helper.MakeGoFuncArgs(args)
	for arg := range itArgs {
		pushJsProxyValue(ctx, arg)
		argc += 1
	}
	// [ some-obj function arg1 arg2 ... argN ]

	// call JS function
	C.duk_call(ctx, C.int(argc)) // [ some-obj retval ]

	// convert result to golang
	goVal, err := fromJsValue(ctx)
	results = helper.ToGolangResults(goVal, C.duk_is_array(ctx, -1) != 0, err)
	C.duk_pop_n(ctx, 2) // [ ]
	return
}

func callFunc(ctx *C.duk_context, args ...interface{}) {
	// [ obj function ]
	n := len(args)
	for _, arg := range args {
		pushJsProxyValue(ctx, arg)
	}
	// [ obj function arg1 arg2 ... argN ]

	C.duk_call(ctx, C.int(n)) // [ obj retval ]
}

//export freeJsFunc
func freeJsFunc(ctx *C.duk_context) C.duk_ret_t {
	// [0] function
	// [1] ...
	idx := getTargetIdx(ctx)
	C.duk_push_global_stash(ctx)            // [ ... stash ]
	C.duk_del_prop_index(ctx, -1, C.duk_uarridx_t(idx)) // [ ... stash ]
	C.duk_pop(ctx)                          // [ ... ]
	return 0
}

// called by value.go::fromJsValue
func fromJsFunc(ctx *C.duk_context) (bindGoFunc elutils.FnBindGoFunc) {
	// [ function ]
	var name *C.char
	var idx uint32

	getStrPtr(&idxName, &name)
	if C.duk_get_prop_string(ctx, -1, name) != 0 {
		// [ function idx ]
		idx = uint32(C.duk_get_uint(ctx, -1))
		C.duk_pop(ctx) // [ function ]
	} else {
		// [ funciton undefined ]
		C.duk_pop(ctx) // [ function ]

		idx = uint32(time.Now().UnixNano()) // NOTE: make sure different functions with different idx-s. Maybe powerful CPU can product same idx-s.
		C.duk_push_uint(ctx, C.duk_uint_t(idx)) // [ funciton idx ]
		C.duk_put_prop_string(ctx, -2, name) // [ function ] with function[idxName] = idx

		C.duk_push_c_function(ctx, (C.duk_c_function)(C.freeJsFunc), 2) // [ function finalizer ]
		C.duk_set_finalizer(ctx, -2) // [ function ]

		C.duk_push_global_stash(ctx) // [ function stash ]
		C.duk_dup(ctx, -2) // [ function stash function ]
		C.duk_put_prop_index(ctx, -2, C.duk_uarridx_t(idx)) // [ function stash ] with stash[idx] = function
		C.duk_pop(ctx) // [ function ]
	}

	bindGoFunc = func(fnVarPtr interface{}) elutils.FnGoFunc {
		helper, e := elutils.NewEmbeddingFuncHelper(fnVarPtr)
		if e != nil {
			return nil
		}

		return func(args []reflect.Value) (results []reflect.Value) {
			// reload the function when calling go-function
			C.duk_push_global_stash(ctx) // [ stash ]
			C.duk_get_prop_index(ctx, -1, C.duk_uarridx_t(idx)) // [ stash function ]

			return callJsFuncFromGo(ctx, helper, args)
		}
	}

	return bindGoFunc
}
