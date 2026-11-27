<script lang="ts">
  import Icon from '../lib/components/Icon.svelte';
  import { tr } from '../lib/i18n/index.svelte';
  import { core } from '../lib/stores/core.svelte';
  import HeroCard from './overview/HeroCard.svelte';
  import StatTiles from './overview/StatTiles.svelte';
  import TrafficCard from './overview/TrafficCard.svelte';
  import UsageCard from './overview/UsageCard.svelte';
</script>

<section class="page" aria-labelledby="overview-title">
  <div class="page-head"><h1 id="overview-title">{tr('概览', 'Overview')}</h1></div>

  {#if core.loadError !== ''}
    <div class="inline-error load-error" role="alert">
      <Icon name="alert" />
      <span class="grow selectable">{tr(`读取内核状态失败：${core.loadError}`, `Failed to load core status: ${core.loadError}`)}</span>
      <button type="button" class="btn sm" onclick={() => void core.refresh()}>{tr('重试', 'Retry')}</button>
    </div>
  {/if}

  <div class="ov">
    <HeroCard />
    <TrafficCard />
    <div class="span2"><StatTiles /></div>
    <div class="span2"><UsageCard /></div>
  </div>
</section>

<style>
  .ov {
    display: grid;
    grid-template-columns: minmax(0, 1.3fr) minmax(0, 1fr);
    gap: 12px;
  }

  .span2 {
    grid-column: span 2;
  }

  .load-error {
    align-items: center;
    margin-bottom: 12px;
  }

  .grow {
    flex: 1;
    min-width: 0;
  }

  @container content (max-width: 860px) {
    .ov {
      grid-template-columns: minmax(0, 1fr);
    }

    .span2 {
      grid-column: auto;
    }
  }
</style>
