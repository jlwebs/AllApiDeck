<template>
  <div class="layout-view-cache">
    <router-view v-slot="{ Component, route }">
      <KeepAlive>
        <component
          :is="Component"
          v-if="route.meta.keepAlive"
          :key="route.name || route.path"
        />
      </KeepAlive>
      <component
        :is="Component"
        v-if="!route.meta.keepAlive"
        :key="route.name || route.path"
      />
    </router-view>
  </div>
</template>

<script>
export default {
  name: 'Layout',
  components: {},
};
</script>

<style scoped>
.layout-view-cache {
  min-width: 0;
  min-height: 100vh;
}
</style>
