<template>
  <div class="typed-folder-view vorgang-view">
    <div class="typed-folder-header">
      <span class="typed-folder-type-badge vorgang">Vorgang</span>
      <button class="typed-action-btn" @click="$emit('new-child', 'register')">
        <span class="typed-action-icon">+</span>
        Neues Register
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
.typed-folder-type-badge.vorgang {
  background: #fff3e0;
  color: #e65100;
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
  border: 1px solid #e65100;
  border-radius: 6px;
  background: #e65100;
  color: white;
  font-size: 13px;
  cursor: pointer;
}
.typed-action-btn:hover { background: #bf360c; }
.typed-action-icon { font-size: 16px; font-weight: 700; }
</style>
