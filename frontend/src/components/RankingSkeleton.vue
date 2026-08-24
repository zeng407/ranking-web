<script setup lang="ts">
/**
 * The community ranking's shape, drawn before its rows arrive.
 *
 * It wears the ranking's own layout classes rather than sizes of its own, so the space
 * it holds is the space the ranking will take and neither the page nor the card beside
 * it moves when the two swap. Decorative on purpose: whatever renders it owns the
 * status a screen reader is told.
 */
withDefaults(defineProps<{
  /** Rows to draw. The default fills the list's shortest height. */
  rows?: number
}>(), {
  rows: 6,
})
</script>

<template>
  <div class="game-community-layout" aria-hidden="true">
    <section class="game-rank-trend">
      <header>
        <div>
          <span class="skeleton-line is-short" />
          <h2><span class="skeleton-line is-half" /></h2>
        </div>
        <dl class="game-selected-group-ranks">
          <div>
            <dt><span class="skeleton-line is-half" /></dt>
            <dd><strong><span class="skeleton-line is-short" /></strong></dd>
            <dd class="game-winrate-bar" />
          </div>
        </dl>
      </header>
      <span class="game-rank-figure skeleton" />
      <div class="game-trend-slot" />
    </section>

    <ol class="game-community-list">
      <li v-for="row in rows" :key="row">
        <div class="game-community-row">
          <span class="game-community-position"><span class="skeleton-line" /></span>
          <span class="game-community-thumb skeleton" />
          <span class="game-community-open">
            <span class="game-community-title">
              <strong><span class="skeleton-line is-half" /></strong>
              <small><span class="skeleton-line is-short" /></small>
              <span class="game-winrate-bar" />
            </span>
          </span>
        </div>
      </li>
    </ol>
  </div>
</template>
