import type { DocsCategory } from '../types'

const en: DocsCategory = {
  id: 'image',
  category: 'Image Generation',
  beforeProtocols: [
    {
      id: 'image-overview',
      title: 'Overview',
      blocks: [
        {
          type: 'p',
          text: 'Image generation supports the OpenAI Images protocol (called synchronously or asynchronously) and the native Google Gemini protocol. Authentication matches text chat: the Authorization: Bearer header (x-goog-api-key is recommended for Gemini endpoints).'
        },
        {
          type: 'table',
          headers: ['Mode', 'Entry point', 'Groups', 'Best for'],
          rows: [
            ['OpenAI sync', 'POST /v1/images/generations, /v1/images/edits', 'OpenAI, Grok', 'Fast generations where the caller can hold a long connection'],
            ['OpenAI async', 'POST /v1/images/generations/async, /v1/images/edits/async', 'OpenAI, Grok', 'Slow generations or traffic through a CDN / proxy, avoiding long-connection timeouts (such as Cloudflare 524)'],
            ['Gemini native', 'POST /v1beta/models/{model}:generateContent', 'Gemini', 'gemini-*-image models, with mixed text and image output'],
            ['Batch images', 'POST /v1/images/batches', 'Gemini (when enabled for the group)', 'Large offline batches, see the Batch Images page']
          ]
        },
        {
          type: 'list',
          items: [
            'The group must allow image generation, otherwise requests return 403 "Image generation is not enabled for this group". Anthropic groups cannot generate images.',
            'Sync endpoints return only after the image is ready, which can take over a minute; connections through a CDN or reverse proxy may time out, so use the async endpoints there.',
            'Async tasks require object storage to be enabled on this site; otherwise the async endpoints return 404. Generated images are stored in object storage and returned as URLs.',
            'Every path above also has an alias without the /v1 prefix (such as /images/generations and /images/tasks/{task_id}).'
          ]
        },
        { type: 'link', to: '/async-tasks', text: 'You can also track async image progress and results on the Async tasks page' }
      ]
    },
    {
      id: 'image-billing',
      title: 'Billing & Balance',
      blocks: [
        {
          type: 'list',
          items: [
            'Charged per returned image, priced in 1K / 2K / 4K tiers by size; an unrecognized size counts as 2K. Channels configured for token billing charge by token usage instead.',
            'Native Gemini image generation is charged by the number of inline images actually returned in the response.',
            'A balance ≤ 0 returns 403 billing_error; an exhausted quota window or image concurrency limit returns 429. When Retry-After is present, wait that long.',
            'Async tasks follow the same billing rules as the sync endpoints. Submission only performs basic validation; insufficient balance, upstream failures and similar errors happen during background execution and show up as status: failed with the matching http_status and error.'
          ]
        }
      ]
    }
  ],
  protocolsId: 'image-protocols',
  protocolsTitle: 'Per-Protocol Integration',
  protocols: [
    {
      id: 'image-sync',
      name: 'OpenAI Sync',
      sections: [
        {
          id: 'endpoints',
          title: 'Endpoints',
          blocks: [
            {
              type: 'table',
              headers: ['Method', 'Path', 'Description'],
              rows: [
                ['POST', '/v1/images/generations', 'Text to image, JSON body'],
                ['POST', '/v1/images/edits', 'Image editing / image to image; upload images as multipart/form-data, or pass image URLs in JSON']
              ]
            },
            { type: 'p', text: 'OpenAI and Grok groups only; other groups get 404 "Images API is not supported for this platform". Requests and responses keep the OpenAI Images format; the gateway only rewrites the model name per account configuration and passes other fields through.' }
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
                ['model', 'string', 'No for OpenAI groups / Yes for Grok groups', 'Image model; must be a gpt-image-*, grok-imagine* or gemini-*-image* model, otherwise 400. OpenAI groups default to gpt-image-2 when omitted'],
                ['prompt', 'string', 'Yes', 'Image description'],
                ['n', 'integer', 'No', 'Number of images; must be a positive integer. Billed by the number actually returned'],
                ['size', 'string', 'No', 'Size such as 1024x1024, 1536x1024 or auto; determines the billing tier. Grok groups convert it to resolution and aspect_ratio'],
                ['quality / background / output_format', 'string', 'No', 'Quality, background (such as transparent) and output format (png / jpeg / webp), applied as the upstream supports'],
                ['response_format', 'string', 'No', 'b64_json (default) or url; for some accounts url is a data:image/…;base64 Data URL rather than an http link'],
                ['stream / partial_images', 'boolean / integer', 'No', 'stream: true returns preview and final images over SSE; partial_images sets the number of previews'],
                ['image / image[] / mask', 'file', 'image required for edits', 'Multipart fields for edits: source images (several allowed, up to 3 for Grok) and an optional mask. The JSON form uses images[].image_url and mask.image_url; file_id is not supported']
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
              id: 'generations',
              title: 'Text to image',
              description: 'Generate an image synchronously; the response carries the image as base64.',
              request: `curl '{{BASE_URL}}/v1/images/generations' \\
  -H 'Authorization: Bearer {{API_KEY}}' \\
  -H 'Content-Type: application/json' \\
  -d '{"model":"gpt-image-2","prompt":"A lighthouse during a winter storm, cinematic","size":"1536x1024","quality":"high","n":1}'`,
              responses: [
                {
                  title: 'Success',
                  status: 200,
                  body: '{"created":1784092923,"data":[{"b64_json":"<BASE64_IMAGE>","revised_prompt":"A lighthouse during a winter storm, cinematic composition"}],"size":"1536x1024","quality":"high","output_format":"png","usage":{"input_tokens":32,"output_tokens":6240,"total_tokens":6272}}'
                }
              ]
            },
            {
              type: 'example',
              id: 'edits',
              title: 'Image editing',
              description: 'Upload the source image (and an optional mask) as multipart/form-data and edit it with a prompt.',
              request: `curl '{{BASE_URL}}/v1/images/edits' \\
  -H 'Authorization: Bearer {{API_KEY}}' \\
  -F 'model=gpt-image-2' \\
  -F 'prompt=Replace the background with a seaside sunset' \\
  -F 'image=@input.png' \\
  -F 'mask=@mask.png'`,
              responses: [
                {
                  title: 'Success',
                  status: 200,
                  body: '{"created":1784092923,"data":[{"b64_json":"<BASE64_IMAGE>"}],"usage":{"input_tokens":1200,"output_tokens":4160,"total_tokens":5360}}'
                }
              ]
            },
            {
              type: 'example',
              id: 'stream',
              title: 'Streaming generation',
              description: 'Set stream: true to receive partial_image preview events followed by a completed event; edit events use the image_edit prefix.',
              request: `curl -N '{{BASE_URL}}/v1/images/generations' \\
  -H 'Authorization: Bearer {{API_KEY}}' \\
  -H 'Content-Type: application/json' \\
  -d '{"model":"gpt-image-2","prompt":"A lighthouse during a winter storm","stream":true,"partial_images":1}'`,
              responses: [
                {
                  title: 'Event stream',
                  status: 200,
                  description: 'Content-Type: text/event-stream.',
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
      name: 'OpenAI Async',
      sections: [
        {
          id: 'endpoints',
          title: 'Endpoints',
          blocks: [
            {
              type: 'table',
              headers: ['Method', 'Path', 'Description'],
              rows: [
                ['POST', '/v1/images/generations/async', 'Submit a text-to-image task with the same body as the sync endpoint; returns 202'],
                ['POST', '/v1/images/edits/async', 'Submit an image-editing task, multipart or JSON; returns 202'],
                ['GET', '/v1/images/tasks/{task_id}', 'Get task status and result']
              ]
            },
            {
              type: 'list',
              items: [
                'OpenAI and Grok groups only (Composite groups cannot submit async tasks). When object storage is not enabled on this site the endpoints return 404 "async image tasks are not enabled".',
                'Async tasks do not support stream: true; such submissions return 400.',
                'Poll with the same API key that submitted the task; other keys and unknown tasks both get 404 IMAGE_TASK_NOT_FOUND.',
                'Submit and poll responses carry Cache-Control: no-store; in-progress responses carry Retry-After: 3, so poll every 3 seconds.',
                'A task runs for at most 30 minutes and fails on timeout (http_status 504); tasks and results are kept for 24 hours after their latest state update.'
              ]
            }
          ]
        },
        {
          id: 'fields',
          title: 'Task Fields',
          blocks: [
            {
              type: 'table',
              headers: ['Field', 'Type', 'Description'],
              rows: [
                ['id / task_id', 'string', 'Task ID such as imgtask_…; both fields hold the same value'],
                ['object', 'string', 'Always image.generation.task (edit tasks included)'],
                ['status', 'string', 'processing, completed or failed'],
                ['poll_url', 'string', 'Submit response only; the polling path, same as the Location header'],
                ['http_status', 'integer', 'Present in terminal states; the HTTP status of the equivalent sync call'],
                ['result', 'object', 'Present on success, shaped like the sync response; images have been stored, so data[].url points to object storage and b64_json is removed'],
                ['image_url', 'string', 'Present on success; equals the first data[].url for simple clients'],
                ['error', 'object', 'Present on failure; an OpenAI-compatible error object (type, message)'],
                ['created_at / completed_at / expires_at', 'integer', 'Unix timestamps in seconds: creation, completion and expiry']
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
              id: 'submit',
              title: 'Submit a text-to-image task',
              description: 'Same body as the sync endpoint; returns the task ID and polling path immediately.',
              request: `curl -i '{{BASE_URL}}/v1/images/generations/async' \\
  -H 'Authorization: Bearer {{API_KEY}}' \\
  -H 'Content-Type: application/json' \\
  -d '{"model":"gpt-image-2","prompt":"A lighthouse during a winter storm, cinematic","size":"1536x1024"}'`,
              responses: [
                {
                  title: 'Accepted',
                  status: 202,
                  description: 'Response headers include Location: /v1/images/tasks/{task_id} and Retry-After: 3.',
                  body: '{"id":"imgtask_0123456789abcdef","task_id":"imgtask_0123456789abcdef","object":"image.generation.task","status":"processing","created_at":1784092800,"expires_at":1784179200,"poll_url":"/v1/images/tasks/imgtask_0123456789abcdef"}'
                }
              ]
            },
            {
              type: 'example',
              id: 'edit-submit',
              title: 'Submit an image-editing task',
              description: 'Upload the source image as multipart/form-data, just like the sync edit endpoint.',
              request: `curl -i '{{BASE_URL}}/v1/images/edits/async' \\
  -H 'Authorization: Bearer {{API_KEY}}' \\
  -F 'model=gpt-image-2' \\
  -F 'prompt=Replace the background with a seaside sunset' \\
  -F 'image=@input.png'`,
              responses: [
                {
                  title: 'Accepted',
                  status: 202,
                  body: '{"id":"imgtask_fedcba9876543210","task_id":"imgtask_fedcba9876543210","object":"image.generation.task","status":"processing","created_at":1784092800,"expires_at":1784179200,"poll_url":"/v1/images/tasks/imgtask_fedcba9876543210"}'
                }
              ]
            },
            {
              type: 'example',
              id: 'query',
              title: 'Get a task',
              description: 'Poll with the same API key until status becomes completed or failed.',
              request: `curl '{{BASE_URL}}/v1/images/tasks/imgtask_0123456789abcdef' \\
  -H 'Authorization: Bearer {{API_KEY}}'`,
              responses: [
                {
                  title: 'Processing',
                  status: 200,
                  body: '{"id":"imgtask_0123456789abcdef","task_id":"imgtask_0123456789abcdef","object":"image.generation.task","status":"processing","created_at":1784092800,"expires_at":1784179200}'
                },
                {
                  title: 'Completed',
                  status: 200,
                  body: '{"id":"imgtask_0123456789abcdef","task_id":"imgtask_0123456789abcdef","object":"image.generation.task","status":"completed","http_status":200,"image_url":"https://images.example.com/images/imgtask_0123456789abcdef-0.png","result":{"created":1784092923,"data":[{"url":"https://images.example.com/images/imgtask_0123456789abcdef-0.png"}]},"created_at":1784092800,"completed_at":1784092923,"expires_at":1784179323}'
                },
                {
                  title: 'Failed',
                  status: 200,
                  description: 'The poll itself returns 200; the failure reason is in http_status and error.',
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
          title: 'Endpoints',
          blocks: [
            {
              type: 'table',
              headers: ['Method', 'Path', 'Description'],
              rows: [
                ['POST', '/v1beta/models/{model}:generateContent', 'Generate or edit images with gemini-*-image models'],
                ['POST', '/v1beta/models/{model}:streamGenerateContent?alt=sse', 'Streaming; images arrive as inlineData in the data chunks']
              ]
            },
            {
              type: 'list',
              items: [
                'Gemini groups only; other groups get 400 "API key group platform is not gemini".',
                'Requests whose model matches gemini-*-image*, or whose generationConfig.responseModalities includes IMAGE, are treated as image generation (subject to the group\'s image generation switch).',
                'The Imagen :predict endpoint is not supported and returns 404.',
                'Generated images are returned as base64 in candidates[].content.parts[].inlineData; the gateway does not store them.'
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
                ['contents', 'array', 'Yes', 'Prompt and reference images: text parts carry text, inlineData parts (mimeType + base64 data) carry input images'],
                ['generationConfig.responseModalities', 'array', 'No', 'Output modalities such as ["TEXT","IMAGE"]; set it explicitly to make sure images are returned'],
                ['generationConfig.imageConfig', 'object', 'No', 'Image settings such as aspectRatio (16:9 and others) and imageSize (1K / 2K / 4K, model dependent)']
              ]
            },
            { type: 'p', text: 'Fields follow the official Google Gemini API; the gateway only rewrites the model name per account configuration.' }
          ]
        },
        {
          id: 'examples',
          title: 'Examples',
          blocks: [
            {
              type: 'example',
              id: 'text-to-image',
              title: 'Text to image',
              description: 'Ask for mixed text and image output; the response parts contain both.',
              request: `curl '{{BASE_URL}}/v1beta/models/gemini-2.5-flash-image:generateContent' \\
  -H 'x-goog-api-key: {{API_KEY}}' \\
  -H 'Content-Type: application/json' \\
  -d '{"contents":[{"role":"user","parts":[{"text":"An orange cat wearing an astronaut helmet, watercolor"}]}],"generationConfig":{"responseModalities":["TEXT","IMAGE"],"imageConfig":{"aspectRatio":"16:9"}}}'`,
              responses: [
                {
                  title: 'Success',
                  status: 200,
                  body: '{"candidates":[{"content":{"role":"model","parts":[{"text":"Here is your image."},{"inlineData":{"mimeType":"image/png","data":"<BASE64_IMAGE>"}}]},"finishReason":"STOP","index":0}],"usageMetadata":{"promptTokenCount":12,"candidatesTokenCount":1300,"totalTokenCount":1312},"modelVersion":"gemini-2.5-flash-image"}'
                }
              ]
            },
            {
              type: 'example',
              id: 'image-edit',
              title: 'Image editing',
              description: 'Send the text instruction and the source image as inlineData in the same parts array.',
              request: `curl '{{BASE_URL}}/v1beta/models/gemini-2.5-flash-image:generateContent' \\
  -H 'x-goog-api-key: {{API_KEY}}' \\
  -H 'Content-Type: application/json' \\
  -d '{"contents":[{"role":"user","parts":[{"text":"Replace the background with a seaside sunset"},{"inlineData":{"mimeType":"image/png","data":"<BASE64_IMAGE>"}}]}],"generationConfig":{"responseModalities":["IMAGE"]}}'`,
              responses: [
                {
                  title: 'Success',
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
      title: 'Error Codes',
      blocks: [
        {
          type: 'table',
          headers: ['Status', 'Meaning'],
          rows: [
            ['400', 'Invalid parameters: a non-image model, n not a positive integer, an edit without an image, an async submission with stream, and so on'],
            ['401', 'API key missing or invalid'],
            ['403', 'Balance ≤ 0 (billing_error); image generation not allowed for the group (permission_error); key not assigned to a group'],
            ['404', 'The group platform does not support the endpoint; async images not enabled; task missing or owned by another key (IMAGE_TASK_NOT_FOUND); unsupported Gemini actions such as :predict'],
            ['413', 'Request body exceeds the size limit'],
            ['429', 'Rate, quota window or image concurrency limit exceeded; when Retry-After is present, wait that long'],
            ['502', 'Upstream error or no image output; an async task failed to store its images (task failed, http_status 502)'],
            ['503', 'No account is currently available in the group; async task storage unavailable (IMAGE_TASK_UNAVAILABLE)'],
            ['504', 'An async task ran longer than 30 minutes (task failed, http_status 504, error.type timeout_error)']
          ]
        },
        {
          type: 'p',
          text: 'Sync endpoints return OpenAI-format errors {"error":{"type","message"}}. Errors raised while submitting or polling async tasks use {"error":{"type","code","message"}}, while execution failures appear as status, http_status and error in the poll result. Gemini endpoints return {"error":{"code","message","status"}}. Streaming generation reports failures as an event: error event.'
        }
      ]
    }
  ]
}

export default en
