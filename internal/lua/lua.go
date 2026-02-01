package lua

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"sync"

	lua "github.com/yuin/gopher-lua"
)

/*
Package lua provides a Transformer that runs user-provided Lua scripts to
transform incoming messages. In addition to the Lua environment, this package
exposes several Go-backed helper functions to Lua scripts for convenience.

Available Go-backed functions (registered in the global Lua state):

- rot13(str) -> string
    Applies ROT13 to ASCII alphabetic characters and returns the transformed string.

- base64_encode(str) -> string
    Encodes the input string using base64 (StdEncoding) and returns the encoded string.

- base64_decode(str) -> (string | nil, error | nil)
    Decodes a base64-encoded string. On success returns (decoded_string, nil).
    On failure returns (nil, error_message).

- hex_encode(str) -> string
    Encodes the input string into hexadecimal representation.

- hex_decode(str) -> (string | nil, error | nil)
    Decodes a hexadecimal string. On success returns (decoded_string, nil).
    On failure returns (nil, error_message).

- hmac_sha256(key, message) -> string
    Computes HMAC-SHA256 of message with the provided key and returns the
    result as a lowercase hex string.

- json_encode(value) -> (string | nil, error | nil)
    Serializes the provided Lua value (table/primitive) to a JSON string.
    Returns (json_string, nil) on success, or (nil, error_message) on error.

- json_decode(json_string) -> (value | nil, error | nil)
    Parses a JSON string and returns the corresponding Lua value (table/primitive)
    on success, or (nil, error_message) on failure.

Notes:
- json_encode/json_decode convert between Lua tables and Go maps/arrays using
  reasonable heuristics: numeric sequential keys -> arrays, string keys -> maps.
- All encode/decode helpers operate on UTF-8 strings. Binary data returned by
  decode functions are returned as Lua strings (may contain arbitrary bytes).
*/

// Transformer handles Lua script execution for message transformation
type Transformer struct {
	scriptPath string
	mu         sync.Mutex // Protects against concurrent access to Lua state
	state      *lua.LState
}

// New creates a new Lua transformer
func New(scriptPath string) (*Transformer, error) {
	L := lua.NewState()

	// Register Go-backed functions for Lua scripts
	registerFunctions(L)

	// Load the Lua script
	if err := L.DoFile(scriptPath); err != nil {
		L.Close()
		return nil, fmt.Errorf("failed to load Lua script: %w", err)
	}

	return &Transformer{
		scriptPath: scriptPath,
		state:      L,
	}, nil
}

// registerFunctions registers Go functions into the given Lua state
func registerFunctions(L *lua.LState) {
	// rot13(s)
	L.SetGlobal("rot13", L.NewFunction(func(L *lua.LState) int {
		s := L.CheckString(1)
		L.Push(lua.LString(rot13String(s)))
		return 1
	}))

	// base64_encode(s)
	L.SetGlobal("base64_encode", L.NewFunction(func(L *lua.LState) int {
		s := L.CheckString(1)
		enc := base64.StdEncoding.EncodeToString([]byte(s))
		L.Push(lua.LString(enc))
		return 1
	}))

	// base64_decode(s) -> (decoded, err)
	L.SetGlobal("base64_decode", L.NewFunction(func(L *lua.LState) int {
		s := L.CheckString(1)
		data, err := base64.StdEncoding.DecodeString(s)
		if err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		L.Push(lua.LString(string(data)))
		L.Push(lua.LNil)
		return 2
	}))

	// hex_encode(s)
	L.SetGlobal("hex_encode", L.NewFunction(func(L *lua.LState) int {
		s := L.CheckString(1)
		enc := hex.EncodeToString([]byte(s))
		L.Push(lua.LString(enc))
		return 1
	}))

	// hex_decode(s) -> (decoded, err)
	L.SetGlobal("hex_decode", L.NewFunction(func(L *lua.LState) int {
		s := L.CheckString(1)
		data, err := hex.DecodeString(s)
		if err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		L.Push(lua.LString(string(data)))
		L.Push(lua.LNil)
		return 2
	}))

	// hmac_sha256(key, message)
	L.SetGlobal("hmac_sha256", L.NewFunction(func(L *lua.LState) int {
		key := L.CheckString(1)
		msg := L.CheckString(2)
		h := hmac.New(sha256.New, []byte(key))
		h.Write([]byte(msg))
		sum := h.Sum(nil)
		L.Push(lua.LString(hex.EncodeToString(sum)))
		return 1
	}))

	// json_encode(value) -> (json_string, err)
	L.SetGlobal("json_encode", L.NewFunction(func(L *lua.LState) int {
		val := luaValueToGo(L.CheckAny(1))
		b, err := json.Marshal(val)
		if err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		L.Push(lua.LString(string(b)))
		L.Push(lua.LNil)
		return 2
	}))

	// json_decode(json_string) -> (value, err)
	L.SetGlobal("json_decode", L.NewFunction(func(L *lua.LState) int {
		js := L.CheckString(1)
		var v interface{}
		if err := json.Unmarshal([]byte(js), &v); err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		L.Push(goValueToLua(L, v))
		L.Push(lua.LNil)
		return 2
	}))
}

// luaValueToGo converts a lua.LValue into Go native types (recursively)
func luaValueToGo(lv lua.LValue) interface{} {
	switch v := lv.(type) {
	case lua.LString:
		return string(v)
	case lua.LNumber:
		return float64(v)
	case lua.LBool:
		return bool(v)
	case *lua.LTable:
		// Decide between array and map
		maxN := v.MaxN()
		if maxN > 0 {
			arr := make([]interface{}, 0, maxN)
			for i := 1; i <= maxN; i++ {
				arr = append(arr, luaValueToGo(v.RawGetInt(i)))
			}
			return arr
		}
		m := make(map[string]interface{})
		v.ForEach(func(key, value lua.LValue) {
			if ks, ok := key.(lua.LString); ok {
				m[string(ks)] = luaValueToGo(value)
			}
		})
		return m
	default:
		if lv == lua.LNil {
			return nil
		}
		return lv.String()
	}
}

// goValueToLua converts Go native types (from json.Unmarshal) into lua.LValue
func goValueToLua(L *lua.LState, v interface{}) lua.LValue {
	switch x := v.(type) {
	case string:
		return lua.LString(x)
	case float64:
		return lua.LNumber(x)
	case bool:
		return lua.LBool(x)
	case nil:
		return lua.LNil
	case []interface{}:
		tbl := L.NewTable()
		for i, el := range x {
			tbl.RawSetInt(i+1, goValueToLua(L, el))
		}
		return tbl
	case map[string]interface{}:
		tbl := L.NewTable()
		for k, el := range x {
			tbl.RawSetString(k, goValueToLua(L, el))
		}
		return tbl
	default:
		// Fallback to string representation
		return lua.LString(fmt.Sprintf("%v", x))
	}
}

// rot13String applies ROT13 cipher to alphabetic characters in the string.
func rot13String(s string) string {
	b := []rune(s)
	for i, r := range b {
		if r >= 'a' && r <= 'z' {
			b[i] = 'a' + (r-'a'+13)%26
		} else if r >= 'A' && r <= 'Z' {
			b[i] = 'A' + (r-'A'+13)%26
		}
	}
	return string(b)
}

// Transform executes the Lua transformation on the input data
func (t *Transformer) Transform(data map[string]interface{}) (map[string]interface{}, error) {
	// Lock to ensure thread-safe access to Lua state
	t.mu.Lock()
	defer t.mu.Unlock()

	// Get the transform function from Lua
	fn := t.state.GetGlobal("transform")
	if fn.Type() != lua.LTFunction {
		return nil, fmt.Errorf("transform function not found in Lua script")
	}

	// Convert Go map to Lua table
	table := t.mapToTable(data)

	// Call the transform function
	if err := t.state.CallByParam(lua.P{
		Fn:      fn,
		NRet:    1,
		Protect: true,
	}, table); err != nil {
		return nil, fmt.Errorf("failed to execute Lua transform: %w", err)
	}

	// Get the result
	result := t.state.Get(-1)
	t.state.Pop(1)

	if result.Type() != lua.LTTable {
		return nil, fmt.Errorf("transform function must return a table")
	}

	// Convert Lua table back to Go map
	return t.tableToMap(result.(*lua.LTable)), nil
}

// Close closes the Lua state
func (t *Transformer) Close() {
	if t.state != nil {
		t.state.Close()
	}
}

// mapToTable converts a Go map to a Lua table
func (t *Transformer) mapToTable(m map[string]interface{}) *lua.LTable {
	table := t.state.NewTable()
	for k, v := range m {
		table.RawSetString(k, t.interfaceToLValue(v))
	}
	return table
}

// interfaceToLValue converts a Go interface{} to a Lua value
func (t *Transformer) interfaceToLValue(i interface{}) lua.LValue {
	switch v := i.(type) {
	case string:
		return lua.LString(v)
	case float64:
		return lua.LNumber(v)
	case int:
		return lua.LNumber(v)
	case bool:
		return lua.LBool(v)
	case map[string]interface{}:
		return t.mapToTable(v)
	case []interface{}:
		arr := t.state.NewTable()
		for i, val := range v {
			arr.RawSetInt(i+1, t.interfaceToLValue(val))
		}
		return arr
	case nil:
		return lua.LNil
	default:
		// Try to marshal to JSON and back for complex types
		data, err := json.Marshal(v)
		if err != nil {
			log.Printf("Warning: failed to marshal value to JSON: %v", err)
			return lua.LString(fmt.Sprintf("%v", v))
		}
		return lua.LString(data)
	}
}

// tableToMap converts a Lua table to a Go map
func (t *Transformer) tableToMap(tbl *lua.LTable) map[string]interface{} {
	result := make(map[string]interface{})
	tbl.ForEach(func(key, value lua.LValue) {
		if keyStr, ok := key.(lua.LString); ok {
			result[string(keyStr)] = t.lvalueToInterface(value)
		}
	})
	return result
}

// lvalueToInterface converts a Lua value to a Go interface{}
func (t *Transformer) lvalueToInterface(lv lua.LValue) interface{} {
	switch v := lv.(type) {
	case lua.LString:
		return string(v)
	case lua.LNumber:
		return float64(v)
	case lua.LBool:
		return bool(v)
	case *lua.LTable:
		// Check if it's an array or a map
		maxN := v.MaxN()
		if maxN > 0 {
			// It's an array
			arr := make([]interface{}, 0, maxN)
			for i := 1; i <= maxN; i++ {
				arr = append(arr, t.lvalueToInterface(v.RawGetInt(i)))
			}
			return arr
		}
		// It's a map
		return t.tableToMap(v)
	default:
		if lv == lua.LNil {
			return nil
		}
		return lv.String()
	}
}
