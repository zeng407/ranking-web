<template>
  <div class="image-mask" v-if="mask">
    <div>{{ $t('post.nsfw') }}</div>
    <div class="btn btn-outline-light" @click="show"><i class="fa-solid fa-eye"></i>&nbsp;{{ $t('post.nsfw.show') }}</div>
  </div>
</template>

<script>
export default {
  data() {
    return {
      mask: true
    }
  },
  methods: {
    show() {
      this.mask = false;
    }
  }
};
</script>
