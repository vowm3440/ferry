// 假后端的连接与日志部分（仅用于冒烟）：行为与报错文案照抄 internal/app 的 connections.go、logs.go。
//
//   createFakeBackend({ capabilities: ['subs.text', 'connections', 'logs'], extra: [fakeInspect] })
//
// 状态挂在 s 上：
//   s.conns        当前活动连接（Connections.Snapshot 的返回值）
//   s.connsPaused  Connections.Pause 设置的全局开关
//   s.logEntries   后端收集的日志（最多 5000 条，seq 递增）
//   s.logsPaused   Logs.Pause 设置的全局开关
//   s.files        已经存在的文件路径（Logs.Export 不覆盖已有文件）
//   s.exports      每次成功导出的记录：{ path, count }
// Go 侧的事件由脚本自己推送，下面的 connectionsEvent / appendLogs 负责「暂停时不推送」这条规则。

const LOG_BUFFER_MAX = 5000;
const LOG_LEVELS = ['trace', 'debug', 'info', 'warn', 'error', 'fatal', 'panic'];

export function makeConnection(overrides = {}) {
  return {
    id: 'c1',
    process: 'chrome.exe',
    processPath: 'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe',
    host: 'www.example.invalid',
    port: 443,
    ruleName: '默认规则',
    ruleId: 'builtin-default',
    action: 'proxy',
    outboundTag: '节点甲',
    chain: [],
    up: 0,
    down: 0,
    startAt: Date.now() - 65_000,
    ...overrides,
  };
}

/** connections:snapshot 事件的负载；暂停时后端不推送，返回 null。 */
export function connectionsEvent(s) {
  if (s.connsPaused || s.status !== 'running') return null;
  return { items: s.conns, added: [], updated: [], removed: [] };
}

/**
 * 让后端「收集」到几条日志（[level, message] 或 { level, message }）。
 * 返回应当推送的 logs:entry 负载；暂停时后端只收集不推送，返回 null。
 */
export function appendLogs(s, items) {
  const batch = items.map((item) => {
    const [level, message] = Array.isArray(item) ? item : [item.level, item.message];
    s.logSeq += 1;
    return { seq: s.logSeq, time: Date.now(), level, message };
  });
  s.logEntries = [...s.logEntries, ...batch].slice(-LOG_BUFFER_MAX);
  return s.logsPaused ? null : batch;
}

export function fakeInspect({ s, Rejected }) {
  s.conns = [];
  s.connsPaused = false;
  s.logEntries = [];
  s.logSeq = 0;
  s.logsPaused = false;
  s.files = new Set();
  s.exports = [];

  return {
    // 内核停止后没有连接可读，返回空列表而不是报错。
    'Connections.Snapshot': () => (s.status === 'running' ? s.conns : []),
    'Connections.Close': (id) => {
      if (id === '') throw new Rejected('请选择连接');
      s.conns = s.conns.filter((conn) => conn.id !== id);
    },
    'Connections.CloseAll': () => {
      s.conns = [];
    },
    'Connections.Pause': (on) => {
      s.connsPaused = on === true;
    },
    'Logs.Snapshot': (limit) => {
      if (!Number.isInteger(limit) || limit < 1 || limit > LOG_BUFFER_MAX) throw new Rejected('日志数量须在 1 到 5000 之间');
      return s.logEntries.slice(-limit);
    },
    'Logs.Pause': (on) => {
      s.logsPaused = on === true;
    },
    'Logs.SetLevel': (level) => {
      if (!LOG_LEVELS.includes(level)) throw new Rejected('日志级别无效');
      if (s.status === 'starting') throw new Rejected('内核正在启动，请稍后重试');
      s.settings = { ...s.settings, logLevel: level };
    },
    'Logs.Export': (path) => {
      if (path === '' || s.files.has(path)) throw new Rejected('无法创建日志文件，请选择尚不存在的文件');
      s.files.add(path);
      s.exports = [...s.exports, { path, count: s.logEntries.length }];
    },
  };
}
