<template>
  <div class="typed-folder-view akte-view">
    <div class="typed-folder-header">
      <span class="typed-folder-type-badge akte">Akte</span>
      <button class="typed-action-btn" @click="$emit('new-child', 'vorgang')">
        <span class="typed-action-icon">+</span>
        Neuer Vorgang
      </button>
    </div>
    <slot />
  </div>
</template>

<script setup lang="ts">
defineEmits<{ 'new-child': [type: string] }>()
</script>

<style scoped>
.typed-folder-header {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 8px 16px;
  border-bottom: 1px solid #e0e0e0;
}
.typed-folder-type-badge.akte {
  background: #e8f5e9;
  color: #2e7d32;
  font-size: 12px;
  font-weight: 600;
  padding: 2px 8px;
  border-radius: 4px;
}
.typed-action-btn {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 6px 14px;
  border: 1px solid #2e7d32;
  border-radius: 6px;
  background: #2e7d32;
  color: white;
  font-size: 13px;
  cursor: pointer;
}
.typed-action-btn:hover { background: #1b5e20; }
.typed-action-icon { font-size: 16px; font-weight: 700; }
</style>
