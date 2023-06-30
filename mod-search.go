package djs

// #include "duktape.h"
// extern duk_ret_t modSearch(duk_context *ctx);
// static const char *getCString(duk_context *ctx, duk_idx_t idx);
import "C"
import (
	"strings"
	"fmt"
	"os"
	"path"
)

//export modSearch
func modSearch(ctx *C.duk_context) C.duk_ret_t {
	/* Nargs was given as 4 and we get the following stack arguments:
	 *   index 0: id
	 *   index 1: require
	 *   index 2: exports
	 *   index 3: module
	 */
	modPath := C.GoString(C.getCString(ctx, 0))
	if !strings.HasSuffix(modPath, ".js") {
		modPath = fmt.Sprintf("%s.js", modPath)
	}
	C.duk_pop_n(ctx, 4)

	absModPath := toAbsPath(exePath, modPath)
	b, err := os.ReadFile(absModPath)
	if err != nil {
		C.duk_push_undefined(ctx);
		return 1;
	}

	var src *C.char;
	var size C.int;
	getBytesPtrLen(b, &src, &size)

	C.duk_push_lstring(ctx, src, C.size_t(size))
	return 1;
}

func setObjFunction(ctx *C.duk_context, funcName string, fn C.duk_c_function, nargs int) {
	var cFuncName *C.char
	var funcNameLen C.int
	getStrPtrLen(&funcName, &cFuncName, &funcNameLen)

	// [ obj ]
	C.duk_push_lstring(ctx, cFuncName, C.size_t(funcNameLen))  // [ obj funcName ]
	C.duk_push_c_function(ctx, fn, C.duk_idx_t(nargs)) // [ obj funcName fn ]
	C.duk_put_prop(ctx, -3)  // [ obj ] with obj[funcName]=fn
}

func setModSearch(ctx *C.duk_context) {
	duktape := "Duktape"
	var cDuktape *C.char
	var length C.int
	getStrPtrLen(&duktape, &cDuktape, &length)

	C.duk_get_global_lstring(ctx, cDuktape, C.size_t(length))
	setObjFunction(ctx, "modSearch", (C.duk_c_function)(C.modSearch), 4)
	C.duk_pop(ctx)
}

func getExecWD() (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", err
	}
	return path.Dir(exePath), nil
}

func toAbsPath(absRoot, filePath string) string {
	if path.IsAbs(filePath) {
		return filePath
	}
	return path.Join(absRoot, filePath)
}

var exePath string
func init() {
	p, e := getExecWD()
	if e != nil {
		panic(e)
	}
	exePath = p
}
