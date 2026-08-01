<template>
  <svg class="candy-guard-icon" viewBox="0 0 24 24" fill="none" aria-hidden="true">
    <path class="candy-guard-icon__twist" d="m5.2 8.1-2.1-1.4.8 3.2-1.1 2.4 2.9.1" />
    <path class="candy-guard-icon__twist" d="m18.8 15.9 2.1 1.4-.8-3.2 1.1-2.4-2.9-.1" />
    <rect class="candy-guard-icon__body" x="5.7" y="7" width="12.6" height="10" rx="4.7" />
    <path class="candy-guard-icon__stripe" d="m8.4 7.5 8.3 8.3M7 11.1l5.8 5.8M12.1 7.2l5.1 5.1" />
    <path class="candy-guard-icon__guard" d="m8.1 5.2 3.9-1.6 3.9 1.6v3.1c0 2.5-1.6 4.3-3.9 5.2-2.3-.9-3.9-2.7-3.9-5.2z" />
    <path class="candy-guard-icon__check" d="m10.2 8.8 1.1 1.1 2.5-2.5" />
  </svg>
</template>

<style scoped>
.candy-guard-icon {
  display: block;
  width: 18px;
  height: 18px;
}

.candy-guard-icon__body,
.candy-guard-icon__guard {
  stroke: currentColor;
  stroke-width: 1.45;
  stroke-linejoin: round;
}

.candy-guard-icon__body {
  fill: currentColor;
  fill-opacity: .13;
}

.candy-guard-icon__twist,
.candy-guard-icon__stripe,
.candy-guard-icon__check {
  stroke: currentColor;
  stroke-width: 1.45;
  stroke-linecap: round;
  stroke-linejoin: round;
}

.candy-guard-icon__stripe {
  opacity: .72;
}

.candy-guard-icon__guard {
  fill: currentColor;
  fill-opacity: .18;
}

.candy-guard-icon__check {
  stroke-width: 1.6;
}
</style>
