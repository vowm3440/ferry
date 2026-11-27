import assert from 'node:assert/strict';
import test from 'node:test';

test('i18n：校验语言标识、选项与中英文切换文案', async () => {
  const { LOCALE_OPTIONS, getLocale, isLocale, setCurrentLocale, t, translateText } = await import(
    '../src/lib/i18n/locale.ts'
  );
  assert.equal(isLocale('zh-CN'), true);
  assert.equal(isLocale('en-US'), true);
  assert.equal(isLocale('fr-FR'), false);
  assert.deepEqual(
    LOCALE_OPTIONS.map((item) => item.value),
    ['zh-CN', 'en-US'],
  );
  assert.equal(translateText('zh-CN', '概览', 'Overview'), '概览');
  assert.equal(translateText('en-US', '概览', 'Overview'), 'Overview');

  assert.equal(getLocale(), 'zh-CN');
  assert.equal(t('概览', 'Overview'), '概览');
  setCurrentLocale('en-US');
  try {
    assert.equal(getLocale(), 'en-US');
    assert.equal(t('概览', 'Overview'), 'Overview');
  } finally {
    setCurrentLocale('zh-CN');
  }
});

test('i18n：纯 TS 视图辅助函数支持 en-US 与 zh-CN 双语输出', async () => {
  const { setCurrentLocale } = await import('../src/lib/i18n/locale.ts');
  const { formatDuration, formatDurationText, formatLatency } = await import('../src/lib/format.ts');
  const { formatAgo } = await import('../src/lib/time.ts');
  const { expiryView } = await import('../src/lib/sub-usage.ts');
  const { usageRows } = await import('../src/lib/overview-view.ts');
  const { buildGroupViews, currentSelection, latencyView, protocolLabel } = await import('../src/lib/nodes-view.ts');
  const { autoUpdateText, checkSubUrl, progressText, shareFormatOptions, statusPill } = await import(
    '../src/lib/subs-view.ts'
  );
  const { actionFilterOptions, actionLabel, appName } = await import('../src/lib/conns-view.ts');
  const {
    actionLabel: ruleActionLabel,
    appText,
    importOutcomeText,
    networkOptions,
    ruleSetStatus,
    validateAppGroupForm,
    validateRuleForm,
    validateTestInput,
  } = await import('../src/lib/rules-view.ts');
  const { ruleSetRefs } = await import('../src/lib/rules-validate.ts');
  const {
    diagnosticsContains,
    diagnosticsExcludes,
    fileActionBlock,
    persistenceNote,
    restoreReplaces,
    sysProxyTile,
  } = await import('../src/lib/system-view.ts');
  const { checkCidr, dnsStrategyOptions, logLevelOptions, updateHoursOptions, userAgentError } = await import(
    '../src/lib/settings-view.ts'
  );
  const { modeLabel } = await import('../src/lib/api/types.ts');

  setCurrentLocale('en-US');
  try {
    // format.ts
    assert.equal(formatDuration(90_061_000), '1d 01:01:01');
    assert.equal(formatDurationText(7500_000), '2h 5m');
    assert.equal(formatLatency(-1), 'Untested');

    // time.ts
    assert.equal(formatAgo(10_000, 20_000), 'Just now');
    assert.equal(formatAgo(10_000, 190_000), '3m ago');

    // sub-usage.ts & overview-view.ts
    assert.equal(expiryView(1000, 5000).remainText, 'Expired');
    assert.equal(expiryView(1000 + 5 * 86400_000, 1000).remainText, '5 days left');
    const summary = usageRows(
      [
        {
          id: 's1',
          name: 'Sub',
          urlMasked: 'https://example.com/****',
          nodeCount: 3,
          groupCount: 1,
          usedBytes: 1024,
          totalBytes: 0,
          expiresAt: 0,
          lastUpdatedAt: 0,
          intervalHours: 12,
          autoUpdate: true,
          userAgent: 'clash.meta',
          status: 'ok',
          skipped: [],
          warnings: [],
        },
      ],
      10_000,
      4,
    );
    assert.equal(summary.rows[0]?.usageText, 'Used 1.00 KB');
    assert.equal(summary.rows[0]?.autoText, 'Every 12h');

    // nodes-view.ts
    assert.equal(protocolLabel(''), 'Unknown');
    assert.equal(latencyView(-1, 'testing').text, 'Testing…');
    assert.equal(latencyView(-1, 'timeout').text, 'Timeout');
    const nodeList = {
      groups: [{ id: 'g1', name: 'Auto', type: 'urltest', members: ['n1'], testUrl: '', intervalMin: 5 }],
      nodes: [
        {
          id: 'n1',
          tag: 'n1',
          name: 'HK 01',
          groupId: '',
          type: 'vless',
          server: '1.1.1.1',
          port: 443,
          latencyMs: -1,
          latencyStatus: 'idle',
          selected: false,
          favorite: false,
        },
      ],
    };
    assert.equal(currentSelection(nodeList)?.detail, 'Auto-selected by core');
    const views = buildGroupViews(nodeList, { query: '', favoritesOnly: false, sort: 'default' });
    assert.equal(views[1]?.name, 'Ungrouped');

    // subs-view.ts
    assert.equal(checkSubUrl(''), 'Please enter a subscription URL');
    assert.equal(progressText('fetch'), 'Downloading…');
    assert.equal(
      statusPill({
        id: '1',
        name: 'A',
        urlMasked: '',
        nodeCount: 1,
        groupCount: 0,
        usedBytes: 0,
        totalBytes: 0,
        expiresAt: 0,
        lastUpdatedAt: 0,
        intervalHours: 24,
        autoUpdate: false,
        userAgent: '',
        status: 'ok',
        skipped: [],
        warnings: [],
      }).text,
      'Manual',
    );
    assert.equal(
      autoUpdateText({
        id: '1',
        name: 'A',
        urlMasked: 'https://a.com/****',
        nodeCount: 1,
        groupCount: 0,
        usedBytes: 0,
        totalBytes: 0,
        expiresAt: 0,
        lastUpdatedAt: 0,
        intervalHours: 6,
        autoUpdate: false,
        userAgent: '',
        status: 'ok',
        skipped: [],
        warnings: [],
      }),
      'Off',
    );
    assert.equal(shareFormatOptions().find((o) => o.value === 'uri')?.label, 'URI');

    // conns-view.ts
    assert.equal(actionFilterOptions()[0]?.label, 'All');
    assert.equal(actionLabel('proxy'), 'Proxy');
    assert.equal(
      appName({
        id: 'c1',
        process: '',
        processPath: '',
        network: 'tcp',
        host: 'example.com',
        port: 443,
        ruleName: '',
        action: 'proxy',
        outboundTag: '',
        chain: [],
        up: 0,
        down: 0,
        startAt: 0,
      }),
      'Unknown App',
    );

    // rules-view.ts & rules-validate.ts
    assert.equal(ruleActionLabel('direct'), 'Direct');
    assert.equal(networkOptions()[0]?.label, 'Any');
    assert.equal(ruleSetRefs()[0]?.label, 'Mainland China Domains');
    assert.equal(appText('@group:g1', [{ id: 'g1', name: 'Browsers', apps: ['chrome.exe'] }]), 'Group: Browsers');
    assert.equal(ruleSetStatus('ready').label, 'Ready');
    assert.equal(importOutcomeText(3, ''), 'Imported 3 rules');
    assert.equal(validateRuleForm({ id: '', name: '', enabled: true, appsText: '', hostsText: '', portsText: '', network: 'any', action: 'direct', targetId: '', resolveBefore: true }, null, null).name, 'Please enter a rule name');
    assert.equal(validateAppGroupForm('', '').name, 'Please enter a group name');
    assert.equal(validateTestInput('', '', '443'), 'Please enter a target host');

    // system-view.ts & settings-view.ts & types.ts
    assert.equal(
      sysProxyTile({
        capsLoaded: true,
        capable: true,
        status: { enabled: false, address: '' },
        error: '',
        running: true,
        busy: false,
        mixedPort: 7890,
      }).detail,
      'Off',
    );
    assert.match(persistenceNote({ capsLoaded: true, persistent: true, demo: false }), /encrypted locally/i);
    assert.match(fileActionBlock({ capsLoaded: true, capable: false, dialogs: true }), /not supported/i);
    assert.equal(restoreReplaces().length, 4);
    assert.equal(diagnosticsContains().length, 4);
    assert.match(diagnosticsExcludes(), /Does not include/i);
    assert.equal(dnsStrategyOptions()[0]?.label, 'Prefer IPv4');
    assert.match(logLevelOptions()[2]?.label ?? '', /info/i);
    assert.equal(updateHoursOptions(24).find((o) => o.value === '24')?.label, 'Every 24h');
    assert.match(userAgentError('x'.repeat(300)), /256 bytes/i);
    assert.match(checkCidr('invalid'), /CIDR/i);
    assert.equal(modeLabel('rule'), 'Rule');
  } finally {
    setCurrentLocale('zh-CN');
  }

  // Verify default zh-CN remains untouched
  assert.equal(formatLatency(-1), '未测速');
  assert.equal(modeLabel('rule'), '规则');
});
