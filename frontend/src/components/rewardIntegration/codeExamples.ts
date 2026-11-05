// 「任务接入文档」代码示例：六种语言各一段可直接运行的「签名 + 调用 events」片段。
// 代码示例本身不翻译；示例中的本站地址用 __BASE_URL__ 占位，运行时替换为当前 origin。
// 签名算法（契约 §5.5）：hex_lower(HMAC_SHA256(app_secret, timestamp + "\n" + nonce + "\n" + raw_body))

export type RewardCodeExampleId = 'curl' | 'node' | 'python' | 'php' | 'go' | 'java'

export interface RewardCodeExample {
  id: RewardCodeExampleId
  /** Tab 文案的 i18n key */
  labelKey: string
  /** 运行方式说明的 i18n key */
  runKey: string
  /** 代码块右上角展示的文件名 */
  filename: string
  code: string
}

const CURL_TEMPLATE = String.raw`#!/usr/bin/env bash
# 依赖：bash + curl + openssl
set -euo pipefail

BASE="__BASE_URL__"
APP_KEY="rk_0123456789abcdef01234567"
APP_SECRET="rs_0123456789abcdef0123456789abcdef0123456789abcdef"

# 1) timestamp（Unix 秒）与 nonce（16~64 位 [A-Za-z0-9_-]）
TIMESTAMP="$(date +%s)"
NONCE="$(openssl rand -hex 16)"

# 2) 原始请求体：签名用的就是这串字节，发送时不要再重新序列化
BODY='{"event_id":"evt_20261105_0001","event_type":"register","ticket":"rt_0123456789abcdef0123456789abcdef01234567","external_user_id":"u_1001"}'

# 3) 待签名字符串 = timestamp + "\n" + nonce + "\n" + body
#    用 printf '%s\n%s\n%s' 拼接，避免 echo 在部分 shell 下多出或吃掉换行
SIGNATURE="$(printf '%s\n%s\n%s' "$TIMESTAMP" "$NONCE" "$BODY" \
  | openssl dgst -sha256 -hmac "$APP_SECRET" -hex \
  | sed 's/^.*= //')"

# 4) 调用 events（响应形如 {"code":0,"message":"success","data":{...}}）
curl -sS -X POST "$BASE/api/v1/reward-open/events" \
  -H 'Content-Type: application/json' \
  -H "X-Reward-App-Key: $APP_KEY" \
  -H "X-Reward-Timestamp: $TIMESTAMP" \
  -H "X-Reward-Nonce: $NONCE" \
  -H "X-Reward-Signature: $SIGNATURE" \
  --data-binary "$BODY"
`

const NODE_TEMPLATE = String.raw`// Node.js 18+（内置 fetch 与 node:crypto）；保存为 events.mjs 后执行：node events.mjs
import { createHmac, randomBytes } from 'node:crypto'

const BASE = '__BASE_URL__'
const APP_KEY = 'rk_0123456789abcdef01234567'
const APP_SECRET = 'rs_0123456789abcdef0123456789abcdef0123456789abcdef'

// 原始请求体：签名与发送必须使用同一份字符串
const body = JSON.stringify({
  event_id: 'evt_20261105_0001',
  event_type: 'register',
  ticket: 'rt_0123456789abcdef0123456789abcdef01234567',
  external_user_id: 'u_1001',
})

const timestamp = Math.floor(Date.now() / 1000).toString()
const nonce = randomBytes(16).toString('hex')

// 待签名字符串 = timestamp + "\n" + nonce + "\n" + body，HMAC-SHA256 后取小写 hex
const signature = createHmac('sha256', APP_SECRET)
  .update(timestamp + '\n' + nonce + '\n' + body, 'utf8')
  .digest('hex')

const res = await fetch(BASE + '/api/v1/reward-open/events', {
  method: 'POST',
  headers: {
    'Content-Type': 'application/json',
    'X-Reward-App-Key': APP_KEY,
    'X-Reward-Timestamp': timestamp,
    'X-Reward-Nonce': nonce,
    'X-Reward-Signature': signature,
  },
  body,
})

console.log(res.status, await res.text())
`

const PYTHON_TEMPLATE = String.raw`# Python 3.8+（仅标准库）；保存为 events.py 后执行：python3 events.py
import hashlib
import hmac
import json
import secrets
import time
import urllib.request

BASE = "__BASE_URL__"
APP_KEY = "rk_0123456789abcdef01234567"
APP_SECRET = "rs_0123456789abcdef0123456789abcdef0123456789abcdef"

# 原始请求体：去掉多余空格，保证签名与发送完全一致
body = json.dumps(
    {
        "event_id": "evt_20261105_0001",
        "event_type": "register",
        "ticket": "rt_0123456789abcdef0123456789abcdef01234567",
        "external_user_id": "u_1001",
    },
    separators=(",", ":"),
    ensure_ascii=False,
)

timestamp = str(int(time.time()))
nonce = secrets.token_hex(16)

# 待签名字符串 = timestamp + "\n" + nonce + "\n" + body
signing_string = timestamp + "\n" + nonce + "\n" + body
signature = hmac.new(
    APP_SECRET.encode("utf-8"),
    signing_string.encode("utf-8"),
    hashlib.sha256,
).hexdigest()  # 小写 hex

request = urllib.request.Request(
    BASE + "/api/v1/reward-open/events",
    data=body.encode("utf-8"),
    method="POST",
    headers={
        "Content-Type": "application/json",
        "X-Reward-App-Key": APP_KEY,
        "X-Reward-Timestamp": timestamp,
        "X-Reward-Nonce": nonce,
        "X-Reward-Signature": signature,
    },
)

with urllib.request.urlopen(request) as response:
    print(response.status, response.read().decode("utf-8"))
`

const PHP_TEMPLATE = String.raw`<?php
// PHP 8+（需要 curl 扩展）；保存为 events.php 后执行：php events.php
$base = '__BASE_URL__';
$appKey = 'rk_0123456789abcdef01234567';
$appSecret = 'rs_0123456789abcdef0123456789abcdef0123456789abcdef';

// 原始请求体：签名与发送必须使用同一份字符串
$body = json_encode([
    'event_id' => 'evt_20261105_0001',
    'event_type' => 'register',
    'ticket' => 'rt_0123456789abcdef0123456789abcdef01234567',
    'external_user_id' => 'u_1001',
], JSON_UNESCAPED_SLASHES | JSON_UNESCAPED_UNICODE);

$timestamp = (string) time();
$nonce = bin2hex(random_bytes(16));

// 待签名字符串 = timestamp + "\n" + nonce + "\n" + body
// hash_hmac() 第四个参数默认 false，返回的就是小写 hex
$signature = hash_hmac('sha256', $timestamp . "\n" . $nonce . "\n" . $body, $appSecret);

$ch = curl_init($base . '/api/v1/reward-open/events');
curl_setopt_array($ch, [
    CURLOPT_POST => true,
    CURLOPT_POSTFIELDS => $body,
    CURLOPT_RETURNTRANSFER => true,
    CURLOPT_HTTPHEADER => [
        'Content-Type: application/json',
        'X-Reward-App-Key: ' . $appKey,
        'X-Reward-Timestamp: ' . $timestamp,
        'X-Reward-Nonce: ' . $nonce,
        'X-Reward-Signature: ' . $signature,
    ],
]);

$response = curl_exec($ch);
if ($response === false) {
    fwrite(STDERR, curl_error($ch) . PHP_EOL);
    exit(1);
}
$status = curl_getinfo($ch, CURLINFO_HTTP_CODE);
curl_close($ch);

echo $status . PHP_EOL . $response . PHP_EOL;
`

const GO_TEMPLATE = String.raw`// Go 1.18+；保存为 main.go 后执行：go run main.go
package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

const (
	baseURL   = "__BASE_URL__"
	appKey    = "rk_0123456789abcdef01234567"
	appSecret = "rs_0123456789abcdef0123456789abcdef0123456789abcdef"
)

func main() {
	// 原始请求体：签名与发送必须使用同一份字节
	body := []byte("{\"event_id\":\"evt_20261105_0001\",\"event_type\":\"register\",\"ticket\":\"rt_0123456789abcdef0123456789abcdef01234567\",\"external_user_id\":\"u_1001\"}")

	timestamp := strconv.FormatInt(time.Now().Unix(), 10)

	nonceBytes := make([]byte, 16)
	if _, err := rand.Read(nonceBytes); err != nil {
		panic(err)
	}
	nonce := hex.EncodeToString(nonceBytes)

	// 待签名字符串 = timestamp + "\n" + nonce + "\n" + body
	mac := hmac.New(sha256.New, []byte(appSecret))
	mac.Write([]byte(timestamp + "\n" + nonce + "\n" + string(body)))
	signature := hex.EncodeToString(mac.Sum(nil)) // 小写 hex

	req, err := http.NewRequest(http.MethodPost, baseURL+"/api/v1/reward-open/events", bytes.NewReader(body))
	if err != nil {
		panic(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Reward-App-Key", appKey)
	req.Header.Set("X-Reward-Timestamp", timestamp)
	req.Header.Set("X-Reward-Nonce", nonce)
	req.Header.Set("X-Reward-Signature", signature)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}
	fmt.Println(resp.Status)
	fmt.Println(string(respBody))
}
`

const JAVA_TEMPLATE = String.raw`// Java 17+（HexFormat）；保存为 RewardEventsCall.java 后执行：java RewardEventsCall.java
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.nio.charset.StandardCharsets;
import java.security.SecureRandom;
import java.time.Instant;
import java.util.HexFormat;
import javax.crypto.Mac;
import javax.crypto.spec.SecretKeySpec;

public class RewardEventsCall {
    public static void main(String[] args) throws Exception {
        String base = "__BASE_URL__";
        String appKey = "rk_0123456789abcdef01234567";
        String appSecret = "rs_0123456789abcdef0123456789abcdef0123456789abcdef";

        // 原始请求体：签名与发送必须使用同一份字符串
        String body = "{\"event_id\":\"evt_20261105_0001\",\"event_type\":\"register\",\"ticket\":\"rt_0123456789abcdef0123456789abcdef01234567\",\"external_user_id\":\"u_1001\"}";

        String timestamp = String.valueOf(Instant.now().getEpochSecond());

        byte[] nonceBytes = new byte[16];
        new SecureRandom().nextBytes(nonceBytes);
        String nonce = HexFormat.of().formatHex(nonceBytes);

        // 待签名字符串 = timestamp + "\n" + nonce + "\n" + body
        String signingString = timestamp + "\n" + nonce + "\n" + body;
        Mac mac = Mac.getInstance("HmacSHA256");
        mac.init(new SecretKeySpec(appSecret.getBytes(StandardCharsets.UTF_8), "HmacSHA256"));
        String signature = HexFormat.of().formatHex(mac.doFinal(signingString.getBytes(StandardCharsets.UTF_8)));

        HttpRequest request = HttpRequest.newBuilder()
            .uri(URI.create(base + "/api/v1/reward-open/events"))
            .header("Content-Type", "application/json")
            .header("X-Reward-App-Key", appKey)
            .header("X-Reward-Timestamp", timestamp)
            .header("X-Reward-Nonce", nonce)
            .header("X-Reward-Signature", signature)
            .POST(HttpRequest.BodyPublishers.ofString(body, StandardCharsets.UTF_8))
            .build();

        HttpResponse<String> response = HttpClient.newHttpClient()
            .send(request, HttpResponse.BodyHandlers.ofString(StandardCharsets.UTF_8));

        System.out.println(response.statusCode());
        System.out.println(response.body());
    }
}
`

const TEMPLATES: Array<{ id: RewardCodeExampleId, template: string, filename: string }> = [
  { id: 'curl', template: CURL_TEMPLATE, filename: 'events.sh' },
  { id: 'node', template: NODE_TEMPLATE, filename: 'events.mjs' },
  { id: 'python', template: PYTHON_TEMPLATE, filename: 'events.py' },
  { id: 'php', template: PHP_TEMPLATE, filename: 'events.php' },
  { id: 'go', template: GO_TEMPLATE, filename: 'main.go' },
  { id: 'java', template: JAVA_TEMPLATE, filename: 'RewardEventsCall.java' },
]

export function buildRewardCodeExamples(origin: string): RewardCodeExample[] {
  const base = origin.replace(/\/+$/, '')
  return TEMPLATES.map(({ id, template, filename }) => ({
    id,
    labelKey: `rewardIntegration.examples.langs.${id}`,
    runKey: `rewardIntegration.examples.run.${id}`,
    filename,
    code: template.split('__BASE_URL__').join(base),
  }))
}
