(function () {
  'use strict';

  if (window.RewardTaskSDK) return;

  var state = {
    status: 'idle',
    paused_reason: null,
    progress_seconds: 0,
    browse_seconds: 0,
    completed: false,
    error: null
  };
  var listeners = [];

  function getState() {
    return Object.assign({}, state);
  }

  window.RewardTaskSDK = {
    getState: getState,
    onChange: function (callback) {
      if (typeof callback !== 'function') return function () {};
      listeners.push(callback);
      return function () {
        listeners = listeners.filter(function (listener) { return listener !== callback; });
      };
    }
  };

  var script = document.currentScript;
  if (!script) {
    var scripts = document.querySelectorAll('script[src]');
    for (var index = scripts.length - 1; index >= 0; index -= 1) {
      if (scripts[index].src.indexOf('reward-task-sdk.js') !== -1) {
        script = scripts[index];
        break;
      }
    }
  }
  if (!script) return;

  var storageKey = 'reward_task_ticket';
  var ticket = '';
  var url = new URL(window.location.href);
  var queryTicket = url.searchParams.get('reward_ticket');
  if (queryTicket) {
    ticket = queryTicket;
    try { window.sessionStorage.setItem(storageKey, ticket); } catch (_) {
      // 存储被禁用时仍使用当前页面的票据。
    }
    url.searchParams.delete('reward_ticket');
    try { window.history.replaceState(window.history.state, '', url.pathname + url.search + url.hash); } catch (_) {
      // 地址栏更新失败不影响任务心跳。
    }
  } else {
    try { ticket = window.sessionStorage.getItem(storageKey) || ''; } catch (_) {
      // 无可读票据时保持空值，跳过初始化。
    }
  }

  var appKey = script.getAttribute('data-app-key') || '';
  if (!ticket || !appKey || typeof window.fetch !== 'function') return;
  var base = new URL(script.src, window.location.href).origin;
  var language = script.getAttribute('data-lang') || window.navigator.language || 'en';
  var chinese = /^zh/i.test(language);
  var badgeEnabled = script.getAttribute('data-badge') !== 'false';
  var badgeClosed = false;
  var badge = null;
  var badgeText = null;
  var timer = null;
  var stopped = false;
  var inFlight = false;
  var idleTimeout = 15;
  var lastActivity = Date.now();
  var visibility = document.visibilityState;
  var nextBeat = Date.now();
  var retryAt = 0;
  var failures = 0;
  var activityEvents = ['mousemove', 'mousedown', 'pointerdown', 'keydown', 'wheel', 'touchstart', 'touchmove', 'scroll'];
  var terminalReasons = [
    'REWARD_TICKET_NOT_FOUND',
    'REWARD_TICKET_EXPIRED',
    'REWARD_TICKET_PARTNER_MISMATCH',
    'REWARD_EVENT_TYPE_MISMATCH',
    'REWARD_ORIGIN_NOT_ALLOWED',
    'REWARD_PARTNER_INVALID',
    'REWARD_TASKS_DISABLED',
    'REWARD_TASK_UNAVAILABLE',
    'REWARD_TASK_ALREADY_COMPLETED',
    'REWARD_TASK_QUOTA_EXHAUSTED'
  ];

  function errorText(reason) {
    if (reason === 'REWARD_TICKET_EXPIRED' || reason === 'REWARD_TICKET_NOT_FOUND') {
      return chinese ? '任务票据已失效，请返回任务中心重新开始。' : 'Task ticket expired. Please restart from the task center.';
    }
    if (reason === 'REWARD_TASK_ALREADY_COMPLETED') {
      return chinese ? '本次任务已完成，请返回任务中心查看。' : 'Task already completed. Check the task center.';
    }
    if (reason === 'REWARD_TASK_QUOTA_EXHAUSTED') {
      return chinese ? '任务奖励名额已满。' : 'All task rewards have been claimed.';
    }
    if (terminalReasons.indexOf(reason) !== -1) {
      return chinese ? '任务暂不可用，请返回任务中心查看。' : 'Task unavailable. Please check the task center.';
    }
    return chinese ? '连接暂时中断，正在重试…' : 'Connection interrupted. Retrying…';
  }

  function renderBadge() {
    if (!badgeEnabled || badgeClosed || !document.body) return;
    document.removeEventListener('DOMContentLoaded', renderBadge);
    if (!badge) {
      badge = document.createElement('div');
      badge.setAttribute('data-reward-task-badge', '');
      badge.setAttribute('role', 'status');
      badge.setAttribute('aria-live', 'polite');
      badge.lang = chinese ? 'zh' : 'en';
      badge.style.cssText = 'position:fixed;right:16px;bottom:16px;z-index:2147483000;box-sizing:border-box;display:flex;align-items:flex-start;gap:12px;max-width:calc(100vw - 32px);width:300px;padding:14px 16px;border:1px solid #334155;border-radius:12px;background:#152235;color:#f8fafc;box-shadow:0 8px 28px #0003;font:14px/1.5 sans-serif;text-align:left;';
      badgeText = document.createElement('span');
      badgeText.style.cssText = 'flex:1;min-width:0;overflow-wrap:anywhere;';
      var close = document.createElement('button');
      close.type = 'button';
      close.textContent = '×';
      close.setAttribute('aria-label', chinese ? '关闭任务进度' : 'Close task progress');
      close.style.cssText = 'flex:none;display:block;min-width:24px;min-height:24px;margin:-2px -4px 0 0;padding:0;border:0;border-radius:4px;background:transparent;color:inherit;cursor:pointer;font:20px/24px sans-serif;';
      close.addEventListener('click', function () {
        badgeClosed = true;
        badge.remove();
        badge = null;
        badgeText = null;
      });
      badge.appendChild(badgeText);
      badge.appendChild(close);
      document.body.appendChild(badge);
    }
    var idle = state.status === 'paused' && state.paused_reason === 'idle';
    badge.style.backgroundColor = idle ? '#fffbeb' : '#152235';
    badge.style.color = idle ? '#92400e' : '#f8fafc';
    badge.style.borderColor = idle ? '#f59e0b' : '#334155';
    if (state.completed) {
      badgeText.textContent = chinese ? '任务完成，奖励已发放到你的账户 ✓' : 'Task complete! Your account has been rewarded ✓';
    } else if (idle) {
      badgeText.textContent = chinese
        ? '检测到你没有在浏览，计时已暂停（' + state.progress_seconds + ' / ' + state.browse_seconds + ' 秒）。滚动页面或移动鼠标即可继续。'
        : 'No browsing activity detected. Timing paused (' + state.progress_seconds + ' / ' + state.browse_seconds + ' seconds). Scroll or move your mouse to continue.';
    } else if (state.error) {
      badgeText.textContent = errorText(state.error);
    } else {
      badgeText.textContent = chinese
        ? '浏览任务进行中 ' + state.progress_seconds + ' / ' + state.browse_seconds + ' 秒'
        : 'Browse task in progress ' + state.progress_seconds + ' / ' + state.browse_seconds + ' seconds';
    }
  }

  function updateState(changes) {
    var next = Object.assign({}, state, changes);
    if (Object.keys(state).every(function (key) { return state[key] === next[key]; })) return;
    if (next.paused_reason === 'idle' && state.paused_reason !== 'idle') badgeClosed = false;
    state = next;
    renderBadge();
    listeners.slice().forEach(function (callback) {
      // 合作站点的回调不能中断心跳，也不能修改其他订阅者拿到的状态。
      try { callback(getState()); } catch (_) {
        // 忽略合作站点回调异常，继续通知其他订阅者。
      }
    });
  }

  function clearTimer() {
    if (timer !== null) window.clearTimeout(timer);
    timer = null;
  }

  function stop() {
    stopped = true;
    clearTimer();
    document.removeEventListener('visibilitychange', onVisibilityChange);
    activityEvents.forEach(function (event) {
      document.removeEventListener(event, onActivity, true);
    });
    try { window.sessionStorage.removeItem(storageKey); } catch (_) {
      // 存储不可用时仍已停止本实例的心跳。
    }
  }

  function reportError(reason, terminal) {
    if (terminal) {
      stop();
      updateState({ status: 'error', paused_reason: null, completed: false, error: reason });
      return;
    }
    retryAt = Date.now() + Math.min(2 * Math.pow(2, failures), 30) * 1000;
    failures = Math.min(failures + 1, 4);
    syncActivityState({ error: reason });
  }

  function pausedReason() {
    if (document.visibilityState !== 'visible') return 'hidden';
    if (idleTimeout > 0 && Date.now() - lastActivity > idleTimeout * 1000) return 'idle';
    return null;
  }

  function syncActivityState(changes) {
    if (stopped) return;
    var next = Object.assign({}, state, changes);
    next.paused_reason = pausedReason();
    next.status = next.paused_reason ? 'paused' : next.error ? 'error' : 'in_progress';
    updateState(next);
  }

  function beat() {
    timer = null;
    if (stopped) return;
    syncActivityState();
    if (!state.paused_reason && !inFlight && Date.now() >= retryAt) heartbeat();
    if (stopped) return;
    // 以计划时间推进节拍；响应耗时不累积到下一拍，后台延迟的拍次直接跳过。
    nextBeat += 1000;
    var now = Date.now();
    if (nextBeat <= now) nextBeat += (Math.floor((now - nextBeat) / 1000) + 1) * 1000;
    timer = window.setTimeout(beat, nextBeat - now);
  }

  function validProgress(data) {
    return data && (data.status === 'in_progress' || data.status === 'completed') &&
      Number.isFinite(data.progress_seconds) && data.progress_seconds >= 0 &&
      Number.isFinite(data.browse_seconds) && data.browse_seconds > 0 &&
      typeof data.completed === 'boolean';
  }

  async function heartbeat() {
    if (stopped || inFlight || pausedReason()) return;
    inFlight = true;
    try {
      var response = await window.fetch(base + '/api/v1/reward-open/sdk/heartbeat', {
        method: 'POST',
        headers: { 'Content-Type': 'text/plain' },
        body: JSON.stringify({ app_key: appKey, ticket: ticket }),
        credentials: 'omit',
        keepalive: true
      });
      // 代理的 5xx 可能返回 HTML，不能依赖 JSON 解析成功才重试。
      if (response.status >= 500) {
        reportError('HTTP_' + response.status, false);
        return;
      }
      var payload;
      try { payload = await response.json(); } catch (_) {
        reportError('INVALID_RESPONSE', false);
        return;
      }
      if (!response.ok || !payload || payload.code !== 0) {
        var reason = payload && typeof payload.reason === 'string' ? payload.reason : 'HTTP_' + response.status;
        reportError(reason, terminalReasons.indexOf(reason) !== -1);
        return;
      }
      var data = payload.data;
      if (!validProgress(data)) {
        reportError('INVALID_RESPONSE', false);
        return;
      }
      failures = 0;
      retryAt = 0;
      if (Number.isInteger(data.idle_timeout_seconds) &&
        (data.idle_timeout_seconds === 0 || (data.idle_timeout_seconds >= 5 && data.idle_timeout_seconds <= 3600))) {
        idleTimeout = data.idle_timeout_seconds;
      }
      var progress = {
        progress_seconds: data.progress_seconds,
        browse_seconds: data.browse_seconds,
        completed: data.completed,
        error: null
      };
      if (data.completed) {
        stop();
        updateState(Object.assign(progress, { status: 'completed', paused_reason: null }));
      } else {
        // 迟到响应可以刷新进度，但必须按当前可见性和活动时间保留暂停状态。
        syncActivityState(progress);
      }
    } catch (_) {
      reportError('NETWORK_ERROR', false);
    } finally {
      inFlight = false;
    }
  }

  function onActivity(event) {
    if (stopped || event.isTrusted !== true) return;
    lastActivity = Date.now();
    syncActivityState({ error: null });
  }

  function onVisibilityChange() {
    if (stopped) return;
    var current = document.visibilityState;
    if (current === 'visible' && visibility !== 'visible') {
      lastActivity = Date.now();
      syncActivityState({ error: null });
    } else {
      syncActivityState();
    }
    visibility = current;
  }

  if (badgeEnabled && !document.body) document.addEventListener('DOMContentLoaded', renderBadge);
  document.addEventListener('visibilitychange', onVisibilityChange);
  activityEvents.forEach(function (event) {
    document.addEventListener(event, onActivity, { capture: true, passive: true });
  });
  beat();
}());
