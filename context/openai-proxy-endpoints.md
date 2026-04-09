# OpenAI Proxy Endpoint Reference

Minimal reference for the 8 OpenAI-compatible endpoints LLMate proxies. Extracted from the full OpenAI API spec (v2.3.0) in `openapi.with-code-samples.yml`.

## 1. POST /v1/chat/completions

Creates a chat completion. This is the primary endpoint.

**Request** (`Content-Type: application/json`):
```json
{
  "model": "string (required)",
  "messages": [
    {"role": "system|user|assistant|tool", "content": "string"}
  ],
  "temperature": 0.0-2.0,
  "top_p": 0.0-1.0,
  "n": 1,
  "stream": false,
  "stream_options": {"include_usage": true},
  "stop": "string or array",
  "max_tokens": 1234,
  "max_completion_tokens": 1234,
  "presence_penalty": -2.0-2.0,
  "frequency_penalty": -2.0-2.0,
  "tools": [...],
  "tool_choice": "auto|none|required|{...}",
  "response_format": {"type": "text|json_object|json_schema"},
  "seed": 1234,
  "user": "string"
}
```

**Response** (non-streaming):
```json
{
  "id": "chatcmpl-...",
  "object": "chat.completion",
  "created": 1234567890,
  "model": "string",
  "choices": [
    {
      "index": 0,
      "message": {"role": "assistant", "content": "string", "tool_calls": [...]},
      "finish_reason": "stop|length|tool_calls|content_filter",
      "logprobs": null
    }
  ],
  "usage": {
    "prompt_tokens": 10,
    "completion_tokens": 20,
    "total_tokens": 30,
    "prompt_tokens_details": {"cached_tokens": 5},
    "completion_tokens_details": {"reasoning_tokens": 0}
  },
  "system_fingerprint": "string"
}
```

**Streaming** (when `stream: true`): Server-Sent Events (SSE). Each line is `data: {json}\n\n`.

Streaming chunk format:
```json
{
  "id": "chatcmpl-...",
  "object": "chat.completion.chunk",
  "created": 1234567890,
  "model": "string",
  "choices": [
    {
      "index": 0,
      "delta": {"role": "assistant", "content": "token"},
      "finish_reason": null
    }
  ]
}
```

Final chunk (when `stream_options.include_usage: true`):
```json
{
  "id": "chatcmpl-...",
  "object": "chat.completion.chunk",
  "created": 1234567890,
  "model": "string",
  "choices": [],
  "usage": {
    "prompt_tokens": 10,
    "completion_tokens": 20,
    "total_tokens": 30,
    "prompt_tokens_details": {"cached_tokens": 5}
  }
}
```

Stream ends with `data: [DONE]\n\n`.

## 2. POST /v1/completions

Legacy text completions. Similar to chat completions but with `prompt` instead of `messages`.

**Request**:
```json
{
  "model": "string (required)",
  "prompt": "string or array (required)",
  "max_tokens": 16,
  "temperature": 1.0,
  "top_p": 1.0,
  "n": 1,
  "stream": false,
  "stop": "string or array",
  "presence_penalty": 0,
  "frequency_penalty": 0,
  "user": "string"
}
```

**Response**:
```json
{
  "id": "cmpl-...",
  "object": "text_completion",
  "created": 1234567890,
  "model": "string",
  "choices": [
    {
      "text": "string",
      "index": 0,
      "logprobs": null,
      "finish_reason": "stop|length"
    }
  ],
  "usage": {
    "prompt_tokens": 5,
    "completion_tokens": 7,
    "total_tokens": 12
  }
}
```

Streaming works the same way as chat completions (SSE with `data: {json}` lines).

## 3. POST /v1/embeddings

Creates embedding vectors from text.

**Request**:
```json
{
  "model": "string (required)",
  "input": "string or array of strings (required)",
  "encoding_format": "float|base64",
  "dimensions": 1234,
  "user": "string"
}
```

**Response**:
```json
{
  "object": "list",
  "data": [
    {
      "object": "embedding",
      "index": 0,
      "embedding": [0.1, 0.2, ...]
    }
  ],
  "model": "string",
  "usage": {
    "prompt_tokens": 5,
    "total_tokens": 5
  }
}
```

No streaming support.

## 4. POST /v1/images/generations

Generates images from a prompt.

**Request**:
```json
{
  "model": "string (required)",
  "prompt": "string (required)",
  "n": 1,
  "size": "256x256|512x512|1024x1024|1792x1024|1024x1792",
  "quality": "standard|hd",
  "response_format": "url|b64_json",
  "style": "vivid|natural",
  "user": "string"
}
```

**Response**:
```json
{
  "created": 1234567890,
  "data": [
    {
      "url": "https://...",
      "b64_json": "...",
      "revised_prompt": "string"
    }
  ]
}
```

No streaming. No `usage` field in standard response.

## 5. POST /v1/audio/speech

Generates audio from text (text-to-speech).

**Request** (`Content-Type: application/json`):
```json
{
  "model": "string (required)",
  "input": "string (required)",
  "voice": "alloy|echo|fable|onyx|nova|shimmer (required)",
  "response_format": "mp3|opus|aac|flac|wav|pcm",
  "speed": 0.25-4.0
}
```

**Response**: Raw audio bytes (`Content-Type: application/octet-stream`). No JSON body. No `usage` field.

## 6. POST /v1/audio/transcriptions

Transcribes audio to text.

**Request** (`Content-Type: multipart/form-data`):
- `file`: audio file (required)
- `model`: string (required)
- `language`: ISO-639-1 code
- `prompt`: string
- `response_format`: json|text|srt|verbose_json|vtt
- `temperature`: 0.0-1.0

**Response** (json format):
```json
{
  "text": "Transcribed text here."
}
```

No streaming. No `usage` field in standard response.

## 7. GET /v1/models

Lists available models.

**Response**:
```json
{
  "object": "list",
  "data": [
    {
      "id": "model-id",
      "object": "model",
      "created": 1234567890,
      "owned_by": "organization"
    }
  ]
}
```

## 8. GET /v1/models/{model}

Retrieves details for a specific model.

**Response**:
```json
{
  "id": "model-id",
  "object": "model",
  "created": 1234567890,
  "owned_by": "organization"
}
```

## Key Implementation Notes for the Proxy

1. **Model extraction**: The `model` field is always at the top level of the JSON body for POST endpoints. For multipart requests (audio/transcriptions), it's a form field.

2. **Streaming detection**: Check if `"stream": true` is present in the request body. Only chat/completions and completions support streaming.

3. **Usage extraction**: Parse the `usage` object from the response for token tracking. For streaming, usage comes in the final chunk when `stream_options.include_usage` is set. The proxy should inject `"stream_options": {"include_usage": true}` into streaming requests to ensure backends report usage.

4. **Passthrough principle**: The proxy should forward request bodies as-is (after extracting the model name for routing). Don't validate or transform fields -- let the backend handle validation.

5. **Response passthrough**: Forward response bodies as-is. The proxy only needs to parse enough to extract `usage` for logging.

6. **Cache tokens**: The `usage.prompt_tokens_details.cached_tokens` field indicates how many prompt tokens were served from cache. Extract this for analytics.

7. **Error responses**: Backends typically return errors as `{"error": {"message": "...", "type": "...", "code": "..."}}`. Forward these as-is with the original status code.
