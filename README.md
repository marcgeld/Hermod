# Hermod

Hermod is a lightweight data ingestion and transformation engine designed for IoT and real-time data processing.

## Features

- **MQTT Integration**: Subscribe to multiple MQTT topics with configurable QoS
- **JSON Decoding**: Automatically decode JSON payloads
- **Lua Transformations**: Transform messages using Lua scripts (via gopher-lua)
- **PostgreSQL/TimescaleDB**: Store data in PostgreSQL or TimescaleDB for time-series analysis
- **TOML Configuration**: Simple configuration management via TOML files
- **Connection Pooling**: Efficient database connection management with pgxpool

## Architecture

```
MQTT Broker → Hermod (MQTT Client) → Lua Transformer → PostgreSQL/TimescaleDB
```

Hermod acts as a bridge between MQTT-based IoT devices and a time-series database:
1. Subscribes to configured MQTT topics
2. Receives messages from devices/sensors
3. Decodes JSON payloads (or stores raw data)
4. Optionally transforms data using Lua scripts
5. Writes records to PostgreSQL/TimescaleDB

## Installation

### Prerequisites

- Go 1.21 or later
- PostgreSQL 12+ or TimescaleDB 2.0+
- MQTT broker (e.g., Mosquitto, EMQX)

### Build from Source

```bash
git clone https://github.com/marcgeld/Hermod.git
cd Hermod
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

Create a `config.toml` file (see `examples/config.toml` for a complete example):

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
level = "INFO"       # DEBUG, INFO, or ERROR
dry_run = false      # Set to true to log SQL instead of executing
```

### Configuration Options

#### MQTT Section
- `broker`: MQTT broker URL (e.g., `tcp://localhost:1883`)
- `client_id`: Unique client identifier
- `username`: MQTT username (optional)
- `password`: MQTT password (optional)
- `topics`: Array of topics to subscribe to (supports wildcards `+` and `#`)
- `qos`: Quality of Service (0, 1, or 2)

#### Database Section
- `host`: PostgreSQL host
- `port`: PostgreSQL port
- `user`: Database user
- `password`: Database password
- `database`: Database name
- `sslmode`: SSL mode (`disable`, `require`, `verify-ca`, `verify-full`)
- `pool_size`: Maximum number of connections in the pool

#### Pipeline Section
- `lua_script`: Path to Lua transformation script (optional, leave empty to skip transformation)
- `table_name`: Name of the database table to insert records into

#### Logging Section
- `level`: Log level - `DEBUG` (verbose, shows message content), `INFO` (general events), or `ERROR` (errors only)
- `dry_run`: If `true`, logs SQL statements instead of executing them (useful for testing without database connection)

## Database Setup

1. Create a PostgreSQL database:

```sql
CREATE DATABASE hermod;
CREATE USER hermod WITH PASSWORD 'hermod_password';
GRANT ALL PRIVILEGES ON DATABASE hermod TO hermod;
```

2. Run the migration:

```bash
psql -U hermod -d hermod -f migrations/001_initial_schema.sql
```

For TimescaleDB, enable the extension first:

```sql
CREATE EXTENSION IF NOT EXISTS timescaledb;
```

Then uncomment the TimescaleDB-specific sections in the migration file.

## Lua Transformations

Create a Lua script with a `transform` function that takes a table and returns a transformed table:

```lua
function transform(data)
    local result = {}
    
    -- Add timestamp
    result.timestamp = os.time()
    
    -- Transform temperature from Celsius to Fahrenheit
    if data.temperature then
        result.temperature_celsius = data.temperature
        result.temperature_fahrenheit = (data.temperature * 9/5) + 32
    end
    
    -- Copy other fields
    for key, value in pairs(data) do
        if key ~= "temperature" then
            result[key] = value
        end
    end
    
    return result
end
```

See `examples/transform.lua` for a complete example.

## Logging

Hermod uses Go's standard logging framework with configurable log levels:

### Log Levels

- **DEBUG**: Verbose logging that shows message content when it arrives and after transformation. Useful for debugging data flow and transformation issues.
- **INFO**: General events like MQTT connections, subscriptions, and successful message processing.
- **ERROR**: Only errors are logged.

### Dry-Run Mode

Set `dry_run = true` in the `[logging]` section to run Hermod without connecting to the database. In this mode:
- SQL statements are logged instead of being executed
- No database connection is required
- Useful for testing configurations and transformations

Example configuration for debugging:

```toml
[logging]
level = "DEBUG"
dry_run = true
```

Example output with DEBUG level:
```
DEBUG: Received message from topic 'sensors/temperature': {"temperature": 25.5}
DEBUG: Transforming message data
DEBUG: Message transformed: map[temperature_celsius:25.5 temperature_fahrenheit:77.9]
INFO: SQL (dry-run): INSERT INTO mqtt_messages (temperature_celsius, temperature_fahrenheit, topic) VALUES ($1, $2, $3)
DEBUG: SQL Values: [25.5 77.9 sensors/temperature]
INFO: Successfully processed message from topic sensors/temperature
```

## Usage

Run Hermod with the default configuration file:

```bash
./hermod
```

Or specify a custom configuration file:

```bash
./hermod -config /path/to/config.toml
```

## Development

### Project Structure

```
Hermod/
├── cmd/
│   └── hermod/
│       └── main.go           # Application entry point
├── internal/
│   ├── config/              # Configuration management
│   ├── mqtt/                # MQTT client wrapper
│   ├── lua/                 # Lua transformation engine
│   ├── pipeline/            # Message processing pipeline
│   └── storage/             # Database operations
├── examples/
│   ├── config.toml          # Example configuration
│   └── transform.lua        # Example Lua script
├── migrations/
│   └── 001_initial_schema.sql  # Database schema
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
