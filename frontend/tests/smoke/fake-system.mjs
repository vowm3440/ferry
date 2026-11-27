// 假后端的「系统」部分：系统代理、备份与恢复、诊断。行为与错误文案照抄
// internal/app/sysproxy.go 与 backup.go；状态都挂在 s 上，供断言使用。
//
//   s.sysProxy     Ferry 是否已把系统代理设成自己
//   s.files        「磁盘」上已有的文件：路径 -> 内容（备份是状态快照，诊断是 JSON 文本）
const BYPASS = ['<local>', 'localhost', '127.*', '[::1]'];
const ABSOLUTE = /^([A-Za-z]:[\\/]|\\\\)/;

const copy = (value) => JSON.parse(JSON.stringify(value));

export function fakeSystem({ s, Rejected, state }) {
  s.sysProxy = false;
  s.files = new Map();

  /** 共享的 Core.Stop 不认识系统代理：这里按 Go 的行为补上「内核停止即关闭系统代理」。 */
  const syncProxy = () => {
    if (s.status !== 'running') s.sysProxy = false;
    return s.sysProxy;
  };

  /** 导出类操作共用的路径检查；文案由调用方给出。 */
  const target = (path, cancelled, relative, exists) => {
    if (path === '') throw new Rejected(cancelled);
    if (!ABSOLUTE.test(path)) throw new Rejected(relative);
    if (s.files.has(path)) throw new Rejected(exists);
  };

  return {
    'System.SysProxyStatus': () => {
      const enabled = syncProxy();
      return { enabled, address: enabled ? `127.0.0.1:${s.settings.mixedPort}` : '', bypass: [...BYPASS] };
    },
    'System.SetSysProxy': (on) => {
      if (!on) {
        s.sysProxy = false;
        return;
      }
      if (s.status !== 'running') throw new Rejected('请先启动内核，再启用系统代理');
      s.sysProxy = true;
    },
    'System.ExportBackup': (path) => {
      target(path, '备份已取消', '备份路径须为绝对路径', '无法创建备份，请选择同一磁盘上尚不存在的文件');
      s.files.set(path, {
        kind: 'backup',
        nodes: copy(s.nodes),
        groups: copy(s.groups),
        subs: copy(s.subs),
        settings: copy(s.settings),
        mode: s.mode,
      });
    },
    'System.RestoreBackup': (path) => {
      if (path === '') throw new Rejected('恢复已取消');
      if (!ABSOLUTE.test(path)) throw new Rejected('备份路径须为绝对路径');
      const backup = s.files.get(path);
      if (backup === undefined || backup.kind !== 'backup') throw new Rejected('备份为空或不存在');
      s.nodes = copy(backup.nodes);
      s.groups = copy(backup.groups);
      s.subs = copy(backup.subs);
      s.settings = copy(backup.settings);
      s.mode = backup.mode;
      s.selected = new Map();
      // 运行中的内核会按新配置重启。
      if (s.status === 'running') {
        s.startedAt = Date.now();
        s.ticks = 0;
      }
    },
    'System.ExportDiagnostics': (path) => {
      target(path, '诊断导出已取消', '诊断路径须为绝对路径', '无法创建诊断文件，请选择尚不存在的文件');
      const now = state();
      s.files.set(path, {
        kind: 'diagnostics',
        text: JSON.stringify({
          schema: 1,
          version: now.version,
          coreVersion: now.coreVersion,
          status: now.status,
          mode: now.mode,
          serviceMode: now.serviceMode,
          nodeCount: s.nodes.length,
          groupCount: s.groups.length,
          subscriptionCount: s.subs.length,
          ruleCount: 0,
          persistent: s.capabilities.includes('settings.persist'),
          platform: 'windows',
        }),
      });
    },
  };
}
