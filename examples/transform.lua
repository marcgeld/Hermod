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

    -- Demonstrate Go-backed helpers exposed to Lua
    -- rot13: rotate ASCII letters by 13
    if data.text then
        result.original_text = data.text
        result.text_rot13 = rot13(data.text)
    end

    -- base64 encode / decode
    if data.payload then
        result.payload = data.payload
        result.payload_b64 = base64_encode(data.payload)
        -- decode to verify round-trip (base64_decode returns value, err)
        local decoded, err = base64_decode(result.payload_b64)
        if err == nil then
            result.payload_b64_roundtrip = decoded
        else
            result.payload_b64_decode_error = err
        end
    end

    -- Copy other fields
    for key, value in pairs(data) do
        if key ~= "temperature" and key ~= "topic" and key ~= "timestamp" and key ~= "text" and key ~= "payload" then
            result[key] = value
        end
    end
    
    -- Add metadata
    result.processed_by = "hermod"
    result.processed_at = os.time()  -- Unix timestamp for proper database storage
    
    return result
end
