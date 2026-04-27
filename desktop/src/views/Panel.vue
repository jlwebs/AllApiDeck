<template>
  <div class="panel-view">
    <key-side-panel />
  </div>
</template>

<script setup>
import KeySidePanel from '../components/KeySidePanel.vue';
</script>

<style scoped>
:global(html),
:global(body),
:global(#app),
:global(.app-shell) {
  background: transparent;
  height: 100%;
  width: 100%;
  min-width: 0;
  min-height: 100%;
  margin: 0;
  overflow: hidden;
}

:global(body.light-mode),
:global(body.dark-mode),
:global(body.gaia-dark) {
  background: transparent !important;
  background-color: transparent !important;
  background-image: none !important;
}

:global(body.gaia-dark .app-shell::before) {
  display: none !important;
}

.panel-view {
  height: 100vh;
  width: 100%;
  min-height: 100vh;
  min-width: 0;
  background: transparent;
  overflow: hidden;
}
</style>
