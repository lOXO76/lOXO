import type { DocsCategory } from '../types'

const zh: DocsCategory = {
  id: 'chat',
  category: '文本对话',
  beforeProtocols: [
    {
      id: 'chat-overview',
      title: '概览',
      blocks: [
        {
          type: 'p',
          text: '文本对话接口兼容 OpenAI、Anthropic 与 Google Gemini 三种原生协议，可直接使用各厂商官方 SDK 或客户端，只需把 Base URL 换成本站地址、API Key 换成本站密钥。Key 在「API 密钥」页创建：'
        },
        {
          type: 'code',
          title: '鉴权方式（任选其一）',
          code: `Authorization: Bearer {{API_KEY}}
x-api-key: {{API_KEY}}
x-goog-api-key: {{API_KEY}}`
        },
        {
          type: 'code',
          title: '各 SDK 的 Base URL',
          code: `OpenAI SDK:                  {{BASE_URL}}/v1
Anthropic SDK / Claude Code: {{BASE_URL}}
Gemini SDK:                  {{BASE_URL}}（SDK 自动拼接 /v1beta）`
        },
        {
          type: 'list',
          items: [
            'Base URL 为当前站点地址，本页示例已自动填入；注意 OpenAI SDK 需要带 /v1，Anthropic 与 Gemini SDK 不带。',
            '/v1 下的接口不接受通过 URL 查询参数（?key=）传 Key，会返回 400；只有 /v1beta 的 Gemini 原生接口兼容 ?key=。',
            '可用的模型由 Key 所在分组决定，示例中的模型名仅作演示，请以模型列表接口的返回为准。',
            '同一请求使用哪个入口，与 Key 所在分组的平台有关，见下表；网关会在入口协议与上游协议之间自动转换，返回格式始终与入口协议一致。'
          ]
        },
        {
          type: 'table',
          headers: ['入口', '协议', '可用分组'],
          rows: [
            ['POST /v1/chat/completions', 'OpenAI Chat Completions', 'OpenAI、Anthropic、Gemini 及其它 OpenAI 兼容厂商分组'],
            ['POST /v1/responses', 'OpenAI Responses', 'OpenAI、Anthropic 及其它 OpenAI 兼容厂商分组（Gemini 分组不支持）'],
            ['POST /v1/messages', 'Anthropic Messages', 'Anthropic、Gemini 及其它厂商分组；OpenAI 分组需管理员开启 Messages 调度；Antigravity 分组使用 /antigravity/v1/messages'],
            ['POST /v1beta/models/{model}:generateContent', 'Google Gemini', '仅 Gemini 分组；Antigravity 分组使用 /antigravity/v1beta']
          ]
        }
      ]
    },
    {
      id: 'chat-billing',
      title: '计费与余额',
      blocks: [
        {
          type: 'list',
          items: [
            '按实际 token 用量（输入、输出、缓存读写）× 模型单价 × 分组倍率扣费；订阅分组扣减订阅额度。',
            '请求前会检查余额与额度：余额 ≤ 0 返回 403，额度窗口或 Key 限额用尽返回 429，请求都不会发往上游。',
            '流式请求在流结束后按上游返回的用量结算。',
            '每次请求的用量、费用与耗时可在「使用记录」页查看。'
          ]
        }
      ]
    }
  ],
  protocolsId: 'chat-protocols',
  protocolsTitle: '按协议接入',
  protocols: [
    {
      id: 'chat-openai',
      name: 'OpenAI',
      sections: [
        {
          id: 'endpoints',
          title: '接口',
          blocks: [
            {
              type: 'table',
              headers: ['方法', '路径', '说明'],
              rows: [
                ['POST', '/v1/chat/completions', 'Chat Completions 对话补全，支持 stream 流式'],
                ['POST', '/v1/responses', 'Responses 接口，支持 stream 流式'],
                ['GET', '/v1/responses', 'Responses WebSocket 模式，需以 WebSocket 升级连接，普通 GET 返回 426'],
                ['GET', '/v1/models', '列出当前 Key 可用的模型'],
                ['POST', '/v1/embeddings', '文本向量，仅 OpenAI 分组可用']
              ]
            },
            { type: 'p', text: '以上路径同时提供不带 /v1 前缀的别名（如 /chat/completions、/responses、/models），行为一致。' }
          ]
        },
        {
          id: 'params',
          title: '主要参数（Chat Completions）',
          blocks: [
            {
              type: 'table',
              headers: ['字段', '类型', '必填', '说明'],
              rows: [
                ['model', 'string', '是', '模型名，须在分组可用模型内，否则返回 404 model_not_found'],
                ['messages', 'array', '是', '对话消息，role 为 system／developer／user／assistant／tool；content 可为字符串或多模态数组（text、image_url）'],
                ['stream', 'boolean', '否', 'true 时以 SSE 流式返回 chat.completion.chunk，以 data: [DONE] 结束'],
                ['stream_options.include_usage', 'boolean', '否', '流式时在最后附带 usage 块；网关为了计费通常会自动开启，客户端应能处理 choices 为空的 usage 块'],
                ['max_tokens / max_completion_tokens', 'integer', '否', '最大输出 token 数'],
                ['temperature / top_p', 'number', '否', '采样参数，部分推理模型会忽略或拒绝'],
                ['reasoning_effort', 'string', '否', '推理强度，如 low／medium／high，仅推理模型支持'],
                ['tools / tool_choice', 'array / string|object', '否', '函数调用定义与选择策略']
              ]
            },
            {
              type: 'p',
              text: 'Responses 接口的主要字段为 model、input（字符串或消息数组）、instructions、stream、tools、reasoning，语义与 OpenAI 官方一致。其余字段按 OpenAI 官方定义透传或转换，具体能否生效以所在分组的上游为准。'
            }
          ]
        },
        {
          id: 'examples',
          title: '示例',
          blocks: [
            {
              type: 'example',
              id: 'chat-completions',
              title: 'Chat Completions',
              description: '非流式对话，一次性返回完整回复与用量。',
              request: `curl '{{BASE_URL}}/v1/chat/completions' \\
  -H 'Authorization: Bearer {{API_KEY}}' \\
  -H 'Content-Type: application/json' \\
  -d '{"model":"gpt-5.4","messages":[{"role":"system","content":"你是一个简洁的助手。"},{"role":"user","content":"用一句话介绍你自己"}]}'`,
              responses: [
                {
                  title: '成功',
                  status: 200,
                  body: '{"id":"chatcmpl-example","object":"chat.completion","created":1784092800,"model":"gpt-5.4","choices":[{"index":0,"message":{"role":"assistant","content":"我是一个可以回答问题、写作和编程的 AI 助手。"},"finish_reason":"stop"}],"usage":{"prompt_tokens":24,"completion_tokens":18,"total_tokens":42}}'
                }
              ]
            },
            {
              type: 'example',
              id: 'chat-stream',
              title: 'Chat Completions 流式',
              description: '设置 stream: true，以 SSE 逐块返回；最后一个数据块携带 usage，随后以 [DONE] 结束。',
              request: `curl -N '{{BASE_URL}}/v1/chat/completions' \\
  -H 'Authorization: Bearer {{API_KEY}}' \\
  -H 'Content-Type: application/json' \\
  -d '{"model":"gpt-5.4","stream":true,"stream_options":{"include_usage":true},"messages":[{"role":"user","content":"你好"}]}'`,
              responses: [
                {
                  title: '事件流',
                  status: 200,
                  description: 'Content-Type: text/event-stream。',
                  body: `data: {"id":"chatcmpl-example","object":"chat.completion.chunk","created":1784092800,"model":"gpt-5.4","choices":[{"index":0,"delta":{"role":"assistant","content":"你好"},"finish_reason":null}]}

data: {"id":"chatcmpl-example","object":"chat.completion.chunk","created":1784092800,"model":"gpt-5.4","choices":[{"index":0,"delta":{"content":"！有什么可以帮你？"},"finish_reason":null}]}

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
              description: '使用 Responses 接口，input 可以是字符串或消息数组；加 "stream": true 即为流式事件。',
              request: `curl '{{BASE_URL}}/v1/responses' \\
  -H 'Authorization: Bearer {{API_KEY}}' \\
  -H 'Content-Type: application/json' \\
  -d '{"model":"gpt-5.4","instructions":"你是一个简洁的助手。","input":"用一句话介绍你自己"}'`,
              responses: [
                {
                  title: '成功',
                  status: 200,
                  body: '{"id":"resp_example","object":"response","created_at":1784092800,"status":"completed","model":"gpt-5.4","output":[{"type":"message","id":"msg_example","status":"completed","role":"assistant","content":[{"type":"output_text","text":"我是一个可以回答问题、写作和编程的 AI 助手。","annotations":[]}]}],"usage":{"input_tokens":24,"output_tokens":18,"total_tokens":42}}'
                }
              ]
            },
            {
              type: 'example',
              id: 'models',
              title: '模型列表',
              description: '返回当前 Key 所在分组可用的模型；已开启模型白名单的分组只返回白名单内的模型。',
              request: `curl '{{BASE_URL}}/v1/models' \\
  -H 'Authorization: Bearer {{API_KEY}}'`,
              responses: [
                {
                  title: '成功（OpenAI 分组）',
                  status: 200,
                  description: 'Anthropic 等分组的元素为 {"id","type":"model","display_name","created_at"}。',
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
          title: '接口',
          blocks: [
            {
              type: 'table',
              headers: ['方法', '路径', '说明'],
              rows: [
                ['POST', '/v1/messages', 'Messages 对话，支持 stream 流式'],
                ['POST', '/v1/messages/count_tokens', '计算输入 token 数，不产生费用'],
                ['GET', '/v1/models', '列出当前 Key 可用的模型']
              ]
            },
            {
              type: 'list',
              items: [
                'Claude Code 将 ANTHROPIC_BASE_URL 设为本站地址、ANTHROPIC_AUTH_TOKEN 设为本站 Key 即可使用；也可在「导入客户端」页一键配置。',
                'anthropic-version 请求头可省略，缺省时网关按 2023-06-01 补齐。',
                'Antigravity 分组请在本站地址后加 /antigravity 作为 Base URL（即 /antigravity/v1/messages）。',
                'OpenAI 分组调用 /v1/messages 时，网关把 Anthropic 格式转换为上游协议再转换回来；需管理员为该分组开启 Messages 调度，否则返回 403。'
              ]
            }
          ]
        },
        {
          id: 'params',
          title: '主要参数',
          blocks: [
            {
              type: 'table',
              headers: ['字段', '类型', '必填', '说明'],
              rows: [
                ['model', 'string', '是', '模型名，缺失时网关直接返回 400'],
                ['messages', 'array', '是', '对话消息，role 为 user／assistant；content 可为字符串或内容块数组（text、image、tool_use、tool_result 等）'],
                ['max_tokens', 'integer', '是', '最大输出 token 数（由上游校验）'],
                ['system', 'string | array', '否', '系统提示词'],
                ['stream', 'boolean', '否', 'true 时以 SSE 返回 message_start、content_block_delta、message_delta、message_stop 等事件'],
                ['temperature / top_p / top_k', 'number', '否', '采样参数'],
                ['thinking', 'object', '否', '扩展思考配置，如 {"type":"enabled","budget_tokens":2048}'],
                ['tools / tool_choice', 'array / object', '否', '工具定义与选择策略']
              ]
            }
          ]
        },
        {
          id: 'examples',
          title: '示例',
          blocks: [
            {
              type: 'example',
              id: 'messages',
              title: 'Messages',
              description: '非流式对话，返回完整的 message 对象。',
              request: `curl '{{BASE_URL}}/v1/messages' \\
  -H 'x-api-key: {{API_KEY}}' \\
  -H 'anthropic-version: 2023-06-01' \\
  -H 'Content-Type: application/json' \\
  -d '{"model":"claude-sonnet-4-6","max_tokens":1024,"system":"你是一个简洁的助手。","messages":[{"role":"user","content":"用一句话介绍你自己"}]}'`,
              responses: [
                {
                  title: '成功',
                  status: 200,
                  body: '{"id":"msg_example","type":"message","role":"assistant","model":"claude-sonnet-4-6","content":[{"type":"text","text":"我是 Claude，一个可以帮你回答问题、写作和编程的 AI 助手。"}],"stop_reason":"end_turn","stop_sequence":null,"usage":{"input_tokens":24,"output_tokens":22}}'
                }
              ]
            },
            {
              type: 'example',
              id: 'messages-stream',
              title: 'Messages 流式',
              description: '设置 stream: true，按 Anthropic SSE 事件顺序返回。',
              request: `curl -N '{{BASE_URL}}/v1/messages' \\
  -H 'x-api-key: {{API_KEY}}' \\
  -H 'anthropic-version: 2023-06-01' \\
  -H 'Content-Type: application/json' \\
  -d '{"model":"claude-sonnet-4-6","max_tokens":1024,"stream":true,"messages":[{"role":"user","content":"你好"}]}'`,
              responses: [
                {
                  title: '事件流',
                  status: 200,
                  description: 'Content-Type: text/event-stream。',
                  body: `event: message_start
data: {"type":"message_start","message":{"id":"msg_example","type":"message","role":"assistant","model":"claude-sonnet-4-6","content":[],"stop_reason":null,"usage":{"input_tokens":8,"output_tokens":1}}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"你好！有什么可以帮你？"}}

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
              title: '计算 token',
              description: '在发送前估算输入 token 数；部分账号类型不支持时返回 404。',
              request: `curl '{{BASE_URL}}/v1/messages/count_tokens' \\
  -H 'x-api-key: {{API_KEY}}' \\
  -H 'anthropic-version: 2023-06-01' \\
  -H 'Content-Type: application/json' \\
  -d '{"model":"claude-sonnet-4-6","messages":[{"role":"user","content":"你好"}]}'`,
              responses: [
                { title: '成功', status: 200, body: '{"input_tokens":8}' }
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
          title: '接口',
          blocks: [
            {
              type: 'table',
              headers: ['方法', '路径', '说明'],
              rows: [
                ['POST', '/v1beta/models/{model}:generateContent', '生成内容（非流式）'],
                ['POST', '/v1beta/models/{model}:streamGenerateContent?alt=sse', '生成内容（流式），始终以 SSE 返回'],
                ['POST', '/v1beta/models/{model}:countTokens', '计算 token 数'],
                ['GET', '/v1beta/models', '列出可用模型（Gemini 格式）'],
                ['GET', '/v1beta/models/{model}', '查询单个模型信息']
              ]
            },
            {
              type: 'list',
              items: [
                '仅 Gemini 分组可用，其它分组返回 400「API key group platform is not gemini」；Antigravity 分组请改用 /antigravity/v1beta 前缀。',
                '鉴权推荐 x-goog-api-key 请求头，也兼容 Authorization: Bearer、x-api-key 与 ?key= 查询参数。',
                '仅支持 generateContent、streamGenerateContent、countTokens 三种动作，其它动作（如 Imagen 的 :predict）返回 404。',
                '使用 Gemini SDK 时把 base_url（或 http_options.base_url）设为本站地址，api_key 设为本站 Key 即可。'
              ]
            }
          ]
        },
        {
          id: 'params',
          title: '主要参数',
          blocks: [
            {
              type: 'table',
              headers: ['字段', '类型', '必填', '说明'],
              rows: [
                ['contents', 'array', '是', '对话内容，每项含 role（user／model）与 parts（text、inlineData 等）'],
                ['systemInstruction', 'object', '否', '系统指令，如 {"parts":[{"text":"…"}]}'],
                ['generationConfig', 'object', '否', '生成配置：temperature、maxOutputTokens、topP、thinkingConfig、responseMimeType 等'],
                ['tools / toolConfig', 'array / object', '否', '函数调用、Google 搜索等工具配置'],
                ['safetySettings', 'array', '否', '安全过滤配置']
              ]
            },
            { type: 'p', text: '模型名写在 URL 路径中，字段语义与 Google Gemini API 官方文档一致，网关仅按账号配置改写模型名。' }
          ]
        },
        {
          id: 'examples',
          title: '示例',
          blocks: [
            {
              type: 'example',
              id: 'generate-content',
              title: 'generateContent',
              description: '非流式生成，返回 candidates 与 usageMetadata。',
              request: `curl '{{BASE_URL}}/v1beta/models/gemini-2.5-flash:generateContent' \\
  -H 'x-goog-api-key: {{API_KEY}}' \\
  -H 'Content-Type: application/json' \\
  -d '{"systemInstruction":{"parts":[{"text":"你是一个简洁的助手。"}]},"contents":[{"role":"user","parts":[{"text":"用一句话介绍你自己"}]}],"generationConfig":{"temperature":0.7,"maxOutputTokens":1024}}'`,
              responses: [
                {
                  title: '成功',
                  status: 200,
                  body: '{"candidates":[{"content":{"role":"model","parts":[{"text":"我是 Gemini，一个可以帮你回答问题、写作和编程的 AI 助手。"}]},"finishReason":"STOP","index":0}],"usageMetadata":{"promptTokenCount":16,"candidatesTokenCount":20,"totalTokenCount":36},"modelVersion":"gemini-2.5-flash"}'
                }
              ]
            },
            {
              type: 'example',
              id: 'stream-generate-content',
              title: 'streamGenerateContent 流式',
              description: '流式生成，每个 data: 行是一段 GenerateContentResponse，最后一段携带 finishReason 与 usageMetadata。',
              request: `curl -N '{{BASE_URL}}/v1beta/models/gemini-2.5-flash:streamGenerateContent?alt=sse' \\
  -H 'x-goog-api-key: {{API_KEY}}' \\
  -H 'Content-Type: application/json' \\
  -d '{"contents":[{"role":"user","parts":[{"text":"你好"}]}]}'`,
              responses: [
                {
                  title: '事件流',
                  status: 200,
                  description: 'Content-Type: text/event-stream。',
                  body: `data: {"candidates":[{"content":{"role":"model","parts":[{"text":"你好！"}]},"index":0}],"modelVersion":"gemini-2.5-flash"}

data: {"candidates":[{"content":{"role":"model","parts":[{"text":"有什么可以帮你？"}]},"finishReason":"STOP","index":0}],"usageMetadata":{"promptTokenCount":4,"candidatesTokenCount":9,"totalTokenCount":13},"modelVersion":"gemini-2.5-flash"}`
                }
              ]
            },
            {
              type: 'example',
              id: 'list-models',
              title: '模型列表',
              description: '返回 Gemini 格式的模型列表；已开启模型白名单的分组只返回白名单内的模型。',
              request: `curl '{{BASE_URL}}/v1beta/models' \\
  -H 'x-goog-api-key: {{API_KEY}}'`,
              responses: [
                {
                  title: '成功',
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
      title: '错误码',
      blocks: [
        {
          type: 'table',
          headers: ['状态码', '含义'],
          rows: [
            ['400', '参数非法（如缺少 model）；在 /v1 下用 ?key= 传 Key（api_key_in_query_deprecated）；非 Gemini 分组调用 /v1beta 接口'],
            ['401', '未提供 API Key 或 Key 无效（API_KEY_REQUIRED／INVALID_API_KEY）'],
            ['403', '余额 ≤ 0（INSUFFICIENT_BALANCE／billing_error）；Key 未分配分组；OpenAI 分组未开启 Messages 调度'],
            ['404', '模型不在分组可用范围（model_not_found）；当前分组不支持该接口（如 embeddings、count_tokens）'],
            ['413', '请求体超过大小限制'],
            ['429', '请求频率、并发或额度超限；带 Retry-After 时请按其等待后重试'],
            ['502', '上游返回错误或连接异常'],
            ['503', '分组内暂无可用账号（未配置、已禁用或繁忙），请稍后重试']
          ]
        },
        {
          type: 'p',
          text: '错误体格式随入口协议：Anthropic 入口为 {"type":"error","error":{"type","message"}}；OpenAI 入口为 {"error":{"type","message"}}（部分错误附带 code）；Gemini 入口为 {"error":{"code","message","status"}}。鉴权阶段（Key 缺失、无效、余额不足）的错误为 {"code","message"} 扁平结构。上游的原生错误尽量按原格式透传，请同时检查 HTTP 状态码与错误体。'
        }
      ]
    }
  ]
}

export default zh
