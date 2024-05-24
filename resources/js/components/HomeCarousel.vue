<template>
  <div id="home-carousel" class="carousel slide mb-5" data-ride="carousel" data-interval="false">
    <!-- carousel -->
    <div id="preserve-for-carousel" class="preserve-for-carousel"></div>
    <div v-show="items.length" class="carousel-inner position-absolute" style="top:0">
      <div v-for="(item, index) in items" :key="index" class="carousel-item" :class="{ active: index === 0 }">
        <div class="d-flex justify-content-center">
          <!-- loading -->
          <div class="d-flex w-100 position-absolute justify-content-center">
            <i v-show="firstCarouselLoading" class="fas fa-spinner fa-spin preview-carsousel-loading"></i>
            <img v-show="item.image_url && item.type === 'video' && index === 0 && firstCarouselLoading"
              :src="item.image_url" class="preview-carsousel-image" :alt="item.title" style="pointer-events:none">
          </div>

          <!-- iframe -->
          <div v-if="item.video_source === 'youtube'" class="home-carousel-container">
            <youtube :ref="'player'+index" :video-id="item.video_id" width="100%"
              @ready="handleIframeLoaded(index)"
              :player-vars="{ controls: 1, autoplay: 0, rel: 0, origin: origin, start:item.video_start_second }">
            </youtube>
          </div>
          <div v-else-if="item.video_source === 'twitch_video'" class="home-carousel-container twitch-container">
            <iframe :ref="'player'+index" @load="handleIframeLoaded(index)" :src="getTwitchVideoUrl(item)"  width="100%"
              preload="metadata" allowfullscreen></iframe>
          </div>
          <div v-else-if="item.video_source === 'twitch_channel'" class="home-carousel-container twitch-container">
            <iframe :ref="'player'+index" @load="handleIframeLoaded(index)" :src="getTwitchChannelUrl(item)" width="100%"
              preload="metadata" allowfullscreen></iframe>
          </div>
          <div v-else-if="item.video_source === 'twitch_clip'" class="home-carousel-container twitch-container">
            <iframe :ref="'player'+index" @load="handleIframeLoaded(index)" :src="getTwitchClipUrl(item)"  width="100%"
              preload="metadata" allowfullscreen></iframe>
          </div>
          <img v-else-if="item.image_url" :src="item.image_url" class="d-block" :alt="item.title">
        </div>

        <!-- title -->
        <div v-if="!firstCarouselLoading && titleVisible && item.title" class="text-center">
          <h5 class="d-inline-block px-2 pt-1 bg-dark">
            <!-- mobile -->
            <span class="d-block d-sm-none font-size-small text-white">{{ item.title }}</span>
            <!-- desktop -->
            <span class="d-none d-sm-block text-white">{{ item.title }}</span>
          </h5>
        </div>
      </div>
      
    </div>

    <div v-show="items.length > 1">
      <!-- left button -->
      <button class="carousel-control-prev position-absolute" style="width: 10%; height: 30px; top: 50%; transform: translateY(-50%);" type="button" data-target="#home-carousel"
        data-slide="prev" @click="onclickSlide">
        <!-- mobile -->
        <i class="d-block d-sm-none fa-solid fa-angle-left fa-3x text-wihte"></i>
        <!-- desktop -->
        <i class="d-none d-sm-block fa-solid fa-angle-left fa-3x text-dark"></i>
      </button>
      <!-- right button -->
      <button class="carousel-control-next position-absolute" style="width: 10%; height: 30px; top: 50%; transform: translateY(-50%);" type="button" data-target="#home-carousel"
        data-slide="next" @click="onclickSlide">
        <!-- mobile -->
        <i class="d-block d-sm-none fa-solid fa-angle-right fa-3x text-white"></i>
        <!-- desktop -->
        <i class="d-none d-sm-block fa-solid fa-angle-right fa-3x text-dark"></i>
      </button>
    </div>
  </div>
</template>

<script>
export default {
  mounted() {
    this.loadCarouselItems();
    this.handleSlide();
  },
  props: {
    indexEndpoint: String,
  },
  data: function () {
    return {
      titleVisible: true,
      firstCarouselLoading: true,
      items: [],
      origin: window.location.origin,
      host: window.location.host,
    }
  },
  watch: {

  },
  methods: {
    loadCarouselItems() {
      axios.get(this.indexEndpoint)
        .then(response => {
          this.items = response.data.data;
          //set css position absolute to the first carousel item
          
          if(this.items.length == 0){
            $('#preserve-for-carousel').remove();
          }
        })
        .catch(error => {
          console.log(error);
        });
    },
    handleIframeLoaded(index) {
      if (index === 0) {
        this.firstCarouselLoading = false;
        console.log('first carousel loaded');
      }
    },
    waitFirstCarouselLoading(index) {
      return index !== 0 && this.firstCarouselLoading;
    },
    getTwitchChannelUrl(item) {
      // :src="'https://player.twitch.tv/?video=' + item.video_id + '&parent=' + host + ' &autoplay=false'"
      return `https://player.twitch.tv/?channel=${item.video_id}&parent=${this.host}&autoplay=false`;
    },
    getTwitchVideoUrl(item) {
      // :src="'https://player.twitch.tv/?video=' + item.video_id + '&parent=' + host + ' &autoplay=false'"
      let time = this.formatTwitchTime(item.video_start_second);
      return `https://player.twitch.tv/?video=${item.video_id}&parent=${this.host}&autoplay=false&time=${time}`;
    },
    getTwitchClipUrl(item) {
      // :src="'https://clips.twitch.tv/embed?clip=' + item.video_id + '&parent=' + host + ' &autoplay=0'"
      let time = this.formatTwitchTime(item.video_start_second);
      return `https://clips.twitch.tv/embed?clip=${item.video_id}&parent=${this.host}&autoplay=false&time=${time}`;
    },
    hideTitle() {
      this.titleVisible = false;
    },
    onclickSlide() {
      this.showTitle(500);
    },
    showTitle(delay = 0) {
      if(this.titleVisible == false){
        setTimeout(() => {
          this.titleVisible = true;
        }, delay);
      }
    },
    stopVideo(id) {
      let iframes = this.$refs['player' + id];

      // check iframe object
      if (!iframes || iframes[0] === undefined) {
        return;
      }

      let iframe = iframes[0];

      // Check if iframe is an iframe object
      if (iframe instanceof HTMLIFrameElement) {
        //keep the src
        let src = iframe.src;

        //make iframe src empty to stop
        iframe.src = '';

        //then restore the src
        setTimeout(() => {
          iframe.src = src;
        }, 100); 
      } else if(iframe.player){
        iframe.player.pauseVideo();
        return;
      }
    },
    formatTwitchTime(time) {
      // seconds to h:m:s
      let hours = Math.floor(time / 3600);
      let minutes = Math.floor(time % 3600 / 60);
      let seconds = Math.floor(time % 3600 % 60);
      return `${hours}h${minutes}m${seconds}s`;
    },
    handleSlide() {
      $('#home-carousel').on('slide.bs.carousel', (event) => {
        let currentSlide = event.from;
        this.stopVideo(currentSlide);
      })
    }
  }
}
</script>