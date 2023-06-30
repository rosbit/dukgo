package djs

// #include "duktape.h"
// extern duk_ret_t readFile(duk_context *ctx);
// static const char *getCString(duk_context *ctx, duk_idx_t idx);
// static duk_int_t pEval(duk_context *ctx, const char *src, duk_size_t len);
import "C"
import (
	"fmt"
	"os"
	"path"
)

var (
	modSearch_impl = `
	Duktape.modSearch = function (id) {
		/* readFile() reads a file from disk, and returns a string or undefined.
		 * 'id' is in resolved canonical form so it only contains terms and
		 * slashes, and no '.' or '..' terms.
		 */
		var name;
		if (id.endsWith('.js')) {
			name = id;
		} else {
			name = id + '.js';
		}

		var res = readFile(name);
		if (typeof res === 'string') {
			return res;
		}

		throw new Error('module not found: ' + id);
	}`
)

//export readFile
func readFile(ctx *C.duk_context) C.duk_ret_t {
	modPath := C.GoString(C.getCString(ctx, 0))

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

func initGlobalFuncs(ctx *C.duk_context) {
	C.duk_push_global_object(ctx)
	setGlobalFunction(ctx, "readFile", (C.duk_c_function)(C.readFile), 1)
	C.duk_pop(ctx)
}

func setGlobalFunction(ctx *C.duk_context, funcName string, fn C.duk_c_function, nargs int) {
	var cFuncName *C.char
	var funcNameLen C.int
	getStrPtrLen(&funcName, &cFuncName, &funcNameLen)

	// [ global ]
	C.duk_push_lstring(ctx, cFuncName, C.size_t(funcNameLen))  // [ global, funcName ]
	C.duk_push_c_function(ctx, fn, C.duk_idx_t(nargs)) // [ global, funcName, fn ]
	C.duk_put_prop(ctx, -3)  // [ global ] with global[funcName]=fn
}

func set_modSearch(ctx *C.duk_context) {
	var impl *C.char
	var length C.int
	getStrPtrLen(&modSearch_impl, &impl, &length)
	if C.pEval(ctx, impl, C.size_t(length)) != 0 {
		estr := C.getCString(ctx, -1)
		fmt.Fprintf(os.Stderr, "failed to set modSearch: %s", C.GoString(estr))
	}
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
