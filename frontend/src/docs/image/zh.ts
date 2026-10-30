import type { DocsCategory } from '../types'

const zh: DocsCategory = {
  id: 'image',
  category: '图片生成',
  beforeProtocols: [
    {
      id: 'image-overview',
      title: '概览',
      blocks: [
        {
          type: 'p',
          text: '图片生成支持 OpenAI Images 协议（同步与异步两种调用方式）和 Google Gemini 原生协议。鉴权与文本对话相同，使用 Authorization: Bearer 请求头（Gemini 接口推荐 x-goog-api-key）。'
        },
        {
          type: 'table',
          headers: ['方式', '入口', '可用分组', '适用场景'],
          rows: [
            ['OpenAI 同步', 'POST /v1/images/generations、/v1/images/edits', 'OpenAI、Grok', '出图快、调用方能保持长连接'],
            ['OpenAI 异步', 'POST /v1/images/generations/async、/v1/images/edits/async', 'OpenAI、Grok', '出图慢或经过 CDN／代理，避免长连接超时（如 Cloudflare 524）'],
            ['Gemini 原生', 'POST /v1beta/models/{model}:generateContent', 'Gemini', '使用 gemini-*-image 系列模型，可图文混合输出'],
            ['批量生图', 'POST /v1/images/batches', 'Gemini（需分组开启）', '大批量离线出图，见「批量生图」页']
          ]
        },
        {
          type: 'list',
          items: [
            '分组需允许生图，否则返回 403「Image generation is not enabled for this group」；Anthropic 分组不支持生图。',
            '同步接口要等图片生成完才返回，单次可能需要一分钟以上；经过 CDN 或反向代理时容易超时断开，此时请使用异步接口。',
            '异步任务需站点开启对象存储，未开启时异步接口返回 404；生成的图片会转存到对象存储并以 URL 返回。',
            '以上路径同时提供不带 /v1 前缀的别名（如 /images/generations、/images/tasks/{task_id}）。'
          ]
        },
        { type: 'link', to: '/async-tasks', text: '也可以在「异步任务」页查看异步生图的进度和结果' }
      ]
    },
    {
      id: 'image-billing',
      title: '计费与余额',
      blocks: [
        {
          type: 'list',
          items: [
            '按实际返回的图片张数计费，按尺寸分 1K／2K／4K 三档单价，无法识别尺寸时按 2K 计；渠道配置为按 token 计费时按 token 用量计费。',
            'Gemini 原生生图按响应中实际返回的内联图片张数计费。',
            '余额 ≤ 0 返回 403 billing_error；额度窗口超限或生图并发超限返回 429，带 Retry-After 时请按其等待。',
            '异步任务与同步接口计费规则相同。提交时只做基本校验，余额不足、上游失败等错误在后台执行时产生，体现为任务 status: failed 及对应的 http_status 与 error。'
          ]
        }
      ]
    }
  ],
  protocolsId: 'image-protocols',
  protocolsTitle: '按协议接入',
  protocols: [
    {
      id: 'image-sync',
      name: 'OpenAI 同步',
      sections: [
        {
          id: 'endpoints',
          title: '接口',
          blocks: [
            {
              type: 'table',
              headers: ['方法', '路径', '说明'],
              rows: [
                ['POST', '/v1/images/generations', '文生图，JSON 请求体'],
                ['POST', '/v1/images/edits', '图片编辑／图生图，multipart/form-data 上传图片，或 JSON 传图片 URL']
              ]
            },
            { type: 'p', text: '仅 OpenAI 与 Grok 分组可用，其它分组返回 404「Images API is not supported for this platform」。请求与响应保持 OpenAI Images 格式，网关只按账号配置改写模型名，其它字段透传。' }
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
                ['model', 'string', 'OpenAI 分组否／Grok 分组是', '生图模型，须为 gpt-image-*、grok-imagine* 或 gemini-*-image* 系列，否则返回 400；OpenAI 分组未填写时默认 gpt-image-2'],
                ['prompt', 'string', '是', '图片描述'],
                ['n', 'integer', '否', '生成张数，须为正整数，按实际返回张数计费'],
                ['size', 'string', '否', '尺寸，如 1024x1024、1536x1024、auto；决定计费档位。Grok 分组会转换为 resolution 与 aspect_ratio'],
                ['quality / background / output_format', 'string', '否', '画质、背景（如 transparent）、输出格式（png／jpeg／webp），按上游支持情况生效'],
                ['response_format', 'string', '否', 'b64_json（默认）或 url；部分账号返回的 url 是 data:image/…;base64 形式的 Data URL，而非 http 链接'],
                ['stream / partial_images', 'boolean / integer', '否', 'stream: true 时以 SSE 返回预览图与最终图，partial_images 控制预览张数'],
                ['image / image[] / mask', 'file', 'edits 必填 image', 'edits 的 multipart 字段：待编辑图片（可多张，Grok 最多 3 张）与可选蒙版；JSON 写法使用 images[].image_url 与 mask.image_url，不支持 file_id']
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
              id: 'generations',
              title: '文生图',
              description: '同步生成图片，响应中直接包含图片的 base64 数据。',
              request: `curl '{{BASE_URL}}/v1/images/generations' \\
  -H 'Authorization: Bearer {{API_KEY}}' \\
  -H 'Content-Type: application/json' \\
  -d '{"model":"gpt-image-2","prompt":"冬日暴风雪中的灯塔，电影感","size":"1536x1024","quality":"high","n":1}'`,
              responses: [
                {
                  title: '成功',
                  status: 200,
                  body: '{"created":1784092923,"data":[{"b64_json":"<BASE64_IMAGE>","revised_prompt":"冬日暴风雪中的灯塔，电影感构图"}],"size":"1536x1024","quality":"high","output_format":"png","usage":{"input_tokens":32,"output_tokens":6240,"total_tokens":6272}}'
                }
              ]
            },
            {
              type: 'example',
              id: 'edits',
              title: '图片编辑',
              description: '以 multipart/form-data 上传原图（可附加蒙版），按提示词编辑。',
              request: `curl '{{BASE_URL}}/v1/images/edits' \\
  -H 'Authorization: Bearer {{API_KEY}}' \\
  -F 'model=gpt-image-2' \\
  -F 'prompt=把背景换成海边日落' \\
  -F 'image=@input.png' \\
  -F 'mask=@mask.png'`,
              responses: [
                {
                  title: '成功',
                  status: 200,
                  body: '{"created":1784092923,"data":[{"b64_json":"<BASE64_IMAGE>"}],"usage":{"input_tokens":1200,"output_tokens":4160,"total_tokens":5360}}'
                }
              ]
            },
            {
              type: 'example',
              id: 'stream',
              title: '流式生图',
              description: '设置 stream: true，先返回 partial_image 预览事件，最后返回 completed 事件；图片编辑的事件名前缀为 image_edit。',
              request: `curl -N '{{BASE_URL}}/v1/images/generations' \\
  -H 'Authorization: Bearer {{API_KEY}}' \\
  -H 'Content-Type: application/json' \\
  -d '{"model":"gpt-image-2","prompt":"冬日暴风雪中的灯塔","stream":true,"partial_images":1}'`,
              responses: [
                {
                  title: '事件流',
                  status: 200,
                  description: 'Content-Type: text/event-stream。',
                  body: `event: image_generation.partial_image
data: {"type":"image_generation.partial_image","created_at":1784092900,"partial_image_index":0,"b64_json":"<BASE64_PREVIEW>"}

event: image_generation.completed
data: {"type":"image_generation.completed","created_at":1784092900,"b64_json":"<BASE64_IMAGE>","output_format":"png","usage":{"input_tokens":32,"output_tokens":6240,"total_tokens":6272}}`
                }
              ]
            }
          ]
        }
      ]
    },
    {
      id: 'image-async',
      name: 'OpenAI 异步',
      sections: [
        {
          id: 'endpoints',
          title: '接口',
          blocks: [
            {
              type: 'table',
              headers: ['方法', '路径', '说明'],
              rows: [
                ['POST', '/v1/images/generations/async', '提交文生图任务，请求体与同步接口相同，返回 202'],
                ['POST', '/v1/images/edits/async', '提交图片编辑任务，支持 multipart 与 JSON，返回 202'],
                ['GET', '/v1/images/tasks/{task_id}', '查询任务状态与结果']
              ]
            },
            {
              type: 'list',
              items: [
                '仅 OpenAI 与 Grok 分组可用（Composite 分组不支持异步提交）；站点未开启对象存储时返回 404「async image tasks are not enabled」。',
                '异步任务不支持 stream: true，提交时返回 400。',
                '必须使用提交任务的同一个 API Key 查询；其它 Key 或不存在的任务都返回 404 IMAGE_TASK_NOT_FOUND。',
                '提交与查询响应都带 Cache-Control: no-store；处理中的响应带 Retry-After: 3，建议每 3 秒轮询一次。',
                '任务最长执行 30 分钟，超时按失败处理（http_status 504）；任务与结果在最后一次状态更新后保留 24 小时。'
              ]
            }
          ]
        },
        {
          id: 'fields',
          title: '任务字段',
          blocks: [
            {
              type: 'table',
              headers: ['字段', '类型', '说明'],
              rows: [
                ['id / task_id', 'string', '任务 ID，形如 imgtask_…，两个字段取值相同'],
                ['object', 'string', '固定为 image.generation.task（编辑任务同样如此）'],
                ['status', 'string', 'processing 处理中；completed 已完成；failed 失败'],
                ['poll_url', 'string', '仅提交响应返回，查询路径，与响应头 Location 相同'],
                ['http_status', 'integer', '终态时返回，对应同步调用的 HTTP 状态码'],
                ['result', 'object', '成功时返回，结构同同步接口响应；图片已转存，data[].url 为对象存储链接，b64_json 被移除'],
                ['image_url', 'string', '成功时返回，等于第一张图片的 data[].url，便于简单客户端直接使用'],
                ['error', 'object', '失败时返回，OpenAI 兼容错误对象（type、message）'],
                ['created_at / completed_at / expires_at', 'integer', 'Unix 秒级时间戳：创建、完成与过期时间']
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
              id: 'submit',
              title: '提交文生图任务',
              description: '请求体与同步接口相同，立即返回任务 ID 与轮询地址。',
              request: `curl -i '{{BASE_URL}}/v1/images/generations/async' \\
  -H 'Authorization: Bearer {{API_KEY}}' \\
  -H 'Content-Type: application/json' \\
  -d '{"model":"gpt-image-2","prompt":"冬日暴风雪中的灯塔，电影感","size":"1536x1024"}'`,
              responses: [
                {
                  title: '已接受',
                  status: 202,
                  description: '响应头含 Location: /v1/images/tasks/{task_id} 与 Retry-After: 3。',
                  body: '{"id":"imgtask_0123456789abcdef","task_id":"imgtask_0123456789abcdef","object":"image.generation.task","status":"processing","created_at":1784092800,"expires_at":1784179200,"poll_url":"/v1/images/tasks/imgtask_0123456789abcdef"}'
                }
              ]
            },
            {
              type: 'example',
              id: 'edit-submit',
              title: '提交图片编辑任务',
              description: '与同步编辑接口一样以 multipart/form-data 上传原图。',
              request: `curl -i '{{BASE_URL}}/v1/images/edits/async' \\
  -H 'Authorization: Bearer {{API_KEY}}' \\
  -F 'model=gpt-image-2' \\
  -F 'prompt=把背景换成海边日落' \\
  -F 'image=@input.png'`,
              responses: [
                {
                  title: '已接受',
                  status: 202,
                  body: '{"id":"imgtask_fedcba9876543210","task_id":"imgtask_fedcba9876543210","object":"image.generation.task","status":"processing","created_at":1784092800,"expires_at":1784179200,"poll_url":"/v1/images/tasks/imgtask_fedcba9876543210"}'
                }
              ]
            },
            {
              type: 'example',
              id: 'query',
              title: '查询任务',
              description: '使用同一个 API Key 轮询，直到 status 变为 completed 或 failed。',
              request: `curl '{{BASE_URL}}/v1/images/tasks/imgtask_0123456789abcdef' \\
  -H 'Authorization: Bearer {{API_KEY}}'`,
              responses: [
                {
                  title: '处理中',
                  status: 200,
                  body: '{"id":"imgtask_0123456789abcdef","task_id":"imgtask_0123456789abcdef","object":"image.generation.task","status":"processing","created_at":1784092800,"expires_at":1784179200}'
                },
                {
                  title: '已完成',
                  status: 200,
                  body: '{"id":"imgtask_0123456789abcdef","task_id":"imgtask_0123456789abcdef","object":"image.generation.task","status":"completed","http_status":200,"image_url":"https://images.example.com/images/imgtask_0123456789abcdef-0.png","result":{"created":1784092923,"data":[{"url":"https://images.example.com/images/imgtask_0123456789abcdef-0.png"}]},"created_at":1784092800,"completed_at":1784092923,"expires_at":1784179323}'
                },
                {
                  title: '失败',
                  status: 200,
                  description: '查询接口本身返回 200，任务失败原因见 http_status 与 error。',
                  body: '{"id":"imgtask_0123456789abcdef","task_id":"imgtask_0123456789abcdef","object":"image.generation.task","status":"failed","http_status":502,"error":{"type":"api_error","message":"Upstream request failed"},"created_at":1784092800,"completed_at":1784092923,"expires_at":1784179323}'
                }
              ]
            }
          ]
        }
      ]
    },
    {
      id: 'image-gemini',
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
                ['POST', '/v1beta/models/{model}:generateContent', '使用 gemini-*-image 系列模型生成或编辑图片'],
                ['POST', '/v1beta/models/{model}:streamGenerateContent?alt=sse', '流式返回，图片作为 inlineData 出现在数据块中']
              ]
            },
            {
              type: 'list',
              items: [
                '仅 Gemini 分组可用，其它分组返回 400「API key group platform is not gemini」。',
                '模型名匹配 gemini-*-image*，或 generationConfig.responseModalities 包含 IMAGE 时，按生图请求处理（受分组生图开关约束）。',
                '不支持 Imagen 的 :predict 接口，调用返回 404。',
                '生成的图片以 base64 形式出现在 candidates[].content.parts[].inlineData 中，网关不转存。'
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
                ['contents', 'array', '是', '提示词与参考图：parts 中 text 为文本，inlineData（mimeType + base64 data）为输入图片'],
                ['generationConfig.responseModalities', 'array', '否', '输出模态，如 ["TEXT","IMAGE"]；建议显式填写以确保返回图片'],
                ['generationConfig.imageConfig', 'object', '否', '图片配置，如 aspectRatio（16:9 等）、imageSize（1K／2K／4K，取决于模型）']
              ]
            },
            { type: 'p', text: '字段语义与 Google Gemini API 官方文档一致，网关仅按账号配置改写模型名。' }
          ]
        },
        {
          id: 'examples',
          title: '示例',
          blocks: [
            {
              type: 'example',
              id: 'text-to-image',
              title: '文生图',
              description: '请求图文混合输出，响应的 parts 中包含文本与图片。',
              request: `curl '{{BASE_URL}}/v1beta/models/gemini-2.5-flash-image:generateContent' \\
  -H 'x-goog-api-key: {{API_KEY}}' \\
  -H 'Content-Type: application/json' \\
  -d '{"contents":[{"role":"user","parts":[{"text":"一只戴宇航头盔的橘猫，水彩风格"}]}],"generationConfig":{"responseModalities":["TEXT","IMAGE"],"imageConfig":{"aspectRatio":"16:9"}}}'`,
              responses: [
                {
                  title: '成功',
                  status: 200,
                  body: '{"candidates":[{"content":{"role":"model","parts":[{"text":"这是为你生成的图片。"},{"inlineData":{"mimeType":"image/png","data":"<BASE64_IMAGE>"}}]},"finishReason":"STOP","index":0}],"usageMetadata":{"promptTokenCount":12,"candidatesTokenCount":1300,"totalTokenCount":1312},"modelVersion":"gemini-2.5-flash-image"}'
                }
              ]
            },
            {
              type: 'example',
              id: 'image-edit',
              title: '图片编辑',
              description: '在 parts 中同时传入文本指令与 inlineData 原图。',
              request: `curl '{{BASE_URL}}/v1beta/models/gemini-2.5-flash-image:generateContent' \\
  -H 'x-goog-api-key: {{API_KEY}}' \\
  -H 'Content-Type: application/json' \\
  -d '{"contents":[{"role":"user","parts":[{"text":"把背景换成海边日落"},{"inlineData":{"mimeType":"image/png","data":"<BASE64_IMAGE>"}}]}],"generationConfig":{"responseModalities":["IMAGE"]}}'`,
              responses: [
                {
                  title: '成功',
                  status: 200,
                  body: '{"candidates":[{"content":{"role":"model","parts":[{"inlineData":{"mimeType":"image/png","data":"<BASE64_IMAGE>"}}]},"finishReason":"STOP","index":0}],"usageMetadata":{"promptTokenCount":1300,"candidatesTokenCount":1290,"totalTokenCount":2590},"modelVersion":"gemini-2.5-flash-image"}'
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
      id: 'image-errors',
      title: '错误码',
      blocks: [
        {
          type: 'table',
          headers: ['状态码', '含义'],
          rows: [
            ['400', '参数非法：非生图模型、n 不是正整数、编辑请求缺少图片、异步提交带 stream 等'],
            ['401', '未提供 API Key 或 Key 无效'],
            ['403', '余额 ≤ 0（billing_error）；分组未允许生图（permission_error）；Key 未分配分组'],
            ['404', '分组平台不支持该接口；异步生图未开启；任务不存在或不属于当前 Key（IMAGE_TASK_NOT_FOUND）；Gemini :predict 等不支持的动作'],
            ['413', '请求体超过大小限制'],
            ['429', '请求频率、额度窗口或生图并发超限；带 Retry-After 时请按其等待'],
            ['502', '上游返回错误或无图片输出；异步任务转存对象存储失败（任务 failed，http_status 502）'],
            ['503', '分组内暂无可用账号；异步任务存储不可用（IMAGE_TASK_UNAVAILABLE）'],
            ['504', '异步任务执行超过 30 分钟（任务 failed，http_status 504，error.type 为 timeout_error）']
          ]
        },
        {
          type: 'p',
          text: '同步接口的错误为 OpenAI 格式 {"error":{"type","message"}}；异步接口在提交和查询阶段产生的错误为 {"error":{"type","code","message"}}，任务执行失败则体现在查询结果的 status、http_status 与 error 中；Gemini 接口为 {"error":{"code","message","status"}}。流式生图出错时会推送 event: error 事件。'
        }
      ]
    }
  ]
}

export default zh
