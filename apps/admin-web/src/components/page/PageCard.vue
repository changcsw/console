<template>
  <section class="page-card">
    <header v-if="title || description" class="page-card__header">
      <h2 v-if="title" class="page-card__title">{{ title }}</h2>
      <p v-if="description" class="page-card__desc">{{ description }}</p>
    </header>
    <div class="page-card__body">
      <slot />
    </div>
  </section>
</template>

<script setup lang="ts">
defineProps<{
  title?: string;
  description?: string;
}>();
</script>

