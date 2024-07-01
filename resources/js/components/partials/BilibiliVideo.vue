<template>
  <div class="w-100" :class="{'loading-animation': loading}">
    <div v-if="!previewImage && element" :id="'embed'+element.id" ref="embedDiv" :style="{width: '100%',height: height+'px'}">
    </div>
    <div v-else-if="previewImage">
      <img class="w-100 cursor-pointer" :src="element.thumb_url" @click="loadBilibilVideo(element)" :style="{width: '100%',height: height+'px'}">
    </div>
  </div>
</template>

<script>
export default {
  mounted() {
    if(this.element && !this.previewImage){
       this.loadBilibilVideo(this.element);
    }
    if(this.previewImage){
      this.loading = false;
    }
  },
  watch: {
    element(newVal) {
      if (newVal && !this.previewImage) {
        this.loadBilibilVideo(newVal);
      }
    }
  },

  props: {
    element: {
      type: Object,
      required: true
    },
    height: {
      type: String|Number,
      default: '270'
    },
    width: {
      type: String|Number,
      default: '100%'
    },
    autoplay: {
      type: Boolean,
      default: true
    },
    muted: {
      type: Boolean,
      default: true
    },
    preview: {
      type: Boolean,
      default: false
    }
  },
  data() {
    return {
      previewImage: this.preview,
      loading: true
    }
  },
  methods: {
    loadBilibilVideo: function (element) {
      this.previewImage = false;
      this.loading = true;
      // console.log('loading');

      new Promise((resolve, reject) => {
        setTimeout(() => {
          this.loadEmbed(element);
          resolve();
        }, 100);
      });
    },
    loadEmbed: async function (element) {
     // remove child

      let parser = new DOMParser();
      let code = element.video_id;
      code = `<iframe src="https://player.bilibili.com/player.html?bvid=${code}&autoplay=1&danmaku=0&muted=1" scrolling="no" border="0" frameborder="no" framespacing="0" allowfullscreen="true" width="100%" height="270"></iframe>`;
      // repalce height and width
      code = code.replace(/width="\d+"/, `width="${this.width}"`);
      code = code.replace(/height="\d+"/, `height="${this.height}"`);
      // console.log(code);

      if(this.autoplay){
        code = code.replace(/autoplay=0/, `autoplay=1`);
      }else{
        code = code.replace(/autoplay=1/, `autoplay=0`);
      }

      if(this.muted == true){
        code = code.replace(/muted=0/, `muted=1`);
      }else{
        code = code.replace(/muted=1/, `muted=0`);
      }

      let doc = parser.parseFromString(code, 'text/html');

      let iframe = doc.querySelector('iframe');
      // wait until embedDiv is loaded
      let interval = setInterval(() => {
        // console.log('waiting');
        if(this.$refs.embedDiv){
          // console.log('appending iframe');
          while (this.$refs.embedDiv && this.$refs.embedDiv.firstChild) {
            this.$refs.embedDiv.firstChild.remove();
          }
          this.$refs.embedDiv.appendChild(iframe);
          clearInterval(interval);
          this.loading = false;
        }
      }, 100);
    },
  }
};
</script>
