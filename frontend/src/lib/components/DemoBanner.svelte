<script lang="ts">
  import { tr } from '../i18n/index.svelte';
  import { connection } from '../stores/connection.svelte';
  import Icon from './Icon.svelte';
</script>

<div class="demo-bar" role="note">
  <Icon name="flask" size={14} />
  <span class="grow">
    <b>{tr('演示模式', 'Demo Mode')}</b> · {tr('界面中的数据均为合成示例，没有连接任何内核，也不会产生真实流量。', 'All data shown is synthetic; no proxy core is attached and no real traffic flows.')}
  </span>
  <button type="button" class="btn sm" onclick={() => connection.leaveDemo()}>{tr('退出演示', 'Exit Demo')}</button>
</div>

<style>
  .demo-bar {
    flex: none;
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 7px 24px;
    background: var(--warn-soft);
    color: var(--warn);
    font-size: 12.5px;
    border-bottom: 1px solid var(--border);
  }

  .grow {
    flex: 1;
    min-width: 0;
  }

  .btn {
    border-color: transparent;
    background: transparent;
    color: inherit;
  }

  .btn:hover:not(:disabled) {
    background: var(--muted-soft);
  }
</style>
