# Hermod

Hermod is a lightweight data ingestion and transformation engine designed for IoT and real-time data processing.

## Features

- **MQTT Integration**: Subscribe to multiple MQTT topics with configurable QoS
- **Topic-Based Routing**: Route messages by MQTT topic filters to different Lua scripts or passthrough handlers
- **Per-Route Worker Pools**: Independent worker pools for each route with configurable concurrency
- **JSON Decoding**: Automatically decode JSON payloads
- **Lua Transformations**: Transform messages using Lua scripts (via gopher-lua)
  - New transform contract with `msg.topic`, `msg.payload`, `msg.ts`, and `msg.json`
  - Multi-table writes from single Lua script
  - Schema declarations in Lua for validation and SQL generation
- **Schema Validation**: Runtime validation of emitted records against declared schema
- **SQL Schema Generation**: Generate SQL DDL from Lua schema declarations with `-sql` flag
- **Passthrough Mode**: Messages without Lua scripts stored in canonical format
- **PostgreSQL/TimescaleDB**: Store data in PostgreSQL or TimescaleDB for time-series analysis
- **TOML Configuration**: Simple configuration management via TOML files
- **Connection Pooling**: Efficient database connection management with pgxpool
- **Backward Compatible**: Legacy configuration still supported

## Architecture

```
MQTT Broker → Hermod Router → Route Workers → PostgreSQL/TimescaleDB
                   ↓
              Lua Transform (optional)
                   ↓
              Schema Validation
                   ↓
              Multi-Table Insert
```

Hermod acts as a bridge between MQTT-based IoT devices and a time-series database:
1. Subscribes to MQTT topics via route filters
2. Routes messages to appropriate handlers based on topic
3. Each route has N workers processing messages concurrently
4. Decodes JSON payloads (or stores raw data)
5. Optionally transforms data using Lua scripts with schema validation
6. Writes records to one or multiple PostgreSQL/TimescaleDB tables

## Installation

### Prerequisites

- Go 1.25.6 or later
- PostgreSQL 12+ or TimescaleDB 2.0+
- MQTT broker (e.g., Mosquitto, EMQX)

### Build from Source

```bash
git clone https://github.com/marcgeld/Hermod.git
cd hermod
go mod download
go build -o hermod cmd/hermod/main.go
```

### Container Image

Hermod is available as a multi-arch container image supporting `linux/amd64` and `linux/arm64`:

```bash
# Pull the latest version
docker pull ghcr.io/marcgeld/hermod:latest

# Pull a specific version
docker pull ghcr.io/marcgeld/hermod:v1.0.0

# Run with a config file
docker run -v $(pwd)/config.toml:/config.toml ghcr.io/marcgeld/hermod:latest -config /config.toml
```

The container image is built on distroless for minimal size and enhanced security, running as a non-root user.

## Configuration

Hermod supports two configuration modes:

### New: Routing Configuration

Use routes for per-topic filtering, independent worker pools, and multi-table writes:

```toml
[mqtt]
broker = "tcp://localhost:1883"
client_id = "hermod-client"
qos = 1

[database]
host = "localhost"
port = 5432
user = "hermod"
password = "your_password"
database = "hermod"
sslmode = "disable"
pool_size = 10

[logging]
level = "INFO"

# Route 1: Ruuvi sensors with transformation
[[routes]]
filter = "ruuvi/+"
script = "scripts/ruuvi.lua"
workers = 2
queue_size = 100
table = "ruuvi_data"

# Route 2: Passthrough for legacy devices
[[routes]]
filter = "legacy/#"
script = ""  # Empty = passthrough
workers = 1
queue_size = 50
table = "legacy_raw"
```

### Legacy: Single Pipeline (Still Supported)

```toml
[mqtt]
broker = "tcp://localhost:1883"
client_id = "hermod-client"
topics = ["sensors/#"]
qos = 1

[database]
host = "localhost"
port = 5432
user = "hermod"
password = "your_password"
database = "hermod"
sslmode = "disable"
pool_size = 10

[pipeline]
lua_script = "examples/transform.lua"  # Optional
table_name = "mqtt_messages"

[logging]
level = "INFO"
```

### Configuration Options

#### MQTT Section
- `broker`: MQTT broker URL (e.g., `tcp://localhost:1883`)
- `client_id`: Unique client identifier
- `username`: MQTT username (optional)
- `password`: MQTT password (optional)
- `topics`: Array of topics to subscribe to (legacy mode, supports wildcards `+` and `#`)
- `qos`: Quality of Service (0, 1, or 2)

#### Database Section
- `host`: PostgreSQL host
- `port`: PostgreSQL port
- `user`: Database user
- `password`: Database password
- `database`: Database name
- `sslmode`: SSL mode (`disable`, `require`, `verify-ca`, `verify-full`)
- `pool_size`: Maximum number of connections in the pool

#### Pipeline Section (Legacy Mode)
- `lua_script`: Path to Lua transformation script (optional, leave empty to skip transformation)
- `table_name`: Name of the database table to insert records into

#### Routes Section (New Routing Mode)
Each `[[routes]]` block defines a route:
- `filter`: MQTT topic filter (e.g., `"sensors/+"`, `"devices/#"`)
- `script`: Path to Lua script (empty string = passthrough mode)
- `workers`: Number of worker goroutines (default: 1)
- `queue_size`: Buffered channel size (default: 100)
- `table`: Default table name for this route (default: `iot_data`)

#### Logging Section
- `level`: Log level - `DEBUG` (verbose, shows message content), `INFO` (general events), or `ERROR` (errors only)

## Lua Transformations

### New Transform Contract

Lua scripts now receive a message structure and return an array of records:

```lua
-- Schema declaration (optional, enables validation and SQL generation)
schema = {
  tables = {
    sensor_data = {
      time = "timestamptz",
      sensor_id = "text",
      temperature = "double precision",
      humidity = "double precision"
    }
  }
}

function transform(msg)
  -- msg.topic:   string (e.g., "sensors/temp1")
  -- msg.payload: string (raw bytes)
  -- msg.ts:      string (RFC3339Nano UTC timestamp)
  -- msg.json:    table or nil (parsed JSON if valid)
  
  local records = {}
  
  if msg.json then
    table.insert(records, {
      table = "sensor_data",  -- Optional: uses route default if omitted
      columns = {
        time = msg.ts,
        sensor_id = msg.topic,
        temperature = msg.json.temperature or 0,
        humidity = msg.json.humidity or 0
      }
    })
  end
  
  return records  -- Can return multiple records for different tables
end
```

### Multi-Table Writes

A single Lua script can write to multiple tables:

```lua
schema = {
  tables = {
    readings = {
      time = "timestamptz",
      value = "double precision"
    },
    events = {
      time = "timestamptz",
      event_type = "text"
    }
  }
}

function transform(msg)
  local records = {}
  
  -- Always write reading
  table.insert(records, {
    table = "readings",
    columns = { time = msg.ts, value = msg.json.value }
  })
  
  -- Conditionally write event
  if msg.json.alert then
    table.insert(records, {
      table = "events",
      columns = { time = msg.ts, event_type = "alert" }
    })
  end
  
  return records
end
```

### Legacy Transform (Still Supported)

The old transform contract still works in legacy mode:

```lua
function transform(data)
    local result = {}
    result.topic = data.topic
    result.value = data.temperature * 2
    return result
end
```

See `examples/routing_transform.lua` and `examples/multi_table.lua` for more examples.

## SQL Schema Generation

Generate database schema from Lua script declarations:

```bash
# Generate SQL to stdout
hermod -config config.toml -sql

# Save to file
hermod -config config.toml -sql > schema.sql

# Apply to database
hermod -config config.toml -sql | psql -U hermod -d iot
```

Output example:
```sql
CREATE TABLE IF NOT EXISTS sensor_data (
  humidity double precision,
  sensor_id text,
  temperature double precision,
  time timestamptz
);
```

## Passthrough Mode

Routes without Lua scripts automatically store messages in a canonical format:

```toml
[[routes]]
filter = "legacy/#"
script = ""       # Empty = passthrough
table = "raw_data"
```

Passthrough record format:
```
time:   timestamptz (message arrival time in UTC)
topic:  text (MQTT topic)
qos:    int (MQTT QoS level)
retain: boolean (MQTT retain flag)
raw:    text (raw payload as string)
json:   jsonb (parsed JSON, NULL if not valid JSON)
```

Messages that don't match any route also use passthrough with table `iot_raw`.

## Database Setup

1. Create a PostgreSQL database:

```sql
CREATE DATABASE hermod;
CREATE USER hermod WITH PASSWORD 'hermod_password';
GRANT ALL PRIVILEGES ON DATABASE hermod TO hermod;
```

1.Generate and apply schema:

```bash
# Generate SQL from Lua scripts
hermod -config config.toml -sql > schema.sql

# Review and apply
psql -U hermod -d hermod -f schema.sql
```

Or run the legacy migration:

```bash
psql -U hermod -d hermod -f migrations/001_initial_schema.sql
```

For TimescaleDB, enable the extension first:

```sql
CREATE EXTENSION IF NOT EXISTS timescaledb;
```

Then uncomment the TimescaleDB-specific sections in the migration file.

## Usage

### Command-Line Flags

```bash
hermod [options]

Options:
  -config string
        Path to configuration file (default "config.toml")
  -sql
        Generate SQL schema from Lua scripts and exit
  -dry-run
        Don't execute SQL statements, just log them
  -log string
        Log level DEBUG, INFO, or ERROR (overrides config file)
  -version
        Print version information
```

### Run Hermod

```bash
# Default configuration
./hermod

# Custom configuration
./hermod -config /path/to/config.toml

# Generate SQL schema
./hermod -config config.toml -sql > schema.sql

# Dry-run mode (test without database)
./hermod -config config.toml -dry-run

# Debug logging
./hermod -config config.toml -log DEBUG
```

### Example Workflow

1. Create Lua scripts with schema declarations
2. Configure routes in `config.toml`
3. Generate and review SQL schema:
   ```bash
   hermod -config config.toml -sql > schema.sql
   ```
4. Apply schema to database:
   ```bash
   psql -U hermod -d iot -f schema.sql
   ```
5. Test configuration with dry-run:
   ```bash
   hermod -config config.toml -dry-run
   ```
6. Run Hermod:
   ```bash
   hermod -config config.toml
   ```

## Development

### Project Structure

```
Hermod/
├── cmd/
│   └── hermod/
│       └── main.go              # Application entry point
├── internal/
│   ├── config/                  # Configuration management
│   ├── mqtt/                    # MQTT client wrapper
│   ├── lua/                     # Lua transformation engine (legacy)
│   ├── pipeline/                # Message processing pipeline (legacy)
│   ├── router/                  # Routing and worker pools
│   ├── schema/                  # Lua schema parsing and SQL generation
│   ├── storage/                 # Database operations
│   └── logger/                  # Logging
├── examples/
│   ├── config.toml              # Legacy configuration example
│   ├── config_routing.toml      # Routing configuration example
│   ├── config_multi_table.toml  # Multi-table example
│   ├── transform.lua            # Legacy transform example
│   ├── routing_transform.lua    # New transform contract example
│   ├── multi_table.lua          # Multi-table transform example
│   └── README_ROUTING.md        # Routing quick start
├── migrations/
│   └── 001_initial_schema.sql   # Database schema (legacy)
├── go.mod
├── go.sum
└── README.md
```
### Building

Build the application:

```bash
go build ./cmd/hermod
```

### Testing

Run all tests:

```bash
go test ./...
```

Run tests with verbose output:

```bash
go test -v ./...
```

Run tests with coverage:

```bash
go test -cover ./...
```

Run tests for a specific package:

```bash
go test -v ./internal/config/
go test -v ./internal/lua/
go test -v ./internal/pipeline/
go test -v ./internal/storage/
go test -v ./internal/mqtt/
```

The test suite covers:
- **Config package**: TOML configuration loading and validation
- **Lua package**: Script loading, data transformation, and concurrent access
- **Pipeline package**: Message processing with JSON/non-JSON payloads and transformation integration
- **Storage package**: SQL injection prevention and data validation
- **MQTT package**: Configuration and message handler functionality

### Dependencies

- **BurntSushi/toml**: TOML configuration parsing
- **eclipse/paho.mqtt.golang**: MQTT client
- **yuin/gopher-lua**: Lua VM for Go
- **jackc/pgx/v5**: PostgreSQL driver and connection pooling

## Examples

### Publishing Test Messages

Using `mosquitto_pub`:

```bash
# Simple sensor data
mosquitto_pub -h localhost -t "sensors/temperature" -m '{"temperature": 22.5, "humidity": 65}'

# Device telemetry
mosquitto_pub -h localhost -t "devices/sensor01/telemetry" -m '{"temperature": 23.1, "pressure": 1013.25}'
```

### Querying Data

```sql
-- Get recent messages
SELECT * FROM mqtt_messages ORDER BY timestamp DESC LIMIT 10;

-- Get average temperature by topic
SELECT topic, AVG(temperature_celsius) as avg_temp
FROM mqtt_messages
WHERE timestamp > NOW() - INTERVAL '1 hour'
GROUP BY topic;

-- Query JSON payload
SELECT topic, payload->>'sensor_id' as sensor_id, timestamp
FROM mqtt_messages
WHERE payload->>'location' = 'warehouse';
```

## License

See LICENSE file for details.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.
