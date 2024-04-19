<template>
  <div :id="'embed'+element.id" ref="embedDiv" style="width: 100%;">
  </div>
</template>

<script>
export default {
  mounted() {
    if(this.element){
       this.loadYoutubeEmbed(this.element);
    }
  },
  watch: {
    element(newVal) {
      if (newVal) {
        this.loadYoutubeEmbed(newVal);
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
    }
  },
  methods: {
    loadYoutubeEmbed: function (element) {
      // remove child
      while (this.$refs.embedDiv.firstChild) {
        this.$refs.embedDiv.firstChild.remove();
      }
    
      
      let parser = new DOMParser();
      let code = element.source_url;
      // repalce height and width
      code = code.replace(/width="\d+"/, `width="${this.width}"`);
      code = code.replace(/height="\d+"/, `height="${this.height}"`);
      
      if(this.autoplay == false){
        code = code.replace(/autoplay=1/, `autoplay=0`);
      }

      let doc = parser.parseFromString(code, 'text/html');

      let iframe = doc.querySelector('iframe');
      if (iframe && iframe.src.startsWith('https://www.youtube.com/embed/')) {
        this.$refs.embedDiv.appendChild(iframe);
      }
    },
  }
};
</script>