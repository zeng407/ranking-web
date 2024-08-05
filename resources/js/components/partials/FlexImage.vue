<template>
  <img ref="flexImage" :src="src" @error="onImageError">
</template>

<script>
export default {
  mounted() {
    this.checkImgurUrl();
    this.initImgAttributes();
  },
  watch: {

  },
  computed: {
    src() {
      if(this.imgurUrl && !this.isImgurUrlFailed){
        return this.imgurUrl;
      }else if(this.thumbUrl && !this.isThumbUrlFailed){
        return this.thumbUrl;
      }else if(this.thumbUrl2 && !this.isThumbUrl2Failed){
        return this.thumbUrl2;
      }else{
        return '';
      }
    }
  },

  props: {
    elementId: {
      type: String|Number,
      required: true
    },
    thumbUrl: {
      type: String,
      required: true
    },
    thumbUrl2: {
      type: String,
    },
    imgurUrl: {
      type: String,
    },
    alt: {
      type: String,
    },
    height: {
      type: String|Number,
    },
    imageKey: {
      type: String|Number,
    },
    handleLoaded: {
      type: Function,
      default: () => {}
    },
    srcSet: {
      type: String,
    },
    sizes: {
      type: String,
    },
    customClass: {
      type: String,
    },
  },
  data() {
    return {
      reportUrl: "/api/image/report/removed",
      isImgurUrlFailed: false,
      isThumbUrlFailed: false,
      isThumbUrl2Failed: false,
      isReported: false,
    }
  },
  methods: {
    initImgAttributes() {
      if(this.height == 'auto'){
        this.$refs.flexImage.style.height = 'auto';
      }else if(this.height > 0){
        this.$refs.flexImage.style.height = this.height + 'px';
      }

      if(this.customClass){
        this.$refs.flexImage.classList.add(this.customClass);
      }

      if(this.sizes){
        this.$refs.flexImage.setAttribute('sizes', this.sizes);
      }

      if(this.srcSet){
        this.$refs.flexImage.setAttribute('srcset', this.srcSet);
      }

      if(this.imageKey){
        this.$refs.flexImage.setAttribute('key', this.imageKey);
      }

      if(this.handleLoaded){
        this.$refs.flexImage.addEventListener('load', this.handleLoaded);
      }

      if(this.alt){
        this.$refs.flexImage.setAttribute('alt', this.alt);
      }
    },
    onImageError() {
      if(this.imgurUrl && !this.isImgurUrlFailed){
        this.isImgurUrlFailed = true;
      }else if(this.thumbUrl && !this.isThumbUrlFailed){
        this.isThumbUrlFailed = true;
      }else if(this.thumbUrl2 && !this.isThumbUrl2Failed){
        this.isThumbUrl2Failed = true;
      }
    },
    checkImgurUrl() {
      if(this.imgurUrl){
        fetch(this.imgurUrl)
          .then(response => {
            if (!response.ok) {
              this.isImgurUrlFailed = true;
            }else if(response.url === 'https://i.imgur.com/removed.png'){
              this.isImgurUrlFailed = true;
              if(this.isReported === false){
                this.reportRemovedImage(this.imgurUrl);
                this.isReported = true;
              }
            } else if(response.url == this.imgurUrl && response.ok){
              this.isImgurUrlFailed = false;
            }
          })
          .catch(error => {
            this.isImgurUrlFailed = true;
          })
      }
    },
    reportRemovedImage(){
      axios.post(this.reportUrl, {
        element_id: this.elementId
      })
    },

  }
};
</script>
