<template>
  <div class="col-md-6 col-12 pt-2" v-if="post">
    <div class="card shadow">
      <div class="card-header text-center">
        <h2 class="post-title">{{post.title}}</h2>
      </div>
      <div class="row no-gutters">
        <div class="row no-gutters m-0 w-100 position-relative">
          <image-mask v-if="post.is_censored"></image-mask>
          <div class="col-6">
            <div class="post-element-container">
              <flex-image
                v-if="post.element1.previewable"
                :key="post.element1.id"
                :image-key="post.element1.id"
                :element-id="post.element1.id"
                :thumb-url="post.element1.url"
                :imgur-url="post.element1.url2"
                :alt="post.element1.title"
                :lazy="true"
              ></flex-image>
              <video v-else :src="getVideoPreviewUrl(post.element1.url)"></video>
            </div>
          </div>
          <div class="col-6">
            <div class="post-element-container">
              <flex-image
                v-if="post.element2.previewable"
                :key="post.element2.id"
                :image-key="post.element2.id"
                :element-id="post.element2.id"
                :thumb-url="post.element2.url"
                :imgur-url="post.element2.url2"
                :alt="post.element2.title"
                :lazy="true"
              ></flex-image>
              <video v-else :src="getVideoPreviewUrl(post.element2.url)"></video>
            </div>
          </div>
        </div>
        <div class="row no-gutters m-0 w-100">
          <div class="col-6">
            <h5 class="text-center mt-1 p-1 element-title">{{ post.element1.title }}</h5>
          </div>
          <div class="col-6">
            <h5 class="text-center mt-1 p-1 element-title">{{ post.element2.title }}</h5>
          </div>
        </div>
        <div class="card-body pt-0 text-center">
          <p class="text-break">{{ post.description }}</p>
          <div class="row">
            <div class="col-6">
              <a class="btn btn-primary btn-block" :href="getShowGameUrl(post.serial)" target="_blank">
                <i class="fas fa-play"></i> {{$t('home.start')}}
              </a>
            </div>
            <div class="col-6">
              <a class="btn btn-secondary btn-block" :href="getShowRankUrl(post.serial)" target="_blank">
                <i class="fas fa-trophy"></i> {{$t('home.rank')}}
              </a>
            </div>
          </div>
          <span class="mt-2 card-text float-left">
            <button :id="'popover-button-event'+post.serial" type="button" class="btn btn-outline-dark btn-sm"
              @click="onClickShare(getShowGameUrl(post.serial), post.serial)">
              {{$t('Share')}} &nbsp;<i class="fas fa-share-square"></i>
            </button>
            <b-popover :ref="'popover'+post.serial" :target="'popover-button-event'+post.serial" :disabled="true">
              {{$t('Copied link')}}
            </b-popover>
          </span>
          <span class="mt-2 card-text float-right white-space-no-wrap">
            <span class="pr-2">
              <i class="fas fa-play-circle"></i>&nbsp;{{ post.play_count }}
            </span>
            <small class="text-muted white-space-no-wrap">{{ post.created_at | datetime }}</small>
          </span>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
export default {
  name: 'home-post',
  mounted() {

  },
  props: {
    showGameEndpoint: {
      type: String,
      required: true
    },
    showRankEndpoint: {
      type: String,
      required: true
    },
    post: {
      type: Object,
      required: true
    },
    onImageError: {
      type: Function,
      required: true
    }
  },
  data: function () {
    return {

    }
  },
  watch: {

  },
  methods: {
    getVideoPreviewUrl(videoUrl) {
      return videoUrl+'?t=0.01';
    },
    getShowGameUrl(serial) {
      return this.showGameEndpoint.replace('_serial', serial);
    },
    getShowRankUrl(serial) {
      return this.showRankEndpoint.replace('_serial', serial);
    },
    onClickShare(url, serial) {
      let popover = this.$refs['popover'+serial];
      this.$emit('share', popover, url, serial);
    }
  }
}
</script>
