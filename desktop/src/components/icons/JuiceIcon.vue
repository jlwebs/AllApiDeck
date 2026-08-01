<template>
  <svg class="juice-icon" viewBox="0 0 24 24" fill="none" aria-hidden="true">
    <path class="juice-icon__straw" d="m14.6 4.7 2.1-2.1h2.1" />
    <path class="juice-icon__cup" d="m5.4 6.4 1.3 13.1h10.6l1.3-13.1z" />
    <path class="juice-icon__rim" d="M4.5 6.4h15" />
    <path class="juice-icon__juice" d="M6.4 10.1h11.2l-.7 7.3H7.1z" />
    <path class="juice-icon__slice" d="m8.6 12.3 2.2 2.2 2.4-2.4 2.2 2.2" />
  </svg>
</template>

<style scoped>
.juice-icon {
  display: block;
  width: 18px;
  height: 18px;
}

.juice-icon__cup,
.juice-icon__juice {
  stroke: currentColor;
  stroke-width: 1.45;
  stroke-linejoin: round;
}

.juice-icon__juice {
  fill: currentColor;
  fill-opacity: .2;
}

.juice-icon__straw,
.juice-icon__rim,
.juice-icon__slice {
  stroke: currentColor;
  stroke-width: 1.45;
  stroke-linecap: round;
  stroke-linejoin: round;
}

.juice-icon__slice {
  opacity: .78;
}
</style>
