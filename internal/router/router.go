package router

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/marcgeld/Hermod/internal/logger"
	lua "github.com/yuin/gopher-lua"
)

// Record represents a database record to be inserted
type Record struct {
	Table   string                 // Target table name
	Columns map[string]interface{} // Column name -> value
}

// Message represents an incoming MQTT message
type Message struct {
	Topic   string
	Payload []byte
	QoS     byte
	Retain  bool
	Time    time.Time
}

// Route configuration for MQTT message routing
type Route struct {
	Filter    string // MQTT topic filter (e.g., "ruuvi/+", "p1ib/#")
	Script    string // Path to Lua script (empty = passthrough)
	Workers   int    // Number of worker goroutines
	QueueSize int    // Buffered channel size
	Table     string // Default table name
}

// Router handles message routing and processing
type Router struct {
	routes      []*routeHandler
	passthrough *passthroughHandler
	logger      *logger.Logger
	wg          sync.WaitGroup
	ctx         context.Context
	cancel      context.CancelFunc
}

// routeHandler manages workers for a single route
type routeHandler struct {
	route   Route
	msgChan chan Message
	workers []*worker
	logger  *logger.Logger
}

// worker processes messages for a route
type worker struct {
	id      int
	state   *lua.LState
	msgChan chan Message
	storage Storage
	logger  *logger.Logger
	ctx     context.Context
	table   string // Default table from route config
}

// Storage interface for database operations
type Storage interface {
	InsertIntoTable(ctx context.Context, table string, data map[string]interface{}) error
}

// validIdentifier ensures table/column names are safe for SQL
var validIdentifier = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

// New creates a new router with the given routes
func New(ctx context.Context, routes []Route, storage Storage, log *logger.Logger) (*Router, error) {
	if log == nil {
		log = logger.New(logger.INFO)
	}

	routeCtx, cancel := context.WithCancel(ctx)

	r := &Router{
		routes:      make([]*routeHandler, 0, len(routes)),
		passthrough: newPassthroughHandler(storage, log),
		logger:      log,
		ctx:         routeCtx,
		cancel:      cancel,
	}

	// Initialize route handlers
	for _, route := range routes {
		handler, err := r.newRouteHandler(route, storage)
		if err != nil {
			cancel()
			return nil, fmt.Errorf("failed to initialize route %s: %w", route.Filter, err)
		}
		r.routes = append(r.routes, handler)
	}

	return r, nil
}

// newRouteHandler creates a handler for a single route
func (r *Router) newRouteHandler(route Route, storage Storage) (*routeHandler, error) {
	// Set defaults
	if route.Workers <= 0 {
		route.Workers = 1
	}
	if route.QueueSize <= 0 {
		route.QueueSize = 100
	}
	if route.Table == "" {
		route.Table = "iot_data"
	}

	// Validate table name
	if !validIdentifier.MatchString(route.Table) {
		return nil, fmt.Errorf("invalid table name: %s", route.Table)
	}

	handler := &routeHandler{
		route:   route,
		msgChan: make(chan Message, route.QueueSize),
		workers: make([]*worker, route.Workers),
		logger:  r.logger,
	}

	// Start workers
	for i := 0; i < route.Workers; i++ {
		w, err := newWorker(i, route.Script, route.Table, handler.msgChan, storage, r.ctx, r.logger)
		if err != nil {
			return nil, fmt.Errorf("failed to create worker %d: %w", i, err)
		}
		handler.workers[i] = w
		r.wg.Add(1)
		go w.run(&r.wg)
	}

	r.logger.Infof("Route initialized: filter=%s, script=%s, workers=%d, queue=%d, table=%s",
		route.Filter, route.Script, route.Workers, route.QueueSize, route.Table)

	return handler, nil
}

// newWorker creates a new worker with its own Lua state
func newWorker(id int, scriptPath string, defaultTable string, msgChan chan Message, storage Storage, ctx context.Context, log *logger.Logger) (*worker, error) {
	w := &worker{
		id:      id,
		msgChan: msgChan,
		storage: storage,
		logger:  log,
		ctx:     ctx,
		table:   defaultTable,
	}

	// Only create Lua state if script is provided
	if scriptPath != "" {
		L := lua.NewState()
		if err := L.DoFile(scriptPath); err != nil {
			L.Close()
			return nil, fmt.Errorf("failed to load Lua script: %w", err)
		}
		w.state = L
	}

	return w, nil
}

// run is the worker main loop
func (w *worker) run(wg *sync.WaitGroup) {
	defer wg.Done()
	if w.state != nil {
		defer w.state.Close()
	}

	for {
		select {
		case <-w.ctx.Done():
			return
		case msg, ok := <-w.msgChan:
			if !ok {
				return
			}
			if err := w.process(msg); err != nil {
				w.logger.Errorf("Worker %d failed to process message from %s: %v", w.id, msg.Topic, err)
			}
		}
	}
}

// process handles a single message
func (w *worker) process(msg Message) error {
	// If no Lua script, passthrough
	if w.state == nil {
		record := buildPassthroughRecord(msg)
		table := w.table
		if table == "" || table == "iot_data" {
			table = "iot_raw"
		}
		return w.storage.InsertIntoTable(w.ctx, table, record)
	}

	// Execute Lua transform
	records, err := w.executeTransform(msg)
	if err != nil {
		return fmt.Errorf("transform failed: %w", err)
	}

	// Insert records into database
	for _, rec := range records {
		// Use default table if not specified
		table := rec.Table
		if table == "" {
			table = w.table
		}
		if err := w.storage.InsertIntoTable(w.ctx, table, rec.Columns); err != nil {
			return fmt.Errorf("failed to insert into %s: %w", table, err)
		}
	}

	return nil
}

// executeTransform runs the Lua transform function
func (w *worker) executeTransform(msg Message) ([]Record, error) {
	// Get transform function
	fn := w.state.GetGlobal("transform")
	if fn.Type() != lua.LTFunction {
		return nil, fmt.Errorf("transform function not found in Lua script")
	}

	// Build input message table
	msgTable := w.state.NewTable()
	msgTable.RawSetString("topic", lua.LString(msg.Topic))
	msgTable.RawSetString("payload", lua.LString(string(msg.Payload)))
	msgTable.RawSetString("ts", lua.LString(msg.Time.Format(time.RFC3339Nano)))

	// Try to parse payload as JSON
	var jsonData interface{}
	if err := json.Unmarshal(msg.Payload, &jsonData); err == nil {
		msgTable.RawSetString("json", jsonToLTable(w.state, jsonData))
	} else {
		msgTable.RawSetString("json", lua.LNil)
	}

	// Call transform function
	if err := w.state.CallByParam(lua.P{
		Fn:      fn,
		NRet:    1,
		Protect: true,
	}, msgTable); err != nil {
		return nil, fmt.Errorf("Lua transform error: %w", err)
	}

	// Get result
	result := w.state.Get(-1)
	w.state.Pop(1)

	if result.Type() != lua.LTTable {
		return nil, fmt.Errorf("transform must return a table (array of records)")
	}

	// Parse result as array of records
	return w.parseRecords(result.(*lua.LTable))
}

// parseRecords converts Lua table array to []Record
func (w *worker) parseRecords(tbl *lua.LTable) ([]Record, error) {
	var records []Record

	// Check if it's an array
	maxN := tbl.MaxN()
	if maxN == 0 {
		return nil, fmt.Errorf("transform must return an array of records")
	}

	for i := 1; i <= maxN; i++ {
		recLV := tbl.RawGetInt(i)
		if recLV.Type() != lua.LTTable {
			return nil, fmt.Errorf("record %d is not a table", i)
		}

		recTable := recLV.(*lua.LTable)
		rec := Record{
			Columns: make(map[string]interface{}),
		}

		// Extract table name
		if tableLV := recTable.RawGetString("table"); tableLV != lua.LNil {
			if tableStr, ok := tableLV.(lua.LString); ok {
				rec.Table = string(tableStr)
			}
		}

		// Extract columns
		columnsLV := recTable.RawGetString("columns")
		if columnsLV.Type() != lua.LTTable {
			return nil, fmt.Errorf("record %d missing 'columns' table", i)
		}

		columnsTable := columnsLV.(*lua.LTable)
		columnsTable.ForEach(func(key, value lua.LValue) {
			if keyStr, ok := key.(lua.LString); ok {
				colName := string(keyStr)
				// Validate column name
				if !validIdentifier.MatchString(colName) {
					return // Skip invalid columns
				}
				rec.Columns[colName] = lvalueToInterface(value)
			}
		})

		records = append(records, rec)
	}

	return records, nil
}

// Dispatch routes an incoming message to the appropriate handler
func (r *Router) Dispatch(msg Message) error {
	// Find first matching route
	for _, handler := range r.routes {
		if topicMatches(handler.route.Filter, msg.Topic) {
			select {
			case handler.msgChan <- msg:
				r.logger.Debugf("Message from %s dispatched to route %s", msg.Topic, handler.route.Filter)
				return nil
			case <-r.ctx.Done():
				return fmt.Errorf("router context cancelled")
			default:
				return fmt.Errorf("route %s queue full", handler.route.Filter)
			}
		}
	}

	// No route matched, use passthrough
	r.logger.Debugf("No route matched for %s, using passthrough", msg.Topic)
	return r.passthrough.handle(msg)
}

// Close shuts down the router and all workers
func (r *Router) Close() {
	r.cancel()
	
	// Close all route channels
	for _, handler := range r.routes {
		close(handler.msgChan)
	}
	
	// Wait for all workers to finish
	r.wg.Wait()
	r.logger.Info("Router closed")
}

// passthroughHandler handles messages that don't match any route
type passthroughHandler struct {
	storage Storage
	logger  *logger.Logger
}

func newPassthroughHandler(storage Storage, log *logger.Logger) *passthroughHandler {
	return &passthroughHandler{
		storage: storage,
		logger:  log,
	}
}

func (h *passthroughHandler) handle(msg Message) error {
	record := buildPassthroughRecord(msg)
	if err := h.storage.InsertIntoTable(context.Background(), "iot_raw", record); err != nil {
		return fmt.Errorf("passthrough insert failed: %w", err)
	}
	h.logger.Debugf("Passthrough: stored message from %s", msg.Topic)
	return nil
}

// buildPassthroughRecord creates the canonical passthrough record format
func buildPassthroughRecord(msg Message) map[string]interface{} {
	record := map[string]interface{}{
		"time":   msg.Time,
		"topic":  msg.Topic,
		"qos":    int(msg.QoS),
		"retain": msg.Retain,
		"raw":    string(msg.Payload),
	}

	// Add json field only if payload is valid JSON
	var jsonData interface{}
	if err := json.Unmarshal(msg.Payload, &jsonData); err == nil {
		record["json"] = jsonData
	}

	return record
}

// topicMatches returns true if a subscription filter matches a concrete topic
// Supports MQTT wildcards: '+' (single level) and '#' (multi level, only last)
func topicMatches(filter, topic string) bool {
	if filter == topic || filter == "#" {
		return true
	}

	fs := strings.Split(filter, "/")
	ts := strings.Split(topic, "/")

	for i := 0; i < len(fs); i++ {
		if i >= len(ts) {
			return fs[i] == "#" && i == len(fs)-1
		}

		switch fs[i] {
		case "#":
			return i == len(fs)-1
		case "+":
			continue
		default:
			if fs[i] != ts[i] {
				return false
			}
		}
	}

	return len(ts) == len(fs)
}

// jsonToLTable converts a Go JSON value to Lua table
func jsonToLTable(L *lua.LState, data interface{}) lua.LValue {
	switch v := data.(type) {
	case map[string]interface{}:
		tbl := L.NewTable()
		for key, val := range v {
			tbl.RawSetString(key, jsonToLTable(L, val))
		}
		return tbl
	case []interface{}:
		tbl := L.NewTable()
		for i, val := range v {
			tbl.RawSetInt(i+1, jsonToLTable(L, val))
		}
		return tbl
	case string:
		return lua.LString(v)
	case float64:
		return lua.LNumber(v)
	case bool:
		return lua.LBool(v)
	case nil:
		return lua.LNil
	default:
		return lua.LString(fmt.Sprintf("%v", v))
	}
}

// lvalueToInterface converts Lua value to Go interface{}
func lvalueToInterface(lv lua.LValue) interface{} {
	switch v := lv.(type) {
	case lua.LString:
		return string(v)
	case lua.LNumber:
		return float64(v)
	case lua.LBool:
		return bool(v)
	case *lua.LTable:
		// Check if array or map
		maxN := v.MaxN()
		if maxN > 0 {
			arr := make([]interface{}, maxN)
			for i := 1; i <= maxN; i++ {
				arr[i-1] = lvalueToInterface(v.RawGetInt(i))
			}
			return arr
		}
		// Map
		m := make(map[string]interface{})
		v.ForEach(func(key, value lua.LValue) {
			if keyStr, ok := key.(lua.LString); ok {
				m[string(keyStr)] = lvalueToInterface(value)
			}
		})
		return m
	default:
		if lv.Type() == lua.LTNil {
			return nil
		}
		return v.String()
	}
}
