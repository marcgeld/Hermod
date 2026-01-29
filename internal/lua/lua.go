package lua

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"

	lua "github.com/yuin/gopher-lua"
)

// Transformer handles Lua script execution for message transformation
type Transformer struct {
	scriptPath string
	mu         sync.Mutex // Protects against concurrent access to Lua state
	state      *lua.LState
}

// New creates a new Lua transformer
func New(scriptPath string) (*Transformer, error) {
	L := lua.NewState()

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
		return lua.LString(string(data))
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
