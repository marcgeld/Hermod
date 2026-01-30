-- Example: Multi-table Lua script with schema validation

schema = {
  tables = {
    sensor_readings = {
      time = "timestamptz",
      sensor_id = "text",
      temperature = "double precision",
      humidity = "double precision",
      battery = "double precision"
    },
    sensor_events = {
      time = "timestamptz",
      sensor_id = "text",
      event_type = "text",
      details = "jsonb"
    }
  }
}

function transform(msg)
  local records = {}
  
  -- Only process JSON payloads
  if not msg.json then
    return records
  end
  
  local data = msg.json
  local sensor_id = msg.topic:match("sensors/([^/]+)")
  
  -- Create a sensor reading record
  if data.temperature or data.humidity then
    local reading = {
      table = "sensor_readings",
      columns = {
        time = msg.ts,
        sensor_id = sensor_id or "unknown",
        temperature = data.temperature or 0,
        humidity = data.humidity or 0,
        battery = data.battery or 100
      }
    }
    table.insert(records, reading)
  end
  
  -- Create an event record if there's an alert
  if data.alert then
    local event = {
      table = "sensor_events",
      columns = {
        time = msg.ts,
        sensor_id = sensor_id or "unknown",
        event_type = "alert",
        details = data
      }
    }
    table.insert(records, event)
  end
  
  return records
end
