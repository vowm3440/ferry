<script lang="ts">
  import { i18n, tr } from '../../lib/i18n/index.svelte';

  /*
   * GPL-3.0 第 0 条所说的「适当法律声明」：版权、无担保、可依本许可证再分发、如何查看许可证。
   * 文字需与仓库根目录的 LICENSE、NOTICE 保持一致；这里不放任何外链跳转，只给可复制的地址。
   */
  const LICENSE_URL = 'https://www.gnu.org/licenses/gpl-3.0.html';
  const CORE_URL = 'https://github.com/SagerNet/sing-box';
</script>

<div class="legal" aria-label={tr('许可与法律声明', 'License and Legal Notices')}>
  {#if i18n.isEnglish}
    <p>
      Ferry, Copyright © 2026 Ferry contributors. This program is free software: you can redistribute it and/or modify
      it under the terms of the GNU General Public License as published by the Free Software Foundation, either version 3
      of the License, or (at your option) any later version.
    </p>
    <p>
      <strong>This program is distributed WITHOUT ANY WARRANTY</strong>; without even the implied warranty of
      MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See sections 15 and 16 of the License for details.
    </p>
    <p>
      Embedded sing-box core Copyright © 2022 nekohasekai and contributors, licensed under GPL-3.0-or-later (<span
        class="mono selectable">{CORE_URL}</span
      >). Ferry is an independent project and is neither affiliated with nor endorsed by sing-box.
    </p>
    <p>
      See the bundled LICENSE file or <span class="mono selectable">{LICENSE_URL}</span> for the full license text, and
      NOTICE for third-party attributions. Complete corresponding source code is distributed alongside releases.
    </p>
  {:else}
    <p>
      Ferry，版权所有 © 2026 Ferry 贡献者。本程序是自由软件：你可以依据自由软件基金会发布的
      GNU 通用公共许可证第 3 版，或（由你选择）任何更高版本，重新分发和修改它。
    </p>
    <p>
      <strong>本程序不提供任何担保</strong>，包括对适销性或特定用途适用性的默示担保；详见许可证第 15、16 条。
    </p>
    <p>
      内嵌的 sing-box 内核版权所有 © 2022 nekohasekai 及贡献者，同样以 GPL-3.0-or-later 授权（<span
        class="mono selectable">{CORE_URL}</span
      >）。Ferry 是独立项目，与 sing-box 官方没有隶属或合作关系，也未获其背书。
    </p>
    <p>
      许可证全文见随程序分发的 LICENSE 文件，或 <span class="mono selectable">{LICENSE_URL}</span>；第三方声明见
      NOTICE 文件。对应的完整源代码随发行包一同提供，也可以向你获得本程序的分发者索取。
    </p>
  {/if}
</div>

<style>
  .legal {
    padding: 12px 16px 14px;
    border-top: 1px solid var(--border);
    font-size: 12px;
    line-height: 1.7;
    color: var(--text-2);
  }

  p {
    margin: 0;
  }

  p + p {
    margin-top: 6px;
  }

  strong {
    font-weight: 600;
    color: var(--text);
  }

  .mono {
    overflow-wrap: anywhere;
  }
</style>
