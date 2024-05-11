<template>
  <div id="home-carousel" class="carousel slide mb-4" data-ride="carousel" data-interval="false">
    <!-- carousel -->
    <div id="preserve-for-carousel" style="height: 350px"></div>
    <div v-show="items.length" class="carousel-inner position-absolute" style="top:0">
      <div v-for="(item, index) in items" :key="index" class="carousel-item" :class="{ active: index === 0 }">
        <div class="d-flex justify-content-center">
          <!-- loading -->
          <div class="d-flex w-100 position-absolute justify-content-center">
            <i v-show="firstCarouselLoading" class="fas fa-spinner fa-spin fa-3x preview-carsousel-loading"></i>
            <img v-show="item.image_url && item.type === 'video' && index === 0 && firstCarouselLoading"
              :src="item.image_url" class="preview-carsousel-image" :alt="item.title" style="pointer-events:none">
          </div>

          <!-- iframe -->
          <div v-if="item.video_source === 'youtube'" class="home-carousel-container">
            <youtube :video-id="item.video_id" width="100%" height="350px"
              @ready="handleIframeLoaded(index)"
              :player-vars="{ controls: 1, autoplay: 0, rel: 0, origin: origin, start:item.video_start_second }">
            </youtube>
          </div>
          <div v-else-if="item.video_source === 'twitch_video'" class="home-carousel-container twitch-container">
            <iframe @load="handleIframeLoaded(index)" :src="getTwitchVideoUrl(item)" height="350" width="100%"
              preload="metadata" allowfullscreen></iframe>
          </div>
          <div v-else-if="item.video_source === 'twitch_channel'" class="home-carousel-container twitch-container">
            <iframe @load="handleIframeLoaded(index)" :src="getTwitchChannelUrl(item)" height="350" width="100%"
              preload="metadata" allowfullscreen></iframe>
          </div>
          <div v-else-if="item.video_source === 'twitch_clip'" class="home-carousel-container twitch-container">
            <iframe @load="handleIframeLoaded(index)" :src="getTwitchClipUrl(item)" height="350" width="100%"
              preload="metadata" allowfullscreen></iframe>
          </div>
          <img v-else-if="item.image_url" :src="item.image_url" class="d-block" :alt="item.title"
            style="height: 350px;">
          <div v-if="!firstCarouselLoading && titleVisible" class="carousel-caption bg-gray cursor-pointer" @click="hideTitle">
            <h5 class="bg-dark d-block px-2">
              <!-- show cancel icon only in mobile -->
              <div class="d-block d-md-none">
                <i class="fa-solid fa-times text-white position-absolute" style="right: 5px; margin-top: 5px; margin-right: 5px;"></i>
              </div>
              <span class="reset-link">{{ item.title }}</span>
            </h5>
          </div>
        </div>
      </div>
    </div>

    <div v-show="items.length > 1">
      <!-- left button -->
      <button class="carousel-control-prev position-absolute" style="top: 50%; transform: translateY(-50%);" type="button" data-target="#home-carousel"
        data-slide="prev" @click="showTitle(500)">
        <i class="fa-solid fa-angle-left fa-3x text-dark"></i>
      </button>
      <!-- right button -->
      <button class="carousel-control-next position-absolute" style="top: 50%; transform: translateY(-50%);" type="button" data-target="#home-carousel"
        data-slide="next" @click="showTitle(500)">
        <i class="fa-solid fa-angle-right fa-3x text-dark"></i>
      </button>
    </div>
  </div>
</template>

<script>
export default {
  mounted() {
    this.loadCarouselItems();
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
          console.log(response.data.data);
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
    showTitle(delay = 0) {
      if(this.titleVisible == false){
        setTimeout(() => {
          this.titleVisible = true;
        }, delay);
      }
    },
    formatTwitchTime(time) {
      // seconds to h:m:s
      let hours = Math.floor(time / 3600);
      let minutes = Math.floor(time % 3600 / 60);
      let seconds = Math.floor(time % 3600 % 60);
      return `${hours}h${minutes}m${seconds}s`;
    }
  }
}
</script>