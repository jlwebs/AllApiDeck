<template>
  <a-modal
    :open="open"
    :title="title"
    :confirm-loading="confirmLoading"
    :ok-text="okText"
    :cancel-text="cancelText"
    :destroy-on-close="true"
    @ok="handleOk"
    @cancel="handleCancel"
  >
    <a-textarea
      v-if="multiline"
      :value="value"
      :placeholder="placeholder"
      :rows="rows"
      :maxlength="maxLength || undefined"
      :show-count="showCount && maxLength > 0"
      @update:value="handleValueChange"
    />
    <a-input
      v-else
      :value="value"
      :placeholder="placeholder"
      :maxlength="maxLength || undefined"
      :show-count="showCount && maxLength > 0"
      @update:value="handleValueChange"
      @pressEnter="handleOk"
    />
  </a-modal>
</template>

<script setup>
const props = defineProps({
  open: {
    type: Boolean,
    default: false,
  },
  value: {
    type: String,
    default: '',
  },
  title: {
    type: String,
    default: '',
  },
  placeholder: {
    type: String,
    default: '',
  },
  okText: {
    type: String,
    default: '确定',
  },
  cancelText: {
    type: String,
    default: '取消',
  },
  multiline: {
    type: Boolean,
    default: false,
  },
  rows: {
    type: Number,
    default: 4,
  },
  maxLength: {
    type: Number,
    default: 0,
  },
  confirmLoading: {
    type: Boolean,
    default: false,
  },
  showCount: {
    type: Boolean,
    default: false,
  },
});

const emit = defineEmits(['update:open', 'update:value', 'ok', 'cancel']);

function handleValueChange(value) {
  emit('update:value', String(value ?? ''));
}

function handleOk() {
  emit('ok', props.value);
}

function handleCancel() {
  emit('cancel');
  emit('update:open', false);
}
</script>
