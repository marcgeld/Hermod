-- Example Lua transformation script for Hermod
-- This function is called for each incoming message

function transform(data)
    -- Create a new transformed table
    local result = {}
    
    -- Copy the topic
    result.topic = data.topic
    
    -- Add a timestamp if not present
    if not data.timestamp then
        result.timestamp = os.time()
    else
        result.timestamp = data.timestamp
    end
    
    -- Transform temperature from Celsius to Fahrenheit if present
    if data.temperature then
        result.temperature_celsius = data.temperature
        result.temperature_fahrenheit = (data.temperature * 9/5) + 32
    end
    
    -- Copy other fields
    for key, value in pairs(data) do
        if key ~= "temperature" and key ~= "topic" and key ~= "timestamp" then
            result[key] = value
        end
    end
    
    -- Add metadata
    result.processed_by = "hermod"
    result.processed_at = os.date("%Y-%m-%d %H:%M:%S")
    
    return result
end
