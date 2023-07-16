package djs

// #include "duktape.h"
import "C"
import (
	elutils "github.com/rosbit/go-embedding-utils"
	"reflect"
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

		// push js args
		argc := 0
		itArgs := helper.MakeGoFuncArgs(args)
		for arg := range itArgs {
			pushJsProxyValue(ctx, arg)
			argc += 1
		}
		// [ global function arg1 arg2 ... argN ]

		// call JS function
		C.duk_call(ctx, C.int(argc)) // [ global retval ]

		// convert result to golang
		goVal, err := fromJsValue(ctx)
		results = helper.ToGolangResults(goVal, C.duk_is_array(ctx, -1) != 0, err)
		C.duk_pop_n(ctx, 2) // [ ]
		return
	}
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
