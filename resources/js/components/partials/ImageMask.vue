<template>
  <div class="image-mask" v-if="mask">
    <div>{{ $t('post.nsfw') }}</div>
    <h6 class="mt-2">{{ $t('post.nsfw.alert') }}</h6>
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
