# Hermod Routing and Schema Guide

See `docs/ROUTING_GUIDE.md` for complete documentation on:
- Routing configuration
- Lua schema declarations
- Transform contract
- Passthrough behavior
- SQL generation
- Configuration examples

## Quick Start

### 1. Define Schema in Lua

```lua
schema = {
  tables = {
    sensor_data = {
      time = "timestamptz",
      sensor_id = "text",
      value = "double precision"
    }
  }
}

function transform(msg)
  return {
    {
      table = "sensor_data",
      columns = {
        time = msg.ts,
        sensor_id = msg.topic,
        value = msg.json.temperature or 0
      }
    }
  }
end
```

### 2. Configure Routes

```toml
[[routes]]
filter = "sensors/+"
script = "scripts/sensor.lua"
workers = 2
queue_size = 100
table = "sensor_data"
```

### 3. Generate SQL Schema

```bash
hermod -config config.toml -sql > schema.sql
psql -U hermod -d iot < schema.sql
```

### 4. Run Hermod

```bash
hermod -config config.toml
```
