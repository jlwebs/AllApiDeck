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
:global(#app) {
  background: transparent;
  height: 100%;
  width: 100%;
  min-width: 0;
  min-height: 100%;
  margin: 0;
  overflow: hidden;
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
