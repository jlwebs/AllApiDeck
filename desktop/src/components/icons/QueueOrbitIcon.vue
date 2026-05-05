<template>
  <svg class="queue-orbit-icon" viewBox="0 0 24 24" fill="none" aria-hidden="true">
    <path class="queue-orbit-icon__segment queue-orbit-icon__segment-faint" d="M9.7 3.9a8.7 8.7 0 0 1 4.8 0" />
    <path class="queue-orbit-icon__segment queue-orbit-icon__segment-soft" d="M16.3 4.8a8.7 8.7 0 0 1 3 3.2" />
    <path class="queue-orbit-icon__segment queue-orbit-icon__segment-bright" d="M20.1 9.7a8.8 8.8 0 0 1-.1 4.9" />
    <path class="queue-orbit-icon__segment queue-orbit-icon__segment-strong" d="M19.1 16.4a8.7 8.7 0 0 1-3.2 2.9" />
    <path class="queue-orbit-icon__segment queue-orbit-icon__segment-soft" d="M14.2 20.1a8.8 8.8 0 0 1-4.9-.1" />
    <path class="queue-orbit-icon__segment queue-orbit-icon__segment-faint" d="M7.6 19a8.7 8.7 0 0 1-2.9-3.2" />
    <path class="queue-orbit-icon__segment queue-orbit-icon__segment-faint" d="M3.9 14.2a8.8 8.8 0 0 1 .2-4.9" />
    <path class="queue-orbit-icon__lead" d="M5 7.5a8.7 8.7 0 0 1 2.6-2.7" />
    <path class="queue-orbit-icon__arrow" d="m8.1 3.9-.5 1.8 1.8-.4" />
    <circle class="queue-orbit-icon__core" cx="12" cy="12" r="1.9" />
    <circle class="queue-orbit-icon__pulse" cx="17.6" cy="16.8" r="1.15" />
  </svg>
</template>

<style scoped>
.queue-orbit-icon {
  display: block;
  width: 18px;
  height: 18px;
}

.queue-orbit-icon__segment,
.queue-orbit-icon__lead,
.queue-orbit-icon__arrow {
  stroke: currentColor;
  stroke-width: 1.9;
  stroke-linecap: round;
  stroke-linejoin: round;
}

.queue-orbit-icon__segment-faint {
  opacity: 0.3;
}

.queue-orbit-icon__segment-soft {
  opacity: 0.5;
}

.queue-orbit-icon__segment-bright {
  opacity: 0.78;
}

.queue-orbit-icon__segment-strong,
.queue-orbit-icon__lead,
.queue-orbit-icon__arrow {
  opacity: 1;
}

.queue-orbit-icon__core {
  fill: currentColor;
  opacity: 0.9;
}

.queue-orbit-icon__pulse {
  fill: currentColor;
  opacity: 0.72;
}
</style>
