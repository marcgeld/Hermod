-- Example Lua script with schema definition for routing

schema = {
  tables = {
    iot_metrics = {
      time = "timestamptz",
      device = "text",
      value = "double precision",
      raw = "jsonb"
    }
  }
}

function transform(msg)
  local records = {}
  
  -- Parse JSON if available
  if msg.json then
    local record = {
      table = "iot_metrics",
      columns = {
        time = msg.ts,
        device = msg.topic,
        value = msg.json.temperature or msg.json.value or 0,
        raw = msg.json
      }
    }
    table.insert(records, record)
  end
  
  return records
end
