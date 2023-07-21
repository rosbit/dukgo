package djs

/*
#include "duktape.h"
#include "duk_config.h"
#include "duk_console.h"
#include "duk_print_alert.h"
#include "duk_module_duktape.h"
static duk_context *createContext() {
	return duk_create_heap_default();
}
static const char *getCString(duk_context *ctx, duk_idx_t idx) {
	return duk_safe_to_string(ctx, idx);
}
static duk_int_t pEval(duk_context *ctx, const char *src, duk_size_t len) {
	return duk_peval_lstring(ctx, src, len);
}
*/
import "C"
import (
	"reflect"
	"unsafe"
	"fmt"
	"os"
	"runtime"
)

type JsContext struct {
	c *C.duk_context
}

func NewContext() (*JsContext, error) {
	ctx := C.createContext()
	if ctx == (*C.duk_context)(unsafe.Pointer(nil)) {
		return nil, fmt.Errorf("failed to create context")
	}
	loadPreludeModules(ctx)
	c := &JsContext {
		c: ctx,
	}
	runtime.SetFinalizer(c, freeJsContext)
	return c, nil
}

func freeJsContext(ctx *JsContext) {
	// fmt.Printf("context freed\n")
	c := ctx.c
	delPtrStore((uintptr(unsafe.Pointer(c))))
	C.duk_destroy_heap(c)
}

func loadPreludeModules(ctx *C.duk_context) {
	C.duk_print_alert_init(ctx, 0)
	C.duk_console_init(ctx, 0)
	C.duk_module_duktape_init(ctx)
	setModSearch(ctx)
	registerGoObjProxyHandler(ctx)
}

func (ctx *JsContext) Eval(script string, env map[string]interface{}) (res interface{}, err error) {
	var cstr *C.char
	var length C.int
	getStrPtrLen(&script, &cstr, &length)
	return ctx.eval(cstr, length, env)
}

func (ctx *JsContext) EvalFile(scriptFile string, env map[string]interface{}) (res interface{}, err error) {
	b, e := os.ReadFile(scriptFile)
	if e != nil {
		err = e
		return
	}
	var cstr *C.char
	var length C.int
	getBytesPtrLen(b, &cstr, &length)

	return ctx.eval(cstr, length, env)
}

func (ctx *JsContext) eval(script *C.char, scriptLen C.int, env map[string]interface{}) (res interface{}, err error) {
	c := ctx.c
	setEnv(c, env)

	if C.pEval(c, script, C.size_t(scriptLen)) != 0 { // [ result ]
		estr := C.getCString(c, -1)
		C.duk_pop(c)
		err = fmt.Errorf("%s", C.GoString(estr))
		return
	}

	defer C.duk_pop(c)
	return fromJsValue(c)
}

/*
func dump(ctx *C.duk_context, prompt string) {
	fmt.Printf("--- %s BEGIN ---\n", prompt)
	C.duk_push_context_dump(ctx)
	fmt.Printf("%s\n", C.GoString(C.getCString(ctx, -1)))
	C.duk_pop(ctx)
	fmt.Printf("--- %s END ---\n", prompt)
}*/

func setEnv(ctx *C.duk_context, env map[string]interface{}) {
	C.duk_push_global_object(ctx) // [ global ]
	defer C.duk_pop(ctx) // [ ]

	for k, _ := range env {
		v := env[k]
		pushString(ctx, k)  // [ global k ]
		pushJsProxyValue(ctx, v)  // [ global k v ]
		C.duk_put_prop(ctx, -3) // [ global ] with global[k] = v
	}
}

func getVar(ctx *C.duk_context, name string) (exsiting bool) {
	// [ obj ]
	var cstr *C.char
	var nameLen C.int
	getStrPtrLen(&name, &cstr, &nameLen)
	return C.duk_get_prop_lstring(ctx, -1, cstr, C.size_t(nameLen)) != 0 // [ obj result ]
}

func (ctx *JsContext) GetGlobal(name string) (res interface{}, err error) {
	c := ctx.c
	C.duk_push_global_object(c) // [ global ]
	defer C.duk_pop_n(c, 2) // [ ]

	if !getVar(c, name) { // [ global result ]
		err = fmt.Errorf("global %s not found", name)
		return
	}
	return fromJsValue(c)
}

func (ctx *JsContext) CallFunc(funcName string, args ...interface{}) (res interface{}, err error) {
	c := ctx.c
	C.duk_push_global_object(c) // [ global ]
	defer C.duk_pop_n(c, 2) // [ ]

	if !getVar(c, funcName) { // [ global funcName-result ]
		err = fmt.Errorf("function %s not found", funcName)
		return
	}

	if C.duk_is_function(c, -1) == 0 {
		err = fmt.Errorf("var %s is not with type function", funcName)
		return
	}

	callFunc(c, args...) // [ global retval ]
	return fromJsValue(c)
}

// bind a var of golang func with a JS function name, so calling JS function
// is just calling the related golang func.
// @param funcVarPtr  in format `var funcVar func(....) ...; funcVarPtr = &funcVar`
func (ctx *JsContext) BindFunc(funcName string, funcVarPtr interface{}) (err error) {
	if funcVarPtr == nil {
		err = fmt.Errorf("funcVarPtr must be a non-nil poiter of func")
		return
	}
	t := reflect.TypeOf(funcVarPtr)
	if t.Kind() != reflect.Ptr || t.Elem().Kind() != reflect.Func {
		err = fmt.Errorf("funcVarPtr expected to be a pointer of func")
		return
	}

	c := ctx.c

	C.duk_push_global_object(c) // [ global ]
	if !getVar(c, funcName) { // [ global funcName-result ]
		err = fmt.Errorf("function %s not found", funcName)
		C.duk_pop_n(c, 2) // [ ]
		return
	}

	if C.duk_is_function(c, -1) == 0 {
		err = fmt.Errorf("var %s is not with type function", funcName)
		C.duk_pop_n(c, 2) // [ ]
		return
	}

	C.duk_pop_n(c, 2) // [ ] function will be restored when calling
	return bindFunc(c, funcName, funcVarPtr)
}

func (ctx *JsContext) BindFuncs(funcName2FuncVarPtr map[string]interface{}) (err error) {
	for funcName, funcVarPtr := range funcName2FuncVarPtr {
		if err = ctx.BindFunc(funcName, funcVarPtr); err != nil {
			return
		}
	}
	return
}
