import type { DocsCategory } from '../types'

const en: DocsCategory = {
  id: 'chat',
  category: 'Text Chat',
  beforeProtocols: [
    {
      id: 'chat-overview',
      title: 'Overview',
      blocks: [
        {
          type: 'p',
          text: 'The text chat endpoints speak the native OpenAI, Anthropic and Google Gemini protocols, so official SDKs and clients work as-is: point the base URL at this site and use a key from this site. Create keys on the API Keys page:'
        },
        {
          type: 'code',
          title: 'Authentication (any one of)',
          code: `Authorization: Bearer {{API_KEY}}
x-api-key: {{API_KEY}}
x-goog-api-key: {{API_KEY}}`
        },
        {
          type: 'code',
          title: 'Base URL per SDK',
          code: `OpenAI SDK:                  {{BASE_URL}}/v1
Anthropic SDK / Claude Code: {{BASE_URL}}
Gemini SDK:                  {{BASE_URL}} (the SDK appends /v1beta)`
        },
        {
          type: 'list',
          items: [
            'The base URL is this site; examples on this page fill it in. The OpenAI SDK needs the /v1 suffix, the Anthropic and Gemini SDKs do not.',
            'Endpoints under /v1 reject keys passed as a query parameter (?key=) with 400; only the native Gemini endpoints under /v1beta accept ?key=.',
            'Available models depend on the key\'s group. Model names in the examples are illustrative; rely on the model list endpoint.',
            'Which entry point works depends on the platform of the key\'s group, see the table below. The gateway converts between the entry protocol and the upstream protocol, and responses always use the entry protocol\'s format.'
          ]
        },
        {
          type: 'table',
          headers: ['Entry point', 'Protocol', 'Groups'],
          rows: [
            ['POST /v1/chat/completions', 'OpenAI Chat Completions', 'OpenAI, Anthropic, Gemini and other OpenAI-compatible vendor groups'],
            ['POST /v1/responses', 'OpenAI Responses', 'OpenAI, Anthropic and other OpenAI-compatible vendor groups (not Gemini groups)'],
            ['POST /v1/messages', 'Anthropic Messages', 'Anthropic, Gemini and other vendor groups; OpenAI groups need Messages dispatch enabled by an admin; Antigravity groups use /antigravity/v1/messages'],
            ['POST /v1beta/models/{model}:generateContent', 'Google Gemini', 'Gemini groups only; Antigravity groups use /antigravity/v1beta']
          ]
        }
      ]
    },
    {
      id: 'chat-billing',
      title: 'Billing & Balance',
      blocks: [
        {
          type: 'list',
          items: [
            'Charged by actual token usage (input, output, cache reads and writes) × model price × group multiplier; subscription groups consume subscription quota instead.',
            'Balance and quota are checked first: a balance ≤ 0 returns 403 and an exhausted quota window or key limit returns 429; neither request reaches the upstream.',
            'Streaming requests are settled after the stream ends, using the usage reported by the upstream.',
            'Usage, cost and latency of every request are listed on the Usage page.'
          ]
        }
      ]
    }
  ],
  protocolsId: 'chat-protocols',
  protocolsTitle: 'Per-Protocol Integration',
  protocols: [
    {
      id: 'chat-openai',
      name: 'OpenAI',
      sections: [
        {
          id: 'endpoints',
          title: 'Endpoints',
          blocks: [
            {
              type: 'table',
              headers: ['Method', 'Path', 'Description'],
              rows: [
                ['POST', '/v1/chat/completions', 'Chat Completions, supports stream'],
                ['POST', '/v1/responses', 'Responses API, supports stream'],
                ['GET', '/v1/responses', 'Responses WebSocket mode; the connection must upgrade to WebSocket, a plain GET returns 426'],
                ['GET', '/v1/models', 'List models available to the current key'],
                ['POST', '/v1/embeddings', 'Text embeddings, OpenAI groups only']
              ]
            },
            { type: 'p', text: 'Every path above also has an alias without the /v1 prefix (such as /chat/completions, /responses, /models) that behaves the same.' }
          ]
        },
        {
          id: 'params',
          title: 'Main Parameters (Chat Completions)',
          blocks: [
            {
              type: 'table',
              headers: ['Field', 'Type', 'Required', 'Description'],
              rows: [
                ['model', 'string', 'Yes', 'Model name; must be available in the group, otherwise 404 model_not_found'],
                ['messages', 'array', 'Yes', 'Conversation messages with role system / developer / user / assistant / tool; content is a string or a multimodal array (text, image_url)'],
                ['stream', 'boolean', 'No', 'When true, returns chat.completion.chunk objects over SSE, ending with data: [DONE]'],
                ['stream_options.include_usage', 'boolean', 'No', 'Appends a usage chunk when streaming; the gateway usually enables it for billing, so clients should handle a usage chunk with empty choices'],
                ['max_tokens / max_completion_tokens', 'integer', 'No', 'Maximum output tokens'],
                ['temperature / top_p', 'number', 'No', 'Sampling parameters; some reasoning models ignore or reject them'],
                ['reasoning_effort', 'string', 'No', 'Reasoning effort such as low / medium / high, reasoning models only'],
                ['tools / tool_choice', 'array / string|object', 'No', 'Function definitions and selection strategy']
              ]
            },
            {
              type: 'p',
              text: 'The Responses API mainly uses model, input (a string or message array), instructions, stream, tools and reasoning, with the same semantics as OpenAI. Other fields follow the official OpenAI definitions and are passed through or converted; whether they take effect depends on the group\'s upstream.'
            }
          ]
        },
        {
          id: 'examples',
          title: 'Examples',
          blocks: [
            {
              type: 'example',
              id: 'chat-completions',
              title: 'Chat Completions',
              description: 'Non-streaming chat that returns the full reply and usage at once.',
              request: `curl '{{BASE_URL}}/v1/chat/completions' \\
  -H 'Authorization: Bearer {{API_KEY}}' \\
  -H 'Content-Type: application/json' \\
  -d '{"model":"gpt-5.4","messages":[{"role":"system","content":"You are a concise assistant."},{"role":"user","content":"Introduce yourself in one sentence"}]}'`,
              responses: [
                {
                  title: 'Success',
                  status: 200,
                  body: '{"id":"chatcmpl-example","object":"chat.completion","created":1784092800,"model":"gpt-5.4","choices":[{"index":0,"message":{"role":"assistant","content":"I am an AI assistant that can answer questions, write and code."},"finish_reason":"stop"}],"usage":{"prompt_tokens":24,"completion_tokens":18,"total_tokens":42}}'
                }
              ]
            },
            {
              type: 'example',
              id: 'chat-stream',
              title: 'Chat Completions streaming',
              description: 'Set stream: true to receive chunks over SSE; the last data chunk carries usage, followed by [DONE].',
              request: `curl -N '{{BASE_URL}}/v1/chat/completions' \\
  -H 'Authorization: Bearer {{API_KEY}}' \\
  -H 'Content-Type: application/json' \\
  -d '{"model":"gpt-5.4","stream":true,"stream_options":{"include_usage":true},"messages":[{"role":"user","content":"Hello"}]}'`,
              responses: [
                {
                  title: 'Event stream',
                  status: 200,
                  description: 'Content-Type: text/event-stream.',
                  body: `data: {"id":"chatcmpl-example","object":"chat.completion.chunk","created":1784092800,"model":"gpt-5.4","choices":[{"index":0,"delta":{"role":"assistant","content":"Hello"},"finish_reason":null}]}

data: {"id":"chatcmpl-example","object":"chat.completion.chunk","created":1784092800,"model":"gpt-5.4","choices":[{"index":0,"delta":{"content":"! How can I help?"},"finish_reason":null}]}

data: {"id":"chatcmpl-example","object":"chat.completion.chunk","created":1784092800,"model":"gpt-5.4","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}

data: {"id":"chatcmpl-example","object":"chat.completion.chunk","created":1784092800,"model":"gpt-5.4","choices":[],"usage":{"prompt_tokens":8,"completion_tokens":10,"total_tokens":18}}

data: [DONE]`
                }
              ]
            },
            {
              type: 'example',
              id: 'responses',
              title: 'Responses',
              description: 'Call the Responses API; input is a string or a message array. Add "stream": true for streaming events.',
              request: `curl '{{BASE_URL}}/v1/responses' \\
  -H 'Authorization: Bearer {{API_KEY}}' \\
  -H 'Content-Type: application/json' \\
  -d '{"model":"gpt-5.4","instructions":"You are a concise assistant.","input":"Introduce yourself in one sentence"}'`,
              responses: [
                {
                  title: 'Success',
                  status: 200,
                  body: '{"id":"resp_example","object":"response","created_at":1784092800,"status":"completed","model":"gpt-5.4","output":[{"type":"message","id":"msg_example","status":"completed","role":"assistant","content":[{"type":"output_text","text":"I am an AI assistant that can answer questions, write and code.","annotations":[]}]}],"usage":{"input_tokens":24,"output_tokens":18,"total_tokens":42}}'
                }
              ]
            },
            {
              type: 'example',
              id: 'models',
              title: 'List models',
              description: 'Returns the models available to the key\'s group; groups with a model allowlist only return allowlisted models.',
              request: `curl '{{BASE_URL}}/v1/models' \\
  -H 'Authorization: Bearer {{API_KEY}}'`,
              responses: [
                {
                  title: 'Success (OpenAI group)',
                  status: 200,
                  description: 'Anthropic and other groups return items shaped as {"id","type":"model","display_name","created_at"}.',
                  body: '{"object":"list","data":[{"id":"gpt-5.4","object":"model","created":1784092800,"owned_by":"openai"}]}'
                }
              ]
            }
          ]
        }
      ]
    },
    {
      id: 'chat-anthropic',
      name: 'Anthropic',
      sections: [
        {
          id: 'endpoints',
          title: 'Endpoints',
          blocks: [
            {
              type: 'table',
              headers: ['Method', 'Path', 'Description'],
              rows: [
                ['POST', '/v1/messages', 'Messages API, supports stream'],
                ['POST', '/v1/messages/count_tokens', 'Count input tokens, free of charge'],
                ['GET', '/v1/models', 'List models available to the current key']
              ]
            },
            {
              type: 'list',
              items: [
                'For Claude Code, set ANTHROPIC_BASE_URL to this site and ANTHROPIC_AUTH_TOKEN to your key, or configure it in one click on the Import Clients page.',
                'The anthropic-version header is optional; the gateway fills in 2023-06-01 when it is missing.',
                'Antigravity groups append /antigravity to this site as the base URL (that is, /antigravity/v1/messages).',
                'When an OpenAI group calls /v1/messages, the gateway converts the Anthropic format to the upstream protocol and back; an admin must enable Messages dispatch for that group, otherwise it returns 403.'
              ]
            }
          ]
        },
        {
          id: 'params',
          title: 'Main Parameters',
          blocks: [
            {
              type: 'table',
              headers: ['Field', 'Type', 'Required', 'Description'],
              rows: [
                ['model', 'string', 'Yes', 'Model name; the gateway returns 400 when it is missing'],
                ['messages', 'array', 'Yes', 'Conversation messages with role user / assistant; content is a string or an array of content blocks (text, image, tool_use, tool_result and more)'],
                ['max_tokens', 'integer', 'Yes', 'Maximum output tokens (validated by the upstream)'],
                ['system', 'string | array', 'No', 'System prompt'],
                ['stream', 'boolean', 'No', 'When true, returns SSE events such as message_start, content_block_delta, message_delta and message_stop'],
                ['temperature / top_p / top_k', 'number', 'No', 'Sampling parameters'],
                ['thinking', 'object', 'No', 'Extended thinking, for example {"type":"enabled","budget_tokens":2048}'],
                ['tools / tool_choice', 'array / object', 'No', 'Tool definitions and selection strategy']
              ]
            }
          ]
        },
        {
          id: 'examples',
          title: 'Examples',
          blocks: [
            {
              type: 'example',
              id: 'messages',
              title: 'Messages',
              description: 'Non-streaming chat that returns a complete message object.',
              request: `curl '{{BASE_URL}}/v1/messages' \\
  -H 'x-api-key: {{API_KEY}}' \\
  -H 'anthropic-version: 2023-06-01' \\
  -H 'Content-Type: application/json' \\
  -d '{"model":"claude-sonnet-4-6","max_tokens":1024,"system":"You are a concise assistant.","messages":[{"role":"user","content":"Introduce yourself in one sentence"}]}'`,
              responses: [
                {
                  title: 'Success',
                  status: 200,
                  body: '{"id":"msg_example","type":"message","role":"assistant","model":"claude-sonnet-4-6","content":[{"type":"text","text":"I am Claude, an AI assistant that can help you answer questions, write and code."}],"stop_reason":"end_turn","stop_sequence":null,"usage":{"input_tokens":24,"output_tokens":22}}'
                }
              ]
            },
            {
              type: 'example',
              id: 'messages-stream',
              title: 'Messages streaming',
              description: 'Set stream: true to receive events in Anthropic SSE order.',
              request: `curl -N '{{BASE_URL}}/v1/messages' \\
  -H 'x-api-key: {{API_KEY}}' \\
  -H 'anthropic-version: 2023-06-01' \\
  -H 'Content-Type: application/json' \\
  -d '{"model":"claude-sonnet-4-6","max_tokens":1024,"stream":true,"messages":[{"role":"user","content":"Hello"}]}'`,
              responses: [
                {
                  title: 'Event stream',
                  status: 200,
                  description: 'Content-Type: text/event-stream.',
                  body: `event: message_start
data: {"type":"message_start","message":{"id":"msg_example","type":"message","role":"assistant","model":"claude-sonnet-4-6","content":[],"stop_reason":null,"usage":{"input_tokens":8,"output_tokens":1}}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello! How can I help?"}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":null},"usage":{"output_tokens":10}}

event: message_stop
data: {"type":"message_stop"}`
                }
              ]
            },
            {
              type: 'example',
              id: 'count-tokens',
              title: 'Count tokens',
              description: 'Estimate input tokens before sending; returns 404 for account types that do not support it.',
              request: `curl '{{BASE_URL}}/v1/messages/count_tokens' \\
  -H 'x-api-key: {{API_KEY}}' \\
  -H 'anthropic-version: 2023-06-01' \\
  -H 'Content-Type: application/json' \\
  -d '{"model":"claude-sonnet-4-6","messages":[{"role":"user","content":"Hello"}]}'`,
              responses: [
                { title: 'Success', status: 200, body: '{"input_tokens":8}' }
              ]
            }
          ]
        }
      ]
    },
    {
      id: 'chat-gemini',
      name: 'Google Gemini',
      sections: [
        {
          id: 'endpoints',
          title: 'Endpoints',
          blocks: [
            {
              type: 'table',
              headers: ['Method', 'Path', 'Description'],
              rows: [
                ['POST', '/v1beta/models/{model}:generateContent', 'Generate content (non-streaming)'],
                ['POST', '/v1beta/models/{model}:streamGenerateContent?alt=sse', 'Generate content (streaming), always returned as SSE'],
                ['POST', '/v1beta/models/{model}:countTokens', 'Count tokens'],
                ['GET', '/v1beta/models', 'List available models (Gemini format)'],
                ['GET', '/v1beta/models/{model}', 'Get a single model']
              ]
            },
            {
              type: 'list',
              items: [
                'Gemini groups only; other groups get 400 "API key group platform is not gemini". Antigravity groups use the /antigravity/v1beta prefix instead.',
                'The x-goog-api-key header is recommended; Authorization: Bearer, x-api-key and the ?key= query parameter also work.',
                'Only the generateContent, streamGenerateContent and countTokens actions are supported; other actions (such as Imagen :predict) return 404.',
                'With the Gemini SDK, set base_url (or http_options.base_url) to this site and api_key to your key.'
              ]
            }
          ]
        },
        {
          id: 'params',
          title: 'Main Parameters',
          blocks: [
            {
              type: 'table',
              headers: ['Field', 'Type', 'Required', 'Description'],
              rows: [
                ['contents', 'array', 'Yes', 'Conversation content; each item has a role (user / model) and parts (text, inlineData and more)'],
                ['systemInstruction', 'object', 'No', 'System instruction, for example {"parts":[{"text":"…"}]}'],
                ['generationConfig', 'object', 'No', 'Generation config: temperature, maxOutputTokens, topP, thinkingConfig, responseMimeType and more'],
                ['tools / toolConfig', 'array / object', 'No', 'Function calling, Google Search and other tools'],
                ['safetySettings', 'array', 'No', 'Safety filter settings']
              ]
            },
            { type: 'p', text: 'The model name goes in the URL path. Fields follow the official Google Gemini API; the gateway only rewrites the model name per account configuration.' }
          ]
        },
        {
          id: 'examples',
          title: 'Examples',
          blocks: [
            {
              type: 'example',
              id: 'generate-content',
              title: 'generateContent',
              description: 'Non-streaming generation that returns candidates and usageMetadata.',
              request: `curl '{{BASE_URL}}/v1beta/models/gemini-2.5-flash:generateContent' \\
  -H 'x-goog-api-key: {{API_KEY}}' \\
  -H 'Content-Type: application/json' \\
  -d '{"systemInstruction":{"parts":[{"text":"You are a concise assistant."}]},"contents":[{"role":"user","parts":[{"text":"Introduce yourself in one sentence"}]}],"generationConfig":{"temperature":0.7,"maxOutputTokens":1024}}'`,
              responses: [
                {
                  title: 'Success',
                  status: 200,
                  body: '{"candidates":[{"content":{"role":"model","parts":[{"text":"I am Gemini, an AI assistant that can help you answer questions, write and code."}]},"finishReason":"STOP","index":0}],"usageMetadata":{"promptTokenCount":16,"candidatesTokenCount":20,"totalTokenCount":36},"modelVersion":"gemini-2.5-flash"}'
                }
              ]
            },
            {
              type: 'example',
              id: 'stream-generate-content',
              title: 'streamGenerateContent streaming',
              description: 'Streaming generation; each data: line is a GenerateContentResponse and the last one carries finishReason and usageMetadata.',
              request: `curl -N '{{BASE_URL}}/v1beta/models/gemini-2.5-flash:streamGenerateContent?alt=sse' \\
  -H 'x-goog-api-key: {{API_KEY}}' \\
  -H 'Content-Type: application/json' \\
  -d '{"contents":[{"role":"user","parts":[{"text":"Hello"}]}]}'`,
              responses: [
                {
                  title: 'Event stream',
                  status: 200,
                  description: 'Content-Type: text/event-stream.',
                  body: `data: {"candidates":[{"content":{"role":"model","parts":[{"text":"Hello! "}]},"index":0}],"modelVersion":"gemini-2.5-flash"}

data: {"candidates":[{"content":{"role":"model","parts":[{"text":"How can I help?"}]},"finishReason":"STOP","index":0}],"usageMetadata":{"promptTokenCount":4,"candidatesTokenCount":9,"totalTokenCount":13},"modelVersion":"gemini-2.5-flash"}`
                }
              ]
            },
            {
              type: 'example',
              id: 'list-models',
              title: 'List models',
              description: 'Returns models in Gemini format; groups with a model allowlist only return allowlisted models.',
              request: `curl '{{BASE_URL}}/v1beta/models' \\
  -H 'x-goog-api-key: {{API_KEY}}'`,
              responses: [
                {
                  title: 'Success',
                  status: 200,
                  body: '{"models":[{"name":"models/gemini-2.5-flash","displayName":"Gemini 2.5 Flash","supportedGenerationMethods":["generateContent","streamGenerateContent"]}]}'
                }
              ]
            }
          ]
        }
      ]
    }
  ],
  afterProtocols: [
    {
      id: 'chat-errors',
      title: 'Error Codes',
      blocks: [
        {
          type: 'table',
          headers: ['Status', 'Meaning'],
          rows: [
            ['400', 'Invalid parameters (such as a missing model); key passed as ?key= under /v1 (api_key_in_query_deprecated); a non-Gemini group calling /v1beta'],
            ['401', 'API key missing or invalid (API_KEY_REQUIRED / INVALID_API_KEY)'],
            ['403', 'Balance ≤ 0 (INSUFFICIENT_BALANCE / billing_error); key not assigned to a group; Messages dispatch not enabled for an OpenAI group'],
            ['404', 'Model not available in the group (model_not_found); the group does not support the endpoint (such as embeddings or count_tokens)'],
            ['413', 'Request body exceeds the size limit'],
            ['429', 'Rate, concurrency or quota limit exceeded; when Retry-After is present, wait that long before retrying'],
            ['502', 'Upstream error or connection failure'],
            ['503', 'No account is currently available in the group (not configured, disabled or busy); retry later']
          ]
        },
        {
          type: 'p',
          text: 'The error body follows the entry protocol: Anthropic endpoints return {"type":"error","error":{"type","message"}}; OpenAI endpoints return {"error":{"type","message"}} (some errors include code); Gemini endpoints return {"error":{"code","message","status"}}. Errors raised during authentication (missing or invalid key, insufficient balance) use the flat {"code","message"} shape. Native upstream errors are passed through in their original format where possible, so check both the HTTP status and the body.'
        }
      ]
    }
  ]
}

export default en
